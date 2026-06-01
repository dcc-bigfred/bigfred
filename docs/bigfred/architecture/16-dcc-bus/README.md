## §7e DCC bus daemon (`dcc-bus`)

This section specifies the **`dcc-bus`** sibling daemon that owns one
DCC connection to a **command station** (*centralka*) **in the context
of one layout** (*makieta*) and exposes a **dedicated WebSocket port**
to the frontend for the **throttle** (*tryb sterowania*) data plane.
It is the runtime counterpart of the `Throttle` UI specified in §6.3b.

Until §7e is implemented, the throttle dispatch from §5.3 –
`LocoService.SetSpeed` / `LocoService.ToggleFn` – lives inside
`loco-server` itself, sharing the WebSocket Hub with takeover, radio,
scripts and presence. §7e **splits the data plane out of `loco-server`**
into a per-`(layout, command_station)` daemon. Authorization rules,
domain model, REST surface and security policies (§7a.3) are unchanged
– `dcc-bus` re-uses `pkgs/server/security` byte-for-byte; only the
*location* of the DCC bus and the *delivery* of throttle traffic move.

## Subsections

1. [Overview & design goals](./01-overview-and-goals.md) — why a sibling
   daemon, the (layout × command station) cardinality rule, control
   plane vs. data plane split.
2. [Process model, CLI & supervisord wiring](./02-process-model-and-cli.md)
   — `dcc-bus` cobra command, flags, port allocation, lifecycle under
   `SupervisordService` (§7d).
3. [State & Redis cache](./03-state-and-redis.md) — vehicle subscription
   set per layout, polling, key layout for the loco-state hash, command
   channel for server-initiated DCC writes (scripts, takeover, dead-man).
4. [WebSocket protocol](./04-websocket-protocol.md) — the throttle action
   set hosted by `dcc-bus` (`loco.*`, `system.estop`, `ping`), how it
   relates to `loco-server`'s WS (`session.*`, `takeover.*`, `radio.*`,
   `script.*`).
5. [Authorization & session awareness](./05-authorization.md) — JWT
   validation in `dcc-bus`, re-use of `pkgs/server/security`, takeover /
   lease propagation via Redis pub/sub, dead-man's switch scoped to the
   daemon.
6. [Server integration & orchestration](./06-server-integration.md) —
   how `loco-server` decides which `dcc-bus` programs must run, port
   assignment, hot reload via supervisord, server-initiated DCC
   commands (scripts, dead-man, takeover release), `session.opened`
   payload extension.
7. [Frontend integration](./07-frontend-integration.md) — dual-WebSocket
   model in the browser, throttle overlay (§6.3b) wiring, command-station
   dropdown, reconnect logic, the `Throttle` component tree.
8. [Acceptance criteria](./08-acceptance-criteria.md) — externally
   observable behaviour the milestone must demonstrate.
9. [External state observation](./09-external-state-observation.md) —
   reflecting speed/direction/function changes made by external physical
   throttles; driver subscription-vs-polling research, the
   `commandstation.StateObserver` capability, and Z21 drive encoding
   (direction-at-stop, SET **S=3** vs. INFO **KKK=4**).

## Quick reference

| Concern | Decision |
|---|---|
| Process cardinality | **One `dcc-bus` per `(layoutId, commandStationId)` pair** that has at least one active drive session pinned to it. Lazy-started by `loco-server`; long-lived until shutdown. |
| Binary | The `loco-server` Go binary exposes a `dcc-bus` cobra subcommand (single binary, multiple `main` entry points — same pattern as `scripts-executor` §7d). |
| Imports | `pkgs/loco/commandstation` (DCC), `pkgs/server/domain` (entities), `pkgs/layoutroster` (Redis snapshot types), `pkgs/dcc-bus/state` (Redis client). Does **not** import `pkgs/server/repo`, `pkgs/server/http`, or open SQLite. |
| Listener | Plain `http.Server` upgraded with `coder/websocket` on the CLI-supplied port. Binds to `127.0.0.1` by default; `loco-server` may reverse-proxy it. |
| Authentication | JWT issued by `loco-server` (shared secret); `?token=` query parameter on the WS upgrade, identical to §7a.1. Verifies that the JWT's `layoutId` matches the daemon's `--layout-id`. |
| Authorization | Drive commands are gated by in-memory roster snapshots from Redis (`allowed_vehicles` → `controllerUserIds` per DCC address). Lease / takeover expansion of `controllerUserIds` is planned; today the server publishes the vehicle owner. |
| Coordination with `loco-server` | **Redis only** between processes: `loco:state` cache, roster snapshots (`bigfred:layout:<L>:allowed_vehicles`, `defined_trains`), command/event pub/sub. Command-station connection parameters are passed on the supervisord command line (`--station-*`). Catalogue truth stays in `loco-server`'s SQLite. |
| Throttle WS actions | `loco.subscribe`, `loco.unsubscribe`, `loco.setSpeed`, `loco.toggleFn`, `system.estop`, `ping`. All other WS actions stay on `loco-server`'s `/api/v1/ws`. |
| Dead-man's switch | Per-daemon: each `dcc-bus` runs the heartbeat for its own WS clients and executes the user's `EmergencyPlan` against drive targets on **its own command station** only (§4.5 still applies for cross-cutting concerns like scripts). |
| Supervisord group | `dcc-bus` (alongside `loco` for `scripts-executor`); programs named `dcc-bus-<layoutId>-<commandStationId>`. |
| Failure isolation | A crashing `dcc-bus` does not bring down `loco-server`; supervisord (§7d) restarts it with `autorestart=true`. Affected throttles see a stale-cache banner and re-connect on `RUNNING`. |
