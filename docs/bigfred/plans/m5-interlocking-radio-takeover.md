# Implementation plan — Interlocking view, radio & takeover (M5)

This plan implements the detailed specification spread across:

- terminology — [`../architecture/00-terminology.md`](../architecture/00-terminology.md)
- protocol — [`02-websocket.md`](../architecture/06-communication-protocol/02-websocket.md),
  [`03-takeover-state-machine.md`](../architecture/06-communication-protocol/03-takeover-state-machine.md),
  [`04-radio-delivery.md`](../architecture/06-communication-protocol/04-radio-delivery.md),
  [`06-radio-stop.md`](../architecture/06-communication-protocol/06-radio-stop.md),
  [`01-rest.md`](../architecture/06-communication-protocol/01-rest.md)
- domain model — [`../architecture/05-domain-model/01-entities.md`](../architecture/05-domain-model/01-entities.md)
- frontend — [`../architecture/08-frontend-components.md`](../architecture/08-frontend-components.md) (§6.3b, §6.3d)
- dcc-bus — [`16-dcc-bus/03-state-and-redis.md`](../architecture/16-dcc-bus/03-state-and-redis.md),
  [`05-authorization.md`](../architecture/16-dcc-bus/05-authorization.md)
- auth — [`10-authn-authz/05-permission-matrix.md`](../architecture/10-authn-authz/05-permission-matrix.md)
- acceptance — [`../architecture/14-acceptance-criteria/05-takeover-radio.md`](../architecture/14-acceptance-criteria/05-takeover-radio.md)
- milestones — [`../architecture/13-delivery-order.md`](../architecture/13-delivery-order.md) (M5, steps 17–18)

## Starting point

- `pages/InterlockingPage.tsx` already implements occupy/leave, the
  displacement confirmation, the navigation guard and the displaced
  toast. The radio panel is a placeholder (`view.radioComingSoon`).
- Interlockings, interlocking sessions and presence exist on the backend
  (`server/domain/interlocking*.go`, `server/service/interlocking*.go`,
  `server/service/presence.go`).
- Radio and takeover **do not exist in code yet** — greenfield, aligned
  with the updated docs.
- The throttle command-station picker pattern already exists: a cog
  (`SettingsIcon`) button in `components/throttle/ThrottleCockpit.tsx`
  opens `components/throttle/ThrottleSetupDialog.tsx`. The interlocking
  view reuses this pattern.
- `components/throttle/RadioStopButton.tsx` already renders the throttle
  Radio Stop (icon-only) and gates on `roster.some(owner == me)`;
  `server/security/radio_stop.go` gates on `controllerUserIds`.

## Confirmed design decisions

1. **Takeover** — 15 s reject window; auto-grant → 5-minute self-lease;
   driver evicted from throttle; signalman drives in a closable overlay
   (speed 0 gate).
2. **Radio** — Redis-only, 4 h TTL; required context (vehicle XOR train)
   and target (user XOR interlocking).
3. **Single-target stop** — `system.estopTarget`, „Zatrzymaj skład".
4. **`dcc-bus` is lease-agnostic** — `loco-server` folds driving rights
   into `controllerUserIds` and republishes `allowed_vehicles`.
5. **Interlocking command-station picker** — cog button, same as throttle.
6. **Radio Stop in interlocking view** — icon + „Radio stop" label above
   panels; `signalman` role alone may trigger (even idle).

---

## Implementation stages

Four functional stages. Each stage is independently shippable and ends
with verifiable acceptance criteria. Stages are sequential — later stages
build on earlier ones.

```
Stage 1 ──► Stage 2 ──► Stage 3 ──► Stage 4
 shell &      radio       takeover    estop &
 roster       comms                   polish
```

| Stage | Focus | Delivers |
|-------|-------|----------|
| **1** | Interlocking shell & roster | Staffed box with cog picker, Radio Stop bar, searchable roster with in-motion indicator; `controllerUserIds` lease folding |
| **2** | Radio (walkie-talkie) | End-to-end radio: Redis store, WS/REST, interlocking chat panel, driver throttle radio + chat overlays, sounds |
| **3** | Takeover control | 15 s state machine, 5-min self-lease, roster „Przejęcie" action, throttle overlay, driver eviction |
| **4** | Per-target stop & polish | `system.estopTarget` + „Zatrzymaj skład" button, remaining i18n/assets, full acceptance walkthrough |

---

### Stage 1 — Interlocking shell & roster panel

**Goal:** Replace the radio placeholder with the interlocking work-area
layout and a live vehicle/train roster. No radio or takeover yet — action
buttons in the roster may be disabled or hidden until their stage lands.

#### Backend

- **Migration** — verify `interlockings` / `interlocking_sessions` exist;
  add `layout_vehicles` if missing (roster data source).
- **`controllerUserIds` folding** — extend
  `LayoutVehicleService.SyncLayoutRosterToRedis` (§7e.3) so each
  `AllowedVehicle.ControllerUserIDs` includes the **owner** plus every
  **active lessee** (`VehicleLease` / `TrainLease`). This is the shared
  foundation for takeover (stage 3) and dcc-bus drive checks; the daemon
  still has no lease concept — it only sees the flat set.
- **Radio Stop authorization** — extend
  `server/security/radio_stop.go`:
  `CanTrigger(roles EffectiveRoles, userID uint, roster AllowedVehicles)`
  must `Allow` on **either** drive scope (`userID ∈ controllerUserIds`)
  **or** the `signalman` role (§4.6.2). Update
  `radio_stop_test.go` (idle signalman with no owned vehicles → `Allow`).
  Wire the new signature into the existing `system.radioStop` WS handler.

#### Frontend

- **`pages/InterlockingPage.tsx`** — new layout, top → bottom:
  1. header + action bar (existing occupy/leave),
  2. **Radio Stop bar** (icon + text „Radio stop"),
  3. **two-panel work area** (tabs on narrow screens).
- **Cog command-station picker** — reuse
  `components/throttle/ThrottleSetupDialog.tsx` +
  `hooks/useThrottleCommandStationSelection.ts` in the action bar;
  fires `session.setCommandStation`.
- **Radio Stop bar** — refactor
  `components/throttle/RadioStopButton.tsx` to accept
  `variant?: "icon" | "bar"`: throttle keeps `"icon"` (drive-scope gate),
  interlocking uses `"bar"` (signalman gate via `useMe().isSignalman`).
  Same confirm overlay; dispatches `system.radioStop`.
- **`components/interlocking/InterlockingRosterPanel.tsx`** — fixed width,
  scrollable table; header search box (client-side filter); columns
  **Skład** (`({login}) {name}` + **„w ruchu"** chip when
  `loco.state.speed != 0`) and **Akcje** (placeholder/disabled icons for
  Radio / Stop / Takeover until stages 2–4). Subscribe to `loco.state`
  for the in-motion indicator. Data from layout roster REST +
  presence/ownership enrichment.
- **i18n (partial)** — `interlocking.json`: panel labels, „w ruchu",
  „Radio stop" bar label, cog setup strings (reuse `throttle.json` keys
  where possible).

#### Stage 1 acceptance

- Staffed interlocking view shows cog picker, Radio Stop bar above two
  panels, and a searchable roster with in-motion chips.
- A signalman with no owned vehicles can trigger Radio Stop from the
  interlocking bar; an admin without `driver`/`signalman` gets `403`.
- `allowed_vehicles` snapshot includes active lessees in
  `controllerUserIds` (verify via Redis GET after a manual lease).

---

### Stage 2 — Radio (walkie-talkie)

**Goal:** Full walkie-talkie channel — signalman group chat, driver
send/receive, Redis persistence, sounds.

#### Backend

- **`domain/radio.go`** — `RadioMessage` value type (Redis-only; ULID id),
  `RadioPhrase` vocabulary, `ValidateTarget()` / `ValidateContext()`
  helpers (exactly-one invariants).
- **`server/service/radio_store.go`** — Redis Streams:
  - `bigfred:radio:layout:<L>:interlocking:<I>` (signalman group chat),
  - `bigfred:radio:layout:<L>:user:<U>` (driver personal stream).
  `Append` (fan-in to addressee + sender streams, TTL 4 h),
  `Replay` (oldest→newest, capped). Config: `radio_ttl`, `radio_replay_limit`.
- **`pkgs/bigfred/contract/radio.go`** — key templates + wire envelopes
  (`radio_events.go`).
- **`RadioService`** — `Send` (validate, denormalize `FromLogin` /
  `ContextName`, authorize, append, return for fan-out) and `Replay`
  (visibility projection §4.4.3).
- **`server/security/radio.go`** — `CanSend`, `CanReplayInterlocking`.
- **WS hub** — `radio.send`, `radio.replay` actions;
  `radio.message`, `radio.history` events; fan-out per visibility rules.
- **REST** — `GET /api/v1/interlockings/{id}/radio` (occupant-only),
  `GET /api/v1/radio/mine`; read-only replay shims.

#### Frontend

- **`api/radio.ts`** — `useInterlockingRadio(id)`, `useMyRadio()`;
  `RadioMessage`, `RadioPhrase`, `RadioTarget`, `RadioContext` types.
- **`components/interlocking/RadioPhrasePickerDialog.tsx`** — searchable
  table of the closed `RadioPhrase` vocabulary (client-side filter);
  emits `radio.send`; plays `radio-sent.ogg` on ack. Shared by interlocking
  and throttle overlays.
- **`components/interlocking/InterlockingChatPanel.tsx`** — fixed width,
  scrollable; line format `({login}) {context name}: {translated phrase}`;
  **„Odpowiedz"** icon pinned to a fixed-width right column; seeded by
  `useInterlockingRadio`, live via `radio.message`.
- Enable the **Radio** icon in `InterlockingRosterPanel` — opens
  `RadioPhrasePickerDialog` pre-addressed to the row's driver with the
  row's vehicle/train context.
- **Driver throttle overlays** (§6.3b):
  - `ThrottleRadioButton.tsx` + `ThrottleRadioOverlay.tsx` — searchable
    interlocking picker + `RadioPhrasePickerDialog`; sends in the context
    of the currently driven vehicle/train.
  - `ThrottleChatButton.tsx` + `ThrottleChatOverlay.tsx` — driver's own
    history (`useMyRadio` + `radio.message`); **red unread badge** until
    opened.
  - On-screen **alert popup** on inbound `radio.message`
    (`AutoDismissAlert`).
- **`hooks/useRadioSounds.ts`** — `radio-sent.ogg` on send ack,
  `{phrase}.ogg` on receive.
- **Assets** — `web/public/sounds/interlockings/radio-sent.ogg` + one
  `{phrase}.ogg` per `RadioPhrase` (lower-cased filename; generic-chime
  fallback).
- **i18n** — new `radio.json` namespace (1:1 with `RadioPhrase` vocabulary)
  + `interlocking.json` chat strings + `throttle.json` radio/chat/alert
  strings. PL + EN; regenerate `i18n/types.ts`.

#### Stage 2 acceptance

- Signalman sees all driver traffic in the group chat; driver sees only
  their own conversations.
- Messages persist in Redis (~4 h), replay on mount/reconnect.
- Sender hears `radio-sent.ogg`; receiver hears `{phrase}.ogg`, sees alert
  popup, chat icon turns red.
- Every message has exactly one target and one context (vehicle XOR train).

#### Stage 2 tests

- Go: `RadioService` (validation, visibility, denormalization),
  `RadioStore` (fan-in, TTL, ordered replay).
- Frontend: chat line layout (fixed reply icon), phrase table filtering,
  unread badge, alert popup.

---

### Stage 3 — Takeover control

**Goal:** Signalman can request takeover from the roster; after the 15 s
window the driver is evicted and the signalman drives in an overlay.

Depends on stage 1 (`controllerUserIds` folding, roster panel, cog picker).

#### Backend

- **`domain/takeover.go`** — `TakeoverWindow = 15s`,
  `TakeoverLeaseDuration = 5m`; extend `TakeoverState` with `"released"`;
  add `GrantedLeaseID *uint`, `ReleasedAt *time.Time`.
- **Migration** — `takeover_requests` table (`granted_lease_id`,
  `released_at`, state CHECK).
- **`repo/takeover_requests.go`** — CRUD + `ListPending()` for restart
  recovery.
- **`TakeoverService`** — state machine (§4.3):
  - `Request` → `pending` + 15 s timer → `takeover.requested` to driver.
  - `Reject` / `Cancel` → terminal states.
  - `autoGrant` → create 5-min `VehicleLease`/`TrainLease`,
    republish `allowed_vehicles` (signalman enters `controllerUserIds`),
    `takeover.granted { leaseExpiresAt }`, end driver's drive targets.
  - `Release` (overlay close at speed 0 / box leave / lease expiry) →
    revoke lease, republish, `takeover.released { reason }`.
  - Hook existing lease janitor for `reason:"lease_expired"`.
- **Drive-scope resolver** — throttle picker and REST visibility must
  **exclude targets leased away** and **include leased-in targets**,
  using the same logic as `controllerUserIds`.
- **`server/security/takeover.go`** — `CanRequest` (active signalman).
- **WS hub** — `takeover.request/reject/cancel` actions;
  `takeover.requested/granted/released/rejected/cancelled/expired` events.
- **`pkgs/bigfred/contract/takeover_events.go`** — wire envelopes.

#### Frontend

- **`api/takeover.ts`** — event payload types.
- Enable **Przejęcie kontroli** icon in `InterlockingRosterPanel` →
  `takeover.request { target, targetId }`.
- **`components/interlocking/TakeoverThrottleOverlay.tsx`** — overlay
  above the interlocking view hosting `LocoControlPage` /
  `TrainControlPage`; lease countdown badge; **close disabled while
  speed != 0**; closing releases takeover.
- **Driver eviction** — on `takeover.granted`: show eviction message,
  redirect to dashboard, drop target from picker
  (`useThrottleVehicleSelection.ts`). On `takeover.released`: target
  reappears.
- **i18n** — takeover overlay, eviction message, countdown strings in
  `interlocking.json` + `throttle.json`.

#### Stage 3 acceptance

- 15 s countdown → auto-grant → 5-min lease; driver evicted and redirected;
  target hidden from driver's picker.
- Signalman drives in overlay; can only close at speed 0.
- Lease expiry / manual release / leaving box → `takeover.released`;
  target reappears in driver's picker.
- `controllerUserIds` in Redis includes/excludes the signalman on
  grant/release; dcc-bus accepts/rejects `setSpeed` accordingly.

#### Stage 3 tests

- Go: `TakeoverService` (timer, auto-grant, lease creation, release paths,
  `allowed_vehicles` republish).
- Frontend: overlay speed-0 gate, driver redirect, picker hide/show.

---

### Stage 4 — Per-target stop & polish

**Goal:** „Zatrzymaj skład" action on the roster; final i18n/assets;
full acceptance-criteria walkthrough.

Depends on stages 1 (roster panel) and 3 (takeover overlay uses cog picker
for command-station context).

#### Backend

- **`system.estopTarget`** — `LocoService.EStopTarget(ctx, actor, target,
  targetId)`: resolve DCC address(es), fan EMG-stop to owning `dcc-bus`
  daemon(s) via `dcc-bus:cmd:<L>:<C>` (mirror Radio Stop fan-out in
  `contract/layout_events.go`, scoped to one target). No authority transfer.
- **`server/security/estop_target.go`** — `CanStop` (active signalman in
  layout **or** target driver/owner).
- **WS hub** — wire `system.estopTarget` action.

#### Frontend

- Enable **Stop** icon in `InterlockingRosterPanel` →
  `system.estopTarget { target, targetId }` (labelled „Zatrzymaj skład";
  distinct from layout-wide Radio Stop).
- **Remaining i18n** — any strings deferred from earlier stages; verify
  PL + EN completeness across `radio.json`, `interlocking.json`,
  `throttle.json`.
- **Missing sound assets** — fill gaps in
  `web/public/sounds/interlockings/{phrase}.ogg`.

#### Stage 4 acceptance

- „Zatrzymaj skład" brakes **only** the selected vehicle/train, never the
  whole layout.
- Full §10.3 acceptance-criteria walkthrough passes (interlocking panels,
  radio, takeover, Radio Stop, per-target stop, `controllerUserIds`
  behaviour).

#### Stage 4 tests

- Go: `EStopTargetSecurityContext`, `EStopTarget` fan-out (incl. trains
  spanning multiple command stations).
- End-to-end acceptance checklist from
  [`05-takeover-radio.md`](../architecture/14-acceptance-criteria/05-takeover-radio.md).

---

## Cross-cutting risks

- **Drive-scope consistency** — throttle picker, REST visibility and
  `allowed_vehicles.controllerUserIds` must share one lease/takeover
  resolver (introduced in stage 1, extended in stage 3).
- **`system.estopTarget` fan-out** — a train may span multiple command
  stations; publish to each owning daemon.
- **`RadioStopButton` gate** — frontend gate is UX only; backend
  `CanTrigger` is the real guard.
- **Per-phrase sound mapping** — lower-cased phrase value
  (e.g. `entry_permitted.ogg`); generic-chime fallback.
- **`dcc-bus` stays lease-agnostic** — never teach the daemon about
  leases; only republish `controllerUserIds`.

## Reference — file inventory

Quick lookup of new/changed files across all stages:

| Area | Files |
|------|-------|
| Domain | `server/domain/radio.go`, `server/domain/takeover.go` |
| Repo | `server/repo/takeover_requests.go`, migrations |
| Service | `server/service/radio.go`, `radio_store.go`, `takeover.go` |
| Security | `server/security/radio.go`, `takeover.go`, `estop_target.go`, `radio_stop.go` |
| Contract | `contract/radio.go`, `radio_events.go`, `takeover_events.go` |
| HTTP | `server/http/radio.go` (replay endpoints) |
| WS | `server/ws/hub.go` (radio + takeover + estopTarget handlers) |
| Frontend — interlocking | `InterlockingChatPanel`, `InterlockingRosterPanel`, `RadioPhrasePickerDialog`, `TakeoverThrottleOverlay` |
| Frontend — throttle | `ThrottleRadioButton/Overlay`, `ThrottleChatButton/Overlay`, `useRadioSounds` |
| Frontend — shared | `RadioStopButton` (variant refactor), `api/radio.ts`, `api/takeover.ts` |
| i18n | `radio.json`, extensions to `interlocking.json` + `throttle.json` |
| Assets | `web/public/sounds/interlockings/*.ogg` |
