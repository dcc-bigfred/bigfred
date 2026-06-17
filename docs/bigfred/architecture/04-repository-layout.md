## 3. Repository Layout

The web layer is added next to the existing packages, the existing code is
reused, not duplicated.

### 3.0 Directory roles (layering glossary)

Every application (an *app* is the root of any deliverable — `dcc-bus`,
`server`, `scripts-executor`) is organised from the same set of
single-responsibility directories. Read `<app>/` below as "relative to that
app's root" (e.g. `server/repo`, `dcc-bus/state`). This glossary is the
**target convention**; where existing code does not yet match it, the
mismatch is called out as *legacy* and is expected to migrate.

| Directory | Responsibility |
|-----------|----------------|
| **`<app>/domain`** | Domain objects — `struct` and `interface` types that represent data structures: an entity loaded from the database (e.g. `User`), or any structured value returned from a method (not necessarily DB-backed). No persistence, transport, or policy logic. |
| **`<app>/repo`** | Repository pattern — the data **persistence** layer. Read/write methods that operate on the **database** (not Redis), accept/return `domain.*` objects, and contain no business or transport logic. |
| **`contract`** | Like `domain`, but strictly the data structures exchanged **over Redis** between `server`, `dcc-bus`, and `scripts-executor`. Shared, cross-process, and (unlike the others) lives once at `pkgs/bigfred/contract` rather than per-app — see §3.2. |
| **`<app>/validation`** | **Stateless** input validators, e.g. `UsersValidator.IsValidUserId()`. They answer "is this input well-formed?" and hold no state. |
| **`<app>/ws`** | Client communication over **WebSockets**. Handlers accept external data, run input validation (from `validation`), invoke the matching action handler in `cmd` to perform the work, and map the `cmd` result onto a WebSocket response. Thin adapter — no business logic. |
| **`<app>/http`** | Same as `ws`, but for **HTTP**: parse request → validate → call `cmd` → map result/sentinel errors to an HTTP response. |
| **`<app>/cmd`** | **Actions** to perform — the use-case layer: e.g. "Add a new user", "Add a vehicle to the layout", "Set speed". A `cmd` handler orchestrates `validation`, `security`, `repo`/`state`, `service`, and other actions. (In `dcc-bus`, `cmd` is also where the inbound `loco.*` dispatch/router lives.) |
| **`<app>/service`** | Miscellaneous helper **structs** that do not belong anywhere else and are reused from `cmd`, `ws`, etc.: conversions, calculations, `supervisord` management, the Redis driver wiring, joining or filtering data, and similar. Not the use-case layer. |
| **`<app>/helpers`** | Simple, standalone **functions** too small to justify a `service` struct. |
| **`<app>/errors`** | **Named constants** for machine-readable error codes returned on the wire (REST, WS ack, `loco.error`, session close reasons). No logic — see [§3.0 Typing conventions](#typing-conventions) below. |
| **`<app>/auth`** | Authentication concerns: token verification, encryption, credential/session handling. |
| **`<app>/security`** | **Stateless** policy structs that answer a simple "may this actor do X?" question and return a `Decision` — `Allow` or `Deny("reason")` with a machine-readable reason (see `decision.go` / `drive.go`). Pure functions over already-loaded `domain.*` values; no SQL, Redis, or transport (§7a.3). |
| **`<app>/state`** | Reading from and writing to **Redis**: GET/SET, subscriptions, and the corresponding responses. (Mirrors `repo`, but for Redis instead of the database.) |
| **`<app>/protocol`** | The `Payload` and `Response` types for `ws`/`http`, plus helper functions in the area of communication and sending data over `ws`/`http`. |
| **`<app>/cli`** | Command-line configuration (cobra subcommand + flags). |

**Layered flow (request → action):**
`http`/`ws` (transport) → `validation` (well-formed input) → `cmd`
(action/use-case) → `security` (may the actor?) + `repo`/`state`
(persistence) + `service`/`helpers` (support) → result mapped back to a
`protocol` response.

#### Typing conventions

Every machine-readable string that crosses a process or client boundary
must be a **named constant**, never an inline string literal at the call
site. This keeps the wire contract grep-able, prevents typos, and gives
the frontend a stable `errors:<code>` key for i18n (§7c).

| Kind of code | Where it lives | Go identifier prefix | Example |
|--------------|----------------|----------------------|---------|
| REST / WS / ack error codes | **`<app>/errors`** | `Code…`, `WsCode…` | `errors.WsCodeBadPayload` |
| Authorization denial reasons | **`<app>/security`** | `Reason…` | `security.ReasonNotAuthorized` |
| WS / HTTP payloads & responses | **`<app>/protocol`** | typed `struct`s | `protocol.LocoSubscribePayload` |
| Redis wire snapshots & commands | **`contract`** | typed `struct`s + builders | `contract.AllowedVehicles` |

Rules:

1. **Error codes are constants.** Declare them once in `<app>/errors`
   (transport/command failures) or as `Reason*` in `<app>/security`
   (policy denials returned via `Deny(Reason…)`). String values use
   `snake_case`. Do not write `Deny("forbidden")` or
   `ack{error: "bad_payload"}` inline — reference a `const` instead.
   Reference implementation: `pkgs/bigfred/dcc-bus/errors/`,
   `pkgs/bigfred/dcc-bus/security/decision.go`.
2. **Prefer typed structs over loose maps.** `domain`, `protocol`, and
   `contract` carry structured data; avoid `map[string]any` or ad-hoc
   JSON shapes on the wire.
3. **Add i18n keys in the same change.** Every new error or denial code
   shipped to the UI needs a matching `errors:<code>` entry in the
   frontend catalogues (§7c).
4. **Constructors live next to their type.** A `func New{Name}(…)` factory
   must be defined in the **same file** as the `struct` (or interface
   implementation type) it constructs — not in a separate `constructors.go`,
   not in a sibling package, and not only behind a legacy facade. Example:
   `cmd/vehicle.go` defines both `type Vehicle struct { … }` and
   `func NewVehicle(…) *Vehicle`. During migration, a thin `service`
   re-export may call `cmd.NewVehicle`, but the canonical type + constructor
   pair belongs together in `cmd`.

> **Legacy note (typing).** `pkgs/bigfred/server/security` still contains
> inline `Deny("forbidden")` string literals. Migrate those to named
> `Reason*` constants when touching the affected files.

> **Legacy note.** Today the server's use-case logic lives in
> `pkgs/bigfred/server/service/` (validation + `security` + orchestration +
> `repo` access, ~20 `*Service` files — see §3.1 and §5). Under this
> convention that logic belongs in `cmd`, and `service` narrows to the
> "miscellaneous helper structs" role above. Existing `service/*.go`
> use-cases are therefore **legacy to migrate into `cmd`**; treat the
> descriptions in §3.1/§5 as the current (pre-migration) shape.

> **Infrastructure exception.** Server-side dcc-bus orchestration
> (`DccBusService`, `DccBusLayoutSync`, `DccBusEventConsumer`) remains in
> `pkgs/bigfred/server/service`: it owns supervisord programs, TCP port
> allocation, Redis pub/sub and WS fan-in. Application use-cases in `cmd`
> talk to it only through ports such as `DccBusControlPort`,
> `EStopTargetDccBusPort` and `CommandStationDccSyncPort`.

### 3.1 Backend layer responsibilities

Three packages under `pkgs/bigfred/server/` form the main backend stack. Keep
business rules out of `http` / `ws`; keep authorization policy out of
`service` (delegate to `security` instead of inlining role checks).

> The table below describes the **current** server shape after the
> `service` → `cmd` migration: application use-cases live in `cmd`;
> `service` contains transport/runtime adapters and infrastructure helpers.

| Package | Role |
|--------|------|
| **`pkgs/bigfred/server/http`** (and **`pkgs/bigfred/server/ws`**) | Handle incoming HTTP / WebSocket traffic: routing (chi), middleware, session/JWT authentication, request/response mapping, status codes. Handlers are thin adapters — they parse input, read identity from context, call a `*Service`, and serialize the result or map sentinel errors to HTTP/WS codes. |
| **`pkgs/bigfred/server/cmd`** | Application / use-case layer: input validation handoff, loading entities for authorization, calling `pkgs/bigfred/server/security`, business logic, `pkgs/bigfred/server/repo` access, and side effects through narrow ports. |
| **`pkgs/bigfred/server/service`** | Runtime adapters and infrastructure helpers: Redis, supervisord, dcc-bus orchestration, diagnostics, WebSocket adapter wrappers, and compatibility helpers that bridge infrastructure into `cmd` ports. Not the use-case layer. |
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
    ├── http/                   # transport adapter — §3.1; delegates to cmd/service
    │   ├── router.go           # chi router + middleware
    │   ├── locos.go            # REST handlers (GET/POST/PUT/DELETE)
    │   ├── cv.go
    │   └── middleware.go       # authn, role gates, logging, CORS, recovery
    ├── ws/
    │   ├── hub.go              # central Hub
    │   ├── client.go           # per-connection reader/writer
    │   ├── protocol.go         # typed messages (Action/Event)
    │   └── handlers.go         # WS message dispatching
    ├── errors/                 # machine-readable REST/WS error codes (Code* + sentinels)
    │   ├── auth.go             #   login / session failures
    │   ├── sudo.go             #   sudo / signalman elevation failures
    │   ├── user.go             #   user catalogue
    │   ├── vehicle.go          #   vehicle catalogue
    │   ├── train.go            #   train catalogue
    │   ├── dcc.go              #   DCC pool / address
    │   ├── takeover.go         #   takeover arbitration failures
    │   ├── function.go         #   DCC function slots
    │   ├── layout_roster.go    #   layout roster
    │   ├── template.go         #   vehicle templates (shared)
    │   └── http.go             #   *HTTPStatus mappers
    ├── validation/             # stateless input validators (no repo, no HTTP)
    │   ├── user.go
    │   ├── vehicle.go
    │   ├── train.go
    │   ├── command_station.go
    │   ├── interlocking.go
    │   └── function.go
    ├── protocol/               # REST wire DTOs + To*Input helpers
    │   ├── auth.go
    │   ├── sudo.go
    │   ├── session_control.go
    │   ├── user.go
    │   ├── vehicle.go
    │   ├── train.go
    │   ├── function.go
    │   ├── command_station.go
    │   ├── interlocking.go
    │   ├── template.go
    │   └── layout.go
    ├── cmd/                    # use-case layer (migration in progress)
    │   ├── port.go             #   DCCPoolPort, LayoutRosterHubPort, …
    │   ├── auth.go             #   login, effective roles, JWT (migrated)
    │   ├── sudo.go             #   layout-scoped sudo + signalman elevation (migrated)
    │   ├── session_control.go  #   control-plane session.* WS use case (migrated)
    │   ├── dcc_pool.go         #   DCC pool validation/replacement (migrated)
    │   ├── command_station.go  #   CommandStation catalogue CRUD (migrated)
    │   ├── interlocking.go     #   Interlocking catalogue CRUD (migrated)
    │   ├── interlocking_occupancy.go # Interlocking join/leave + occupant enrichment (migrated)
    │   ├── presence.go         #   layout presence snapshots (migrated)
    │   ├── radio.go            #   walkie-talkie send/replay + fan-out (migrated)
    │   ├── radio_stop.go       #   layout-wide Radio Stop use case (migrated)
    │   ├── estop_target.go     #   per-target emergency stop use case (migrated)
    │   ├── takeover.go         #   takeover arbitration state machine (migrated)
    │   ├── radio_control.go    #   radio.* WS control dispatch (migrated)
    │   ├── takeover_control.go #   takeover.* WS control dispatch (migrated)
    │   ├── user.go             #   User CRUD + DCC pool orchestration (migrated)
    │   ├── vehicle.go          #   Vehicle CRUD (migrated)
    │   ├── train.go            #   Train CRUD (migrated)
    │   ├── function.go         #   Function CRUD (migrated)
    │   ├── vehicle_template.go #   VehicleTemplate CRUD (migrated)
    │   ├── layout.go           #   Layout CRUD + PIN (migrated)
    │   ├── layout_roster.go    #   LayoutRoster CRUD + WS fan-out (migrated)
    │   └── layout_roster_snapshot.go # Redis/dcc-bus roster snapshot building (migrated)
    ├── service/                # adapters + infrastructure helpers (no use-case layer)
    │   ├── session_control.go  # SessionControlService adapter → cmd.SessionControl
    │   ├── presence.go         # Presence adapter: ws.Hub → cmd.Presence ports
    │   ├── interlocking_occupancy.go # InterlockingOccupancyService facade → cmd.InterlockingOccupancy
    │   ├── takeover.go         # TakeoverService facade → cmd.Takeover
    │   ├── takeover_control.go # TakeoverControlService adapter → cmd.TakeoverControl
    │   ├── radio.go            # Radio adapter: Redis store/ws.Hub → cmd.Radio ports
    │   ├── radio_stop.go       # RadioStopService adapter → cmd.RadioStop
    │   ├── radio_control.go    # RadioControlService adapter → cmd.RadioControl
    │   ├── estop_target.go     # EStopTargetService adapter → cmd.EStopTarget
    │   ├── layout_vehicle.go   # LayoutVehicleService facade → cmd.LayoutRoster
    │   │                       #   plus adapter wiring for roster snapshot publishing
    │   ├── layout_drive_scope.go # compatibility helpers for drive-scope lessee lookups
    │   ├── layout_roster_redis.go # Redis roster publisher interface for dcc-bus snapshots
    │   ├── function.go         # FunctionService facade → cmd.Function
    │   ├── vehicle_template.go # VehicleTemplateService facade → cmd.VehicleTemplate
    │   ├── seed.go             # bootstrap admin seeding helper
    │   ├── redis.go            # Redis client + dcc-bus roster/command pub helpers
    │   ├── supervisord.go      # supervisord process manager
    │   ├── diagnostics.go      # whitelisted diagnostics file reader
    │   ├── dcc_bus.go          # (§7e.6) DccBusService: infrastructure orchestrator
    │   │                       #   for sibling dcc-bus daemons (port pool,
    │   │                       #   EnsureRunning/Stop, PublishCommand)
    │   ├── dcc_bus_supervisord.go # supervisord sync for dcc-bus programs
    │   └── dcc_bus_consumer.go # psubscribe on dcc-bus:evt:* and fan
    │                           #   selected daemon events into WebSocket Hub
    ├── security/               # PURE, STATELESS policy layer – see §7a.3
    │   ├── decision.go         # Decision type + Allow / Deny helpers
    │   ├── reasons.go          # Reason* constants (vehicle_not_owned, train_not_owned, …)
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
# pkgs/bigfred/dcc-bus/state (Redis client). Does NOT import
# pkgs/bigfred/server/repo, pkgs/bigfred/server/http, or ws. Layering
# follows [§3.0 Directory roles](#30-directory-roles-layering-glossary):
# ws → validation → cmd → security + service + state.
# Exposes WebSocket on --port for throttle traffic.
pkgs/bigfred/dcc-bus/                   # throttle data-plane daemon
├── daemon.go                   # assembly: Redis, station driver, Router, WS server
├── cli/
│   ├── cli.go                  # cobra subcommand + flags
│   └── station.go              # shared --station-* flag builders (server + daemon)
├── auth/
│   └── jwt.go                  # JWT verification on WS upgrade
├── errors/
│   ├── codes.go                # command / train error codes (Code*)
│   └── ws.go                   # transport error codes (WsCode*)
├── validation/
│   └── ws.go                   # stateless payload validators (subscribe, setSpeed, …)
├── protocol/                   # WS-only payloads + Frame helpers (§7e.4)
├── ws/                         # transport adapter — validate → cmd via adapter
│   ├── handler.go              # upgrade, read loop, dispatch, deadman
│   ├── adapter.go              # RouterAdapter: cmd.Result → ack / Outcome
│   ├── responder.go            # Session → cmd.Responder
│   ├── hub_port.go             # Hub → cmd.HubPort
│   ├── hub.go                  # live session registry + broadcast
│   ├── session.go              # per-connection reader/writer
│   └── metrics.go              # WS command latency + session gauges
├── cmd/                        # use-case layer — one file per action
│   ├── router.go               # Router facade, roster/train cache wiring
│   ├── port.go                 # Actor, Responder, HubPort interfaces
│   ├── result.go               # action Result (OK / Code / Members)
│   ├── subscribe.go            # HandleSubscribe
│   ├── set_speed.go            # HandleSetSpeed
│   ├── train_set_speed.go      # HandleTrainSetSpeed
│   ├── set_function.go         # HandleSetFunction
│   ├── estop.go                # HandleEStop + emergency-stop helpers
│   ├── session.go              # HandleSessionClose (dead-man)
│   ├── radio_stop.go           # HandleRadioStop + layout pub/sub
│   ├── roster_retire.go        # retire locos falling off roster
│   ├── functions.go            # DCC function I/O + timed pulses
│   └── control_redis.go        # HandleControlCommand (dcc-bus:cmd channel)
├── security/                   # stateless policy — Decision / Reason*
│   ├── decision.go             # Allow / Deny helpers
│   └── drive.go                # DrivePolicy, TrainPolicy
├── service/                    # helper structs (not use-case layer)
│   ├── roster.go               # in-memory allowed_vehicles cache
│   ├── function_cache.go       # per-(addr, fn) dedup cache
│   ├── speed.go                # DCCWriter (SetSpeed + retries)
│   ├── loco_state.go           # BroadcastLocoState
│   ├── state_feed.go           # external-throttle observer / poller
│   └── station/                # commandstation driver wiring
│       ├── driver.go           # Open Z21 / LocoNet from domain.CommandStation
│       ├── describe.go         # log-safe connection summary
│       ├── unwrap.go           # AsStateObserver / AsLocoInfoSubscriber
│       └── instrumented.go     # OTLP latency wrapper
└── state/                      # Redis GET/SET + pub/sub
    ├── redis.go                # loco:state keys, cmd/evt channels
    └── roster.go               # GET/SUBSCRIBE allowed_vehicles & defined_trains

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
    │   ├── ThrottlePage.tsx          # full-screen throttle overlay (§6.3b); vehicles + trains (§6.3a)
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
| **Train driving helpers** (`LeadingMember`, `EffectiveMemberSpeed`, `TrainSetSpeedWire`, …) shared by server snapshot builders and dcc-bus fan-out | [`trains.go`](../../../../pkgs/bigfred/contract/trains.go) |
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
