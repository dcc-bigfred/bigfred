### 3a.4 Layout-and-command station addressing rules

The `layout` and `command station` concepts together form the
*addressing* layer of the system. The rules below cut across services
and are worth stating in one place. They replace the older, post-login
"party list / party join" model: with the new model the layout is
picked **on the login form** and the drive session is bound to it for
its entire lifetime.

1. **Every drive session is pinned to exactly one layout, chosen at
   login.** `POST /api/v1/auth/login { login, pin, layoutId }`
   validates credentials, looks up `Layout` by `layoutId`, and rejects
   the request with `422 layout_locked` if `Layout.Locked == true`,
   `422 layout_not_found` if the row does not exist. On success the
   issued JWT carries `{ userId, layoutId }` (immutable inside the
   token); the WS upgrade reads `layoutId` directly from the JWT and
   the resulting `DriveSession.LayoutID` is **read-only** for the
   lifetime of that session. Switching layout requires a full
   logout/login round trip – there is **no `setLayout` WS action and
   no `/layouts/{id}/join` endpoint**.

2. **A layout has one or more attached command stations.** Non-system
   layouts have an explicit `LayoutCommandStation` join table managed
   by an admin (§4.1 `POST/DELETE /api/v1/layouts/{id}/command-stations`).
   Service-side validation refuses to create a non-system layout
   without at least one attached command station and refuses to
   detach the last one from a non-system layout (`422
   layout_needs_at_least_one_command_station`).
   The **system layout** (`IsSystem = true`, seeded with
   `Name = "default"`) instead exposes a **virtual list**: every row
   from `command_stations` is implicitly attached to it without any
   `LayoutCommandStation` row existing. Admin attempts to mutate that
   list are rejected with `422 default_layout_command_stations_immutable`.

3. **`session.CommandStationID` starts as `nil` for every drive
   session.** Right after the WS upgrade the server emits
   `session.opened { layoutId, availableCommandStations: [{id,name}…], …
   }` carrying the layout's command-station set (the join rows for
   non-system layouts, the full `command_stations` catalogue for the
   system layout). The throttle is gated until the user picks one via
   `session.setCommandStation { commandStationId }` (§4.5). Until that
   happens, every action that needs a `Station` returns
   `command_station_not_selected`. The UI MAY auto-fire
   `session.setCommandStation` for the user when the layout's set
   contains **exactly one** entry, but the server contract is the same
   in both cases.

4. **Changing the picked command station mid-session is a controlled
   context switch.** Calling `session.setCommandStation` a second time
   with a different `commandStationId` is allowed in **any** layout.
   The server first runs the user's emergency plan against the
   previous `CommandStationID` (`SetSpeed(0)` on every `DriveTargets`
   entry on the old station; same code path as the dead-man's switch),
   then re-points the session at the new one and broadcasts
   `session.commandStationChanged` to every concurrent session of the
   same user. Attempts to pick a `commandStationId` that is not in the
   session layout's set return `ack { ok:false,
   error:"command_station_not_attached_to_layout" }`. Attempts to pick
   one that was deleted (or detached from the layout) since
   `session.opened` was emitted return the same error and the throttle
   stays gated until the user picks again from the refreshed set.

5. **All driving operations resolve their `Station` via the *session's*
   `CommandStationID`** (not the layout's, because the layout has many).
   `LocoService` keeps a `map[commandStationID]Station` that is lazily
   initialised on first use and shared across **all** drive sessions
   currently pinned to that command station, regardless of which
   layout they entered through. There is **no global station** in the
   service layer.

6. **Locking a layout never kicks anyone out.** Toggling
   `Layout.Locked = true` (admin-only, via `POST /api/v1/layouts/{id}/lock`)
   only removes the row from the unauthenticated login dropdown
   (`GET /api/v1/layouts/login`). Drive sessions already pinned to that
   layout keep running normally; throttle commands, takeover, radio,
   and `session.setCommandStation` continue to work. Once those
   sessions close on their own, the layout becomes effectively
   dormant. Unlocking it (`DELETE /api/v1/layouts/{id}/lock`) puts it
   back into the login picker. The system layout cannot be locked
   (`422 default_layout_cannot_be_locked`).

7. **Interlocking listings, takeover requests and radio messages are
   filtered by the session's layout.** A driver in layout A never sees
   signal boxes from layout B even if they happen to share command
   stations. The system layout's whitelist starts empty (no
   interlockings appear by default); admins or signalmen of the system
   layout may whitelist interlockings just like in any other layout.

8. **Vehicles and trains are *not* layout-scoped.** Ownership and
   leases live at the user level and travel with the user across
   layouts – a driver's locomotive is theirs regardless of which event
   they attend. The vehicle's DCC address, however, only makes sense
   on the command station the driver has currently picked; switching
   the session's command station naturally re-targets every command.

9. **Two drive sessions on the same command station share the DCC
   bus.** This is true whether they entered through the same layout or
   through two different layouts that both list this command station.
   The system does not prevent it, but the UI shows a "shared bus"
   chip on every throttle currently pinned to a command station that
   any other live session is also pinned to. Likewise, an admin
   attaching a command station to a new layout while it is already
   driven elsewhere sees a UI warning.

10. **Deleting a command station cascades safely.** `DELETE
    /api/v1/command-stations/{id}` removes every `LayoutCommandStation`
    row pointing at it (including effectively de-listing it from the
    system layout's virtual view) and detaches every live drive
    session that was pinned to it (`session.CommandStationID → nil`,
    plus a `session.commandStationChanged { commandStationId: null,
    reason: "deleted" }` broadcast). The throttle re-gates until the
    user picks another command station from the refreshed
    `availableCommandStations` list (or, if the layout no longer has
    any, until the admin re-attaches one). Deletion is rejected with
    `409 layout_needs_at_least_one_command_station` if it would leave
    any non-system layout with zero attached command stations.
