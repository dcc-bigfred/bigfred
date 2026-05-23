## 2. High-Level Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│  Browser (React + Vite, mobile/desktop)                            │
│                                                                    │
│  ┌─────────────┐  REST (CRUD)  ┌──────────────┐                    │
│  │ TanStack    │ ────────────► │              │                    │
│  │ Query       │ ◄──────────── │              │                    │
│  └─────────────┘               │              │                    │
│  ┌─────────────┐  WebSocket    │  Go backend  │                    │
│  │ Zustand +   │ ◄────────────►│              │                    │
│  │ useSocket   │  (real-time)  │              │                    │
│  └─────────────┘               └──────┬───────┘                    │
└────────────────────────────────────────┼───────────────────────────┘
                                         │
        ┌────────────────────────────────┼──────────────────────────┐
        │                                ▼                          │
        │  ┌──────────────┐   ┌────────────────────┐                │
        │  │ HTTP (chi)   │   │ WebSocket Hub      │                │
        │  │ /api/v1/...  │   │ (coder/websocket)  │                │
        │  └──────┬───────┘   └─────────┬──────────┘                │
        │         │                     │                           │
        │         └──────────┬──────────┘                           │
        │                    ▼                                      │
        │           ┌─────────────────┐    EventBus (channels +     │
        │           │  Services       │◄── Redis Pub/Sub)           │
        │           │  (LocoApp)      │                             │
        │           └────┬────────┬───┘                             │
        │                │        │                                 │
        │      ┌─────────▼──┐  ┌──▼────────────┐  ┌───────────────┐ │
        │      │ Repository │  │ CommandStation│  │ Cache (Redis) │ │
        │      │ (SQLite)   │  │ (Z21/LocoNet) │  │               │ │
        │      └────────────┘  └───────────────┘  └───────────────┘ │
        │                          ▲                                │
        │                          │ (every DSL call from a         │
        │                          │  running script ends up here)  │
        │                          │                                │
        │  ┌───────────────────────┴────────────────────────────┐   │
        │  │  ExecutorClient                                    │   │
        │  │  ────────────────────────────────────────────────  │   │
        │  │  Unix socket (length-prefixed JSON frames):        │   │
        │  │    server → executor:  run.start / run.stop / ack  │   │
        │  │    executor → server:  run.event { kind=log|call|  │   │
        │  │                                    started|done }  │   │
        │  │    server → executor:  call.result                 │   │
        │  └───────────────────────┬────────────────────────────┘   │
        │  Go backend `server` process                              │
        └──────────────────────────┼────────────────────────────────┘
                                   │ same machine, ~/.cache/loco/exec.sock
                                   ▼
        ┌──────────────────────────────────────────────────────────┐
        │  Go backend `scripts-executor` process (same binary,     │
        │                                  different main entry)   │
        │  ┌─────────────────────────────────────────────────────┐ │
        │  │  one goroutine + one *goja.Runtime per active run   │ │
        │  │                                                     │ │
        │  │  Goja VM owns the user's JS source; bindings:       │ │
        │  │    findFirstLoco, findByDCCAddr, members,           │ │
        │  │    Vehicle.setSpeed/funcOn/funcOff, sleep, log      │ │
        │  │                                                     │ │
        │  │  Each binding → RPC `call` back to server, blocks   │ │
        │  │  on the matching `call.result`. Server runs the     │ │
        │  │  service + re-checks LocoSecurityContext, then      │ │
        │  │  replies. vm.Interrupt() from supervisor goroutine  │ │
        │  │  for deadline / user-stop / dead-man's switch.      │ │
        │  └─────────────────────────────────────────────────────┘ │
        │                                                          │
        │  Supervised by `server`: if this process dies, `server`  │
        │  fan-outs script.runStopped{reason:"executor_crashed"}   │
        │  to every owner of an in-flight run and respawns the     │
        │  executor with exponential backoff (§7.x).               │
        └──────────────────────────────────────────────────────────┘
```

Core idea:

- **REST is used for CRUD-like, idempotent traffic** (list of locos, edit
  metadata, read/write CVs, system status).
- **WebSocket is used for real-time traffic** (throttle, direction,
  functions, push events coming from the command station).
- **The command station (Z21 / LocoNet) lives in the `server`
  process only.** The `scripts-executor` never touches the DCC bus
  directly; it always rounds through `server` so authorization, audit
  and the dead-man's switch stay in one place.
- **The executor is a sibling process, not a child library.** It can
  crash, OOM, or be killed without affecting the throttle. The
  process boundary is what gives that guarantee – inside a single
  process a runaway `for (;;) {}` in Goja, even with `vm.Interrupt`,
  could still starve the Go scheduler.
