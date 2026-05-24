## 7. Cross-Cutting Concerns

1. **Single binary serving the frontend too.** In production, build React
   (`vite build` → `web/dist`) and serve the static files from the same
   Go process. In development, Vite runs on `:5173` and proxies `/api`
   to `:8080`, which avoids CORS conflicts.
2. **WebSocket backpressure.** `dispatch` uses
   `select { case c.send <- ev: default: drop }` so a slow client never
   blocks the others.
3. **Client-side reconnect** with exponential backoff plus automatic
   resubscription on reconnect (the active subscriptions live in the
   store).
4. **Action idempotency.** Every action from the client carries an `id`;
   the server replies with `ack`. This is essential for debugging and for
   optimistic UI updates.
5. **Long operations (e.g. read CV)** must not block the WebSocket
   broadcast loop. Either expose them as REST with a timeout, or as a
   WebSocket request/response with an `id` and a final `ack` carrying the
   result. REST is the simpler default.
6. **Type sync.** Use `tygo` (or a small custom script) to generate
   TypeScript types from Go structs that define the WS protocol. Keeping
   it in sync by hand will degrade quickly.
7. **Authentication & authorization.** See §11 – login + PIN with strong
   hashing (argon2id), session tokens carried as an `HttpOnly` cookie
   for REST and as a `?token=` query parameter for the WebSocket
   upgrade. Permission checks live in dedicated middleware
   (`RequireRole`, `RequireVehicleAccess`, …) so handlers stay thin.
8. **SQLite migrations.** Use REL's own migration tool
   (`github.com/go-rel/rel/cmd/rel`); ship migration packages embedded
   via `embed.FS` and run pending ones on startup.
9. **Time-based grants cleanup.** Temporary roles, leases and takeover
   requests all carry an `expires_at`. A single janitor goroutine wakes
   every 30 s, marks expired rows and emits the corresponding events
   (`role.expired`, `lease.expired`) so the UI updates without a poll.
   Lease expirations also call `AuditService.Log` with action
   `vehicle.lease_expired` / `train.lease_expired` (§3a.5).
10. **Audit log discipline.** Every state-changing service call lists
    in §3a.5 ends with an `AuditService.Log(ctx, …)` invocation. The
    audit row carries the actor's **denormalized login** and the
    object's **denormalized name** so renames or deletions never
    rewrite history. The log is admin-only on read, with no UPDATE or
    DELETE endpoints at all.
11. **Internationalization.** Frontend rendering is locale-aware,
    backend is language-neutral. Stable codes on the wire
    (`ApiError.code`, `RadioPhrase`, `FunctionIcon`, `AuditAction`,
    …) are mapped to human strings by `react-i18next` from JSON
    catalogues bundled into `web/dist`. Persisted denormalized
    strings (audit `user_name` / `object_name`, `RadioMessage.Note`)
    are rendered verbatim regardless of active locale. Full
    specification, including the "what is translated vs. rendered
    verbatim" contract, lives in [§7c i18n](./09a-i18n.md).
12. **`scripts-executor` supervision.** `server` spawns the executor
    child process at boot (`exec.Command("loco", "scripts-executor",
    "--executor-socket", socketPath)`) and supervises it with
    exponential backoff (1 s, 2 s, 4 s, …, capped at 30 s). On
    successful dial of the RPC socket the supervisor flips the
    `executor.healthy` flag and broadcasts `system.status { scriptsExecutor:"healthy" }`.
    If the child exits unexpectedly, every in-flight `runId` is
    marked `executor_crashed`, the owning sessions receive
    `script.runStopped { reason:"executor_crashed" }`, and the
    supervisor schedules a respawn. After **3 consecutive restarts
    inside 60 s** the supervisor stops respawning and emits
    `system.status { scriptsExecutor:"failed", reason }` so the UI
    can show a "Scripts unavailable, contact admin" banner; the
    throttle stays fully functional. The supervisor also handles
    graceful shutdown: on `server` SIGTERM it sends
    `executor.shutdown` over RPC, waits up to 5 s for the executor
    to drain in-flight runs (each run gets a `run.stop { reason:"executor_shutdown" }`),
    then `SIGKILL` if needed.
