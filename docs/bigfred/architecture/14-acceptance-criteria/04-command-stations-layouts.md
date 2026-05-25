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
