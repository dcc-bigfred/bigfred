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

**M4 – Command Stations and layouts.**

11. Add `command_stations`, `layouts` and `layout_command_stations`
    tables, plus `LayoutSignalman` and `LayoutInterlocking` join
    tables. Seed exactly one **system** layout row with
    `name='default'`, `is_system=true`, `locked=false` (uniqueness
    guarded by a partial unique index `UNIQUE(is_system) WHERE is_system = TRUE`).
    Add the DB CHECK `NOT (is_system = TRUE AND locked = TRUE)` and
    the CHECK on `layout_command_stations` that refuses inserts whose
    `layout_id` points at the system row (the system layout's set is
    virtual, §3a.3 invariants).
12. Implement `CommandStationService` (admin CRUD over `centralki`)
    and `LayoutService` (CRUD, lock / unlock, attach / detach command
    stations, signalmen list, interlocking whitelist). Make
    `LayoutService.Create` require `commandStationIds` with ≥1
    entry, reject the system row from `Update` / `Delete` /
    `Lock` / `AttachCommandStation` / `DetachCommandStation` with
    the matching errors, and refuse to leave a non-system layout
    with zero attached stations.
    Move `LocoService` to a `map[commandStationID]Station` resolver
    keyed off `session.CommandStationID` (§3a.4 rule 5). Implement
    the `session.setCommandStation` WS action (§4.2 / §4.5):
    validates the picked id against `LayoutCommandStation` (or
    `command_stations` for the system layout), runs the emergency
    plan on the previous `CommandStationID` first when picking a
    different one, broadcasts `session.commandStationChanged`. Wire
    the `layout.commandStationsChanged` fan-out from
    `CommandStationService.Delete` and the attach/detach endpoints,
    so live drive sessions get their dropdowns refreshed and any
    session pinned to a deleted / detached station is auto-detached
    (CommandStationID → nil) with reason `"deleted"` /
    `"detached"`.
13. Wire **the layout picker into the login flow** (§7a.1): add
    the unauthenticated `GET /api/v1/layouts/login` endpoint
    returning non-locked rows, update `POST /api/v1/auth/login` to
    take `{ login, pin, layoutId }` and bake `layoutId` into the
    JWT, and update the WS upgrade to read `LayoutID` from the
    token (NOT from a `/layouts/{id}/join` call – that endpoint
    does not exist). On the frontend, add the layout dropdown next
    to login + PIN on `LoginPage.tsx`; the system layout is the
    default pre-selection and is rendered via the
    `layout:system_default_label` i18n key. Add an admin-only
    `/admin/layouts` page in `AppShell.tsx` for name / lock /
    attached-stations management (the system layout's row is
    read-only there).
14. Add the **command-station-picker dropdown** to the vehicle
    control view. It is populated from
    `session.opened.availableCommandStations`, fires
    `session.setCommandStation` on change, and re-renders on every
    `session.commandStationChanged` / `layout.commandStationsChanged`
    event. The throttle stays gated (slider disabled, every action
    short-circuited to a UI warning) until `CommandStationID` is
    non-nil. When the dropdown contains exactly one entry, the UI
    MAY auto-fire `setCommandStation` once; the contract is
    identical to a manual pick.
15. Wire `AuditService.Log` into every command-station mutation,
    every layout mutation (create / update / delete), every lock
    toggle (`layout.locked` / `layout.unlocked`) and every attach /
    detach (`layout.command_station_attached` /
    `layout.command_station_detached`) – see §3a.5.
16. Wire **sudo elevation** (§7a.7). Add the `sudo_elevations`
    table and the `Layout.AdminPINHash` column (NOT NULL; the
    bootstrap migration seeds the system layout with a one-shot
    random PIN, logged once). Implement
    `AuthService.Sudo` / `RevokeSudo` and the
    `LayoutService.UpdateAdminPIN` rotation path (with the "blank
    field = no change" semantic). Add the
    `POST/DELETE /api/v1/layouts/{id}/sudo` endpoints and the
    `auth.elevationChanged` WS fan-out. Wire the existing janitor
    goroutine to reap expired rows and emit
    `auth.elevationChanged`. Plumb the sudo admin source through
    `AuthService.Effective` (a flat `domain.EffectiveRoles` set —
    sudo admin grants the same authority as a permanent admin
    everywhere). Cascade the cleanup on `AuthService.Logout` and
    `LayoutService.Delete`. Add the second endpoint
    `POST/DELETE /api/v1/layouts/{id}/signalman` driven by
    `SudoService.GrantSignalman / RevokeSignalman` — the
    engineer's-cap icon writes a permanent
    `LayoutSignalman` row with `expires_at = NULL` after the
    same PIN check. Front-end: add the closed-padlock indicator
    (with live MM:SS countdown badge) and the engineer's-cap
    indicator (binary toggle, no countdown) to `AppShell.tsx`, the
    shared `<SudoPinDialog>`, and the layout admin PIN field on
    the `/admin/layouts` settings page (with the "leaving blank
    does NOT reset the PIN" helper, §7a.7.5). Ship `pl/sudo.json`
    + `en/sudo.json` and the new error codes (`sudo_invalid_pin`,
    `sudo_locked`, `sudo_layout_mismatch`,
    `layout_admin_pin_invalid`, `layout_admin_pin_unset`).

**M5 – Interlockings, takeover, radio.**

17. Add `interlockings`, `interlocking_sessions`, `takeover_requests`
    and `radio_messages` tables and services, all **filtered through
    the active layout's `LayoutInterlocking` whitelist**. Implement the
    15-second takeover state machine and the closed-vocabulary radio.
    Wire `AuditService.Log` for `session.emergency_executed` emitted by
    the dead-man's switch handler in the Hub.
18. Add the **layout dashboard** (`HomePage.tsx` – three live tables,
    §6.3c) and the **interlocking view** (`InterlockingPage.tsx` –
    occupy / leave with displacement confirm, navigation guard, radio
    panel, §6.3d). Add `layout_vehicles` table and presence tracking
    in the Hub.

**M6 – API keys + built-in MCP server.**

19. Add the `api_keys` table, `APIKeyService` (mint / verify / revoke,
    hard cap 365 days), and the corresponding REST endpoints plus a
    React screen to mint and revoke keys (showing plaintext exactly
    once). Each key is bound to the layout that was active when minted.
20. Add the `pkgs/server/mcp` package using
    `github.com/mark3labs/mcp-go`. Wire the SSE handler under `/mcp`
    behind the API-key middleware, and add a `loco server --mcp-stdio`
    subcommand for local clients (Claude Desktop / Cursor). Expose the
    curated tool surface listed in §7b.3.

**M7 – Server-side scripts (Goja, sibling `scripts-executor` process).**

21. Add the `scripts` and `script_attachments` tables,
    `ScriptService`, `ScriptSecurityContext` and the REST endpoints
    listed in §4.1. Enforce the 64 KiB source cap, the
    Vehicle-XOR-Train attachment invariant and the
    `DeadlineSec ∈ [1, 600]` range at both the DB and service
    layers. Wire `AuditService.Log` into every `Script` and
    `ScriptAttachment` mutation (create / update / delete /
    attach / detach).
22. Build `pkgs/server/scripts/runtime.go`: a `Runtime` struct that
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
23. Build `pkgs/server/executor/`: the length-prefixed JSON codec,
    the `Client` used in `server`, the `Server` used in
    `scripts-executor`, and the `Supervisor` (exec the child,
    exponential backoff, health pings, in-flight run accounting).
    The supervisor must surface `system.status { scriptsExecutor }`
    over WS and the "Scripts unavailable" banner when it gives up.
24. Add the `pkgs/scripts-executor/` package with `main.go` and a
    `loco scripts-executor` cobra command. The binary is built from
    the same Go module as `loco server`; CI builds both and the
    Makefile gets a `make scripts-executor` target.
25. Ship the **Scripts page** (`web/src/pages/ScriptsPage.tsx`):
    list, Monaco editor with `language="javascript"`, icon picker,
    attachment management, deadline slider. Add
    `ScriptButtons.tsx` and `ScriptConsole.tsx` to the vehicle and
    train control views so attached scripts render next to `F0`–
    `F32` and per-run logs surface inline.
26. Wire the new WS events (`script.run`, `script.stop`,
    `script.log`, `script.runStarted`, `script.runStopped`,
    `script.changed`) and the dead-man's switch integration
    (`ScriptService.StopAllForUser` invoked from the Hub before
    `SetSpeed(0)` fan-out). Integration test: kill the executor
    mid-run with `SIGKILL` and assert every in-flight `runId`
    receives `script.runStopped { reason:"executor_crashed" }`
    within the supervisor's first detection cycle, while throttle
    commands on unrelated vehicles continue to round-trip.

**M8 – Polish.**

27. Redis (cache + Pub/Sub for multi-instance fan-out), background
    poller upgrades, optimistic UI tweaks, accessibility audit on the
    MUI screens.
