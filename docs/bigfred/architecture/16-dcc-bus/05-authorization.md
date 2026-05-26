### §7e.5 Authorization & session awareness

The daemon's policy stance is "**zero implicit trust**". Every WS
frame is gated by the same `pkgs/server/security` policies that
`loco-server` uses, evaluated against domain objects the daemon
re-loads from SQLite (advisory cached, §7e.3).

#### Authenticating the WS upgrade

1. The HTTP server inside `dcc-bus` reads the `token` query
   parameter from the upgrade request. (Cookie-based auth is rejected
   — the daemon expects the explicit `?token=` flow already used by
   `loco-server`'s WS upgrade, §6.3.)
2. The token is verified against `--jwt-secret` using the same
   HMAC-SHA256 algorithm as `loco-server`. Expired / malformed
   tokens close the upgrade with HTTP 401.
3. The JWT carries `{ userId, layoutId }`. The daemon checks
   `layoutId == --layout-id`; mismatch closes the upgrade with HTTP
   403 and `WWW-Authenticate: dcc-bus realm="layout-mismatch"`.
4. The daemon loads `domain.User` by `userId` (read-through SQLite).
   The user is required to have a non-zero permanent / effective role
   reachable from `domain.EffectiveRoles` (any of `driver`,
   `signalman`, `admin`). The daemon does **not** re-resolve sudo
   elevations (no `SudoService` in `dcc-bus`). Instead it consumes
   the user's effective `Role` set via Redis pub/sub
   (`bigfred:user:<id>:elevation`) — when elevation changes,
   `loco-server` publishes the new role set and the daemon updates
   its in-memory `User.EffectiveRoles` cache. This keeps the daemon
   stateless w.r.t. PIN / 2-minute timers.
5. The daemon allocates a fresh `DriveSession.SessionID`
   (uuid) for the WS connection. The session is logged in Redis at
   `dcc-bus:<L>:<C>:sessions` so operators and `loco-server` can
   inventory it.

The WS Hub inside the daemon stores `(*Client, sessionId, userId,
effectiveRoles, driveTargets, emergencyPlan)` and uses these to
authorize subsequent actions.

#### Per-action authorization

| Action | Domain objects loaded | Policy method | Notes |
|---|---|---|---|
| `loco.subscribe { addr }` | `Vehicle` by `(layoutId, addr)`; *no* lease/takeover lookup | a) `vehicle != nil && vehicle.DCCAddress != nil` and `vehicle.IsOnLayout(L)` ⇒ allow read access; *no policy method* | Subscription is read-only; the policy is "anyone authenticated to the layout may *see* the address". |
| `loco.setSpeed` / `loco.toggleFn` | `Vehicle`, active `VehicleLease`, active `TakeoverRequest` | `LocoSecurityContext.CanDriveLoco` (§7a.3.2) | Deny reasons surface in `loco.error.code` (the existing `Reason` string is used verbatim). |
| `loco.toggleFn { fn }` | additionally `[]VehicleFunction` | `FunctionSecurityContext.CanInvokeFunction` | Refuses unregistered functions with `function_not_registered`. |
| `system.estop` | the user's session-local `DriveTargets` | `LocoSecurityContext.CanDriveLoco` evaluated **per target** with the user as actor | Targets where the policy now denies are silently dropped (e.g. lease expired moments before estop). Audited via the event channel. |
| `ping` | none | none | – |

The daemon **never short-circuits** authorization based on the JWT
alone. A driver whose lease expired between login and `setSpeed` is
correctly rejected with `not_authorized_to_drive`. A signalman whose
takeover got revoked sees `taken_over` flip back to the driver and
`controlledBy: "driver"` on the next `loco.state`.

#### Takeover & lease propagation

`loco-server` is the **sole writer** of takeover and lease state
(§3a.5 audit log, §4.2 takeover state machine, §7a.3). When state
changes, `loco-server` does two things:

1. Updates SQLite (existing path).
2. Publishes the new state on `bigfred:vehicle:<id>:takeover` /
   `bigfred:vehicle:<id>:lease`.

`dcc-bus` subscribes to these channels for vehicles in its
interesting set. On a payload:

- Invalidate the relevant memory cache.
- For each connected WS client subscribed to the affected `addr`,
  push an updated `loco.state` with the new `controlledBy` value (or
  with a `loco.error { code: "lease_revoked" }` if the driver lost
  authority).

In the worst case the pub/sub round trip takes a handful of
milliseconds. Even if pub/sub is unavailable, the next
`loco.setSpeed` from the affected user re-evaluates the policy
against the **fresh** SQLite row and is denied correctly. The
push event is purely a UX nicety.

#### Session lifecycle & emergency plan

The daemon mirrors §4.5's drive session model in miniature:

```go
type daemonSession struct {
    SessionID     uuid.UUID
    UserID        uint
    EffectiveRoles domain.EffectiveRoles
    Client        *wsClient
    OpenedAt      time.Time
    LastHeartbeat time.Time
    DriveTargets  map[uint16]struct{}  // addrs the user has touched on THIS daemon
    EmergencyPlan domain.EmergencyPlan // snapshotted at connect
}
```

The dead-man's switch follows §4.5.5:

- WS-level ping every 30 s (handled by `coder/websocket`).
- App-level `{type:"ping"}` from the client every 3 s while in
  throttle mode (§4.5.2).
- Missing heartbeat for `gracePeriod / 2` → `session.warning`.
- Missing heartbeat for `gracePeriod` OR WS close → session enters
  the `lost` state.
- When the user's *last* daemonSession on this daemon transitions to
  `lost`, the daemon runs the user's emergency plan against
  `DriveTargets`. This is the **per-daemon** rule — a separate
  daemon (different cs) does not fire just because this one did.

The cross-process aggregate "last session of the user anywhere" rule
from §4.5.3 lives in `loco-server`. When the daemon executes its
local plan, it publishes
`bigfred:layout:<L>:emergency:<userId> { source:"dcc-bus", commandStationId, affectedVehicles }`.
`loco-server` consumes that and:

1. Mirrors `session.emergencyExecuted` to the user's *other* control-plane
   sessions (so the dashboard updates).
2. Asks `ScriptService.StopAllForUser(userId)` to interrupt any
   running script the user owns (existing behaviour, §4.5.3 ¶3a).
3. Writes the `session.emergency_executed` audit row (§3a.5).

If the user is also connected to `loco-server`'s `/api/v1/ws` and
**that** is what dies (not the daemon WS), the existing §4.5 path
fires on `loco-server`, which **also** publishes
`bigfred:layout:<L>:emergency:<userId>` so every `dcc-bus-*` the
user has open connections to drops the user's drive targets to 0
on their respective command stations.

In short: whichever process notices a lost handle first triggers the
fan-out; the other side debounces (an emergency that fired within
the last 5 s for this user is ignored to prevent feedback loops).

#### Audit fan-in

The daemon does **not** write the audit log. Every event it produces
(takeover-relevant `loco.state`, emergency plan execution, function
invocation refused by policy, `system.estop`) lands on
`dcc-bus:evt:<L>:<C>`. `loco-server` consumes that channel and:

- Writes the audit row (`session.emergency_executed`, etc.).
- Updates derived state (e.g. `interlocking.occupantChanged` if the
  driver loses authority due to the daemon executing the emergency
  plan — the existing logic already handles this).
- Mirrors the event onto the server WS when it is relevant for
  non-throttle clients (e.g. an admin watching a dashboard).

If `loco-server` is down, audit events are lost — same as today (the
hand-rolled supervisor in §7 #12 had no audit fan-in either). When
`loco-server` comes back up, it picks up new events but does not
retroactively backfill missed ones; an operator can read the daemon
logs (`$XDG_CACHE_HOME/loco/supervisord/dcc-bus-…stdout.log`) for the
gap.

#### Why this is acceptable security-wise

The policy layer (§7a.3) is **pure** and **independent of the
process** it runs in. Re-evaluating `CanDriveLoco` inside `dcc-bus`
against `domain.User{ID:42}` produces the same Decision as
re-evaluating it inside `loco-server`. The only thing the daemon
must get right is the input — and the input is exclusively domain
objects loaded from SQLite (the same DB row `loco-server` would
read) plus the JWT-pinned `(userId, layoutId)`.

Sudo elevation deserves a closer look. A sudo admin minted on
`loco-server` should flip to `admin` everywhere within the 2-minute
window. The daemon's `User.EffectiveRoles` cache is keyed off the
`bigfred:user:<id>:elevation` pub/sub channel (§4.5/§7a.7 fan-out),
which `loco-server` already publishes for `auth.elevationChanged`.
The daemon listens and updates its in-memory copy; the next
authorization check sees the new role set. If pub/sub is delayed,
the worst case is a 2-minute admin briefly losing admin authority
inside the daemon — `CanDriveLoco` does not depend on `admin`, so
the impact is bounded to "may not perform admin-gated operations
the daemon currently exposes", which is zero (the daemon does not
expose admin operations).

#### What the daemon does NOT do

- It does not write to SQLite. Ever.
- It does not bump audit rows directly.
- It does not run `SudoService` rate-limiters.
- It does not own the WS for control-plane traffic.
- It does not host the Hub for `loco-server`'s other sessions.
- It does not spawn or supervise children.

This minimal surface is what lets `dcc-bus` crash and restart
without compromising the integrity of the rest of the system. The
"single source of truth" stays `loco-server` + SQLite + Redis.
