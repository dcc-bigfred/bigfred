### 4.1 REST

All endpoints live under `/api/v1`. Endpoints that mutate or read
restricted data require a valid session (see §11). The column "Roles"
lists who can call the endpoint (`*` = any authenticated user, with
ownership/lease checks applied where applicable).

```
# --- Authentication ---
GET    /api/v1/layouts/login                                     PUBLIC      # unauthenticated: list of layouts to pre-fill the login dropdown. Returns only non-locked rows: [{id, name, isSystem}]. UI substitutes the i18n key `layout:system_default_label` for rows where isSystem == true.
POST   /api/v1/auth/login              { login, pin, layoutId }  *           # exchange login+PIN+layout for a session token. layoutId is REQUIRED. 422 layout_not_found / layout_locked are possible on top of 401 invalid_credentials. The issued JWT carries {userId, layoutId} and the binding is immutable for the token lifetime.
POST   /api/v1/auth/logout                                       *           # also deletes every SudoElevation row for the caller (any layout, any target) and broadcasts `auth.elevationChanged { granted:false, reason:"logout" }` to every other live session of the user (§7a.7).
GET    /api/v1/auth/me                                           *           # current user, effective roles, DCC pool, plus { layoutId, layoutName, layoutIsSystem } from the JWT. The `effectiveRoles` array carries `{ role, source }` tuples where `source ∈ {"permanent", "temp_grant", "layout_signalman", "sudo"}`; the UI uses `source == "sudo"` to render the open-padlock indicator and the live countdown.

# Sudo elevation (§7a.7) – self-grant of `admin` or layout-scoped `signalman`
# powers gated by the LAYOUT admin PIN. The grant is short-lived (default 2 min,
# server-configurable in [1m,10m]); the row is auto-reaped by the janitor.
POST   /api/v1/layouts/{id}/sudo       { target: "admin"|"signalman", pin } *  # `{id}` MUST equal the JWT `layoutId` – cross-layout sudo is rejected with `422 layout_mismatch`. Verifies `pin` against `Layout.AdminPINHash`; on mismatch returns `401 invalid_pin` and bumps `auth:sudo_fail:<userId>:<layoutId>` and `auth:sudo_fail:<ip>` Redis counters with exponential back-off (1s, 2s, 4s, …, 60s); after N consecutive failures returns `429 sudo_locked` for 5 minutes and writes `auth.sudo_locked` (§3a.5). On success returns `{ target, expiresAt, ttlSec }`, writes the `SudoElevation` row, audits `auth.sudo_granted`, and broadcasts `auth.elevationChanged { target, granted:true, expiresAt }` to every live WS session of the caller. **A second call for the same target while a row already exists is a controlled "renew the timer" path: `ExpiresAt` is bumped to `now() + ttl` and a fresh `auth.sudo_granted` row is audited.**
DELETE /api/v1/layouts/{id}/sudo       { target?: "admin"|"signalman" } *      # explicit user-side revoke. With no `target` it revokes both. Idempotent: returns `200 OK` even if no row existed. Writes `auth.sudo_revoked { reason:"user_action" }` and broadcasts `auth.elevationChanged { granted:false }`.

# --- API keys (per-user temporary keys, max lifetime 365d) ---
GET    /api/v1/apikeys                                           *           # own keys only (prefix + metadata, never plaintext)
POST   /api/v1/apikeys                 { name, expiresAt, scopes:[...] } *   # mint; returns plaintext ONCE in response
DELETE /api/v1/apikeys/{id}                                      *           # revoke (own key, or admin for any)

# --- User management ---
GET    /api/v1/users                                             admin
POST   /api/v1/users                   { login, pin, role }       admin
PUT    /api/v1/users/{id}/role         { role }                   admin       # change permanent role
POST   /api/v1/users/{id}/temp-role    { role, expiresAt }        admin       # grant temporary role
DELETE /api/v1/users/{id}/temp-role/{tempRoleId}                  admin       # revoke early
PUT    /api/v1/users/{id}/dcc-pool     { ranges:[{from,to},...] } admin       # assign DCC pool

# --- Vehicles ---
GET    /api/v1/vehicles                                           *           # all visible (own + leased + signalman-overridden)
POST   /api/v1/vehicles                { dccAddress, name, ... }  *           # register inside own DCC pool
PUT    /api/v1/vehicles/{addr}         { name, ... }              owner       # edit
DELETE /api/v1/vehicles/{addr}                                    owner

GET    /api/v1/vehicles/{addr}/cv/{n}                             owner       # CV read (lessee cannot)
POST   /api/v1/vehicles/{addr}/cv      { entries:[{n,v},...] }    owner       # CV write

# --- Vehicle functions (F0-F32; owner-only editing, lessee can only invoke) ---
GET    /api/v1/vehicles/{addr}/functions                          *           # resolved list (template OR vehicle rows; carries `source`)
PUT    /api/v1/vehicles/{addr}/functions/{num}  { name, icon, kind, position } owner   # upsert one slot; auto-detaches if linked
DELETE /api/v1/vehicles/{addr}/functions/{num}                    owner       # remove one slot; auto-detaches if linked
POST   /api/v1/vehicles/{addr}/functions/reorder { positions:[{num,position},…] } owner # auto-detaches if linked
POST   /api/v1/vehicles/{addr}/functions/detach                   owner       # explicit copy-on-write; idempotent
POST   /api/v1/vehicles/{addr}/functions/attach { templateId }    owner       # drop local rows, re-link to template
GET    /api/v1/function-icons                                     *           # closed catalogue of FunctionIcon values

# --- Vehicle templates (anyone creates; only owner or admin edits) ---
GET    /api/v1/vehicle-templates                                  *
GET    /api/v1/vehicle-templates/{id}                             *
POST   /api/v1/vehicle-templates       { name, description }      *
PUT    /api/v1/vehicle-templates/{id}  { name?, description? }    owner OR admin
DELETE /api/v1/vehicle-templates/{id}                             owner OR admin  # 409 unless ?cascade=true (§3a.6.4)

GET    /api/v1/vehicle-templates/{id}/functions                   *
PUT    /api/v1/vehicle-templates/{id}/functions/{num}             owner OR admin
DELETE /api/v1/vehicle-templates/{id}/functions/{num}             owner OR admin
POST   /api/v1/vehicle-templates/{id}/functions/reorder           owner OR admin

# --- Scripts (browser-side Python automation) ---
GET    /api/v1/scripts                                            *           # lists scripts the caller can SEE: owned + those attached to a vehicle/train the caller can drive (lessee). For lessee-visible rows the `source` field is omitted.
GET    /api/v1/scripts/{id}                                       owner       # full source; lessee gets 403 even with an active lease
POST   /api/v1/scripts                 { name, source, runtime, icon, description? } *           # creates a script owned by the caller; source ≤ 64 KiB, runtime ∈ {micropython, pyodide}
PUT    /api/v1/scripts/{id}            { name?, source?, runtime?, icon?, description? } owner   # bumps `version`; fan-out `script.changed` to live throttles
DELETE /api/v1/scripts/{id}                                       owner       # also drops every ScriptAttachment row

# Script attachments – a script may be bound to a vehicle XOR a train.
GET    /api/v1/scripts/{id}/attachments                           *           # owner sees all; lessee sees only attachments to vehicles/trains they can drive
POST   /api/v1/scripts/{id}/attachments { vehicleAddr? , trainId? , position? } owner            # exactly one of vehicleAddr / trainId required; 422 otherwise
DELETE /api/v1/scripts/{id}/attachments/{attachmentId}            owner

# Reverse listing: scripts visible on a given throttle (used by the UI to populate the script-button row alongside F0..F32)
GET    /api/v1/vehicles/{addr}/scripts                            * (driving authority)
GET    /api/v1/trains/{id}/scripts                                * (driving authority)

# --- Leasing ---
POST   /api/v1/vehicles/{addr}/lease   { toUserId, expiresAt }    owner
DELETE /api/v1/vehicles/{addr}/lease                              owner       # revoke active lease
POST   /api/v1/trains                  { name, members:[...] }    *           # only own vehicles
POST   /api/v1/trains/{id}/lease       { toUserId, expiresAt }    owner
DELETE /api/v1/trains/{id}/lease                                  owner

# --- Interlockings ---
GET    /api/v1/interlockings                                      *           # FILTERED to the caller's active layout (only whitelisted IDs). Each row includes `{ id, name, location, occupant?: { userId, login } }` when staffed.
POST   /api/v1/interlockings/{id}/join   { force?: bool }         signalman   # become active occupant; requires interlocking ∈ active layout. When already occupied: `409 interlocking_occupied` unless `force:true`, which ends the incumbent session (`reason:"displaced"`) and opens a new one for the caller.
POST   /api/v1/interlockings/{id}/leave                           signalman   # end own active session for this interlocking (idempotent if not occupying)

# --- Command Stations (catalogue of `centralki`) ---
GET    /api/v1/command-stations                                            *           # list (name + connection type only; admin sees full Connection)
GET    /api/v1/command-stations/{id}                                       admin       # full details incl. Connection
POST   /api/v1/command-stations                 { name, connection }       admin
PUT    /api/v1/command-stations/{id}            { name, connection }       admin
DELETE /api/v1/command-stations/{id}                                       admin       # cascades: every LayoutCommandStation row pointing at it is removed and every live DriveSession pinned to it is detached (CommandStationID → nil + broadcast `session.commandStationChanged { commandStationId: null, reason:"deleted" }`). 409 layout_needs_at_least_one_command_station if removing the row would leave any non-system layout with zero attached stations.

# --- Layouts (modeling events) ---
# Note: there is no /layouts/{id}/join or /leave endpoint. The layout
# is picked on the login form and pinned to the drive session by the
# JWT (§7a.1); switching layout requires logout + login.
GET    /api/v1/layouts                                            *           # full list (incl. locked rows); admin sees an `canEdit:bool` badge. Each row carries: { id, name, isSystem, locked, commandStations:[{id,name}] }. For isSystem rows commandStations mirrors the live `command_stations` catalogue. `AdminPINHash` is **never** included in the response – the page only shows whether a PIN is set (always true after bootstrap).
GET    /api/v1/layouts/{id}                                       *
POST   /api/v1/layouts                 { name, commandStationIds:[id,...], adminPin } admin (non-sudo)   # commandStationIds REQUIRED and MUST contain at least one id; rejects with `layout_needs_at_least_one_command_station` otherwise. `adminPin` is REQUIRED (numeric, default 6 digits, configurable min/max length); rejects with `pin_missing` when empty and `pin_too_weak` when below the minimum length. The PIN is argon2id-hashed before insert. Trying to create a second `IsSystem=true` row is impossible (partial unique index). Sudo-elevated admins are denied with `requires_non_sudo_admin` (§7a.7).
PUT    /api/v1/layouts/{id}            { name?, adminPin? }       admin (non-sudo)       # rename and/or rotate the layout admin PIN (§7a.7). The two fields are independent – a request with `name` alone keeps the PIN, a request with `adminPin` alone keeps the name, a request with both does both. **`adminPin` is treated as "no change" when the field is missing or an empty string** so the UI's "blank field = don't reset" semantic is enforced server-side too. The system layout (isSystem) rejects `name` changes with `default_layout_immutable` but accepts `adminPin` (the system layout still needs a rotatable PIN). Sudo-elevated admins are denied with `requires_non_sudo_admin` regardless of which fields are present – a sudo user cannot rename a layout, attach stations OR rotate the PIN. The attached command-station set is mutated through the dedicated subresource below. PIN rotation writes the `layout.admin_pin_changed` audit row (§3a.5).
DELETE /api/v1/layouts/{id}                                       admin       # 409 if any drive session is still pinned to it; the system layout (isSystem) always returns 422 default_layout_undeletable.

# Lock / unlock (admin only; hides the layout from /api/v1/layouts/login)
POST   /api/v1/layouts/{id}/lock                                  admin       # 422 default_layout_cannot_be_locked when isSystem; idempotent on a non-system layout (returns 200 with `locked:true`); NEVER closes live drive sessions.
DELETE /api/v1/layouts/{id}/lock                                  admin       # unlock; idempotent (returns 200 with `locked:false`).

# Command-station attachment (admin only; not allowed on the system layout)
GET    /api/v1/layouts/{id}/command-stations                      *           # returns the current set: for non-system layouts the LayoutCommandStation rows, for the system layout the entire `command_stations` catalogue (virtual)
POST   /api/v1/layouts/{id}/command-stations { commandStationId } admin       # 422 default_layout_command_stations_immutable when isSystem; 404 command_station_not_found if the id is unknown; 409 already_attached when the row exists
DELETE /api/v1/layouts/{id}/command-stations/{commandStationId}   admin       # 422 default_layout_command_stations_immutable when isSystem; 422 layout_needs_at_least_one_command_station when it would leave the layout with zero rows; live sessions pinned to the detached station are detached (CommandStationID → nil) and re-gated.

# Layout-scoped signalmen
GET    /api/v1/layouts/{id}/signalmen                             *
POST   /api/v1/layouts/{id}/signalmen  { userId, expiresAt? }     admin       # grant signalman role inside this layout
DELETE /api/v1/layouts/{id}/signalmen/{userId}                    admin

# Layout vehicle roster + live presence (dashboard data sources)
GET    /api/v1/layouts/{id}/vehicles                              *           # vehicles on the layout roster. JWT `layoutId` must match `{id}`.
GET    /api/v1/layouts/{id}/vehicles/mine                         *           # caller's registered vehicles with `onLayout: bool` per row ("Show my vehicles" / add picker)
POST   /api/v1/layouts/{id}/vehicles     { vehicleAddr }          owner       # add own vehicle to roster; 409 if already attached
DELETE /api/v1/layouts/{id}/vehicles/{vehicleAddr}                owner       # remove own vehicle from roster
GET    /api/v1/layouts/{id}/presence                              *           # online users in this layout: `[{ userId, login, role, occupiedInterlocking?: { id, name } }]`

# Layout-scoped interlocking whitelist
GET    /api/v1/layouts/{id}/interlockings                         *
POST   /api/v1/layouts/{id}/interlockings { interlockingId }      admin OR signalman-of-this-layout
DELETE /api/v1/layouts/{id}/interlockings/{interlockingId}        admin

# --- Audit log (admin only, append-only) ---
GET    /api/v1/audit-log                                          admin       # filterable: ?action=&actor=&objectType=&objectId=&layoutId=&since=&until=&limit=&offset=
GET    /api/v1/audit-log/{id}                                     admin

# --- System ---
GET    /api/v1/system/status                                      *           # command station info FOR THE CALLER'S CURRENTLY PICKED COMMAND STATION (resolved via the session's CommandStationID); returns `{ commandStationSelected:false }` until the user fires session.setCommandStation
```

Takeover, throttle and radio are **all WebSocket-only** because they are
short, frequent, and event-driven.
