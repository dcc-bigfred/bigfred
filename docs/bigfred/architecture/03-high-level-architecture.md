## 2. High-Level Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│  Browser (React + Vite, mobile/desktop)                                    │
│                                                                            │
│  ┌─────────────┐  REST (CRUD)        ┌──────────────────────────┐          │
│  │ TanStack    │ ───────────────────►│                          │          │
│  │ Query       │ ◄───────────────────│                          │          │
│  └─────────────┘                     │   loco-server            │          │
│  ┌─────────────┐  control-plane WS   │   (control plane)        │          │
│  │ Zustand +   │ ◄──────────────────►│                          │          │
│  │ useSocket   │  /api/v1/ws         │                          │          │
│  └─────────────┘                     └─────────┬────────────────┘          │
│  ┌─────────────┐  data-plane WS                │                           │
│  │ Throttle    │ ◄──────────────────────────┐  │                           │
│  │ Zustand     │ ws://host:<port>/ws        │  │                           │
│  └─────────────┘   (per picked cs)          │  │                           │
└─────────────────────────────────────────────┼──┼───────────────────────────┘
                                              │  │
        ┌─────────────────────────────────────┼──┼───────────────────────────┐
        │                                     │  │                           │
        │  ┌────────────────────────────────  │  ▼   ──────────────────────┐ │
        │  │ HTTP (chi)             ┌────────────────────┐                 │ │
        │  │ /api/v1/...            │ WebSocket Hub      │  control plane  │ │
        │  └──────┬─────────────────└─────────┬──────────┘                 │ │
        │         │                           │                            │ │
        │         └────────────┬──────────────┘                            │ │
        │                      ▼                                           │ │
        │             ┌─────────────────┐                                  │ │
        │             │  Services       │   AuditService, AuthService,     │ │
        │             │  (no DCC writes;│   LayoutService, TakeoverService,│ │
        │             │   delegates to  │   ScriptService, RadioService,   │ │
        │             │   dcc-bus via   │   DccBusService (orchestrator),  │ │
        │             │   Redis cmd ch.)│   SupervisordService             │ │
        │             └────┬────────────┘                                  │ │
        │                  │                                               │ │
        │        ┌─────────▼──┐                       ┌───────────────┐    │ │
        │        │ Repository │                       │ Cache (Redis) │    │ │
        │        │ (SQLite,   │                       │ - loco:state  │    │ │
        │        │  RW writer)│                       │ - dcc-bus:cmd │    │ │
        │        └────────────┘                       │ - dcc-bus:evt │    │ │
        │                                             │ - pubsub      │    │ │
        │                                             └──────┬────────┘    │ │
        │  loco-server process (Go, non-root)                │             │ │
        └────────────────────┬───────────────────────────────┼─────────────┘ │
                             │ owns supervisord              │               │
                             ▼                               │               │
                ┌────────────────────────────────────┐       │               │
                │ supervisord (Python, §7d)          │       │               │
                │   group: loco                      │       │               │
                │     - scripts-executor             │       │               │
                │   group: dcc-bus (§7e)             │       │               │
                │     - dcc-bus-<L>-<C> (per pair)   │       │               │
                └────────┬───────────────────────┬───┘       │               │
                         │                       │           │               │
                         ▼                       ▼           ▼               │
        ┌──────────────────────────┐   ┌──────────────────────────────────┐  │
        │  scripts-executor        │   │  dcc-bus (one per layout × cs)   │  │
        │  (sibling Goja runtime)  │   │  ──────────────────────────────  │  │
        │  ──────────────────────  │   │  · pkgs/loco/commandstation      │  │
        │  Per active run:         │   │    (Z21 / LocoNet)               │  │
        │   1 goroutine            │   │  · ws://*:<port>/ws (data plane) │  │
        │   1 *goja.Runtime        │   │  · pkgs/server/security re-check │  │
        │   vm.Interrupt for       │   │  · subscribe vehicles from       │  │
        │   deadline / user-stop / │   │    LayoutVehicle roster          │  │
        │   dead-man's switch      │   │  · Redis loco:state writer       │  │
        │                          │   │  · consumes dcc-bus:cmd          │  │
        │  DSL bindings (setSpeed, │   │    (scripts, train fan-out,      │  │
        │  funcOn/Off, …)          │   │     dead-man, takeover)          │  │
        │   ─► RPC to loco-server  │   │  · publishes dcc-bus:evt         │  │
        │     ─► DccBusService     │   │    (state, errors, emergency)    │  │
        │       ─► Redis cmd ch.   │   └────────────┬─────────────────────┘  │
        │         ─► dcc-bus       │                │                        │
        │           ─► DCC bus     │                ▼                        │
        └──────────────────────────┘   ┌────────────────────────┐            │
                                       │  Z21 / LocoNet master  │            │
                                       │  (physical command     │            │
                                       │   station hardware)    │            │
                                       └────────────────────────┘            │
```

Core idea:

- **REST is used for CRUD-like, idempotent traffic** (list of locos, edit
  metadata, read/write CVs, system status).
- **Control-plane WebSocket** (`/api/v1/ws` on `loco-server`) carries
  session, takeover, radio, scripts, presence, sudo elevation and the
  layout/command-station picker.
- **Data-plane WebSocket** (`ws://host:<port>/ws` on each `dcc-bus`)
  carries throttle traffic (`loco.subscribe` / `loco.setSpeed` /
  `loco.toggleFn` / `system.estop`). One `dcc-bus` daemon per
  `(layout × command station)` pair, spawned lazily by `loco-server`
  via supervisord (§7d, §7e).
- **The command station (Z21 / LocoNet) lives in the `dcc-bus`
  process, not in `loco-server`.** A `scripts-executor` script never
  touches the DCC bus directly; it rounds through `loco-server` →
  `DccBusService` → Redis command channel → `dcc-bus` so authorization,
  audit and the dead-man's switch stay layered.
- **Both sibling processes are isolated.** `scripts-executor` and
  `dcc-bus` can crash, OOM, or be killed without affecting
  `loco-server`'s REST or control-plane WS. supervisord respawns
  them; the process boundary is what gives that guarantee.
