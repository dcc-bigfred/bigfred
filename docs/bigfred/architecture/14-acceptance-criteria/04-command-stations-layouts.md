### 10.2b Command Stations and layouts (M4)

#### Bootstrap and the system layout

- A fresh installation seeds **exactly one** layout row with
  `name = "default"`, `is_system = true`, `locked = false`. A partial
  unique index `UNIQUE(is_system) WHERE is_system = TRUE` makes this
  uniqueness DB-enforced.
- The UI renders the system layout as **"Domyślna (warsztat)"** in
  Polish and **"Default (workshop)"** in English, via the i18n key
  `layout:system_default_label`. The stored `Name` field is a stable
  marker and is **not** rendered to users.
- The system layout cannot be deleted: `DELETE /api/v1/layouts/{id}`
  returns `422 default_layout_undeletable` when targeting the system
  row. Trying to rename it via `PUT /api/v1/layouts/{id}` returns
  `422 default_layout_immutable`. Trying to lock it via
  `POST /api/v1/layouts/{id}/lock` returns
  `422 default_layout_cannot_be_locked`. A DB CHECK
  `NOT (is_system = TRUE AND locked = TRUE)` is the last line of
  defence.
- The system layout's set of attached command stations is **virtual**:
  no `layout_command_stations` row ever exists for it (the DB CHECK on
  that join table refuses inserts pointing at the system row), and
  `GET /api/v1/layouts/{id}/command-stations` for the system layout
  synthesises the response from the live `command_stations` catalogue.
  `POST` and `DELETE` on that subresource return `422
  default_layout_command_stations_immutable`.

#### Non-system layout CRUD

- Only admins can create, edit, lock, unlock or delete layouts; only
  admins can attach or detach command stations to a layout. A
  non-admin user calling any of these endpoints gets `403`.
- `POST /api/v1/layouts { name, commandStationIds:[…] }` requires a
  non-empty `commandStationIds` array; a request with an empty array
  is rejected with `422 layout_needs_at_least_one_command_station`.
  Every id in the array must resolve to a `command_stations` row;
  unknown ids return `404 command_station_not_found`.
- `DELETE /api/v1/layouts/{id}` is rejected with `409 layout_in_use`
  if any live drive session is still pinned to the layout (admins
  must wait for them to close, or coordinate manually). It does
  **not** kick sessions.
- Detaching the last command station from a non-system layout is
  rejected with `422 layout_needs_at_least_one_command_station`. The
  admin must attach a replacement first or delete the layout
  outright.
- Only admins can create, edit or delete `command_stations`. A
  non-admin user calling `POST /api/v1/command-stations` gets `403`.
  `DELETE /api/v1/command-stations/{id}` is rejected with `409
  layout_needs_at_least_one_command_station` if removing the row
  would leave any non-system layout with zero attached stations.
  Otherwise the deletion cascades: every `LayoutCommandStation` row
  pointing at it disappears (also de-listing it from the system
  layout's virtual view), every live drive session pinned to it has
  its `CommandStationID` set to `nil`, and a
  `session.commandStationChanged { commandStationId: null,
  reason:"deleted" }` is broadcast to every affected session.

#### Login flow

- The login screen renders three inputs side by side: `login`, `PIN`,
  and a **layout dropdown**. The dropdown is populated by
  `GET /api/v1/layouts/login`, called **unauthenticated** by the
  frontend before the form submit. That endpoint returns only the
  rows with `Locked = false`, in a minimal shape:
  `[{ id, name, isSystem }]`. The system layout's row is always
  included (it cannot be locked) and the UI replaces its `name` with
  the i18n key `layout:system_default_label`.
- `POST /api/v1/auth/login { login, pin, layoutId }` runs in this
  order: verify credentials (`401 invalid_credentials` on mismatch),
  look up the layout (`422 layout_not_found` on a stale id), reject
  `Layout.Locked == true` with `422 layout_locked`. On success the
  JWT is issued with `{ userId, layoutId }` baked in.
- A hand-crafted login request pointing at a locked layout is
  rejected by the endpoint with `422 layout_locked` (independent of
  the dropdown filtering).
- The frontend pre-selects the system layout on the dropdown on first
  paint, so a user who never touches the selector lands in the system
  layout.

#### Session pinning to a layout

- The WS upgrade reads `layoutId` directly from the JWT and writes it
  once to `DriveSession.LayoutID`; there is no `/layouts/{id}/join`
  endpoint and no `session.setLayout` WS action. Attempts to change
  the layout mid-session are impossible by construction. The
  frontend exposes a "log out and switch layout" affordance in the
  account menu of `AppShell.tsx`.
- The same user logged in on two devices with two different JWTs
  (potentially into two different layouts) sees two **independent**
  drive sessions: locks, takeovers, radio messages and emergency
  plans evaluate per session.
- `GET /api/v1/auth/me` returns
  `{ layoutId, layoutName, layoutIsSystem }` so the navbar can
  render the active layout badge.

#### Locking and unlocking

- `POST /api/v1/layouts/{id}/lock` (admin) is idempotent and returns
  `200 OK` with `{ id, locked: true }`. The system layout is
  rejected with `422 default_layout_cannot_be_locked`.
- A locked layout is **not** returned by `GET /api/v1/layouts/login`,
  so no new login can pick it; it **is** still returned by the
  authenticated `GET /api/v1/layouts` (the admin layout-management
  page still needs it).
- Locking a layout that already has live drive sessions does **not**
  close those sessions; throttle commands, takeover, radio and
  `session.setCommandStation` keep working in them until they end on
  their own. The live `session.opened` payload carries
  `layoutLocked: true` (or flips to it via a separate
  `layout.lockedChanged` event broadcast to every session pinned to
  the layout), so the UI may surface a "this layout was locked – you
  will not be able to log back in here" banner.
- `DELETE /api/v1/layouts/{id}/lock` (admin) is idempotent and returns
  `200 OK` with `{ id, locked: false }`. It puts the row back into the
  login dropdown.
- Locking and unlocking write audit rows (`layout.locked`,
  `layout.unlocked`) with the actor admin and the layout id/name.

#### Command-station picker in the throttle

- Every drive session starts with `CommandStationID = nil`. Throttle
  actions (`loco.setSpeed`, `train.setSpeed`, `loco.toggleFn`, …)
  return `ack { ok:false, error:"command_station_not_selected" }`
  while it is `nil`, and the slider in the UI stays disabled.
- The vehicle control view renders a **command-station dropdown**
  populated from `session.opened.availableCommandStations`. The list
  contains:
  - the rows from `LayoutCommandStation` for non-system layouts;
  - every row from `command_stations` for the system layout.
- Picking an entry fires `session.setCommandStation { commandStationId }`.
  The server validates the id against the session layout's current
  set; a mismatch returns
  `ack { ok:false, error:"command_station_not_attached_to_layout" }`.
  On success the server emits `session.commandStationChanged
  { sessionId, commandStationId, commandStationName }` to every
  concurrent session of the same user on the same drive session.
- Picking a **different** entry while one is already active is a
  controlled context switch: the server first runs the user's
  emergency plan (`SetSpeed(0)` on every `DriveTargets` entry, same
  code path as the dead-man's switch) against the previous
  `CommandStationID`, then re-points the session and broadcasts the
  change.
- If the dropdown contains exactly one entry, the UI MAY auto-fire
  `session.setCommandStation` for the user; the server contract is
  unchanged.

#### Mid-session cascades on the attached-stations set

- When an admin **attaches** a new command station to a non-system
  layout via `POST /api/v1/layouts/{id}/command-stations`, every live
  drive session pinned to that layout receives a
  `layout.commandStationsChanged { layoutId,
  availableCommandStations:[…] }` fan-out event. The UI re-renders
  the dropdown in place; the active `CommandStationID` (if any)
  stays untouched.
- When an admin **detaches** a station from a non-system layout, the
  same `layout.commandStationsChanged` event fires. If the picked
  `CommandStationID` is the one being detached, the server first
  broadcasts `session.commandStationChanged { commandStationId: null,
  reason:"detached" }` (which re-gates the throttle) and then the
  refreshed `layout.commandStationsChanged`.
- When an admin **deletes** a `command_stations` row, every layout
  loses the entry: non-system layouts lose the matching
  `LayoutCommandStation` row, the system layout's virtual list
  shrinks, and every live drive session pinned to the deleted
  station is detached the same way as in the previous bullet
  (`reason: "deleted"`).

#### Independence and shared-bus warnings

- A driver in layout A and a driver in layout B (each with at least
  one command station in their layout's set) can drive simultaneously
  without interference, provided they pick *different* command
  stations. Their commands reach independent `Station` instances (one
  per command station id) via `LocoService`'s
  `map[commandStationID]Station`.
- Two drivers in any layouts who pick the **same** command station
  (system layout included) share the DCC bus. The UI shows a
  "shared bus" chip on every throttle pinned to a command station
  another live session is also pinned to.

#### Signalmen and interlockings per layout

- The admin can grant the `signalman` role to a user **scoped to one
  specific layout**; that user only has signalman powers while their
  active session is in that layout. Logging out and back in into a
  different layout removes the powers immediately (because the JWT
  no longer matches the grant).
- Both admins and signalmen of a layout can add interlockings to that
  layout's whitelist; `GET /api/v1/interlockings` for a driver in
  that layout returns exactly the whitelisted set, and interlockings
  not on the whitelist are invisible in the UI. This applies to the
  system layout as well – its whitelist starts empty.

#### Layout admin PIN and sudo elevation (§7a.7)

- Every layout row carries a non-empty `admin_pin_hash` column.
  `POST /api/v1/layouts` requires an `adminPin` field that passes the
  configured length / digits-only checks; a missing or too-short PIN
  is rejected with `pin_missing` / `pin_too_weak`. The bootstrap
  migration that seeds the system layout writes a one-shot random
  PIN to the column and prints it to the server log exactly once;
  the first administrator is expected to rotate it from the layout
  settings page.
- `PUT /api/v1/layouts/{id}` accepts an optional `adminPin` field.
  Submitting the request with the field **omitted or set to an
  empty string** is a no-op for the PIN: the existing hash is left
  untouched. Submitting a non-empty value rotates the PIN
  (argon2id-hashed in `LayoutService.UpdateAdminPIN`) and writes a
  `layout.admin_pin_changed` audit row whose `Metadata` carries
  only the first 8 characters of the **previous** hash for
  forensic correlation – never the plaintext, never the full hash.
  The system layout accepts `adminPin` (it still needs a rotatable
  PIN) even though `name` is rejected with `default_layout_immutable`.
- The admin layout-settings UI renders the PIN field as a numeric
  text input with a helper line "Pozostawienie pustego pola NIE
  zmienia PIN-u" (pl) / "Leaving this field blank does NOT change
  the PIN" (en). The page never submits the rotation request when
  the field is blank, so the "no reset" semantic is enforced both
  client- and server-side.

- Two icons live on the top `AppBar` for every authenticated user:
  a **closed-padlock** (admin elevation) and an **engineer's-cap**
  (signalman elevation). Clicking either opens a `<SudoPinDialog>`
  modal; submitting the dialog calls
  `POST /api/v1/layouts/{layoutId}/sudo { target, pin }` against the
  layout the JWT is bound to. Cross-layout sudo is rejected with
  `422 layout_mismatch`; this is structurally impossible from the UI
  but defended at the API.
- On a successful PIN match the server inserts (or, if a row
  already exists, **updates**) a `sudo_elevations` row with
  `expires_at = now() + cfg.SudoTTL` (default 2 minutes; bounds
  `[1m, 10m]` enforced at startup), writes
  `auth.sudo_granted` to the audit log, and broadcasts
  `auth.elevationChanged { target, granted:true, expiresAt,
  reason:"granted"|"renewed" }` over the WS hub to every live
  session of the caller. The icon flips to its **open** variant
  with a live `MM:SS` countdown badge sourced from `expiresAt`.
- A second click on an already-elevated icon revokes the grant:
  the UI calls `DELETE /api/v1/layouts/{layoutId}/sudo { target }`
  (idempotent, returns `200` even when no row existed),
  `auth.sudo_revoked { reason:"user_action" }` is audited, and
  `auth.elevationChanged { granted:false }` fans out to every live
  session – the icon reverts to closed across desktop and phone
  simultaneously.
- The janitor goroutine (the same one that reaps leases and
  takeovers, §7 cross-cutting concern 9) deletes
  `sudo_elevations` rows where `expires_at <= now()` every 30 s and
  emits `auth.sudo_expired` (actor = system user id `0`,
  login `"system"`) plus `auth.elevationChanged { granted:false,
  reason:"expired" }`. The UI countdown reaches 00:00 at the
  expected wall-clock instant regardless of the janitor's lag,
  because the policy layer treats a row with
  `expires_at <= now()` as already deleted (`AuthService.Effective`
  filters with `AndGt("expires_at", now)`).
- Failed PIN attempts increment two Redis counters
  (`auth:sudo_fail:<userId>:<layoutId>` and
  `auth:sudo_fail:<ip>`) with exponential back-off
  (1 s, 2 s, 4 s, …, 60 s); after `cfg.FailAttempts` consecutive
  misses the (userId, layoutId) tuple is soft-locked for
  `cfg.LockDuration` (default 5 minutes). The next call returns
  `429 sudo_locked` with a `Retry-After` header and an
  `auth.sudo_locked` audit row is written.
- `AuthService.Logout` deletes every `sudo_elevations` row of the
  caller (any layout, any target) in the same transaction as the
  JWT-blacklist insert, audits each row with
  `reason:"logout"`, and broadcasts
  `auth.elevationChanged { granted:false, reason:"logout" }` to
  every other live session of the user before disconnecting the
  current one.
- `LayoutService.Delete` cascades `sudo_elevations` for the
  deleted layout, audits `reason:"layout_deleted"`, and fans out
  `auth.elevationChanged` to the affected sessions. In practice
  `409 layout_in_use` rejects deletion while any session is still
  pinned to the layout, so this branch only fires for empty
  layouts.

- A user with **only a sudo `admin` elevation** can:
  - register a vehicle outside their own DCC pool
    (`vehicle.dcc_address_outside_pool` is bypassed by
    `LocoSecurityContext.CanRegisterLoco` when the actor `Has(admin)`),
  - grant a temporary role to another user
    (`POST /api/v1/users/{id}/temp-role`),
  - read the audit log (`GET /api/v1/audit-log`).
  All three operations succeed identically to a permanent admin.
- A user with **only a sudo `admin` elevation** passes EVERY
  admin-only check while the elevation is live: layout creation,
  rename, PIN rotation (`PUT /api/v1/layouts/{id} { adminPin }`),
  lock / unlock, command-station attach/detach, signalman list,
  interlocking removal AND layout deletion all succeed identically
  to a permanent admin. The matching test case grants a sudo
  elevation, rotates the PIN, asserts the new digest verifies and
  the old one does not. The 2-minute window plus the PIN-dialog
  rate-limiter are the only guard rails (§7a.7.6).
- The **engineer's-cap icon** lives next to the padlock and writes
  a **permanent** `LayoutSignalman` row (`expires_at = NULL`) after
  the same PIN check. Acceptance: a non-signalman user clicks the
  icon, types the layout admin PIN, and immediately afterwards
  every signalman-only API call inside the layout (occupy
  interlocking, request takeover, add to whitelist) succeeds for
  the rest of the session and across reconnects. Clicking the icon
  again — or hitting `DELETE /api/v1/layouts/{id}/signalman` — drops
  the row and restores the original effective role.
- `GET /api/v1/auth/me` returns
  `{ effectiveRole, isSignalman, sudo: { grantedAt, expiresAt }|null }`.
  The frontend padlock indicator drives its countdown from
  `sudo.expiresAt`; the engineer's-cap indicator lights up when
  `effectiveRole === "signalman"`. `auth.me` on WS reconnect
  (§7a.6) carries the same payload so the icons restore to the
  correct state across a refresh.
