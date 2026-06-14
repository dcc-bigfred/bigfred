## §7d Process Supervisor (Supervisord)

This section describes a **non-root supervisord integration** inside
`pkgs/bigfred/server`. The Go backend owns declarative process groups, renders
supervisord configuration from embedded Go templates, and applies changes
via supervisord's built-in hot reload (`supervisorctl reread` +
`supervisorctl update`).

The component replaces the ad-hoc `exec.Command` child-process supervisor
described in [§7 cross-cutting #12](../09-cross-cutting.md) with a
general-purpose layer that can manage **multiple** sibling processes
(`scripts-executor` today; command-station bridges, pollers, MCP workers,
… tomorrow) while keeping the same process-isolation guarantees from
[§2 High-Level Architecture](../03-high-level-architecture.md).

## Subsections

1. [Overview & design goals](./01-overview-and-goals.md) — why supervisord,
   non-root constraints, ownership model
2. [Service API & configuration model](./02-service-api-and-config.md) —
   Go structs, `SupervisordService`, Go templates, supervisord INI layout
3. [Lifecycle, reload & health](./03-lifecycle-and-health.md) — daemon
   start/stop, config regeneration, hot reload, status polling
4. [Integration with the server](./04-integration.md) — wiring in
   `cli/root.go`, `scripts-executor` migration, `system.status` events

## Quick reference

| Concern | Decision |
|---|---|
| Runs as | the same Unix user as `loco server` (never root) |
| Config & runtime paths | XDG Base Directory paths under `$XDG_RUNTIME_DIR/loco/` and `$XDG_CACHE_HOME/loco/` |
| Config authoring | embedded `text/template` → atomic write to `supervisord.conf` |
| Apply config changes | regenerate file, then **`supervisorctl reread` + `update`** (built-in hot reload); full daemon restart only when global sections change |
| Process declaration | `(command, autostart, autorestart)` inside a named **process group**; `command` is wrapped as `/bin/bash -c '…'` |
| Single instance | one `loco server` per machine — no multi-instance ownership checks |
| External dependency | `supervisord` + `supervisorctl` binaries on `PATH` (Python `supervisor` package) |
