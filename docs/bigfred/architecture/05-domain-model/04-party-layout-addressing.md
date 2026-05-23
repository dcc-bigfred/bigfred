### 3a.4 Party-and-layout addressing rules

The `party` and `layout` concepts together form the *addressing* layer
of the system. A few rules cut across services and are worth stating in
one place.

1. **Every authenticated user session is attached to exactly one
   party** at any moment. The default party is `default`. Switching
   party requires re-joining (closing the previous drive session and
   opening a new one).
2. **The active layout is resolved differently for `default` vs. other
   parties:**
   - **Non-default party**: `session.LayoutID = party.LayoutID` is
     pinned at join time and **cannot change for the lifetime of the
     drive session**. This is a safety property: dead-man's switch
     emergencies always target a single, unambiguous layout.
   - **`default` party**: `session.LayoutID` starts as `nil`. The
     driver picks a layout from a dropdown in the vehicle control
     view, which fires `session.setLayout { layoutId }`. Until the
     dropdown is set, throttle commands return `layout_not_selected`
     (the UI keeps the throttle disabled). Changing the pick later is
     allowed but is **treated as a controlled context switch**: the
     server first runs the user's emergency plan against the previous
     `LayoutID` (`SetSpeed(0)` on every `DriveTargets` entry on the
     old station), then re-points the session at the new
     `LayoutID`. This keeps the dead-man's switch contract intact
     across the switch.
3. **All driving operations resolve their `Station` via the
   *session's* `LayoutID`** (not the party's, because that may be
   `nil`). `LocoService` keeps a `map[layoutID]Station` that is
   lazily initialised on first use and shared across all sessions
   attached to that layout. There is **no global station** in the
   service layer.
4. **Interlocking listings, takeover requests and radio messages are
   filtered by party.** A driver in party A never sees signal boxes
   from party B even if they happen to be on the same layout. In
   `default`, no interlockings appear by default (no whitelist is
   pre-populated); admins or signalmen of `default` may whitelist
   interlockings just like in any other party.
5. **Vehicles and trains are *not* party-scoped.** Ownership and
   leases live at the user level and travel with the user across
   parties – a driver's locomotive is theirs regardless of which event
   they attend. The vehicle's DCC address, however, only makes sense
   on the layout the driver has currently selected; switching the
   `default` session's layout naturally re-targets every command.
6. **Two parties (or `default` sessions) on the same layout share the
   DCC bus.** The system does not prevent this configuration, but
   documents it: an admin creating a second party on a layout that
   already has active drivers gets a UI warning. Likewise, a `default`
   driver picking a layout that another party is currently using sees
   a warning chip in the UI.
