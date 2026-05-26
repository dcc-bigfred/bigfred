### §7d.4 Integration with the server

#### Wiring in `cli/root.go`

```go
// Pseudocode — exact order matters for shutdown hooks.

supSvc, err := service.NewSupervisordService(service.SupervisordConfig{
    InitialState: defaultProcessState(selfPath, socketPath),
})
if err != nil { return err }

if err := supSvc.Start(ctx); err != nil {
    return fmt.Errorf("supervisord: %w", err)
}
go supSvc.RunHealthLoop(ctx, 5*time.Second)

// ScriptService receives supSvc for status + shutdown ordering
scriptSvc := service.NewScriptService(..., supSvc)

// … HTTP server, hub, etc.

// Shutdown (after SIGINT/SIGTERM):
scriptSvc.Shutdown(ctx)   // RPC drain, run.stop
supSvc.Stop(ctx)          // supervisorctl shutdown
srv.Shutdown(shutdownCtx)
```

`SupervisordService` is constructed **before** services that depend on
managed processes. Shutdown reverses the order.

#### Replacing `executor/supervisor.go`

The hand-rolled supervisor planned in
[`04-repository-layout.md`](../04-repository-layout.md) (`pkgs/server/executor/supervisor.go`)
is **superseded** by this component:

| Before (§7 #12) | After (§7d) |
|---|---|
| `exec.Command("loco", "scripts-executor", …)` in Go | `[program:scripts-executor]` in rendered config |
| custom exponential backoff | supervisord `autorestart=true` + daemon respawn loop |
| `executor.healthy` flag in Go | `SupervisordService.AllStatus` + RPC dial |
| manual SIGKILL after 5 s | `stopwaitsecs` + `ScriptService.Shutdown` RPC |

Keep `pkgs/server/executor/` for the RPC client/server (`client.go`,
`server.go`, `messages.go`, `codec.go`). Remove `supervisor.go` from the
plan — its responsibilities move to `SupervisordService`.

Cross-cutting doc [§7 #12](../09-cross-cutting.md) should be updated
after implementation to reference §7d instead of the hand-rolled spawn
loop.

#### Default process catalogue (M7 scripts milestone)

| Group | Program | Autostart | Autorestart | Notes |
|---|---|---|---|---|
| `loco` | `scripts-executor` | true | true | same flags as today (`--executor-socket`) |

Future rows (not M7):

| Group | Program | Autostart | Autorestart | Notes |
|---|---|---|---|---|
| `command-stations` | `cs-<id>` | per CS config | true | one worker per active command station |
| `loco` | `mcp-stdio-bridge` | false | false | on-demand, admin-triggered |

Adding a row is one `UpsertProgram` call — no supervisord knowledge in
the caller beyond `ProgramSpec`.

#### `system.status` WebSocket event

Extend the existing status payload:

```json
{
  "type": "system.status",
  "payload": {
    "scriptsExecutor": "healthy",
    "supervisord": {
      "daemon": "running",
      "groups": {
        "loco": {
          "scripts-executor": {
            "status": "RUNNING",
            "pid": 12345,
            "uptimeSec": 3600
          }
        }
      }
    }
  }
}
```

Mapping rules:

- `scriptsExecutor: "healthy"` when program `RUNNING` **and** executor RPC
  socket accepts a connection (preserves current semantics).
- `scriptsExecutor: "failed"` when `FATAL` or daemon respawn policy gives
  up (same banner as §7 #12).
- `supervisord.daemon: "degraded"` when daemon respawn backoff exhausted.

Only **diffs** are broadcast on the health ticker to avoid WS noise.

#### Who calls `Apply`

| Caller | When |
|---|---|
| `cli/root.go` | initial `Start` (via `InitialState`) |
| `ScriptService` | never — executor flags are fixed at boot for M7 |
| `CommandStationService` (future) | CS added/removed/connection string changes |
| Admin API (future) | manual restart policies |

M7 does not expose HTTP for process management. Dynamic CS workers are a
later milestone.

#### Repository layout additions

Update [`04-repository-layout.md`](../04-repository-layout.md) when
implementing:

```
pkgs/server/supervisord/
    templates/supervisord.conf.tmpl
    config.go
    render.go
    ctl.go
    daemon.go
    render_test.go              # golden-file template tests

pkgs/server/service/supervisord.go
pkgs/server/service/supervisord_test.go   # fake supervisorctl in tests
```

Remove from the planned tree:

```
pkgs/server/executor/supervisor.go   # deleted — replaced by §7d
```

#### Makefile / dev environment

Add to [`12-makefile.md`](../12-makefile.md) when implementing:

```makefile
# Verify supervisord is available in dev/CI
check-supervisor:
	@command -v supervisord >/dev/null || (echo "install: pip install supervisor" && exit 1)
```

Document in [`02-tech-stack.md`](../02-tech-stack.md):

| Component | Choice | Role |
|---|---|---|
| Process supervisor | **supervisord** (Python `supervisor` package) | Manages sibling processes non-root; configured via Go templates |

#### Acceptance criteria (draft)

To be copied into `14-acceptance-criteria/` when the milestone is scheduled:

1. `loco server` as an unprivileged user creates XDG paths, renders config,
   and starts supervisord without root.
2. `supervisorctl -c $XDG_RUNTIME_DIR/loco/supervisord/supervisord.conf status`
   shows `scripts-executor` in `RUNNING` within 10 s of boot.
3. Changing desired state via `Apply` (add/remove program, toggle
   autostart) rewrites config and applies via `reread` + `update` **without**
   changing the supervisord daemon PID (hot reload).
4. Changing a program's `command` restarts only that program; siblings
   stay `RUNNING`.
5. `kill -9` on a managed program with `autorestart=true` returns it to
   `RUNNING` without restarting `loco server`.
6. `SIGTERM` to `loco server` stops supervisord cleanly; no orphaned
   `scripts-executor` remains (`ps` check).
7. Template unit tests cover at least: empty groups, single group / single
   program, multiple groups, `autostart=false`, `autorestart=false`,
   `shellQuote` escaping for `/bin/bash -c`.
