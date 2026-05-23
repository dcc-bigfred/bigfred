## 3. Repository Layout

The web layer is added next to the existing packages, the existing code is
reused, not duplicated.

```
pkgs/
├── loco/                       # existing – core domain
│   ├── app/                    # LocoApp – controller layer
│   ├── commandstation/         # Z21, LocoNet
│   ├── decoders/
│   └── syntax/
├── rb/                         # existing – Railbox-specific code
│
├── server/                     # NEW – web application
│   ├── main.go                 # cmd entrypoint for `loco server`
│   ├── cli/                    # cobra command: `loco server`
    ├── http/
    │   ├── router.go           # chi router + middleware
    │   ├── locos.go            # REST handlers (GET/POST/PUT/DELETE)
    │   ├── cv.go
    │   └── middleware.go       # logging, CORS, recovery
    ├── ws/
    │   ├── hub.go              # central Hub
    │   ├── client.go           # per-connection reader/writer
    │   ├── protocol.go         # typed messages (Action/Event)
    │   └── handlers.go         # WS message dispatching
    ├── service/
    │   ├── loco.go             # LocoService – business layer; map[layoutID]Station
    │   ├── train.go            # TrainService – CRUD + SetSpeed fan-out
    │   │                       #                (lock-step, Reversed-flip, per-member ack)
    │   ├── auth.go             # AuthService – login + PIN, sessions/JWT
    │   ├── apikey.go           # APIKeyService – mint/revoke/verify, ≤365d
    │   ├── user.go             # UserService – roles, temp grants, DCC pool
    │   ├── lease.go            # LeaseService – vehicle/train leasing
    │   ├── interlocking.go     # InterlockingService – signal boxes (party-filtered)
    │   ├── takeover.go         # TakeoverService – 15s arbitration
    │   ├── radio.go            # RadioService – walkie-talkie messages
    │   ├── layout.go           # LayoutService – CRUD over makiety (admin only)
    │   ├── party.go            # PartyService – CRUD, join, signalmen list,
    │   │                       #                interlocking whitelist
    │   ├── function.go         # FunctionService – vehicle F0-F32 list,
    │   │                       #                copy-on-write detach
    │   ├── template.go         # TemplateService – CRUD over vehicle templates
    │   ├── script.go           # ScriptService – CRUD over user JS scripts,
    │   │                       #                attachments to vehicles/trains,
    │   │                       #                Run/Stop dispatch to executor,
    │   │                       #                fan-out of script.changed / script.runStarted etc.
    │   ├── audit.go            # AuditService – append-only audit log writer
    │   └── poller.go           # background: polls Station, emits events
    ├── security/               # PURE, STATELESS policy layer – see §7a.3
    │   ├── decision.go         # Decision type + Allow / Deny helpers
    │   ├── loco.go             # LocoSecurityContext – CanDriveLoco / CanEditLoco
    │   ├── train.go            # TrainSecurityContext
    │   ├── lease.go            # LeaseSecurityContext – CanLeaseOut, CanRevoke
    │   ├── interlocking.go     # InterlockingSecurityContext – CanOccupy, CanRequestTakeover
    │   ├── radio.go            # RadioSecurityContext – CanSendTo
    │   ├── user.go             # UserSecurityContext – admin policies
    │   ├── apikey.go           # APIKeySecurityContext – CanMint, CanRevoke
    │   ├── layout.go           # LayoutSecurityContext – CanEditLayout (admin only)
    │   ├── party.go            # PartySecurityContext – CanCreate/CanJoin/CanAddSignalman/CanAddInterlocking
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
    │   ├── layout.go           # Layout, LayoutConnection, LayoutConnectionType
    │   ├── party.go            # Party, PartySignalman, PartyInterlocking
    │   ├── function.go         # VehicleFunction, FunctionIcon, FunctionKind
    │   ├── template.go         # VehicleTemplate, TemplateFunction
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
    │   ├── layouts.go
    │   ├── parties.go
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
# `pkgs/server/scripts`, `pkgs/server/executor`, `pkgs/server/service`,
# `pkgs/server/security`. Does NOT import `pkgs/server/http` or `ws`.
pkgs/scripts-executor/          # NEW – sandbox process for user JS
├── main.go                     # cmd entrypoint for `loco scripts-executor`
└── cli/                        # cobra command + flags (socket path, cpu/mem caps)

web/                            # NEW – frontend
├── package.json
├── vite.config.ts
├── index.html
└── src/
    ├── main.tsx
    ├── App.tsx
    ├── theme.ts                # MUI ThemeProvider configuration (palette, breakpoints)
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
    │   ├── FunctionButtons.tsx # MUI ToggleButton / IconButton grid
    │   ├── ScriptButtons.tsx   # row of script buttons rendered alongside FunctionButtons;
    │   │                       # pressing one fires WS `script.run`, second press fires
    │   │                       # `script.stop`. No JS is ever executed in the browser.
    │   ├── ScriptEditor.tsx    # Monaco editor (JavaScript) + Save / Attach.
    │   │                       # The "Run" button is meaningful only from a throttle view,
    │   │                       # not from the editor – running needs a target.
    │   ├── ScriptConsole.tsx   # subscribes to WS `script.log` events for the current run
    │   │                       # and renders run history (start time, duration, reason)
    │   └── AppShell.tsx        # MUI AppBar + Drawer + Container
    ├── pages/
    │   ├── LocoListPage.tsx
    │   ├── LocoControlPage.tsx
    │   ├── TrainControlPage.tsx # same ThrottleSlider as LocoControlPage; sends train.setSpeed
    │   └── ScriptsPage.tsx     # the "Scripts" tab: list, edit, attach
    └── styles/
```
