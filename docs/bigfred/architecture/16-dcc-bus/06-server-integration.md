### §7e.6 Server integration & orchestration

`loco-server` is the orchestrator: it decides which daemons must
exist, allocates ports, talks to supervisord (§7d), and feeds the
daemon's command channel for non-frontend writes.

#### New components in `loco-server`

```
pkgs/bigfred/server/
├── service/
│   ├── dcc_bus.go              # DccBusService — desired-state, port pool, command channel
│   ├── dcc_bus_test.go
│   └── …
├── http/
│   ├── dcc_bus_proxy.go        # optional reverse proxy for /api/v1/dcc-bus/{commandStationId}/ws
│   └── …
└── ws/
    ├── dcc_bus_listener.go     # consumes dcc-bus:evt:<L>:<C> and fans out to control-plane WS
    └── …
```

The existing `pkgs/bigfred/server/service/loco.go` (currently a thin wrapper
around `LocoApp.Station.SetSpeed`) **shrinks**: in-process DCC writes
move out and the file becomes a publish-to-Redis-command-channel
helper used by `TrainService`, `ScriptService`, the takeover state
machine and the dead-man's switch. See `LocoServiceDriver` below.

#### Roster snapshots and supervisord command line

`DccBusService.buildProgramSpec` loads the `CommandStation` row from
SQLite (`CommandStationsRepo.FindByID`) and appends the shared CLI
flags via `pkgs/bigfred/dcc-bus/cliargs.AppendStationFlags` (`--station-name`,
`--station-kind`, `--station-uri`, `--speed-steps`). The daemon never
opens the database.

`LayoutVehicleService.SyncLayoutRosterToRedis` publishes
`bigfred:layout:<L>:allowed_vehicles` and `defined_trains` after roster
and catalogue mutations (and once per layout at server bootstrap). See
§7e.3.

#### `DccBusService` — desired-state and lifecycle

```go
// pkgs/bigfred/server/service/dcc_bus.go
type DccBusConfig struct {
    PortMin           uint16        // default 9200
    PortMax           uint16        // default 9209 (small pool; one main track + one programming track is typical)
    BindAddr          string        // default 127.0.0.1
    PollInterval      time.Duration // default 200ms
    HeartbeatGrace    time.Duration // default 5s
    ShutdownTimeout   time.Duration // default 5s
    Executable        string        // os.Executable(), reused for the cobra subcommand
    Supervisord       *SupervisordService
    Cache             *cache.Redis
    Layouts           LayoutsRepo
    CommandStations   CommandStationsRepo
    JWTSecret         []byte
}

type DccBusService struct {
    cfg DccBusConfig
    mu  sync.Mutex
    // pinned[(layoutID, csID)] = portNumber
    pinned map[dccBusKey]uint16
}

func NewDccBusService(cfg DccBusConfig) (*DccBusService, error)

// EnsureRunning verifies that a dcc-bus is RUNNING for (L, C). If
// not, it allocates a port, registers the program via
// SupervisordService.UpsertProgram, waits up to startSecs+dialTimeout
// for the WS to accept connections, and returns the port.
// Idempotent: identical inputs return the same port (from `pinned`).
func (s *DccBusService) EnsureRunning(ctx context.Context, layoutID, csID uint) (port uint16, err error)

// Stop tears down a dcc-bus when no session is pinned to (L, C) any
// longer AND --dcc-bus-idle-timeout has elapsed.
func (s *DccBusService) Stop(ctx context.Context, layoutID, csID uint) error

// PortFor returns the assigned port without (re)starting the daemon.
// Used by the WS layer when handing out wsUrl values.
func (s *DccBusService) PortFor(layoutID, csID uint) (port uint16, ok bool)

// PublishCommand puts a server-initiated DCC operation onto the
// daemon's command channel (§7e.3). The operation is fire-and-forget;
// the daemon emits an `ack` on dcc-bus:evt:<L>:<C> the server can
// correlate by `id`.
func (s *DccBusService) PublishCommand(ctx context.Context, layoutID, csID uint, cmd DccBusCommand) error
```

`DccBusService` is constructed **after** `SupervisordService` in
`cli/root.go`:

```go
supSvc, _ := service.NewSupervisordService(service.SupervisordConfig{})
supSvc.Start(ctx)
go supSvc.RunHealthLoop(ctx, 5*time.Second, onSupervisordChange)

dccBus, _ := service.NewDccBusService(service.DccBusConfig{
    Executable:      selfPath,
    Supervisord:     supSvc,
    Cache:           cacheClient,
    Layouts:         layoutsRepo,
    CommandStations: csRepo,
    JWTSecret:       jwtSecret,
})

// On boot, restart any dcc-bus daemons that were already in
// supervisord's config from a previous run. The orchestrator reads
// `dcc-bus:ports` from Redis and re-creates its in-memory pinned map.
dccBus.RestoreFromPersisted(ctx)
```

`LocoServiceDriver` (the new shape of `LocoService.SetSpeed` etc.)
takes a `*DccBusService` and a `*service.SessionLookup` to resolve
`(layoutID, csID)` from a session:

```go
func (d *LocoServiceDriver) SetSpeed(ctx context.Context, sess Session, addr uint16, speed uint8, fwd bool) error {
    // §7a.3 + §3a.4: authority gate, identical to today.
    if d := d.sec.CanDriveLoco(...); !d.Allowed {
        return ErrForbidden(d.Reason)
    }
    layoutID, csID := sess.LayoutID, sess.CommandStationID
    return d.dccBus.PublishCommand(ctx, layoutID, csID, DccBusCommand{
        Type:    "loco.setSpeed",
        ID:      uuid.New(),
        Actor:   ActorFromSession(sess, "script_or_train"),
        Payload: map[string]any{"addr": addr, "speed": speed, "forward": fwd},
    })
}
```

(`TrainService.SetSpeed` keeps its current `Reversed`-flip fan-out
across members and calls this driver once per member.)

#### When daemons start

| Trigger | Caller | Effect |
|---|---|---|
| `session.setCommandStation { commandStationId = C }` | `SessionService` (in WS hub) | `DccBusService.EnsureRunning(L, C)`; blocks ack until RUNNING or 10 s timeout. |
| `loco-server` boot with persisted pins in Redis | `DccBusService.RestoreFromPersisted` | Skip — supervisord already has the program running; just record the port mapping. |
| Admin attaches a cs to a layout (`POST /api/v1/layouts/{id}/command-stations`) | `LayoutService.AttachCommandStation` | **No** auto-start. The daemon spawns when the first session selects it. |
| Admin detaches a cs from a layout (`DELETE …/command-stations/{csId}`) | `LayoutService.DetachCommandStation` | `DccBusService.Stop(L, C)` is called **after** the live sessions are detached (existing `CommandStationID → nil` + `session.commandStationChanged { reason:"detached" }` fan-out). |
| Admin deletes a cs (`DELETE /api/v1/command-stations/{id}`) | `CommandStationService.Delete` | For every `(L, C)` row affected, `DccBusService.Stop(L, C)`. |

The orchestrator is the single funnel for daemon lifecycle events; no
service ever calls `SupervisordService.UpsertProgram` directly.

#### `session.opened` payload extension

The WS event `session.opened` (§4.5.4) gains the daemon URL per
command station:

```json
{
  "type": "session.opened",
  "payload": {
    "sessionId": "…",
    "layoutId": 1,
    "layoutName": "Saturday operating session",
    "layoutIsSystem": false,
    "layoutLocked": false,
    "availableCommandStations": [
      {
        "id": 2,
        "name": "Main Z21",
        "wsUrl": "ws://example.com:9200/ws",
        "status": "RUNNING"
      },
      {
        "id": 3,
        "name": "Yard LocoNet",
        "wsUrl": null,
        "status": "STOPPED"
      }
    ],
    "commandStationId": null,
    "commandStationName": null,
    "emergencyPlan": { "action": "stop_my_vehicles", "gracePeriod": 5000 }
  }
}
```

Rules for the `wsUrl` field:

- `null` when no `dcc-bus-<L>-<id>` program exists (lazy lifecycle).
  The frontend renders the option but greys out the "connect"
  affordance; selecting it triggers `session.setCommandStation`,
  which spawns the daemon and emits `session.commandStationChanged`
  with the freshly populated `wsUrl`.
- Otherwise the string `ws://<bindAddr-or-public-host>:<port>/ws`.
  The host portion is derived from the request `Host` header by
  default (so a client reaching `loco-server` at `example.com:8080`
  receives `ws://example.com:<port>/ws`). Operators may override
  with `--dcc-bus-public-host` on `loco-server` for reverse-proxy
  setups.

`session.commandStationChanged` (§4.5.4) extends the same way:

```json
{
  "type": "session.commandStationChanged",
  "payload": {
    "sessionId": "…",
    "commandStationId": 2,
    "commandStationName": "Main Z21",
    "wsUrl": "ws://example.com:9200/ws",
    "reason": null | "deleted" | "detached"
  }
}
```

`layout.commandStationsChanged` (§4.5.4) is unchanged in spirit but
now also lists the per-station `wsUrl` so dropdowns stay in sync:

```json
{
  "type": "layout.commandStationsChanged",
  "payload": {
    "layoutId": 1,
    "availableCommandStations": [
      { "id": 2, "name": "Main Z21", "wsUrl": "ws://example.com:9200/ws", "status": "RUNNING" },
      { "id": 3, "name": "Yard LocoNet", "wsUrl": null, "status": "STOPPED" }
    ]
  }
}
```

#### `loco-server` consumers of daemon events

A new goroutine started from `cli/root.go` subscribes to
`dcc-bus:evt:*` (psubscribe pattern). For each message:

1. Parses the envelope.
2. Calls the appropriate downstream service:
   - `loco.state` events → `LocoEventBus.Publish` (existing
     in-memory bus, §5.4 `bus.LocoStateChanged`). Other server WS
     clients subscribed to `addr` see the same update.
   - `session.emergencyExecuted` → `AuditService.Log` +
     `ScriptService.StopAllForUser` (§4.5.3).
   - `loco.error { code:"command_station_disconnected" }` →
     update `system.status` and broadcast on the server WS.

If `loco-server` itself is restarted while daemons are running, the
goroutine simply resubscribes; events emitted during the gap are not
backfilled but the next polling cycle produces fresh state.

#### Reverse proxy (default)

`loco-server` exposes `/api/v1/dcc-bus/{commandStationId}/ws` as a
reverse-proxy endpoint by default. It accepts the WebSocket upgrade
on the same origin as the rest of the SPA and forwards it to
`127.0.0.1:<port>` of the matching daemon. This keeps a single public
origin (no separate CORS rules, no TLS cert per daemon, no exposed
DCC-bus ports on the LAN/Internet) and lets the daemon stay bound to
loopback — the firewall story is trivially "block everything except
the `loco-server` HTTP listener".

Rules:

- `session.opened.availableCommandStations[i].wsUrl` returns the
  proxy path (`/api/v1/dcc-bus/2/ws`) **without scheme or host** so
  the SPA computes the URL from `window.location` (handles HTTP /
  HTTPS / dev-mode-from-Vite uniformly). When the SPA detects a
  same-origin path it just opens
  `new WebSocket(\`${location.protocol === "https:" ? "wss" : "ws"}://${location.host}${wsUrl}?token=${jwt}\`)`.
- The proxy **verifies the JWT** itself before forwarding (defence
  in depth — same `AuthService.VerifyToken` call as `/api/v1/ws`)
  and adds `X-Forwarded-For`. The daemon **also** re-verifies, so
  removing the proxy in a dev setup does not weaken auth.
- The proxy is **lookup-table-driven**: it resolves
  `commandStationId` to the daemon's loopback port through
  `DccBusService.PortFor(session.LayoutID, commandStationId)`. A
  mismatched layout (the WS user's layout does not own the cs)
  closes the upgrade with HTTP 403 before forwarding.

The opt-out flag `--dcc-bus-proxy=false` on `loco-server` switches
the daemon to direct mode for cross-host deployments; in that case
the daemon's `--bind` MUST be widened beyond loopback and the
operator is responsible for firewalling / TLS-terminating. The
default is `true`.

#### Wiring summary

```
cli/root.go
  ├─ NewSupervisordService            (§7d.4 already)
  ├─ NewDccBusService(cfg)            (NEW)
  ├─ dccBus.RestoreFromPersisted(ctx) (NEW; reads dcc-bus:ports from Redis)
  │
  ├─ Hub
  │    .OnSessionSetCommandStation = func(L, C) { dccBus.EnsureRunning(L, C) }
  │    .OnSessionEnd               = func(sess) { dccBus.MaybeStopIfIdle(sess) }
  │
  ├─ LocoEventConsumer (NEW, psubscribe on dcc-bus:evt:*)
  ├─ LocoServiceDriver (REPLACES LocoService throttle methods)
  │
  └─ chi router
       /api/v1/dcc-bus/{commandStationId}/ws  (optional reverse-proxy)
```

#### Backwards compatibility with §5

Section §5 (Backend Components) currently describes
`LocoService.SetSpeed` calling `app.Station.SetSpeed` directly. After
§7e is implemented, that method is gone from `LocoService`; the
remaining server-side `LocoService` only:

- looks up vehicles for REST `/api/v1/vehicles/...`;
- exposes `LocoEventBus.Subscribe` for non-throttle WS readers (MCP,
  dashboard);
- delegates throttle writes to `LocoServiceDriver` via
  `DccBusService.PublishCommand`.

§5 ¶5 (background poller) **moves into the daemon**. There is no
server-side poller for DCC state anymore; the server reads `loco:state`
from Redis if it ever needs an authoritative snapshot for REST
responses.

§7 #5 (Long operations such as CV read/write on the track) is
**explicitly out of scope** for this milestone. No `cv.read` /
`cv.write` command goes through `DccBusService.PublishCommand` yet,
and the daemon's command channel only carries throttle commands
(`loco.setSpeed`, `loco.setFunction`, `system.estop`,
`session.adopt`). When CV operations are scheduled, they will be
slotted into the same channel without a new daemon-level RPC.

When this milestone lands, §5 should be updated in place to point to
§7e for the throttle data plane; the existing §5 prose stays as a
historical anchor for M1 (when the daemon is not yet present).
