### 4.5 Drive Session & Dead-Man's Switch

The WebSocket connection is the user's **physical handle on the
layout**: while it is open, the user is considered "at the throttle"
and the server keeps issuing their commands. The moment the handle is
lost (closed tab, killed app, lost network) the server must fail
**safe**, not silent.

#### 4.5.1 Drive session

Every successful WS upgrade creates an in-memory `DriveSession`:

```go
// pkgs/server/ws/session.go
type DriveSession struct {
    ID            string              // uuid, also returned to the client
    UserID        uint
    PartyID       uint                // active party for this session (§3a.4)
    LayoutID      *uint               // resolved from Party.LayoutID at join,
                                      // or nil in the `default` party until
                                      // the driver picks via session.setLayout
    Client        *Client
    OpenedAt      time.Time
    LastHeartbeat time.Time           // updated on each ping/pong & action
    DriveTargets  map[uint16]struct{} // vehicle addrs the user is actively driving
    EmergencyPlan EmergencyPlan       // see §4.5.3, snapshotted from User pref
}
```

Throttle dispatch invariant: every command that needs a `Station`
first validates `session.LayoutID != nil`, otherwise it returns
`layout_not_selected`. In non-default parties this is a tautology
(the field is always set); in `default` it gates the throttle until
the dropdown has been used.

A user may have **N concurrent sessions** (phone + desktop +
MCP-via-SSE). Sessions are indexed by `(UserID -> []*DriveSession)`
inside the Hub.

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
- `session.setLayout` `{ layoutId }` – **only valid in the `default`
  party** (the only party with `LayoutPickedPerSession=true`). Selects
  the active layout for this drive session; subsequent throttle
  commands are routed to that layout's `Station`. Calling it again
  with a different `layoutId` is allowed and is treated as a
  controlled context switch (§3a.4 rule 2): the server runs the
  user's emergency plan against the previous `LayoutID` first, then
  re-points the session. Calling this action from any non-default
  party returns `ack { ok:false, error:"layout_already_pinned" }`.

Server → Client:

- `session.opened` `{ sessionId, partyId, partyName, layoutId?, layoutName?, layoutPickedPerSession, availableLayouts?: [{id,name}], emergencyPlan, gracePeriod, resumed? }` –
  sent immediately after the WS upgrade so the UI can render the
  active-party badge, the layout name, and the "Safety: stop my
  vehicles after 5 s" indicator. For sessions in the `default` party,
  `layoutId` / `layoutName` are absent and `layoutPickedPerSession` is
  `true`; `availableLayouts` carries the catalogue the UI uses to
  populate the dropdown in the vehicle control view. For any other
  party, `layoutId` / `layoutName` are present and
  `layoutPickedPerSession` is `false`.
- `session.layoutChanged` `{ sessionId, layoutId, layoutName }` –
  emitted after a successful `session.setLayout` (and to all the
  user's other open sessions, if any are subscribed to the same drive
  session). The UI uses this event to update the dropdown selection
  on every device the driver has open and to gate the throttle.
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
