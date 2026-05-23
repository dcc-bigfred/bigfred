## 9. Delivery Order

Implemented in milestones; each milestone is independently shippable.

**M1 – Real-time throttle (no users).**

1. Add the new `pkgs/server` package with `chi` + `coder/websocket` and a
   single `/api/v1/ws` endpoint that echoes messages. Build it as a third
   binary next to `loco` and `rb`.
2. Expose `LocoService` as a thin wrapper over the existing `app.LocoApp`
   (reuse, do not rewrite).
3. Bring up the Vite + React + TS frontend with a `useSocket` hook and a
   single speed slider, just to validate the full loop:
   UI → WS → `Station.SetSpeed` → poller → broadcast → UI.

**M2 – Persistence (REL) + users + auth.**

4. Wire REL with the SQLite3 adapter, generate the initial migration set
   (`users`, `vehicles`, `dcc_address_ranges`, `temporary_roles`).
5. Implement `AuthService` (login + PIN, argon2id hashing,
   rate-limiting in Redis) and the JWT/cookie middleware.
6. Implement `UserService` (CRUD, role changes, temporary roles, DCC
   pool assignment) behind `RequireRole(admin)`.

**M3 – Ownership, leases, trains, functions & templates + audit log.**

7. Add `vehicles`, `trains`, `vehicle_leases`, `train_leases`
   tables and the corresponding services. Plug `RequireVehicleDrive` /
   `RequireVehicleEdit` middleware into the existing throttle endpoints.
   Implement `TrainService.SetSpeed` (lock-step fan-out across
   `TrainMember`s, `Reversed`-flip, per-member ack) and the matching
   `train.setSpeed` / `train.subscribe` WS actions (§4.2). Ship
   `TrainControlPage.tsx` that **reuses `ThrottleSlider.tsx`
   unchanged** and renders per-member function/script rows below the
   shared slider.
8. Add `vehicle_functions`, `vehicle_templates` and
   `template_functions` tables, `FunctionService` (with
   `EnsureDetached` copy-on-write), `TemplateService`,
   `FunctionSecurityContext` and `TemplateSecurityContext`. Ship the
   closed `FunctionIcon` catalogue plus matching SVG assets in the
   frontend. Wire the `vehicle.functionsChanged` WS event.
9. Add the `audit_log_entries` table, `AuditService` (append-only
   writer + filterable reader) and `AuditSecurityContext`. Wire
   `AuditService.Log` into every vehicle/train/lease/function/template
   mutation listed in §3a.5. Janitor goroutine emits
   `vehicle.lease_expired` / `train.lease_expired`.
10. Add the lease/train UI screens in React (MUI dialogs + tables),
    the function-list editor with an icon picker, the template manager,
    and an admin-only "Activity" screen reading `GET /api/v1/audit-log`.

**M4 – Layouts and parties.**

11. Add `layouts` and `parties` tables, plus `PartySignalman` and
    `PartyInterlocking` join tables. Seed exactly one `default` party
    with **`layout_id = NULL`** (the only row in the system allowed to
    have a null layout — enforced by a DB CHECK; see §3a.3
    invariants).
12. Implement `LayoutService` (admin CRUD over `makiety`) and
    `PartyService` (CRUD, join/leave, signalmen list, interlocking
    whitelist). Move `LocoService` to a `map[layoutID]Station` resolver
    keyed off `session.LayoutID` (§3a.4 rule 3). Implement the
    `session.setLayout` WS action (§4.2) including the controlled
    context-switch semantics that runs the emergency plan on the
    previous layout. Add the post-login *party list* screen with an
    admin-only settings icon next to each row, the
    `layoutPickedPerSession` badge on `default`, and the
    **layout-picker dropdown** in the vehicle control view that
    appears whenever `session.opened.layoutPickedPerSession === true`.
    Wire `AuditService.Log` into every layout/party mutation (§3a.5).

**M5 – Interlockings, takeover, radio.**

13. Add `interlockings`, `interlocking_sessions`, `takeover_requests`
    and `radio_messages` tables and services, all **filtered through
    the active party's `PartyInterlocking` whitelist**. Implement the
    15-second takeover state machine and the closed-vocabulary radio.
    Wire `AuditService.Log` for `session.emergency_executed` emitted by
    the dead-man's switch handler in the Hub.
14. Add the interlocking / signalman UI (occupy box, take over vehicle,
    radio dialog with preset phrases).

**M6 – API keys + built-in MCP server.**

15. Add the `api_keys` table, `APIKeyService` (mint / verify / revoke,
    hard cap 365 days), and the corresponding REST endpoints plus a
    React screen to mint and revoke keys (showing plaintext exactly
    once). Each key is bound to the party that was active when minted.
16. Add the `pkgs/server/mcp` package using
    `github.com/mark3labs/mcp-go`. Wire the SSE handler under `/mcp`
    behind the API-key middleware, and add a `loco server --mcp-stdio`
    subcommand for local clients (Claude Desktop / Cursor). Expose the
    curated tool surface listed in §7b.3.

**M7 – Server-side scripts (Goja, sibling `scripts-executor` process).**

17. Add the `scripts` and `script_attachments` tables,
    `ScriptService`, `ScriptSecurityContext` and the REST endpoints
    listed in §4.1. Enforce the 64 KiB source cap, the
    Vehicle-XOR-Train attachment invariant and the
    `DeadlineSec ∈ [1, 600]` range at both the DB and service
    layers. Wire `AuditService.Log` into every `Script` and
    `ScriptAttachment` mutation (create / update / delete /
    attach / detach).
18. Build `pkgs/server/scripts/runtime.go`: a `Runtime` struct that
    embeds `*goja.Runtime`, wires `findFirstLoco`, `findByDCCAddr`,
    `members`, `sleep`, `log` and the `Vehicle` helper via
    `vm.Set` + `UncapFieldNameMapper`, and exposes
    `Run(ctx context.Context, src string) error` that arms
    `vm.Interrupt` on context cancellation and timeout. Cover with
    `runtime_test.go` running the canonical scenario
    (`findFirstLoco`, `setSpeed`, `findByDCCAddr(815)`, `funcOn`,
    `sleep`, `funcOff`) against a stubbed `LocoService` and
    asserting the exact sequence of `SetSpeed` / `SetFunction`
    calls.
19. Build `pkgs/server/executor/`: the length-prefixed JSON codec,
    the `Client` used in `server`, the `Server` used in
    `scripts-executor`, and the `Supervisor` (exec the child,
    exponential backoff, health pings, in-flight run accounting).
    The supervisor must surface `system.status { scriptsExecutor }`
    over WS and the "Scripts unavailable" banner when it gives up.
20. Add the `pkgs/scripts-executor/` package with `main.go` and a
    `loco scripts-executor` cobra command. The binary is built from
    the same Go module as `loco server`; CI builds both and the
    Makefile gets a `make scripts-executor` target.
21. Ship the **Scripts page** (`web/src/pages/ScriptsPage.tsx`):
    list, Monaco editor with `language="javascript"`, icon picker,
    attachment management, deadline slider. Add
    `ScriptButtons.tsx` and `ScriptConsole.tsx` to the vehicle and
    train control views so attached scripts render next to `F0`–
    `F32` and per-run logs surface inline.
22. Wire the new WS events (`script.run`, `script.stop`,
    `script.log`, `script.runStarted`, `script.runStopped`,
    `script.changed`) and the dead-man's switch integration
    (`ScriptService.StopAllForUser` invoked from the Hub before
    `SetSpeed(0)` fan-out). Integration test: kill the executor
    mid-run with `SIGKILL` and assert every in-flight `runId`
    receives `script.runStopped { reason:"executor_crashed" }`
    within the supervisor's first detection cycle, while throttle
    commands on unrelated vehicles continue to round-trip.

**M8 – Polish.**

20. Redis (cache + Pub/Sub for multi-instance fan-out), background
    poller upgrades, optimistic UI tweaks, accessibility audit on the
    MUI screens.
