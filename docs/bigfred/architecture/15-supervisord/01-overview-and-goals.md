### §7d.1 Overview & design goals

#### Problem

BigFred already relies on **sibling processes** isolated from the main
`server` binary — most notably `scripts-executor` (§3a.7). The current
architecture sketches a hand-rolled supervisor in
`pkgs/server/executor/supervisor.go` (spawn, exponential backoff, health
flag, graceful shutdown). That approach works for one child but does not
scale cleanly when more managed processes appear (per-layout command-station
workers, dedicated pollers, optional sidecars).

The user requirement is to manage **many processes** with confidence they
are running, using **[supervisord](http://supervisord.org/)** as the
battle-tested process manager, while the Go backend remains the **single
source of truth** for *what* should run.

#### Goals

1. **Declarative process groups** — callers register programs as
   `(command, autostart, autorestart)` tuples grouped under a logical
   name (e.g. `loco`, `command-stations`).
2. **Config as code** — supervisord INI is never edited by hand at
   runtime; it is rendered from Go structs via `text/template` and written
   atomically.
3. **Hot reload on change** — after a config change, apply it with
   supervisord's built-in **`supervisorctl reread`** followed by
   **`supervisorctl update`**, which adds/removes/restarts individual
   programs without restarting the daemon. A full supervisord restart
   is used only when global sections (`[supervisord]`, socket, …)
   change — a rare case with stable XDG paths.
4. **Non-root only** — no `/etc/supervisor`, no `/var/run`, no
   privileged ports, no `user=root`. Everything lives under XDG paths
   owned by the user running `loco server`.
5. **Observable** — the service exposes program/group status so higher
   layers (`ScriptService`, WS `system.status`, admin UI) can report
   health without re-implementing process tracking.

#### Non-goals (this milestone)

- HTTP/REST endpoints for process management (internal Go API only).
- Running supervisord as a systemd user unit managed externally; `server`
  owns the daemon lifecycle.
- Multi-instance / ownership guards — `loco server` is assumed to be the
  sole instance on the host for now.
- Replacing supervisord's own autorestart for crash recovery — we rely on
  supervisord for that and only **observe** state from Go.
- Windows support (supervisord is Unix-oriented; the existing architecture
  already targets Linux/macOS for the executor socket).

#### High-level placement

```
┌─────────────────────────────────────────────────────────────┐
│  loco server (Go, non-root)                                 │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  SupervisordService                                   │  │
│  │    · desired state (groups + programs)                │  │
│  │    · render supervisord.conf (Go template)            │  │
│  │    · spawn supervisord daemon; reread/update on change│  │
│  │    · supervisorctl status / shutdown                 │  │
│  └───────────────────────────┬───────────────────────────┘  │
│                              │ owns (child process)         │
│                              ▼                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  supervisord daemon                                   │  │
│  │    group:loco                                         │  │
│  │      ├─ program:scripts-executor   autostart=true    │  │
│  │      └─ program:…                  autorestart=…    │  │
│  │    group:dcc-bus (added by §7e M4.5)                  │  │
│  │      ├─ program:dcc-bus-1-2        autostart=true    │  │
│  │      └─ program:dcc-bus-1-3        autostart=true    │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

`server` remains the DCC/throttle authority. Managed processes stay
siblings — they never import `pkgs/server/http` or `pkgs/server/ws`.

#### Why supervisord instead of extending the hand-rolled supervisor

| Aspect | Hand-rolled `executor/supervisor.go` | SupervisordService |
|---|---|---|
| Multiple processes | one-off spawn loop per child | one daemon, many `[program:…]` sections |
| Crash restart | custom backoff in Go | `autorestart=` handled by supervisord |
| Group operations | manual | `supervisorctl restart loco:*` |
| Log capture | custom pipes | built-in stdout/stderr log files |
| Operational familiarity | project-specific | widely documented tooling |

The Go layer keeps **policy** (which programs exist, when config changes)
and delegates **mechanism** (signal handling, restart timing, log rotation
basics) to supervisord.

#### Non-root path layout

All paths are derived at service construction time from
[`github.com/adrg/xdg`](https://github.com/adrg/xdg) (already idiomatic
for `$XDG_RUNTIME_DIR/loco/exec.sock` in §3a.7):

| Path | Purpose | Mode |
|---|---|---|
| `$XDG_RUNTIME_DIR/loco/supervisord/` | config dir, unix socket, pidfile | `0700` |
| `$XDG_RUNTIME_DIR/loco/supervisord/supervisord.conf` | rendered config | `0600` |
| `$XDG_RUNTIME_DIR/loco/supervisord/supervisor.sock` | `[unix_http_server]` socket | `0700` |
| `$XDG_RUNTIME_DIR/loco/supervisord/supervisord.pid` | supervisord pidfile | `0600` |
| `$XDG_CACHE_HOME/loco/supervisord/` | supervisord main log + per-program logs | `0700` |

The template also sets `user={{ .RunAsUser }}` in `[supervisord]` so
programs cannot accidentally inherit a different identity if the config
is reused elsewhere.

#### External dependency

The host (or dev container) must provide:

- `supervisord` — daemon
- `supervisorctl` — control client

Both are shipped by the Python **`supervisor`** package (`pip install
supervisor` or distro package `supervisor`). The service accepts optional
`SupervisordBin` / `SupervisorctlBin` overrides for non-`PATH` installs.
Missing binaries fail fast at `SupervisordService.Start` with a clear
error — no silent fallback to the old hand-rolled supervisor.
