### 4.2 WebSocket

Two endpoints share the same envelope shape but have different
authoritative scopes (§7e splits the data plane out of `loco-server`):

| Endpoint | Process | Scope |
|---|---|---|
| `GET /api/v1/ws` | `loco-server` | **Control plane** — sessions, takeover, radio, scripts, presence, sudo elevation, layout / command-station picker. Always open while the user is logged in. |
| `GET ws://host:<port>/ws?token=<jwt>` | `dcc-bus` (one process per `(layoutId, commandStationId)` pair, §7e) | **Data plane** — throttle traffic (`loco.subscribe`, `loco.setSpeed`, `loco.toggleFn`, `train.setSpeed`, `system.estop`, `ping`). Opened when the user picks a command station, re-opened against a different daemon when the user switches command stations. |

The frontend therefore holds **two** WebSocket connections in
throttle mode (§6.3b, §7e.7). The control plane carries everything
that is not a per-DCC-address operation; the data plane carries the
high-frequency throttle traffic and is the only WS that ever speaks
to `commandstation.Station`. The two are independent at the WS
layer and reconnect separately. Until §7e is implemented the data
plane is hosted on the same `/api/v1/ws` endpoint as the control
plane (the M1 baseline).

Every frame uses a common envelope format in both directions:

```json
{
  "type": "loco.setSpeed",
  "id": "optional-correlation-uuid",
  "payload": { "addr": 3, "speed": 64, "forward": true }
}
```

The first frame after the upgrade is implicit: the server uses the
session cookie / `?token=` to identify the user and to compute the set
of vehicles/trains this connection is allowed to interact with.

#### Client → Server (Actions)

Throttle / locomotive control — **hosted by `dcc-bus`** (§7e.4) once
§7e ships; on the M1 baseline they live on `loco-server`'s
`/api/v1/ws` exactly as listed:

- `loco.subscribe` `{ addr }` – start receiving events for this locomotive.
- `loco.unsubscribe` `{ addr }`.
- `loco.setSpeed` `{ addr, speed, forward }`.
- `loco.toggleFn` `{ addr, fn, on }`.

Train control — **hosted by `dcc-bus`** on the data-plane WS (§7e.4).
`loco-server` publishes the train roster to Redis (`defined_trains`) but
does **not** mediate driving:

- `train.setSpeed` `{ trainId, speed, forward }` – **drives the entire
  train from a single slider**. The **dcc-bus daemon** for the session's
  picked command station:
  1. looks up the train in its cached `defined_trains` snapshot
     (`train_not_on_layout` when absent);
  2. **authorizes** against the train's `controllerUserIds` (owner +
     active train lessees + takeover self-lease — the same projection
     REST uses for `UserCanDriveTrain`, not the leading member's
     per-vehicle set);
  3. resolves the **leading vehicle** — first member with a DCC address
     in `Position` order (`train_no_powered_members` when none);
  4. for each powered member, applies `speedMultiplier` (leading forced
     to `1.0`), flips `forward` when `member.Reversed`, clamps to the
     command-station max, and writes via `Station.SetSpeed`;
  5. replies with `ack { ok, error?, members:[{ addr, ok, error? }] }`.
     `partial_failure` when any member write fails.

  There is **no** `train.subscribe` / `train.unsubscribe`. The frontend
  issues ordinary `loco.subscribe` for every powered member address so
  `loco.state` witnesses and per-member function toggles work unchanged.

  Semantics:
  - **Best-effort, not transactional.** Partial failure leaves the rest
    of the consist at the new speed; the UI surfaces per-member errors
    from the aggregate ack.
  - **Fan-out source is `"train"`.** Each member's Redis snapshot and
    broadcast `loco.state` carries `source: "train"` and
    `controlledByUserId` of the driving session.
  - **Single command station.** A train is driven on the session's
    currently picked command station (§4.5); all members must be on
    that station's layout roster.
- `system.estop` `{}` – emergency stop. Hosted by `dcc-bus` once §7e
  ships; scope is the command station this `dcc-bus` owns (not a
  global track-power cut). On `loco-server`'s baseline WS the scope
  is global.
- `system.radioStop` `{}` – **Radio Stop** (§4.6). Control-plane only.
  Layout-wide halt of every roster vehicle on all command stations;
  requires drive scope. See §4.6 for fan-out, debounce and audit.
- `ping`. Both endpoints have independent application-level
  heartbeats; the dead-man's switch (§4.5) fires per-endpoint with
  cross-process coordination via Redis.

Interlocking / signal box:

- `interlocking.subscribe` `{ id }` – signalman receives radio + traffic
  events for a given signal box (only the active signalman of that box).

Takeover (signalman → driver arbitration):

- `takeover.request` `{ target: "vehicle" | "train", targetId }` –
  emitted by a signalman occupying an interlocking. The server starts a
  15 s timer and sends `takeover.requested` to the driver.
- `takeover.reject` `{ requestId }` – emitted by the driver during the
  15 s window. Cancels the takeover.
- `takeover.cancel` `{ requestId }` – emitted by the signalman to back
  out of their own request before the timer elapses.

Per-target emergency stop (signalman roster action, §6.3d):

- `system.estopTarget` `{ target: "vehicle" | "train", targetId }` –
  **„Zatrzymaj skład"**: an emergency stop scoped to a **single**
  vehicle or train (NOT the whole layout — that is Radio Stop, §4.6).
  Control-plane only; `loco-server` resolves the target's DCC
  address(es) and fans the EMG-stop to the owning `dcc-bus`
  daemon(s). Authorized for the **active signalman of an interlocking**
  in the target's layout (and for the target's own driver/owner). Unlike
  takeover it transfers no driving authority and opens no throttle — it
  just brakes the target to a standstill.

Radio ("walkie-talkie"):

- `radio.send` `{ to: { userId?, interlockingId? }, context: { vehicleId?, trainId? }, phrase, note? }` –
  sends a structured radio message. Exactly one of `to.userId` /
  `to.interlockingId` must be set, **and** exactly one of
  `context.vehicleId` / `context.trainId` must be set (the message is
  always about a specific vehicle or train, §4.4.1). On a successful
  `ack` the sender's client plays `/sounds/interlockings/radio-sent.ogg`.
- `radio.replay` `{ scope: "interlocking", interlockingId } | { scope: "user" }` –
  request the recent Redis-backed history for the caller's chat surface
  (the signalman's group chat for an interlocking, or the driver's own
  conversations). The server answers with a burst of `radio.message`
  frames (or a single `radio.history` envelope). Used on chat-panel
  mount and after a reconnect; the same data is also reachable over REST
  (§4.1) for the chat-history overlay.

#### Server → Client (Events)

Throttle / state:

- `loco.state` `{ addr, speed, forward, functions: [0,1,5], updatedAt, controlledBy }`
  – `controlledBy` is a tagged object:
  `{ kind: "driver" | "train" | "signalman" | "none", userId?, trainId? }`.
  - `"driver"`: the last write came from an individual `loco.setSpeed`
    against this address; the train control view (if any) renders the
    member as **detached** until the user explicitly re-attaches it.
  - `"train"`: the last write came from `train.setSpeed` with the
    enclosing train's id; carries both `trainId` and `userId`.
  - `"signalman"`: a takeover (§4.2 takeover state machine) is
    currently active; the driver's UI is read-only.
  - `"none"`: nobody owns the throttle right now (also the initial
    state at boot).
  The `functions` array is the runtime *on/off* state, not the
  catalogue of registered slots (see `vehicle.functionsChanged`).
- `loco.error` `{ addr, code, message }`.
- `vehicle.functionsChanged` `{ addr }` – the **definition** of the
  function list for this vehicle changed (rename, icon swap,
  add/remove slot, attach/detach to template, or a template edit
  rippling to a linked vehicle). Clients SHOULD re-fetch
  `GET /api/v1/vehicles/{addr}/functions`. See §3a.6.5.
- `system.status` `{ connected, station: "z21", trackPower: true }`.
- `system.radioStop` `{ triggeredBy: { userId, login }, at }` –
  layout-wide halt was triggered (§4.6). Delivered to every
  control-plane session in the layout; throttle clients play the
  radiostop sound on receipt.

Layout dashboard (presence + roster):

- `layout.presenceChanged` `{ layoutId, users: [{ userId, login, role, occupiedInterlocking? }] }` –
  broadcast to every WS session in the layout when someone connects,
  disconnects, or their occupied interlocking changes. Clients merge
  into the dashboard "online users" table without polling.
- `layout.vehiclesChanged` `{ layoutId, action: "added"|"removed", vehicleAddr }` –
  invalidates the layout vehicle roster table on the dashboard.
- `interlocking.occupantChanged` `{ interlockingId, occupant?: { userId, login }, reason?: "joined"|"left"|"displaced" }` –
  fan-out to the layout. Updates both the interlockings table on the
  dashboard and the interlocking view header. When `reason:"displaced"`,
  the displaced user's client shows a toast and clears local
  "I am occupying" state.

Takeover:

- `takeover.requested` `{ requestId, signalman, target, targetId, autoGrantAt }`
  – sent to the affected driver; client SHOULD render a modal with a
  15-second countdown synced to `autoGrantAt`.
- `takeover.granted` `{ requestId, target, targetId, signalman, leaseExpiresAt }` –
  the 15 s window elapsed without a reject. The server has created a
  **5-minute self-lease** to the signalman (`leaseExpiresAt = now + 5m`,
  §4.3) and driving authority moved to the signalman. On the **driver's**
  side this **ends the throttle session for that target**: the client
  shows "Twoja sesja Throttle zakończyła się z powodu przejęcia składu",
  **redirects to the dashboard**, and the target drops out of the
  driver's throttle picker until release. (Previously the driver's UI
  merely went read-only; now the driver leaves the throttle entirely.)
- `takeover.released` `{ requestId, target, targetId, reason?: "lease_expired"|"signalman_released"|"signalman_left" }` –
  the 5-minute lease expired, the signalman released the takeover
  (closed the throttle overlay at speed 0), or left the interlocking.
  The lease is revoked and the target **reappears in the original
  driver's throttle picker** so they can resume driving.
- `takeover.rejected` `{ requestId }` / `takeover.cancelled` / `takeover.expired`.

Radio:

- `radio.message` `{ messageId, from: { userId, login }, to: { userId? , interlockingId? }, context: { vehicle?: { id, name }, train?: { id, name } }, phrase, note?, sentAt }` –
  delivered to the addressee (all of a specific user's sessions) or to
  the active signalman of the addressed interlocking (§4.4.2). The
  payload carries enough denormalized data to render the chat line
  `({from.login}) {context name}: {translated phrase}` without a second
  fetch. On receipt the recipient plays
  `/sounds/interlockings/{phrase}.ogg`, the driver's throttle **chat
  icon lights red** (unread) and an **alert-style popup** is shown in the
  throttle view (§6.3b). Visibility scoping (which messages a given
  session receives/replays) follows §4.4.3.
- `radio.history` `{ scope, messages: [ radio.message payloads… ] }` –
  optional batched answer to `radio.replay` (server MAY instead stream
  individual `radio.message` frames). Ordered oldest→newest.

Scripts (server-side Goja runs in the sibling executor, §3a.7):

Client → Server:

- `script.run` `{ scriptId, attachmentId }` – press the play button.
  Server validates driving authority over the attached scope,
  generates a `runId`, sends `run.start` to the executor, and emits
  `script.runStarted` to every session the owner has open. Returns
  `ack { ok:false, error:"already_running" }` if a run for the same
  `(attachmentId, userId)` is already in flight.
- `script.stop` `{ runId }` – press the stop button (possibly on a
  different device than the one that started the run). Server sends
  `run.stop { reason:"user" }` to the executor.

Server → Client:

- `script.changed` `{ id, version, kind: "metadata"|"source"|"deleted" }` –
  the script's owner has edited it (or deleted it). UI invalidates
  the source cache; an in-flight run is **not** interrupted (it
  keeps running against the snapshot it loaded at start), except
  for `kind: "deleted"` which triggers a server-side stop with
  `reason: "deleted"`.
- `script.runStarted` `{ sessionId, runId, scriptId, attachedTo:{vehicleAddr|trainId}, startedAt }`
  – fan-out to every session the **owner** has open, so the phone
  shows "running on desktop". Emitted by `ScriptService` as soon as
  it has handed `run.start` to the executor.
- `script.log` `{ runId, ts, msg }` – every `log(msg)` call inside
  the script (forwarded by the executor via `run.event{kind:"log"}`)
  is broadcast to all of the owner's sessions. Throttled at 50
  msgs/sec per run; excess is dropped with a single `script.log`
  reporting the drop count.
- `script.runStopped` `{ sessionId, runId, scriptId, reason, errorMessage?, durationMs }`
  where `reason ∈ { "finished", "stopped", "error", "timeout", "deadman", "deleted", "executor_crashed" }`.
  `"deadman"` means the dead-man's switch (§4.5) interrupted the VM
  as part of the user's emergency plan; `"executor_crashed"` means
  the supervisor lost its RPC channel to the executor and the run
  was implicitly aborted.

Authorization (sudo elevation + signalman self-grant, §7a.7):

- `auth.elevationChanged` `{ layoutId, userId }` – fan-out to **every
  live WS session of the affected user** when their sudo
  elevation OR their signalman self-grant changes. The payload is
  intentionally a "something changed" pulse — the frontend invalidates
  the `useMe` query and re-renders the AppBar indicators (padlock
  countdown for sudo admin, engineer's-cap binary state for the
  permanent signalman row) from the refreshed `/api/v1/auth/me`
  response. The event is emitted by `SudoService.Sudo /
  Revoke / GrantSignalman / RevokeSignalman`, by the sudo janitor on
  expiry, and by `AuthService.Logout` on session teardown. Starting
  sudo on the desktop instantly flips the indicator on the phone,
  and the auto-expiry fan-out is the same code path as a manual
  revoke.

Common:

- `pong`.
- `ack` `{ id, ok, error? }` – correlated acknowledgement for actions
  carrying an `id`.

The protocol is a discriminated union on `type`, both in Go (switch) and
TypeScript (literal union). Sharing types automatically (via `tygo` or
similar) prevents drift.

#### `dcc-bus`-only frames (§7e)

`dcc-bus` adds a small set of events on top of the existing
throttle vocabulary. They are scoped to a single
`(layoutId, commandStationId)` daemon:

Server → Client:

- `dcc-bus.opened` `{ sessionId, layoutId, commandStationId, commandStationName, sharedBus: bool, pollIntervalMs, heartbeatGraceMs }`
  – sent immediately after the data-plane WS upgrade. The daemon
  allocates a fresh `sessionId` distinct from the `loco-server`
  control-plane sessionId. `sharedBus: true` when a peer
  `dcc-bus` is also pinned to the same command station (§3a.4 rule 9).

The daemon also re-emits the existing throttle events
(`loco.state`, `loco.error`, `vehicle.functionsChanged`,
`system.status`, `session.warning`, `session.emergencyExecuted`,
`pong`, `ack`) and hosts **`train.setSpeed`** on the data plane with
semantics identical to those defined in this section, scoped to its
`(layoutId, commandStationId)` slice. See §7e.4 for the full per-frame
contract.
