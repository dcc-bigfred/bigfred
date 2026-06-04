### 3a.7 Server-side scripts (Goja sandbox in a sibling executor process)

Scripts are the user-facing automation layer **on top of** the existing
throttle. The architectural rule of thumb:

> **User JavaScript is executed server-side, in a sandboxed Goja VM,
> inside a sibling `scripts-executor` process – never inside the
> `server` process and never in the browser.** The browser only edits
> source, presses a play button, and renders log output.

The sibling-process design is the load-bearing decision: a runaway
script (infinite loop, OOM, panic in a Go binding, Goja itself
misbehaving) can take down only the executor, never the main
throttle. The browser never even sees the source of someone else's
script.

#### 3a.7.1 Process model and RPC channel

```
            ┌───────────────────────┐       ┌────────────────────────┐
            │  server               │       │  scripts-executor      │
            │  (chi + ws + Station) │       │  (Goja VMs)            │
            ├───────────────────────┤       ├────────────────────────┤
            │                       │       │                        │
WS press ──►│ ScriptService.Run     │       │                        │
            │   ├─ resolve scope    │       │                        │
            │   ├─ assign runId     │       │                        │
            │   └─ exec.Client.Send │──RPC─►│ executor.Server.OnRun  │
            │      run.start{...}   │       │   ├─ goroutine.Start   │
            │                       │       │   ├─ goja.New          │
            │                       │       │   ├─ bind DSL          │
            │                       │       │   └─ vm.RunString(src) │
            │                       │       │                        │
            │ ws.Hub broadcasts     │       │ on every DSL call:     │
            │   script.runStarted   │       │   binding fn ─► RPC ──►│
            │   to user's sessions  │       │     run.event{kind:    │
            │                       │       │       "call", callId,  │
            │                       │       │       method, args}    │
            │ ScriptService.OnCall  │◄─RPC──│   binding blocks on    │
            │   ├─ LocoSecurityCtx  │       │   the matching reply   │
            │   ├─ LocoService.Set..│       │                        │
            │   └─ exec.Send        │──RPC─►│   call.result{ok,...}  │
            │      call.result{...} │       │     binding resumes    │
            │                       │       │                        │
            │ ws.Hub broadcasts     │◄─RPC──│ run.event{kind:"log"}  │
            │   script.log to all   │       │                        │
            │   of user's sessions  │       │                        │
            │                       │       │                        │
            │ ScriptService.OnDone  │◄─RPC──│ run.finished{reason,   │
            │   ├─ release attach   │       │              errMsg?}  │
            │   └─ ws.Hub broadcasts│       │                        │
            │      script.runStopped│       │                        │
            └───────────────────────┘       └────────────────────────┘
```

Both processes are **the same Go binary** (`loco`) invoked with
different subcommands:

- `loco server`             — REST + WS + Station + REL + executor supervisor.
- `loco scripts-executor`   — Goja VMs + executor RPC server.

The two processes share `pkgs/server/scripts`, `pkgs/server/executor`,
`pkgs/server/service` and `pkgs/server/security`. They **do not**
share `pkgs/server/http` or `pkgs/server/ws` (the executor has no
need for the HTTP / WebSocket layers).

The RPC channel is a **Unix domain socket** with `0600` permissions
located by default at `$XDG_RUNTIME_DIR/loco/exec.sock` (configurable
via `--executor-socket`). The wire format is a **length-prefixed
JSON frame**: 4-byte big-endian length followed by a JSON object.
Trivial to implement on top of `bufio.Reader` / `bufio.Writer`, no
extra dependency. On platforms without Unix sockets the fallback is
loopback TCP on `127.0.0.1` with a per-boot shared secret echoed via
an env var to the child process.

```go
// pkgs/server/executor/messages.go (subset)
type RunStart struct {
    Type        string          `json:"type"`        // "run.start"
    RunID       string          `json:"runId"`
    ScriptID    uint            `json:"scriptId"`
    OwnerUserID uint            `json:"ownerUserId"`
    ActorUserID uint            `json:"actorUserId"` // who pressed the button (may be a lessee)
    SessionID   string          `json:"sessionId"`   // owner of the run in WS terms
    Runtime     string          `json:"runtime"`     // "goja"
    Source      string          `json:"source"`
    Scope       ScriptScope     `json:"scope"`       // attached vehicle XOR train, expanded
    DeadlineSec int             `json:"deadlineSec"`
}
type RunStop struct {
    Type   string `json:"type"`  // "run.stop"
    RunID  string `json:"runId"`
    Reason string `json:"reason"` // "user", "deadman", "executor_shutdown"
}
type RunEvent struct {
    Type   string          `json:"type"`   // "run.event"
    RunID  string          `json:"runId"`
    Kind   string          `json:"kind"`   // "started" | "log" | "call" | "finished"
    CallID *string         `json:"callId,omitempty"`
    Method *string         `json:"method,omitempty"`
    Args   []any           `json:"args,omitempty"`
    Msg    *string         `json:"msg,omitempty"`     // for kind=log
    Reason *string         `json:"reason,omitempty"`  // for kind=finished
    Error  *string         `json:"error,omitempty"`
}
type CallResult struct {
    Type   string `json:"type"`   // "call.result"
    RunID  string `json:"runId"`
    CallID string `json:"callId"`
    OK     bool   `json:"ok"`
    Result any    `json:"result,omitempty"`
    Error  string `json:"error,omitempty"`
}
```

#### 3a.7.2 Sandbox properties

Inside the executor:

- One run = one goroutine = one `*goja.Runtime`. Goja VMs are
  **explicitly not goroutine-safe** (see the project FAQ), so this
  mapping is mandatory; the executor enforces it by allocating both
  in the same goroutine that consumes `run.start`.
- `vm.SetFieldNameMapper(goja.UncapFieldNameMapper())` maps Go method
  names to idiomatic JS (`SetSpeed` → `setSpeed`).
- `sleep(seconds)` is a thin wrapper around `time.Sleep` that blocks
  **only the goroutine running this VM**. Other VMs and the
  executor's RPC reader run independently.
- The user's global namespace contains **only** the DSL bindings
  listed in §3a.7.3. No `setTimeout`, no `setInterval` (Goja
  intentionally does not provide them – the host must), no `XMLHttpRequest`,
  no `process`, no `require`, no `globalThis` writes through to the
  Go side.
- Three independent stop signals can interrupt a VM:
  - `vm.Interrupt("user")`     — driver pressed Stop;
  - `vm.Interrupt("deadman")`  — dead-man's switch fired (§4.5);
  - `vm.Interrupt("timeout")`  — script exceeded `DeadlineSec`.
  Goja's `Interrupt` poisons the next instruction the VM executes;
  for purely CPU-bound infinite loops this stops within microseconds
  on a normal box.
- A `for (;;) {}` loop with **no function calls** can theoretically
  starve the Go scheduler before `Interrupt` is observed. This is
  the second reason the executor is a sibling process: in the worst
  case `server`'s supervisor kills it with `SIGKILL` after the
  deadline elapses + a 2-second grace window, the orphan runs are
  marked `executor_crashed`, and the executor is respawned. The DCC
  bus is undisturbed.

#### 3a.7.3 The in-script DSL

The JS source the user writes sees exactly these names as globals.
Examples below use the canonical scenario from goal 18.

| Symbol | Where it operates | Signature | Effect on server |
|---|---|---|---|
| `findFirstLoco()`                  | attached scope             | `() -> Vehicle`                  | none (lookup is local to the `Scope` passed in `run.start`) |
| `findByDCCAddr(addr)`              | attached scope             | `(number) -> Vehicle`            | none |
| `members()`                        | attached scope             | `() -> Vehicle[]`                | none |
| `Vehicle.setSpeed(step)`           | one vehicle                | `(0..126) -> undefined`          | `LocoService.SetSpeed` (re-checks `LocoSecurityContext.CanDriveLoco`) |
| `Vehicle.setDirection(dir)`        | one vehicle                | `('fwd'\|'rev') -> undefined`    | `LocoService.SetDirection` |
| `Vehicle.funcOn(num)`              | one vehicle, num ∈ 0..31   | `(number) -> undefined`          | `LocoService.SetFunction(..., true)` |
| `Vehicle.funcOff(num)`             | one vehicle, num ∈ 0..31   | `(number) -> undefined`          | `LocoService.SetFunction(..., false)` |
| `Vehicle.func(num, on)`            | one vehicle                | `(number, boolean) -> undefined` | `LocoService.SetFunction` (combined form) |
| `Vehicle.dccAddr`                  | one vehicle                | `number`                         | – |
| `Vehicle.isLoco`                   | one vehicle                | `boolean`                        | – |
| `Vehicle.name`                     | one vehicle                | `string`                         | – |
| `sleep(seconds)`                   | n/a                        | `(number) -> undefined`          | none; **only this run's goroutine is paused** |
| `log(msg)`                         | n/a                        | `(string) -> undefined`          | none; pushed to every owner session as `script.log` |

**Scope rules** (enforced by the bindings, not just the source):

- script attached to a Vehicle V → `findByDCCAddr(N)` throws
  `RangeError("addr_out_of_scope")` unless `N === V.dccAddr`;
  `findFirstLoco()` throws unless `V.isLoco`.
- script attached to a Train T → `members()` returns members in
  `Position` order; `findFirstLoco()` returns the first member with
  `isLoco === true`, or throws if T has no loco.
- The bindings are **server-side**, so even if the user forges a
  `Vehicle` literal in their own JS, the binding refuses to dispatch
  anything not in the resolved `Scope`.
- The server **re-validates authorization on every call**. If the
  lease that authorized the run expires mid-script, the next DSL
  call comes back with `call.result { ok:false, error:"not_authorized" }`,
  the JS binding panics with `vm.ToValue("not_authorized")`, and the
  script terminates with `runStopped reason:"error"` unless it
  caught the exception in a `try/catch`.

#### 3a.7.4 Lifecycle (per attachment, per user)

```
                            press script.run
   ┌────────┐ ───────────────────────────────────► ┌────────┐
   │ Idle   │                                       │Running │
   │        │ ◄─────────────────────────────────── │        │
   └────────┘  finished / user-stop / timeout /     └────────┘
                deadman / executor_crashed
```

- Pressing `script.run` while a run is active for the same
  `(scriptAttachmentId, userId)` returns `ack { ok:false, error:"already_running" }`.
- Pressing `script.stop` posts `run.stop { runId, reason:"user" }`
  to the executor, which calls `vm.Interrupt("user")`; the executor
  replies `run.event { kind:"finished", reason:"stopped" }` once the
  goroutine returns.
- Editing the source while a run is active does **not** interrupt
  it – the run finishes against the snapshot it loaded at
  `run.start`. The next press loads the new source. This mirrors the
  user's expectation that a script doesn't change underneath itself.
- `script.changed { id, version, kind:"deleted" }` for a script with
  an active run sends `run.stop { reason:"deleted" }` and detaches
  the button from the throttle UI.

#### 3a.7.5 Cross-device run state

The server keeps `runId` per `DriveSession`, and emits
`script.runStarted { sessionId, runId, scriptId, attachedTo:{vehicleAddr|trainId} }`
plus `script.runStopped { sessionId, runId, scriptId, reason, errorMessage? }`
to **every** session the owning user holds open. The phone UI shows
"running on desktop" with a `script.stop { runId }` button; tapping
it on the phone interrupts the script in the executor regardless of
which device started it. The script itself, of course, never moves –
it always runs in `scripts-executor`, on the server.

#### 3a.7.6 What lives where

- **Source code** – stored as-is in the `scripts` table.
  `GET /api/v1/scripts/{id}` returns it **only to the owner**;
  admins, lessees and signalmen never see it (source-privacy beats
  admin override, by design).
- **Metadata** (`id`, `name`, `icon`, `runtime`, `version`,
  `deadlineSec`, `attachedTo`) – exposed to anyone with driving
  authority on the attached vehicle / train.
- **Source-on-the-wire to the executor** – flows over the local Unix
  socket only, just for runs `server` itself is orchestrating. The
  executor process has no other network bindings (the executor
  binary refuses to start if `--executor-socket` is anything but
  loopback / Unix socket).
- **Run state** (`runId`, `startedAt`, `actorUserId`) – lives in
  `server`'s memory only. On `server` restart all in-flight runs are
  declared `executor_crashed` (because the executor was killed too)
  and the user can press play again.
