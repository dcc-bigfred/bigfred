# Implementation plan — Train control in the Throttle view

Drive a **train** (*skład*) from the same Throttle overlay that already
drives a single **vehicle** (*lokomotywa*). A train is driven through its
**leading vehicle** (*pojazd prowadzący*); every other powered member
follows at `leadingSpeed × speedMultiplier`. Functions are exposed
per member through **collapsible accordions**.

**Architecture (revised).** `loco-server` only publishes the train roster
to Redis (`defined_trains`), exactly as it already does for vehicles
(`allowed_vehicles`); **`dcc-bus` does all the driving**. `train.setSpeed`
is a daemon-hosted data-plane action that reads the cached `defined_trains`
snapshot and fans per-member writes straight to the command station — no
`loco-server` round-trip, no `dcc-bus:cmd` hop. Function toggles are
unchanged: per-vehicle `loco.setFunction` on the data plane. This replaces
the original server-mediated fan-out (see Stage 4).

Specification (authoritative docs, updated alongside this plan):

- terminology — [`../architecture/00-terminology.md`](../architecture/00-terminology.md)
  (*train*, *leading vehicle*, *speed multiplier*)
- domain — [`../architecture/05-domain-model/01-entities.md`](../architecture/05-domain-model/01-entities.md)
  (`TrainMember.SpeedMultiplier`) and
  [`../architecture/05-domain-model/03-invariants.md`](../architecture/05-domain-model/03-invariants.md)
- protocol — [`../architecture/06-communication-protocol/01-rest.md`](../architecture/06-communication-protocol/01-rest.md)
  (member `speedMultiplier`, member PATCH) and
  [`../architecture/06-communication-protocol/02-websocket.md`](../architecture/06-communication-protocol/02-websocket.md)
  (`train.setSpeed` fan-out)
- frontend — [`../architecture/08-frontend-components.md`](../architecture/08-frontend-components.md)
  (§6.3a Train control view)

## Confirmed design decisions

1. **Leading vehicle.** When a train is selected the **first member, in
   `Position` order, that carries a DCC address** is the *leading
   vehicle*. The slider drives the leading vehicle's speed and direction;
   the whole consist follows.
2. **Speed multiplier is persistent.** Each `TrainMember` carries a
   `SpeedMultiplier float64` (DB column + REST field + migration). It is a
   **physical calibration** of how fast that vehicle runs relative to the
   leading one, so it is shared across drivers and devices. The leading
   member's multiplier is implicitly `1.0` and is **not editable** (its
   value is ignored).
3. **Owner-only edit.** The cog popup that edits a member's multiplier is
   shown **only to the train owner** (read-only / hidden for lessees and
   for a signalman driving via takeover), consistent with the standing
   invariant *“a lease grants driving authority but never edit rights.”*
4. **Daemon-side fan-out, single command station.** A train is driven on
   **one command station at a time** — the session's currently picked one
   (§4.5). `train.setSpeed` is resolved **inside `dcc-bus`**, on its
   data-plane WebSocket, exactly like `loco.setSpeed`: the daemon reads the
   train from its own `defined_trains` cache (already published by
   `loco-server` to Redis), computes each member's effective speed/direction
   and writes them straight to the command station it owns. `loco-server`
   is **not** in the train-driving path — it only publishes the roster
   snapshot, just as it does for vehicles. (This supersedes the earlier
   server-side fan-out via the `dcc-bus:cmd` channel.)
5. **Dummies are skipped.** Members without a DCC address (dummies) are
   never written to DCC, never appear in the function accordions, and can
   never be the leading vehicle.
6. **Per-member function accordions.** With a train selected, the function
   container becomes a **vertical stack of collapsible accordions**, one
   per **DCC-addressed** member, ordered leading-first (i.e. by
   `Position`). Each accordion body is that member's existing
   `<FunctionGridButton>` grid. Default **collapsed**; expanded/collapsed
   state is remembered per train in **`sessionStorage`** on the frontend.

---

## Speed model

Evaluated **inside the daemon** (`dcc-bus`), where `maxSpeed` is derived
from the daemon's own `speedSteps` and the member addresses come from the
cached `defined_trains` snapshot:

```
leadingSpeed   = slider value (0 .. maxSpeed for this daemon's speedSteps)
leadingForward = direction toggle
leading        = first member with a DCC address in Position order

for each powered member m (DCC address set):
    mult      = (m == leading) ? 1.0 : m.speedMultiplier   // leading always 1.0
    raw       = round(leadingSpeed * mult)
    speed     = clamp(raw, 0, maxSpeed)
    forward   = leadingForward XOR m.reversed               // existing Reversed flip
    Station.SetSpeed(m.dccAddr, speed, forward)
```

Notes:

- The multiplier scales the **numeric speed step**, then clamps to the
  command station's max (a `1.3` multiplier on a near-max leading speed
  simply saturates rather than overflows).
- `speed == 0` stays `0` for every member regardless of multiplier, so
  **Stop** and dead-man's-switch braking still bring the whole consist to
  a standstill.
- Direction handling is unchanged from today: `member.Reversed` flips the
  DCC direction so a vehicle coupled the other way runs the right way.

---

## Backend changes

The split is the whole point of this revision: **`loco-server` owns the
catalogue and the Redis snapshot; `dcc-bus` owns driving.** Functions are
untouched — they stay per-vehicle `loco.setFunction` on the data plane,
exactly as today.

### loco-server — catalogue, persistence, snapshot (mostly already done)

- `domain.TrainMember`: `SpeedMultiplier float64` (`db:"speed_multiplier"`)
  — **done**.
- Migration `ALTER TABLE train_members ADD COLUMN speed_multiplier REAL
  NOT NULL DEFAULT 1.0` — **done** (range enforced in the service, not a
  DB `CHECK`).
- `TrainMemberInput` / `replaceMembers` / `validateMembers` carry + validate
  the multiplier (`0 < v ≤ 4.0`, default `1.0`) — **done**.
- REST: `speedMultiplier` on member request/response and the owner-only
  `PATCH /api/v1/trains/{id}/members/{memberId} { speedMultiplier }`
  (leading member immutable, `BroadcastTrainUpdated`) — **done**.
- **Redis snapshot already carries the member data the daemon needs**:
  `buildDefinedTrainsSnapshot` publishes `defined_trains` with each
  member's `position`, `reversed`, `speedMultiplier` and resolved `addr`
  (dummies keep `addr == null`). `SyncLayoutRosterToRedis` /
  `SyncLayoutRosterForTrain` republish on every roster, composition,
  multiplier and DCC-address change.
- **One small publisher addition:** fold a train-level
  `ControllerUserIDs []uint` onto `DefinedTrain` (owner + active train
  lessees + takeover self-lease — the same set `UserCanDriveTrain`
  computes), mirroring `AllowedVehicle.ControllerUserIDs`. The snapshot is
  already republished whenever leases/takeovers change (the
  `allowed_vehicles` rebuild and `defined_trains` rebuild share the same
  triggers), so the daemon's train-drive gate stays current.
- **Removed:** the server no longer mediates train driving. Delete
  `service/train_control.go` (`TrainControlService`), the
  `TypeTrainSubscribe/Unsubscribe/SetSpeed` constants in
  `ws/message_types.go`, the `train` arm of `CompositeControlHandler`, and
  the `trainControlSvc` construction/wiring in `cli/root.go`. The
  `dcc-bus:cmd` train fan-out path goes away entirely.
- The server keeps `LeadingMember` (domain-typed) only for the multiplier
  PATCH (blocking edits to the leading member). The speed-fan-out helpers
  (`EffectiveMemberSpeed`, command-station max-speed mapping) move to the
  shared `contract` package (below) so the daemon can reuse them; the
  server copies are deleted.

### contract — shared pure helpers (new)

Add to `pkgs/bigfred/contract` (imported by both processes; the
`DefinedTrain*` types already live here):

- `DefinedTrain.ControllerUserIDs []uint` field (populated by the server,
  read by the daemon's drive gate).
- `(DefinedTrain) LeadingMember() (DefinedTrainMember, bool)` — first member
  with `Addr != nil` in `Position` order.
- `(DefinedTrain) CanDrive(userID uint) bool` — membership in
  `ControllerUserIDs`.
- `EffectiveMemberSpeed(leadingSpeed uint8, multiplier float64, maxSpeed uint8) uint8`
  — the clamp/round arithmetic (leading caller passes `1.0`).
- `MaxSpeedForSpeedSteps(speedSteps uint) uint8` — `14→15`, `28→28`,
  else `127`.
- `TrainSetSpeedWire { TrainID uint, Speed uint8, Forward bool }` — the
  inner payload of a `train.setSpeed` frame on the dcc-bus WS (mirrors
  `LocoSetSpeedWire`).

### dcc-bus — train driving on the data plane (new)

- `protocol.TypeTrainSetSpeed = "train.setSpeed"`.
- `ws/handler.go` dispatch: new `case protocol.TypeTrainSetSpeed` →
  `router.HandleTrainSetSpeed(ctx, sess, payload, env.ID)`; add the method
  to the `Router` interface.
- `Router.HandleTrainSetSpeed(sess, p contract.TrainSetSpeedWire, requestID)`:
  1. look up the train by `p.TrainID` in the cached `r.trains`
     (`defined_trains`); not found → ack `train_not_on_layout`;
  2. `LeadingMember()`; none → ack `train_no_powered_members`;
  3. **authorize** against the train's own controller set
     `DefinedTrain.ControllerUserIDs` (published like `AllowedVehicle`'s —
     owner + active train lessees + takeover self-lease, computed by
     `UserCanDriveTrain`'s logic on the server). Not in the set →
     ack `not_authorized_to_drive`. (We publish a **train-level** set
     rather than reusing the leading member's per-vehicle
     `controllerUserIds`, so an *individual-vehicle* lease on a member does
     not silently grant whole-train driving — preserving today's
     `UserCanDriveTrain` semantics.)
  4. `maxSpeed := contract.MaxSpeedForSpeedSteps(r.speedSteps)`;
  5. for each powered member compute `(speed, forward)` per the speed model
     (leading forced to `1.0`) and write it via a shared
     `applyMemberSetSpeed` helper that does `stationSetSpeed` +
     `redis.StoreState(source:"train", controlledBy: sess.UserID)` +
     `broadcastLocoStateToObservers` — the same store/broadcast path
     `HandleSetSpeed` already uses, so witnesses and cross-tab fan-out keep
     working unchanged;
  6. reply with an aggregate ack
     `{ ok, members:[{ addr, ok, error? }] }`; `ok:false` +
     `partial_failure` when any member write fails.
- **No `train.subscribe` / `train.unsubscribe` on the daemon.** The
  frontend already `loco.subscribe`s every powered member address to read
  their `loco.state`; a dedicated train-subscribe frame is redundant and is
  dropped. The daemon's only new client action is `train.setSpeed`.
- Dead-man's switch: a train slider move is an inbound frame like any
  other, so it already resets the per-session DMS timer. On DMS / WS close
  the existing `collectDriveTargetsForUser` stops every address the session
  drives (including all the train members it just wrote), so no train-aware
  teardown is needed.

### Authorization

- No new policy. Driving authority is the per-vehicle `controllerUserIds`
  already in `allowed_vehicles`; train leases/takeovers are folded onto
  member vehicles by the server. The multiplier PATCH still reuses the
  train **mutate** policy on `loco-server`.

---

## Frontend changes

The Throttle is currently vehicle-only (`ThrottlePage` →
`ThrottleCockpit`, selection by DCC address via
`useThrottleVehicleSelection`). This plan adds trains as a second kind of
drivable target.

### Selection model

- Extend the picker so it lists **vehicles and trains** together. Model a
  selection as a tagged value `{ kind: "vehicle", dccAddress } | { kind:
  "train", trainId }`.
- New `useThrottleTargetSelection(layoutId, vehicles, trains)` (supersedes
  / wraps `useThrottleVehicleSelection`), persisting the last target in
  `localStorage` (same pattern, richer key).
- Roster source: `useLayoutVehicles` (existing) + a `useLayoutTrains`
  query for trains on the layout roster, each train resolved to its
  members + per-member functions.

### Driving a train

- On a train selection, mount the same `ThrottleCockpit` surface but wire:
  - slider / direction / stop → **`train.setSpeed` on the data-plane WS**
    (`useDccBus`), *not* the control plane. Add
    `setTrainSpeed(trainId, speed, forward)` to `DccBusContext` →
    `send("train.setSpeed", { trainId, speed, forward })`, mirroring
    `setSpeed`. `useDebouncedTrainSpeedSend` wraps it.
  - the displayed speed/direction read from the **leading member's**
    `loco.state` on the same data plane (the “witness” is the leading
    vehicle);
  - subscription: just `loco.subscribe` every powered member address (the
    code already does this). The control-plane `train.subscribe` /
    `train.unsubscribe` / `sendAction("train.setSpeed")` calls in
    `ThrottlePage` are **removed**.
- `maxSpeed` continues to come from the picked command station (the daemon
  reports `speedSteps` in `dcc-bus.opened`, already consumed).
- Train-level ack errors (`train_not_on_layout`, `train_no_powered_members`,
  `not_authorized_to_drive`, `partial_failure`) now arrive on the data-plane
  `send` promise; surface them like `loco.error` / `lastError`.

### Function accordions

- Replace the single function grid with a `<TrainFunctionAccordions>`
  component when a train is selected:
  - one MUI `<Accordion>` per **DCC-addressed** member, ordered
    leading-first (by `Position`);
  - summary row: member name (+ a small “prowadzący” chip on the leading
    one) on the left, a **cog `IconButton`** pinned right;
  - body: that member's `<FunctionGridButton>` grid (reusing
    `useVehicleFunctions(member.vehicleId)`), wired to
    `loco.toggleFn { addr: member.dccAddr }` on the data plane;
  - default collapsed; expanded set persisted in `sessionStorage` keyed
    `bigfred.throttle.train.<trainId>.expanded` (array of memberIds).
- A single-vehicle selection keeps today's flat grid (no accordion).

### Multiplier cog popup

- The cog opens `<TrainMemberSettingsDialog>` (MUI `Dialog`):
  - numeric field for `speedMultiplier` (step 0.05, min/max from the
    backend bound), with a short help line;
  - **owner-only**: when `train.ownerId !== me.id` the cog is hidden (the
    dialog is never reachable);
  - submit → `PATCH /api/v1/trains/{id}/members/{memberId}
    { speedMultiplier }`; on success invalidate the train query;
  - the **leading** member shows no editable multiplier (fixed `1.0`).

### i18n

- Extend `throttle.json` (pl + en) with a `train` block: `train.leading`,
  `train.member`, `train.functions`, `train.multiplier.title`,
  `train.multiplier.field`, `train.multiplier.help`,
  `train.multiplier.save`, `train.multiplier.cancel`,
  `train.multiplier.leadingFixed`, plus any new error codes
  (`train_member_multiplier_range`, `train_leading_multiplier_immutable`)
  in `errors.json`.

---

## Implementation stages

```
Stage 1 ─► Stage 2 ─► Stage 3 ─► Stage 4
catalogue   throttle    per-member   move driving
+ snapshot   train UI    UX           into dcc-bus
```

| Stage | Focus | Delivers | Status |
|-------|-------|----------|--------|
| **1** | Catalogue + snapshot | `SpeedMultiplier` column/field/migration, member PATCH, `defined_trains` snapshot carries `speedMultiplier`+`addr`, tests | **done** |
| **2** | Throttle drives trains | picker lists trains, leading-vehicle witness, single flat function grid still works for vehicles | **done** |
| **3** | Per-member UX | function accordions (sessionStorage), cog popup + multiplier edit, owner gating, i18n | **done** |
| **4** | **Re-architecture: daemon-side driving** | move `train.setSpeed` fan-out from `loco-server` into `dcc-bus` (shared `contract` helpers + `TrainSetSpeedWire`); FE sends `train.setSpeed` on the data plane; delete server `TrainControlService` + train WS constants + `dcc-bus:cmd` train path; surface train ack errors | **done** |

### Stage 4 — concrete change list

**loco-server (deletions):**
- `service/train_control.go` — delete `TrainControlService`.
- `ws/message_types.go` — delete `TypeTrainSubscribe/Unsubscribe/SetSpeed`.
- `service/composite_control.go` — drop the `train` field + dispatch arm.
- `cli/root.go` — drop `trainControlSvc` construction and the
  `NewCompositeControlHandler(..., trainControlSvc)` argument.
- `service/train.go` — delete `EffectiveMemberSpeed`,
  `MaxSpeedForCommandStation` (moved to `contract`); **keep** `LeadingMember`
  (domain-typed) for the multiplier PATCH. Update `train_speed_test.go`.

**contract (additions, shared):**
- `(DefinedTrain) LeadingMember()`, `EffectiveMemberSpeed(...)`,
  `MaxSpeedForSpeedSteps(...)`, `TrainSetSpeedWire`.

**dcc-bus (additions):**
- `protocol.TypeTrainSetSpeed`.
- `ws/handler.go` — dispatch `train.setSpeed`; add `HandleTrainSetSpeed`
  to the `Router` interface.
- `cmd/router.go` — `HandleTrainSetSpeed` + an `applyMemberSetSpeed`
  helper factored out of `HandleSetSpeed` (station write + Redis store +
  broadcast). Authorize via the leading member's `controllerUserIds`.
- Tests: `train.setSpeed` fan-out (leading 1.0, multiplier scaling,
  reversed flip, `speed==0` stops all, not-on-layout / no-powered-members /
  unauthorized acks).

**frontend:**
- `context/DccBusContext.tsx` — add `setTrainSpeed(trainId, speed, forward)`.
- `pages/ThrottlePage.tsx` — route train slider/dir/stop through
  `useDccBus().setTrainSpeed` (+ `useDebouncedTrainSpeedSend`); delete the
  control-plane `sendAction("train.subscribe"|"train.unsubscribe"|"train.setSpeed")`
  effects/handlers; surface train ack errors.
- `tygo` regen if `TrainSetSpeedWire` is exported to TS.

---

## Acceptance walkthrough

1. Owner creates a train with ≥2 powered members + ≥1 dummy. Defaults:
   every multiplier `1.0`.
2. In the Throttle, the train appears in the picker; selecting it shows
   the leading vehicle's name as the witness.
3. Move the slider → all powered members move; the dummy is silent;
   pressing **Stop** zeroes the whole consist.
4. Set member #2's multiplier to `1.2` via the cog popup → it now runs
   faster than the leading member for the same slider position; reload
   confirms persistence.
5. The function container shows one collapsed accordion per powered
   member, leading first, dummies absent. Expand two, collapse one,
   reload → the expand/collapse state is restored from `sessionStorage`.
6. Lease the train to a second user → on the lessee's Throttle the cog is
   hidden; they can still drive and toggle functions.
7. A reversed member runs in the opposite DCC direction; flipping the
   direction toggle keeps the consist rigid.

---

## Out of scope

- Per-driver (non-persistent) multiplier overrides.
- Trains spanning multiple command stations simultaneously.
- Acceleration/inertia curves beyond a flat multiplier.
- Editing the train **composition** from inside the Throttle (still done
  in the `MyTrains` catalogue via `TrainDialog`).

## File checklist

| Area | File | Stage |
|------|------|-------|
| Domain | `pkgs/bigfred/server/domain/train.go` | 1 |
| Migration | `pkgs/bigfred/server/repo/migrations/*` | 1 |
| Service | `pkgs/bigfred/server/service/train.go` (+ `train_test.go`) | 1 |
| REST | `pkgs/bigfred/server/http/trains.go` | 1 |
| Snapshot | `pkgs/bigfred/server/service/layout_roster_redis.go` (already carries `speedMultiplier`+`addr`) | 1 |
| Contract (new helpers) | `pkgs/bigfred/contract/allowedvehicles.go`, `pkgs/bigfred/contract/dccbus.go` (`LeadingMember`, `EffectiveMemberSpeed`, `MaxSpeedForSpeedSteps`, `TrainSetSpeedWire`) | 4 |
| dcc-bus protocol | `pkgs/bigfred/dcc-bus/protocol/protocol.go` (`TypeTrainSetSpeed`) | 4 |
| dcc-bus WS | `pkgs/bigfred/dcc-bus/ws/handler.go` (dispatch + `Router` iface) | 4 |
| dcc-bus router | `pkgs/bigfred/dcc-bus/cmd/router.go` (`HandleTrainSetSpeed`, `applyMemberSetSpeed`) | 4 |
| loco-server (delete) | `service/train_control.go`, train consts in `ws/message_types.go`, `train` arm of `service/composite_control.go`, wiring in `cli/root.go` | 4 |
| Frontend data plane | `web/src/context/DccBusContext.tsx` (`setTrainSpeed`) | 4 |
| Frontend API | `web/src/api/vehicles.ts` (train member `speedMultiplier`, member PATCH hook) | 1 |
| Frontend hooks | `web/src/hooks/useThrottleTargetSelection.ts`, `useDebouncedTrainSpeedSend.ts`, `useTrainAccordionExpanded.ts` | 2–3 |
| Frontend components | `web/src/components/throttle/ThrottleCockpit.tsx`, `TrainFunctionAccordions.tsx`, `TrainMemberSettingsDialog.tsx` | 3 |
| Page | `web/src/pages/ThrottlePage.tsx` (train drive now on data plane) | 2 / 4 |
| i18n | `web/src/i18n/locales/{pl,en}/throttle.json`, `errors.json` | 3 |

## Docs to update alongside Stage 4

- `architecture/06-communication-protocol/02-websocket.md` — `train.setSpeed`
  leaves `loco-server`'s `/api/v1/ws`.
- `architecture/16-dcc-bus/04-websocket-protocol.md` — `train.setSpeed` is
  now a daemon-hosted client action; remove it from the "Notably absent"
  list and add it to the action-mapping table.
- `architecture/16-dcc-bus/03-state-and-redis.md` — the `trains`
  cache now serves `train.setSpeed` directly; the `dcc-bus:cmd` train
  fan-out path is gone.
- `architecture/16-dcc-bus/07-frontend-integration.md` — `train.setSpeed`
  on `dccBusWs`.
- `architecture/08-frontend-components.md` — throttle sends `train.setSpeed`
  on the data plane.
- `pkgs/bigfred/contract/dccbus.go` doc comment — drop the "train.setSpeed
  fan-out" reason for the command channel.
