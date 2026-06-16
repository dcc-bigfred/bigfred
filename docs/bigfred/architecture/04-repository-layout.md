## 3. Repository Layout

The web layer is added next to the existing packages, the existing code is
reused, not duplicated.

### 3.1 Backend layer responsibilities

Three packages under `pkgs/bigfred/server/` form the main backend stack. Keep
business rules out of `http` / `ws`; keep authorization policy out of
`service` (delegate to `security` instead of inlining role checks).

| Package | Role |
|--------|------|
| **`pkgs/bigfred/server/http`** (and **`pkgs/bigfred/server/ws`**) | Handle incoming HTTP / WebSocket traffic: routing (chi), middleware, session/JWT authentication, request/response mapping, status codes. Handlers are thin adapters — they parse input, read identity from context, call a `*Service`, and serialize the result or map sentinel errors to HTTP/WS codes. |
| **`pkgs/bigfred/server/service`** | Application / use-case layer: input validation, loading entities for authorization, calling `pkgs/bigfred/server/security` to check permissions, business logic (conditionals, orchestration), `pkgs/bigfred/server/repo` access, and calls to other services (`DCCPoolService`, `AuditService`, `DccBusService`, …). |
| **`pkgs/bigfred/server/security`** | Pure, stateless policy layer (§7a.3): given loaded `domain.*` values, answers whether an actor may perform an action. No SQL, no HTTP. Invoked from services; HTTP middleware may use it for coarse route guards (see §7a.4). |

```
pkgs/
├── loco/                       # existing – core domain
│   ├── app/                    # LocoApp – controller layer
│   ├── commandstation/         # Z21, LocoNet
│   ├── decoders/
│   └── syntax/
├── rb/                         # existing – Railbox-specific code
│
└── bigfred/                    # BigFred application stack (server, dcc-bus, contract)
    ├── contract/               # Redis wire contract between server ↔ dcc-bus (§3.2)
    │   ├── README.md           #   package overview
    │   ├── allowedvehicles.go  #   roster keys, types, Marshal/Unmarshal, builders
    │   └── redis.go            #   key/channel templates + builder functions
    └── server/                 # web application  (also hosts dcc-bus/ — see block below)
    |   ├── main.go             # cmd entrypoint for `loco server`
    |   ├── cli/                # cobra command: `loco server`
    ├── http/                   # transport adapter — §3.1; delegates to service/
    │   ├── router.go           # chi router + middleware
    │   ├── locos.go            # REST handlers (GET/POST/PUT/DELETE)
    │   ├── cv.go
    │   └── middleware.go       # authn, role gates, logging, CORS, recovery
    ├── ws/
    │   ├── hub.go              # central Hub
    │   ├── client.go           # per-connection reader/writer
    │   ├── protocol.go         # typed messages (Action/Event)
    │   └── handlers.go         # WS message dispatching
    ├── service/                # use cases — §3.1: validate, security, logic, repo, other services
    │   ├── loco.go             # LocoService; map[commandStationID]Station
    │   ├── train.go            # TrainService – CRUD + SetSpeed fan-out
    │   │                       #                (lock-step, Reversed-flip, per-member ack)
    │   ├── auth.go             # AuthService – login + PIN, sessions/JWT
    │   ├── apikey.go           # APIKeyService – mint/revoke/verify, ≤365d
    │   ├── user.go             # UserService – roles, temp grants, DCC pool
    │   ├── lease.go            # LeaseService – vehicle/train leasing
    │   ├── interlocking.go     # InterlockingService – signal boxes (layout-filtered)
    │   ├── takeover.go         # TakeoverService – 15s arbitration
    │   ├── radio.go            # RadioService – walkie-talkie messages
    │   ├── command_station.go           # CommandStationService – CRUD over centralki (admin only)
    │   ├── layout.go            # LayoutService – CRUD, vehicle roster, presence,
    │   │                       #                signalmen list, interlocking whitelist
    │   ├── function.go         # FunctionService – dcc_functions CRUD, resolve, detach (§3a.6)
    │   │                       #                copy-on-write detach
    │   ├── template.go         # TemplateService – CRUD over vehicle templates
    │   ├── script.go           # ScriptService – CRUD over user JS scripts,
    │   │                       #                attachments to vehicles/trains,
    │   │                       #                Run/Stop dispatch to executor,
    │   │                       #                fan-out of script.changed / script.runStarted etc.
    │   ├── audit.go            # AuditService – append-only audit log writer
    │   ├── dcc_bus.go          # NEW (§7e.6) – DccBusService: orchestrator
    │   │                       #   for sibling dcc-bus daemons (port pool,
    │   │                       #   EnsureRunning/Stop, PublishCommand)
    │   ├── dcc_bus_consumer.go # NEW – psubscribe on dcc-bus:evt:* and fan
    │   │                       #   incoming events into AuditService / Bus /
    │   │                       #   ScriptService / WebSocket Hub
    │   ├── loco_driver.go      # NEW – LocoServiceDriver: thin replacement
    │   │                       #   of LocoService.{SetSpeed,ToggleFn,EStop}
    │   │                       #   that goes through DccBusService instead of
    │   │                       #   talking to commandstation.Station directly
    │   └── poller.go           # legacy: kept only as a fallback when running
    │                           #   without dcc-bus (--no-supervisor dev mode)
    ├── security/               # PURE, STATELESS policy layer – see §7a.3
    │   ├── decision.go         # Decision type + Allow / Deny helpers
    │   ├── loco.go             # LocoSecurityContext – CanDriveLoco / CanEditLoco
    │   ├── train.go            # TrainSecurityContext
    │   ├── lease.go            # LeaseSecurityContext – CanLeaseOut, CanRevoke
    │   ├── interlocking.go     # InterlockingSecurityContext – CanOccupy, CanRequestTakeover
    │   ├── radio.go            # RadioSecurityContext – CanSendTo
    │   ├── user.go             # UserSecurityContext – admin policies
    │   ├── apikey.go           # APIKeySecurityContext – CanMint, CanRevoke
    │   ├── command_station.go           # CommandStationSecurityContext – CanEditCommandStation (admin only)
    │   ├── layout.go            # LayoutSecurityContext – CanCreate/CanJoin/CanAddSignalman/CanAddInterlocking
    │   ├── function.go         # FunctionSecurityContext – CanEditFunctions / CanInvokeFunction
    │   ├── template.go         # TemplateSecurityContext – CanEditTemplate (owner or admin)
    │   └── audit.go            # AuditSecurityContext – CanReadAuditLog (admin only)
    ├── domain/                 # pure entities (REL maps onto these)
    │   ├── user.go             # User, Role, TemporaryRole, DCCPool
    │   ├── apikey.go           # APIKey
    │   ├── vehicle.go          # Vehicle, Train, TrainMember
    │   ├── lease.go            # VehicleLease, TrainLease
    │   ├── interlocking.go     # Interlocking, InterlockingSession
    │   ├── takeover.go         # TakeoverRequest
    │   ├── radio.go            # RadioMessage, RadioPhrase
    │   ├── command_station.go           # CommandStation, CommandStationConnection, CommandStationConnectionType
    │   ├── layout.go            # Layout, LayoutSignalman, LayoutInterlocking, LayoutVehicle
    │   ├── function.go         # DccFunction, FunctionIcon
    │   ├── template.go         # VehicleTemplate
    │   └── audit.go            # AuditLogEntry, AuditAction
    ├── repo/                   # REL repositories (Data Mapper)
    │   ├── db.go               # *sql.DB + rel.Repository open
    │   ├── users.go            # repository helpers for User
    │   ├── apikeys.go
    │   ├── vehicles.go
    │   ├── trains.go           # Train + TrainMember repo (preload members on lookup)
    │   ├── functions.go
    │   ├── templates.go
    │   ├── scripts.go          # Script + ScriptAttachment repo (with size cap on Source)
    │   ├── leases.go
    │   ├── interlockings.go
    │   ├── command_stations.go
    │   ├── layouts.go
    │   ├── audit.go            # append-only writer + indexed reader
    │   └── migrations/         # REL migrations in Go (embed.FS)
    ├── mcp/                    # built-in MCP server (mark3labs/mcp-go)
    │   ├── server.go           # NewServer(): mounts on /mcp + stdio mode
    │   ├── tools_loco.go       # loco.list / loco.setSpeed / loco.toggleFn
    │   ├── tools_radio.go      # radio.send + standard phrases
    │   └── auth.go             # API-key middleware (header / query / stdio env)
    ├── executor/               # NEW – RPC bridge to scripts-executor process
    │   ├── messages.go         # RunStart / RunStop / RunEvent / CallResult message types
    │   ├── codec.go            # length-prefixed JSON frames over a net.Conn
    │   ├── client.go           # used in `server`: dials the executor's Unix socket,
    │   │                       #                  serialises run.start / run.stop,
    │   │                       #                  routes events back into ws.Hub
    │   ├── server.go           # used in `scripts-executor`: accepts the socket,
    │   │                       #                              spawns one goroutine per run
    │   └── supervisor.go       # used in `server`: spawns the executor child process,
    │                           #                  exponential-backoff restart, health pings
    ├── scripts/                # NEW – Goja runtime + JS DSL
    │   ├── runtime.go          # Runtime: builds *goja.Runtime, wires bindings,
    │   │                       #          runs source under context.Deadline
    │   ├── bindings.go         # findFirstLoco / findByDCCAddr / members / sleep / log
    │   ├── vehicle.go          # the Go-side Vehicle helper exposed as a JS object
    │   │                       # via vm.SetFieldNameMapper(UncapFieldNameMapper{})
    │   └── runtime_test.go     # canonical example, edge cases, vm.Interrupt
    ├── cache/
    │   └── redis.go            # cache + pubsub
    └── bus/
        └── bus.go              # in-process event bus (channels)

# Sibling process – same module, just a different main(). Reuses
# `pkgs/bigfred/server/scripts`, `pkgs/bigfred/server/executor`, `pkgs/bigfred/server/service`,
# `pkgs/bigfred/server/security`. Does NOT import `pkgs/bigfred/server/http` or `ws`.
pkgs/scripts-executor/          # NEW – sandbox process for user JS
├── main.go                     # cmd entrypoint for `loco scripts-executor`
└── cli/                        # cobra command + flags (socket path, cpu/mem caps)

# Sibling process – same module, different main(). One process per
# (layout × command_station) pair (§7e). Reuses pkgs/loco/commandstation
# (DCC), pkgs/bigfred/server/domain (entities), pkgs/bigfred/contract (Redis key
# templates, builders, snapshot types, Marshal/Unmarshal — see §3.2),
# pkgs/bigfred/dcc-bus/state (Redis client). Does NOT import pkgs/bigfred/server/repo or SQLite.
# Exposes WebSocket on --port for throttle traffic.
pkgs/bigfred/dcc-bus/                   # throttle data-plane daemon
├── daemon.go                   # assembly: Redis, Station, Router, WS server
├── cli/cli.go                  # cobra subcommand + flags
├── cliargs/station.go          # shared --station-* flag builders (server + daemon)
├── cmd/router.go               # loco.* dispatch, roster cache, DCC I/O
├── state/redis.go              # loco:state keys, cmd/evt pub/sub
├── state/roster.go             # GET/SUBSCRIBE allowed_vehicles & defined_trains
├── ws/                         # Hub, JWT upgrade, frame routing
├── auth/jwt.go                 # JWT verification
├── station/                    # commandstation driver wiring
└── protocol/                   # WS envelope types

web/                            # NEW – frontend
├── package.json
├── vite.config.ts
├── index.html
├── public/
│   └── sounds/
│       ├── radiostop.ogg       # radiostop alarm, served at /sounds/radiostop.ogg (§4.6.3)
│       └── train-announcements/ # PA Ogg files, served at /sounds/train-announcements/{name}.ogg (§6.3d)
└── src/
    ├── main.tsx
    ├── App.tsx
    ├── theme.ts                # MUI ThemeProvider configuration (palette, breakpoints)
    ├── config/
    │   └── trainAnnouncements.ts # static PA catalogue keyed by interlocking name (§6.3d)
    ├── api/
    │   ├── client.ts           # fetch wrapper
    │   └── locos.ts            # REST endpoints + TanStack Query hooks
    ├── ws/
    │   ├── useSocket.ts        # hook: connect/reconnect, send, subscribe
    │   ├── protocol.ts         # types generated from Go
    │   └── store.ts            # Zustand: locomotive state from WS
    ├── components/             # MUI-based components
    │   ├── LocoCard.tsx        # MUI Card + CardContent + CardActions
    │   ├── ThrottleSlider.tsx  # MUI Slider
    │   ├── FunctionButtons.tsx # MUI ToggleButton / IconButton grid; order = function.position
    │   ├── FunctionList.tsx    # sortable list + icon picker used by VehicleFunctionsPage
    │   ├── ScriptButtons.tsx   # row of script buttons rendered alongside FunctionButtons;
    │   │                       # pressing one fires WS `script.run`, second press fires
    │   │                       # `script.stop`. No JS is ever executed in the browser.
    │   ├── ScriptEditor.tsx    # Monaco editor (JavaScript) + Save / Attach.
    │   │                       # The "Run" button is meaningful only from a throttle view,
    │   │                       # not from the editor – running needs a target.
    │   ├── ScriptConsole.tsx   # subscribes to WS `script.log` events for the current run
    │   │                       # and renders run history (start time, duration, reason)
    │   ├── AppShell.tsx        # MUI AppBar (incl. Throttle toggle) + Drawer + Container
    │   ├── ThrottleOverlay.tsx # full-screen driving layer (§6.3b); hosts Loco/Train control
    │   ├── ThrottleToolbar.tsx # left toolbar inside the overlay: Fullscreen toggle + Radio Stop (§6.3b)
    │   ├── FullscreenButton.tsx # browser Fullscreen API toggle for the overlay container
    │   ├── RadioStopButton.tsx # red radiostop button + centred confirm overlay (§4.6.3)
    │   ├── LayoutVehiclesTable.tsx
    │   ├── OnlineUsersTable.tsx
    │   ├── InterlockingsTable.tsx
    │   ├── InterlockingRadioPanel.tsx
    │   ├── InterlockingTrainAnnouncementsPanel.tsx # train PA list (§6.3d)
    │   └── LeaveInterlockingDialog.tsx
    ├── pages/
    │   ├── HomePage.tsx        # layout dashboard (§6.3c)
    │   ├── InterlockingPage.tsx # /interlockings/:id – occupation + radio (§6.3d)
    │   ├── LocoListPage.tsx        # /vehicles – owner catalogue; row actions Edit + Edit functions (§6.3e)
    │   ├── VehicleEditPage.tsx     # /vehicles/:addr/edit
    │   ├── VehicleFunctionsPage.tsx # /vehicles/:addr/functions – F0–F31 editor, icon picker, reorder (§6.3e)
    │   ├── LocoControlPage.tsx
    │   ├── TrainControlPage.tsx # same ThrottleSlider as LocoControlPage; sends train.setSpeed
    │   └── ScriptsPage.tsx     # the "Scripts" tab: list, edit, attach
    └── styles/
```

### 3.2 Contract package (`pkgs/bigfred/contract`)

`loco-server` and `dcc-bus` are sibling processes that coordinate **only over
Redis**. The `contract` package is their shared wire specification — not a
runtime service, but the typed boundary both sides import:

| Concern | Where in `contract` |
|--------|---------------------|
| Redis key / channel **templates** (`*Tmpl` constants with `%d` / `%s`) | [`redis.go`](../../../../pkgs/bigfred/contract/redis.go) |
| **Builder functions** that turn layout ID, command-station ID, DCC address, … into a concrete key or channel string | same file (`AllowedVehiclesKey`, `DccBusEventChannel`, …) |
| **JSON snapshot types** for roster data the server publishes and dcc-bus caches | [`allowedvehicles.go`](../../../../pkgs/bigfred/contract/allowedvehicles.go) |
| **Payload helpers** — `Marshal` / `Unmarshal*` and `NowMS()` — that encode and decode wire bytes from plain Go values | same file |

The server loads catalogue rows from SQLite, assembles `AllowedVehicles` /
`DefinedTrains` structs from primitive fields, calls `contract.Marshal`, and
`SET` + `PUBLISH`es on the keys from `contract.AllowedVehiclesKey` /
`contract.DefinedTrainsKey`. The daemon never opens SQLite; it
`GET`/`SUBSCRIBE`s those keys and decodes with `contract.Unmarshal*`.

Keep every new cross-process Redis name in `redis.go` first. Do not scatter
literals in `server` or `dcc-bus`. The package must not import either side
(so the dependency graph stays acyclic). See also the package
[`README.md`](../../../../pkgs/bigfred/contract/README.md).
