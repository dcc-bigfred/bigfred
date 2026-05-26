### §7e.2 Process model, CLI & supervisord wiring

#### Cobra subcommand

`dcc-bus` is a new top-level cobra command on the existing
`loco-server` binary, registered alongside the implicit `serve`
command and the `scripts-executor` subcommand:

```
loco-server                 # default: HTTP + WS + supervisord (the existing entry point)
loco-server scripts-executor --executor-socket <path>   # §3a.7
loco-server dcc-bus --layout-id <L> --command-station-id <C> --port <P>   # NEW
```

The single-binary approach (§7d.4) is preserved: `os.Args[0]` is the
absolute path the process was exec'd with, and supervisord is told the
same path with different subcommand arguments. CI builds one binary,
deployment ships one binary.

#### CLI flags

| Flag | Type | Required | Default | Purpose |
|---|---|---|---|---|
| `--layout-id` | uint | yes | – | `LayoutID` the daemon is bound to. Validates against the JWT on every WS upgrade. |
| `--command-station-id` | uint | yes | – | `CommandStationID` the daemon owns. Resolved against `command_stations` at boot; absence → fatal startup error. |
| `--port` | uint16 | yes | – | TCP port the WebSocket listener binds to. **Allocated by `loco-server`**, never hard-coded. |
| `--bind` | string | no | `127.0.0.1` | Interface to bind on. **Always loopback by default** because the frontend reaches the daemon through `loco-server`'s reverse proxy (§7e.6); the operator only widens this when running `dcc-bus` on a separate host. |
| `--db-path` | string | yes | – | SQLite file shared with `loco-server`. Opened read-only (`?mode=ro&_journal=WAL`). |
| `--jwt-secret` | string | no | `$BIGFRED_JWT_SECRET` | HMAC secret used by `AuthService`. Identical to `loco-server` so JWTs validate. Missing secret → fatal startup error (no per-process random fallback like `loco-server` has in dev mode). |
| `--redis-addr` | string | no | `127.0.0.1:6379` | Redis used for state cache + pub/sub. The daemon refuses to start if Redis is unreachable for 10 s. |
| `--redis-db` | int | no | `0` | Redis logical DB index. |
| `--poll-interval` | duration | no | `200ms` | Cadence for `Station.GetSpeed` / `ListFunctions` polling per subscribed address. The poller batches stride-fairly across vehicles. |
| `--heartbeat-grace` | duration | no | `5s` | Default per-session dead-man's switch grace; can be overridden by the user via `session.setEmergencyPlan` (§4.5.3). Hard-capped at 30 s like `loco-server`. |
| `--shutdown-timeout` | duration | no | `5s` | Time given to drain in-flight commands and emit `loco.state` at speed 0 (per emergency plan) before exit. |
| `--log-level` | string | no | `info` | logrus level. |

Validation rules applied at boot (before opening the DCC bus):

1. `layout_id` resolves to a `Layout` row.
2. `command_station_id` resolves to a `CommandStation` row.
3. The `(layout_id, command_station_id)` pair is **attached** (either
   via a `LayoutCommandStation` row for non-system layouts, or by the
   "system layout sees every cs" rule for `IsSystem = true`). A
   mismatch is a fatal startup error (`command_station_not_attached_to_layout`).
4. `--jwt-secret` is non-empty.
5. `--port` is in the unprivileged range (`>1024`).
6. Redis `PING` succeeds within 10 s.
7. The DCC bus is dial-able: `commandstation.New…()` succeeds against
   the `CommandStation.Connection` values (§3a.1) within
   `--shutdown-timeout`. Failure → daemon exits non-zero;
   supervisord's `autorestart=true` retries with exponential backoff.

#### Program registration

`loco-server` registers `dcc-bus` programs through
`SupervisordService.UpsertProgram` (§7d.2). The group name is
`dcc-bus`; the program name is
`dcc-bus-<layoutId>-<commandStationId>` (e.g.
`dcc-bus-1-2`). The shell command rendered into supervisord's INI is:

```
/usr/local/bin/loco-server dcc-bus \
  --layout-id 1 \
  --command-station-id 2 \
  --port 9201 \
  --bind 127.0.0.1 \
  --db-path /home/loco/.local/share/loco/bigfred.db \
  --jwt-secret "$BIGFRED_JWT_SECRET" \
  --redis-addr 127.0.0.1:6379
```

The `--jwt-secret` value is rendered inline by `loco-server` from
the same `BIGFRED_JWT_SECRET` env var that `loco-server` itself reads
(§cli/root.go `resolveJWTSecret`). It is shell-quoted by the
`shellQuote` template helper (§7d.2) so a secret with special
characters is safe.

```go
spec := supervisord.ProgramSpec{
    Name:        fmt.Sprintf("dcc-bus-%d-%d", layoutID, csID),
    Command:     dccBusCommandLine(layoutID, csID, port, …),
    Autostart:   true,
    Autorestart: true,
    StopWaitSecs: 5,
    StartSecs:    1,
}
supSvc.UpsertProgram(ctx, "dcc-bus", spec)
```

A removed daemon is similarly cleaned up by
`SupervisordService.RemoveProgram(ctx, "dcc-bus", name)`. Adding /
removing a program is a hot reload (`reread` + `update`, §7d.3) and
does **not** disturb other `dcc-bus-*` programs.

#### Port allocation

`loco-server` owns the port pool. There is no system call that picks
a "free" port and hands it to a child cleanly across processes, so
the chosen scheme is:

1. A configurable range, **default `[9200, 9209]`** (10 ports — in
   practice an installation has at most a handful of command stations,
   typically one "main track" + one "programming track"), is reserved
   at server boot via `--dcc-bus-port-min` / `--dcc-bus-port-max`
   flags on `loco-server`.
2. `LocoService` (renamed to `DccBusOrchestrator`, §7e.6) keeps a
   `map[(layoutID, csID)] → port` allocation table in memory and
   mirrored to Redis (`HSET dcc-bus:ports <layoutId>:<csID> <port>`)
   so a `loco-server` restart re-uses the previous mapping while
   `dcc-bus` programs continue to run.
3. A new `(layout, cs)` pair gets the lowest unused port in the
   range. If the range is exhausted, `LayoutService` /
   `SessionService` returns `422 no_dcc_bus_ports_available` and
   logs a warning; the operator is expected to widen the range.
4. The chosen port is **rendered into the supervisord config** as a
   plain CLI flag; the `dcc-bus` process listens on whatever its
   `--port` says. There is **no port-discovery handshake** between
   the two processes — supervisord-managed args are the source of
   truth.

#### Lazy lifecycle

A `dcc-bus` program exists in supervisord's desired state iff at
least one of the following is true:

- a live `DriveSession` has `(LayoutID == L, CommandStationID == C)`, or
- the operator pinned the daemon manually via an admin endpoint
  (future, not M3).

State machine:

```
   (none)
     │  first session.setCommandStation { commandStationId = C }
     │  on a session pinned to layout L succeeds
     ▼
   (starting)
     │  supervisord reports RUNNING + WS dial probe succeeds
     ▼
   (running)
     │  last session pinned to (L,C) detaches (closed connection,
     │  setCommandStation to a different id, layout deletion)
     │  AND idle-timeout elapses (default: never; configurable)
     ▼
   (stopping) — supervisorctl stop + RemoveProgram (UpsertProgram with
                desired-state minus this entry)
     │
     ▼
   (none)
```

For the first cut **idle-timeout defaults to `never`**: once started,
a daemon stays running until `loco-server` shutdown or until the
underlying `LayoutCommandStation` row goes away. This keeps the WS
endpoint stable for the frontend; the slight cost is one extra
process per `(L, C)` pair, which we already pay for via supervisord
isolation.

A `--dcc-bus-idle-timeout` flag on `loco-server` may be used to
shorten this (e.g. tests). When non-zero, the orchestrator waits for
"no sessions pinned to (L, C) for `timeout`" before issuing
`RemoveProgram`.

#### Graceful shutdown of a single daemon

When `loco-server` shuts down or the operator stops a daemon
(`supervisorctl stop dcc-bus-1-2`):

1. The daemon receives `SIGTERM` (the default supervisord stop
   signal) and enters drain mode.
2. It publishes `dcc-bus:<layoutId>:<csId>:status = "draining"` on
   Redis so the WS hub on `loco-server` may surface a banner.
3. It rejects any new WS frames with `ack { ok:false, error:"draining" }`.
4. For each connected client, it runs the **per-session emergency
   plan** against that client's `DriveTargets` (§7e.5) using the
   normal `SetSpeed(0)` path. This must finish within
   `--shutdown-timeout` (default 5 s).
5. It writes a final `loco:state` snapshot to Redis for every
   subscribed vehicle.
6. It closes the DCC bus (`Station.CleanUp()` from
   `pkgs/loco/commandstation`).
7. It exits 0.

If `--shutdown-timeout` elapses, supervisord sends `SIGKILL`. The
state in Redis is preserved (the cache reflects the last successfully
written value); peer daemons / a future restart will reconcile against
the live command station within one poll cycle.

#### Boot ordering with respect to `loco-server`

`loco-server` is the **only writer** to the supervisord config
(§7d.3). Therefore:

- supervisord is started by `loco-server.Start` (already true today).
- The initial `DesiredState` rendered by `loco-server` may contain
  zero or more `dcc-bus-*` programs — initially zero on a brand-new
  install (no sessions yet). The very first
  `session.setCommandStation { commandStationId = C }` after login
  triggers `UpsertProgram("dcc-bus", spec)`; `loco-server` blocks
  the WS `ack` until the new daemon is RUNNING and dial-able (or
  returns `ack { ok:false, error:"dcc_bus_unavailable" }` after a 10 s
  timeout).
- On a clean restart of `loco-server`, `dcc-bus` programs that were
  in supervisord's desired state continue running uninterrupted —
  supervisord's hot-reload model (§7d.3) makes the new server's
  config a superset / equal of what is already running, and
  `reread + update` is a no-op when nothing changed.

#### Failure modes summary

| Failure | Behaviour |
|---|---|
| `dcc-bus` panics in the DCC driver | supervisord respawns (`autorestart=true`); throttles see a brief `loco.error { code:"dcc_bus_restarting" }` followed by reconnect. `loco-server` keeps serving REST and non-throttle WS unchanged. |
| `dcc-bus` cannot dial the command station at boot | Exit non-zero; supervisord's BACKOFF state holds it (and surfaces in `system.status`); throttles see `loco.error { code:"command_station_unreachable" }`. |
| `--port` already in use on the host | Daemon exits with `port_in_use`; supervisord backoff; `loco-server` allocates the next port from the pool on the next restart. |
| `loco-server` SIGTERM | `SupervisordService.Stop` (§7d.3) sends `supervisorctl shutdown`, which drains every `dcc-bus-*` program with their own SIGTERM + drain logic. No orphaned daemons (`ps` assertion in §7e.8 #6). |
| `dcc-bus` cannot reach Redis | Daemon stays up but logs warning; subscribers receive cache misses (state empty until next poll cycle); pub/sub-driven catalogue invalidations are lost — the daemon falls back to per-request SQLite lookups. |
| SQLite locked by writer | Daemon retries with exponential backoff (max 100 ms); since SQLite WAL allows readers concurrent with one writer, contention is rare. |
