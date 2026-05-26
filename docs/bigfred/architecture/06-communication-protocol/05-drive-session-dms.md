### 4.5 Drive Session & Dead-Man's Switch

The WebSocket connections are the user's **physical handles on the
command stations**: while they are open, the user is considered "at the
throttle" and the system keeps issuing their commands. The moment a
handle is lost (closed tab, killed app, lost network) the system must
fail **safe**, not silent.

After §7e, "the WebSocket" is a pair — the control-plane WS to
`loco-server` and the data-plane WS to the picked `dcc-bus`. Each
endpoint tracks heartbeats for its own connection; the cross-process
"user has no live handle anywhere" decision is coordinated via Redis
pub/sub. See §7e.5 for the daemon side; this section keeps the canonical
state machine for the control plane.

#### 4.5.1 Drive session

Every successful WS upgrade creates an in-memory `DriveSession`. The
session's `LayoutID` is **not** chosen at WS-upgrade time – it is
read **directly from the JWT** issued by `POST /api/v1/auth/login`
(§7a.1). The layout is therefore baked in at authentication and is
**immutable for the lifetime of the session**.

```go
// pkgs/server/ws/session.go
type DriveSession struct {
    ID            string              // uuid, also returned to the client
    UserID        uint
    LayoutID      uint                // copied from the JWT at WS upgrade; IMMUTABLE for the session lifetime (§3a.4 rule 1)
    CommandStationID *uint            // starts as nil for every session. Set by the FIRST session.setCommandStation; may be changed later (controlled context switch). Cleared back to nil if the picked station is deleted or detached from the layout (§3a.4 rule 10).
    Client        *Client
    OpenedAt      time.Time
    LastHeartbeat time.Time           // updated on each ping/pong & action
    DriveTargets  map[uint16]struct{} // vehicle addrs the user is actively driving
    EmergencyPlan EmergencyPlan       // see §4.5.3, snapshotted from User pref
}
```

Throttle dispatch invariant: every command that needs a `Station`
first validates `session.CommandStationID != nil`, otherwise it returns
`command_station_not_selected`. This is now true for **every** drive
session – the system layout no longer is a special case, because every
layout exposes a list of one or more command stations and the user
must pick one before the throttle becomes active. The UI MAY fire
`session.setCommandStation` automatically when the layout's
`availableCommandStations` list contains exactly one entry, but the
server-side contract is identical in that case.

A user may have **N concurrent sessions** (phone + desktop +
MCP-via-SSE). Sessions are indexed by `(UserID -> []*DriveSession)`
inside the Hub. All those sessions share the same `LayoutID` if the
JWT is the same; a phone + desktop logged in with two different JWTs
(possibly into two different layouts) can coexist independently.

#### 4.5.2 Heartbeat protocol

Two layers run in parallel; either one detecting a dead session is
enough.

| Layer            | Cadence        | Failure threshold                            | Notes                                                                 |
|------------------|----------------|----------------------------------------------|------------------------------------------------------------------------|
| WS-level         | server sends `Ping` every 30 s (`writeLoop`)        | no `Pong` within 10 s → connection treated as closed | Already implemented in §5.2; close hook drives the dead-man's switch. |
| Application-level| client sends `{type:"ping"}` every 3 s while in a driving page | no app `ping` for **grace period** (default 5 s, configurable per user, max 30 s) | Updates `LastHeartbeat`; triggers safety net even if the OS hasn't yet noticed the TCP socket is dead (mobile networks, suspended tabs). |

The client SHOULD send `{type:"ping"}` only while in a *driving*
context (throttle screen / signalman panel). Outside those screens the
session can stay open but the application heartbeat may stop without
triggering the emergency action.

#### 4.5.3 Emergency plan

```go
// pkgs/server/domain/user.go (extension)
type EmergencyAction string

const (
    EmergencyStopMyVehicles EmergencyAction = "stop_my_vehicles" // default
    EmergencyReleaseLeases  EmergencyAction = "release_my_leases" // stop_my_vehicles + auto-revoke outbound leases
    EmergencyNone           EmergencyAction = "none"              // testing only; UI shows a warning badge
    EmergencyEstopAll       EmergencyAction = "estop_all"         // admin-only; full track power cut
)

type EmergencyPlan struct {
    Action       EmergencyAction
    GracePeriod  time.Duration // ≥0 and ≤30 s, default 5 s
}
```

Resolution rules:

1. The emergency plan attached to a `DriveSession` is **snapshotted at
   connection time** from the user's persisted preference, so a
   concurrent UI change cannot weaken safety mid-disconnect.
2. The plan is executed **only when the last remaining session of the
   user terminates**. If the user is connected from phone + desktop and
   the phone dies, no action is taken – the desktop is still in
   control. The acceptance criterion in §10.4 makes this explicit.
3. The plan is executed via the same services and the same security
   layer as a normal user action; in particular, `LocoService.SetSpeed`
   still goes through `LocoSecurityContext.CanDriveLoco` (the user
   stopping their own vehicle is always allowed).
3a. **Running scripts are an explicit part of the emergency path.**
   When the dead-man's switch fires for user U, the Hub asks
   `ScriptService.StopAllForUser(ctx, U)` to enumerate every active
   `runId` owned by U and post `run.stop { reason:"deadman" }` to
   the executor for each. The executor calls `vm.Interrupt("deadman")`
   on the matching VMs, the goroutines unwind, and `run.event{kind:"finished",reason:"deadman"}`
   comes back. The Hub then broadcasts
   `script.runStopped { runId, reason:"deadman" }` to U's surviving
   sessions so their UIs drop the "running on …" badges. The
   `session.emergency_executed` audit row records
   `terminated_scripts: N` for visibility. Crucially, scripts are
   interrupted **before** the throttle's `SetSpeed(0)` fan-out
   starts, so a sleeping `sleep(60)` script cannot race the
   emergency stop and re-issue `setSpeed(50)` after it.
3b. **Cross-process coordination after §7e.** The `loco-server` Hub
   coordinates with every running `dcc-bus` via the Redis channel
   `bigfred:layout:<L>:emergency:<userId>`. When `loco-server` fires
   the plan because its own control-plane WS for U closed, it
   publishes on that channel; every `dcc-bus` U is connected to
   stops U's drive targets on its command station and `acks` on
   `dcc-bus:evt:<L>:<C>` so the audit row aggregates affected
   vehicles across stations. Symmetrically, when a `dcc-bus`
   detects loss of its own last session for U, it publishes the
   same channel; `loco-server` consumes the event and runs
   `ScriptService.StopAllForUser` even though `loco-server`'s own
   WS may still be alive. The "fire only once per 5 s per user"
   debounce on both sides prevents feedback loops.
4. The user may **opt into per-session override** by sending
   `session.setEmergencyPlan { action, gracePeriod }` immediately after
   connecting; this only weakens safety if explicitly chosen (e.g.
   demos, automated test runs).

#### 4.5.4 New WS message types

Client → Server:

- `ping` `{}` – application-level heartbeat (already listed in §4.2,
  formally part of the dead-man's switch contract here).
- `session.setEmergencyPlan` `{ action, gracePeriod }` – override the
  current session's plan (validated against the user's permitted set;
  `estop_all` requires the `admin` role).
- `session.heartbeat` – alias for `ping` kept for symmetry in
  generated SDKs.
- `session.setCommandStation` `{ commandStationId }` – picks the
  command station the throttle will talk to. **Valid in every
  layout**, because every drive session starts with
  `CommandStationID = nil` (§4.5.1). The server validates that
  `commandStationId` is currently attached to the session's layout:
  - for non-system layouts it checks `LayoutCommandStation` rows;
  - for the **system layout** (`IsSystem = true`) it checks the live
    `command_stations` catalogue directly (the system layout's set
    is virtual, §3a.4 rule 2).
  Mismatch returns `ack { ok:false, error:"command_station_not_attached_to_layout" }`.
  Calling it again with a different `commandStationId` is allowed and
  is treated as a controlled context switch (§3a.4 rule 4): the
  server runs the user's emergency plan against the previous
  `CommandStationID` first, then re-points the session and broadcasts
  `session.commandStationChanged` to every concurrent session of the
  same user.

Server → Client:

- `session.opened` `{ sessionId, layoutId, layoutName, layoutIsSystem, layoutLocked, availableCommandStations: [{id,name}], commandStationId?, commandStationName?, emergencyPlan, gracePeriod, resumed? }` –
  sent immediately after the WS upgrade so the UI can render the
  active-layout badge, the **command-station dropdown** in the vehicle
  control view, and the "Safety: stop my vehicles after 5 s" indicator.
  `layoutId` / `layoutName` reflect the JWT-pinned layout
  (`layoutName` is the i18n key `layout:system_default_label` when
  `layoutIsSystem == true`, otherwise the user-entered name).
  `layoutLocked` is informational: it can flip to `true` mid-session
  when an admin locks the layout, which does NOT terminate the
  session (§3a.4 rule 6) but lets the UI surface a "this layout was
  locked – you can keep driving but won't be able to log back in
  here" banner. `availableCommandStations` is always populated:
  - for non-system layouts it lists the rows from `LayoutCommandStation`;
  - for the system layout it lists every `command_stations` row.
  `commandStationId` / `commandStationName` are absent on a fresh
  session (since `CommandStationID == nil` until the user fires
  `session.setCommandStation`) and present on a `resumed: true`
  session where the previous pick was preserved.
- `session.commandStationChanged` `{ sessionId, commandStationId, commandStationName?, reason? }` –
  emitted after a successful `session.setCommandStation` (and broadcast
  to all the user's other open sessions on the same drive session) so
  every device can update its dropdown. `commandStationId` may be
  `null`: this happens when the picked station is deleted or detached
  from the session's layout mid-flight, in which case `reason` is
  `"deleted"` or `"detached"` respectively and the throttle re-gates
  until the user picks again from the refreshed
  `availableCommandStations`.
- `layout.commandStationsChanged` `{ layoutId, availableCommandStations: [{id,name}] }` –
  fan-out event sent to every live drive session pinned to `layoutId`
  after an admin attaches or detaches a command station on a non-system
  layout, or after any `command_stations` mutation on the system layout.
  Clients SHOULD refresh their dropdown contents in place; the current
  `CommandStationID` is preserved if it is still in the new set, otherwise
  the server itself emits a `session.commandStationChanged { commandStationId: null, reason:"detached" }` first.
- `session.warning` `{ secondsUntilEmergency }` – sent when the server
  hasn't seen a heartbeat for `gracePeriod / 2`; lets the UI flash a
  warning so a temporarily backgrounded mobile tab can be brought back
  in time.
- `session.emergencyExecuted` `{ action, affectedVehicles: [addr...] }` –
  fan-out event sent to **all the user's *other* open sessions**, if
  any, and to the active signalman of any interlocking that was
  controlling those vehicles via takeover, so everyone sees that the
  user's vehicles just stopped and why. The same event also generates
  a `session.emergency_executed` row in the audit log (§3a.5), with
  `ObjectType="session"`, `ObjectName=sessionId` and
  `Metadata={action, affected_vehicles}`. This is the "`maszynista
  zasnął`" entry.

#### 4.5.5 State machine

```
                            WS upgrade
   (no session) ─────────────────────────────► (active)
                                                   │
                  ping / pong / any action ◄───────┤
                       (updates LastHeartbeat)     │
                                                   ▼
                                      missing heartbeat ≥ gracePeriod / 2
                                                   │
                                                   ▼
                                             (warning)
                                                   │
                            ┌──────────────────────┼──────────────────────┐
                            │                      │                      │
                heartbeat resumed         heartbeat ≥ gracePeriod    WS close
                            │                      │                      │
                            ▼                      ▼                      ▼
                       (active)               (lost) ◄──────────────  (lost)
                                                   │
                                                   ▼
                                  is this the user's LAST session?
                                          │              │
                                         yes             no
                                          │              │
                                          ▼              ▼
                              run EmergencyPlan      remove session,
                                          │          no action
                                          ▼
                                   session.emergencyExecuted
                                   broadcast to other sessions
```

#### 4.5.6 Reconnect cancels the emergency

When the client reconnects within the grace window with the same user
identity (cookie / API key / `?token`):

1. The new WS upgrade looks up the previous `DriveSession` by
   `(UserID, sessionId)` if the client passes back `sessionId` from the
   previous `session.opened`.
2. The pending `time.AfterFunc(gracePeriod)` is cancelled.
3. A `session.opened` event is sent again, with `resumed: true`.
4. The client is expected to re-emit the throttle state it had locally
   so the server can re-sync without ever firing `SetSpeed(0)`.

If the new connection arrives **after** the emergency already fired, no
cancellation is possible; the user simply reconnects to a fresh session
with all their vehicles at speed 0.

#### 4.5.7 Server-side persistence and crash safety

For the v1 implementation the `DriveSession` table is purely in-memory
inside the Hub. A backend crash therefore loses the "who is driving
what" map, but the **command station keeps applying the last DCC
speeds it received until the next packet**, which is unsafe. To bound
that risk:

- On server startup, the backend issues a **global e-stop** (or, if
  configured `panic_on_startup=false`, a `SetSpeed(0)` over every
  vehicle that has any `DriveTargets` entry persisted in Redis).
- A minimal projection of `DriveSession.DriveTargets` is mirrored into
  Redis on every change (`SET drive:<userId>:<sessionId> "{addrs:[…]}"`)
  with a short TTL refreshed by the heartbeat. After a crash, the
  janitor that wakes up on boot finds those keys, fires the emergency
  plan and clears them.
