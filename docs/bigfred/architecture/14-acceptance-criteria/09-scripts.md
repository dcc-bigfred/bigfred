### 10.7 Scripts (M7)

- A user can create a script via the **Scripts** tab (Monaco editor
  set to JavaScript), paste the canonical example
  (`findFirstLoco`, `setSpeed`, `findByDCCAddr(815)`, `funcOn`,
  `sleep`, `funcOff`), pick an icon from the function-icon catalogue,
  set a deadline, save it, and attach it to either a single vehicle
  or a single train they own. Attaching to a vehicle they do **not**
  own returns `403`.
- An attachment request with both `vehicleAddr` and `trainId`, or
  with neither, is rejected with `422` (DB CHECK + service).
- `make server` launches `loco server`, which **spawns
  `loco scripts-executor` as a child process** and dials its Unix
  socket. `system.status` over WS reports `scriptsExecutor: "healthy"`
  within 2 s of boot. `ps -ef` shows both processes; killing
  `scripts-executor` is observable as a respawn within ≤ 1 s.
- The attached script appears as an **additional icon button on the
  throttle view**, rendered in the same row as `F0`–`F31`. Pressing
  it emits WS `script.run`; the **server's** logs show the dispatch
  to the executor, and the **executor's** logs show
  `goja.New() + RunString(<source>)`. No JavaScript ever runs in
  the browser; browser devtools show no Web Worker.
- During a `sleep(5)` inside the script, the **server stays fully
  responsive**: the speed slider on an unrelated vehicle drives it
  normally, other throttle commands round-trip in milliseconds,
  and the dead-man's switch heartbeat keeps flowing. Verifiable by
  launching a `for (let i=0;i<60;i++) sleep(1);` script and
  observing that manual throttling on a different tab is
  unaffected.
- Every DSL call routes through the **same `LocoService` /
  `TrainService` and the same security policies as a manual
  throttle press**: a script attempting to drive a non-attached DCC
  address either fails inside the binding (`findByDCCAddr` throws
  `RangeError("addr_out_of_scope")`) or, if a lease expires
  mid-run, fails on the next `setSpeed` with
  `Error("not_authorized")` returned via `call.result`. The
  binding rethrows it as a JS exception the user can `try/catch`.
- A lessee of a vehicle that has a script attached **sees the script
  button and can run it**, but `GET /api/v1/scripts/{id}` returns
  `403` for them (source stays private). The executor receives the
  source from `server` only for runs `server` itself orchestrated.
- Editing the script source from the Scripts tab bumps its
  `version` and emits `script.changed { id, version, kind:"source" }`.
  An in-flight run is **not** interrupted (snapshot semantics);
  the next press loads the new source.
- Pressing **Stop** on the script button while it is running emits
  WS `script.stop { runId }`; server posts `run.stop { reason:"user" }`
  to the executor; `vm.Interrupt("user")` fires; the goroutine
  unwinds within ≤ 100 ms on a normal box and
  `script.runStopped { reason:"stopped" }` reaches all of the
  owner's sessions.
- A run that hits `DeadlineSec` produces
  `script.runStopped { reason:"timeout" }`; `vm.Interrupt("timeout")`
  is observable in the executor's logs. A run with
  `DeadlineSec > 600` is rejected on save with `422`.
- The dead-man's switch interrupts every active run of the dying
  user **before** the throttle's `SetSpeed(0)` fan-out. Pulling
  the network on a tab running `setSpeed(50); sleep(60);
  setSpeed(60);` results in: (a) the sleep is interrupted with
  `vm.Interrupt("deadman")` on session-loss tick, (b) the second
  `setSpeed` line never executes, (c)
  `script.runStopped { reason:"deadman" }` is broadcast to
  surviving sessions, (d) the audit row
  `session.emergency_executed` carries `terminated_scripts: 1`.
- The same script can be attached to several vehicles (e.g. a
  "yard-shunt" script wired to multiple shunters); each attachment
  is one independent row. Running the script on vehicle A spawns
  one goroutine + one Goja VM in the executor; running it
  simultaneously on vehicle B spawns a second independent VM; the
  two do not share state.
- Killing the executor process with `SIGKILL` mid-run results in
  `script.runStopped { reason:"executor_crashed" }` on every
  in-flight `runId` within the supervisor's first detection cycle
  (≤ 2 s), the supervisor restarts the child with exponential
  backoff, and **the DCC bus is undisturbed** – manual throttling
  on another vehicle continues without dropping a single command.
  After 3 consecutive crashes inside 60 s the supervisor stops
  retrying and the UI shows "Scripts unavailable, contact admin"
  while throttling remains fully functional.
- An attempted edit on a script owned by another user returns
  `403`; attempting to attach **someone else's** script returns
  `403` even if the actor is admin (source privacy beats admin
  override – admins see audit metadata but never the JavaScript
  source).
- Script source size cap: pushing `source.length > 65536` returns
  `422` from `POST` / `PUT /api/v1/scripts`. The Monaco editor
  enforces the same limit and shows a warning long before that
  size is hit.
