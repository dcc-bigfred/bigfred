### §7d.3 Lifecycle, reload & health

#### Boot sequence (`SupervisordService.Start`)

```
server main
  │
  ├─ NewSupervisordService(cfg)
  ├─ register initial DesiredState (groups + programs)
  │
  └─ SupervisordService.Start(ctx)
        │
        ├─ MkdirAll ConfigDir, LogDir (0700)
        ├─ verify supervisord + supervisorctl binaries exist
        ├─ render → supervisord.conf (atomic write)
        │
        ├─ if pidfile exists AND process alive → skip spawn
        │     else → exec supervisord -c <ConfigPath>
        │
        ├─ if config on disk differs from desired → Apply (reread+update)
        ├─ poll supervisorctl status until daemon responds (≤ 10 s)
        └─ optional: start background health ticker (§7d.3.3)
```

`loco server` is assumed to be the **only instance** on the host — no
multi-instance ownership checks. On server restart, if supervisord is
still running from a previous run, `Start` reuses it and hot-reloads
program changes when needed.

#### Applying configuration changes (`Apply`)

Supervisord provides a built-in hot reload: **`supervisorctl reread`**
re-parses the config file; **`supervisorctl update`** applies diffs to
running programs (start new, stop removed, restart changed) **without**
restarting the supervisord daemon.

```
Apply(newState)
  │
  ├─ lock mutex
  ├─ validate newState
  ├─ render → supervisord.conf (atomic write)
  │
  ├─ if supervisord not running
  │     └─ spawn supervisord -c … → wait healthy → unlock → return
  │
  ├─ compare global-section fingerprint (everything except [group:…] / [program:…])
  │     │
  │     ├─ global sections changed → full daemon restart (§7d.3.1)
  │     │
  │     └─ only program/group sections changed → hot reload:
  │           ├─ supervisorctl reread
  │           ├─ supervisorctl update
  │           └─ poll status until quiescent (≤ 30 s)
  │
  └─ on failure → restore .prev config + one retry → return error
```

**Hot reload** is the default path. Typical `Apply` calls (add/remove
program, change `command` / `autostart` / `autorestart`) touch only
`[group:…]` and `[program:…]` sections — supervisord picks them up
via `reread` + `update` while other programs keep running.

**Full daemon restart** is the fallback when global sections change
(socket path, logfile, pidfile, `user=`, …). With stable XDG paths set
at service construction this should be rare (essentially first boot only).

##### Full daemon restart (fallback)

```
        ├─ supervisorctl shutdown          (graceful: SIGTERM to children, then daemon)
        ├─ wait pidfile gone               (timeout 30 s)
        ├─ if timeout → kill supervisord PID (SIGKILL) + remove stale pidfile
        ├─ spawn supervisord -c …
        └─ poll supervisorctl status until all autostart programs ∈ {RUNNING, STARTING}
              or hard failure (≤ 30 s)
```

When `update` restarts a changed program, supervisord sends SIGTERM to
the old process — `scripts-executor` runs are declared
`executor_crashed` only if the program is **removed** or its `command`
changes (supervisord treats that as a replace). Tweaking unrelated
programs does not disturb running siblings.

#### Shutdown (`Stop`)

Called from `cli/root.go` on SIGINT/SIGTERM **before** the HTTP server
drain:

```
Stop(ctx)
  ├─ stop health ticker
  ├─ supervisorctl shutdown
  ├─ wait pidfile gone (timeout 15 s)
  └─ cancel context passed to supervisord child waiter
```

Programs with long shutdown hooks use `stopwaitsecs` (default 10). The
executor receives `run.stop { reason:"executor_shutdown" }` via RPC before
supervisord sends SIGTERM — same ordering as §7 #12, but triggered from
`ScriptService` shutdown hook rather than a custom kill loop.

#### Health observation

##### Polling goroutine

`SupervisordService.RunHealthLoop(ctx, interval)` (default 5 s):

1. `supervisorctl status`
2. Diff against previous snapshot
3. Emit in-process callbacks / channel for subscribers

Subscribers:

- `ScriptService` — mark executor RPC channel unhealthy when
  `scripts-executor` ∉ RUNNING
- WS hub — broadcast `system.status` patches (see §7d.4)

##### Status interpretation

| supervisord state | Meaning | Action in Go |
|---|---|---|
| `RUNNING` | healthy | expose as healthy |
| `STARTING` | within `startsecs` | treat as transitional |
| `BACKOFF` | crash loop | log warning; supervisord retries if `autorestart=true` |
| `STOPPED` | not running | expected when `autostart=false` |
| `EXITED` | clean exit | expected when `autorestart=false` |
| `FATAL` | gave up restarting | surface error; optional `system.status` alert |
| daemon unreachable | supervisord died | attempt daemon respawn (§7d.3.2) |

##### Daemon respawn (supervisord crash)

If the health loop cannot reach the unix socket but `loco server` is
still running:

1. Log `supervisord daemon unreachable`.
2. Remove stale pidfile if PID is dead.
3. Re-run `supervisord -c ConfigPath` **without** rewriting config.
4. Exponential backoff on repeated failures (1 s → 2 s → … → 30 s cap).
5. After **3 consecutive failures within 60 s**, stop respawning and set
   `supervisord.degraded=true` on `system.status` (mirrors §7 #12 executor
   policy).

This is separate from `Apply` — it covers unexpected daemon death, not
 intentional config reload.

#### Process liveness guarantees

| Layer | Guarantee |
|---|---|
| supervisord | restarts programs with `autorestart=true` on crash |
| SupervisordService | hot-reloads config via reread+update; respawns daemon on crash |
| ScriptService / others | RPC-level health (executor socket dial) in addition to process status |

A program can be `RUNNING` but not yet accepting RPC (slow boot). Keep
the existing executor socket dial probe; supervisord status alone is
necessary but not sufficient for `scripts-executor`.

#### Concurrency & idempotency

- `Apply` with identical desired state (deep-equal) is a no-op — no reload.
- Hash comparison of rendered config skips reload when content unchanged.
- Concurrent `UpsertProgram` calls serialize on the service mutex.

#### Logging

- supervisord main log: `$XDG_CACHE_HOME/loco/supervisord/supervisord.log`
- per-program: `$XDG_CACHE_HOME/loco/supervisord/<name>.stdout.log` /
  `.stderr.log`
- Go service logs structured events: `supervisord.apply`, `supervisord.reread`,
  `supervisord.update`, `supervisord.daemon_restart`,
  `supervisord.program_status_change` with `program`, `group`, `old`,
  `new` fields.

#### Failure modes

| Failure | Behaviour |
|---|---|
| `supervisord` binary missing | `Start` error; server refuses boot (fail-fast) |
| render validation error | no config write; running daemon untouched |
| reload timeout | rollback `.prev`, return `ErrSupervisordReload` |
| program FATAL | logged + `system.status`; no automatic config rewrite |
| disk full on write | atomic write fails; previous config remains |
