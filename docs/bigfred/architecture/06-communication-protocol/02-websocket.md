### 4.2 WebSocket

A single endpoint: `GET /api/v1/ws`, which upgrades to a WebSocket
connection.

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

Throttle / locomotive control:

- `loco.subscribe` `{ addr }` – start receiving events for this locomotive.
- `loco.unsubscribe` `{ addr }`.
- `loco.setSpeed` `{ addr, speed, forward }`.
- `loco.toggleFn` `{ addr, fn, on }`.
- `train.subscribe` `{ trainId }` – convenience action: server expands
  the train into its current member set and subscribes the caller to
  every member's `loco.state` events in a single round trip. Sent by
  the train control view on mount; superseded by per-member
  `loco.subscribe` semantics under the hood (no new event channel).
- `train.unsubscribe` `{ trainId }` – analogous teardown.
- `train.setSpeed` `{ trainId, speed, forward }` – **drives the entire
  train from a single slider**. The server:
  1. resolves the train into its `TrainMember` rows (rejected with
     `not_found` if the user lacks driving authority on the train);
  2. for each member, **flips `forward` iff `member.Reversed == true`**
     so a vehicle coupled the other way around runs in the opposite
     DCC direction and the whole consist moves rigidly;
  3. calls `Station.SetSpeed(member.DCCAddr, speed, effectiveForward)`
     concurrently across members (each on the command station resolved from
     `session.CommandStationID`, §3a.4 rule 3);
  4. replies with a single `ack` that aggregates per-member outcomes:
     `{ ok: true, trainSpeed: { trainId, speed, forward, members:[
        { addr, ok: true | false, error? } ] } }`.
  Semantics:
  - **Best-effort, not transactional.** A DCC bus does not support a
    multi-address atomic write, so a partial failure (one decoder
    silent) leaves the rest of the train at the new speed. The UI is
    expected to render an inline warning on the failing member's row
    and the user decides whether to retry or `system.estop`.
  - **Fan-out is single source of truth for `controlledBy`.** Every
    affected member's broadcast `loco.state` event now carries
    `controlledBy: { kind: "train", trainId, userId }` instead of
    `kind: "driver"`. An individual `loco.setSpeed` from the same
    user's other tab targeting one of those members detaches that
    member from the train view (its `controlledBy` flips to
    `"driver"`) and the train slider in the original tab visually
    "loses" that member – it is grayed out with a re-attach button.
  - **Lease / takeover rules are evaluated per member.** A train
    leased to user B has the same per-member authority as
    `loco.setSpeed { addr, ... }` would: if any single member is
    currently under signalman takeover, the ack lists that member as
    `{ ok:false, error:"taken_over" }`; the rest of the train is
    driven normally.
- `system.estop` `{}` – global emergency stop.
- `ping`.

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

Radio ("walkie-talkie"):

- `radio.send` `{ to: { userId?, interlockingId? }, phrase, note? }` –
  sends a structured radio message. Exactly one of `userId` or
  `interlockingId` must be set.

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
- `takeover.granted` `{ requestId, target, targetId, signalman }` –
  driving authority moved to the signalman; throttle UI on the driver
  side is disabled (read-only telemetry).
- `takeover.released` `{ requestId, target, targetId }` – signalman
  ended the takeover or left the interlocking; driver regains control.
- `takeover.rejected` `{ requestId }` / `takeover.cancelled` / `takeover.expired`.

Radio:

- `radio.message` `{ messageId, from, to, phrase, note?, sentAt }` –
  delivered to the addressee (a specific user) or to the active
  signalman of the addressed interlocking.

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

Authorization (sudo elevation, §7a.7):

- `auth.elevationChanged` `{ target: "admin"|"signalman", granted: bool, expiresAt?, reason?: "granted"|"renewed"|"expired"|"user_action"|"logout"|"layout_deleted" }`
  – fan-out to **every live WS session of the affected user** when a
  `SudoElevation` row is inserted, updated, or deleted. The frontend
  listens for this event in `AppShell.tsx` to flip the lock /
  signalman icons between "closed" and "open with countdown" without
  polling. `expiresAt` is omitted when `granted == false`. The event
  is the WS counterpart of the REST `POST/DELETE
  /api/v1/layouts/{id}/sudo` endpoints; both write paths emit it so
  starting sudo on the desktop instantly enables the indicator on the
  phone, and the auto-expiry fan-out is the same code path as a
  manual revoke.

Common:

- `pong`.
- `ack` `{ id, ok, error? }` – correlated acknowledgement for actions
  carrying an `id`.

The protocol is a discriminated union on `type`, both in Go (switch) and
TypeScript (literal union). Sharing types automatically (via `tygo` or
similar) prevents drift.
