### 10.3 Interlockings, takeover, radio (M5)

#### Layout dashboard

- After login the user lands on `/` and sees three tables scoped to
  their pinned layout: **layout vehicle roster**, **online users**
  (login, role, occupied interlocking if any), and **interlockings**
  (name, occupant or vacant).
- **Pokaż moje pojazdy** toggles the first table between the shared
  roster and the caller's own vehicles with an `onLayout` indicator.
- **Dodaj mój pojazd do makiety** lets an owner attach one of their
  registered vehicles to the roster; the row appears for every online
  user without a manual refresh (`layout.vehiclesChanged`).
- Opening a second browser tab for another user in the same layout
  updates the online-users table on the first tab within one WS
  round trip (`layout.presenceChanged`).

#### Interlocking occupation

- A signalman can occupy an interlocking that is whitelisted in their
  active layout; an interlocking not on the whitelist cannot be
  occupied even by an admin.
- From the dashboard, clicking an interlocking row opens
  `/interlockings/:id` with the radio panel and occupation buttons.
- **Obsadź nastawnię** on a vacant box succeeds immediately; the
  dashboard and interlocking header show the new occupant.
- **Obsadź nastawnię** on an already-staffed box shows a confirmation
  naming the incumbent; confirming with `{ force: true }` displaces
  them (`reason:"displaced"`), opens a session for the caller, and
  notifies the displaced user.
- **Opuść nastawnię** ends the caller's session; the interlocking
  shows as vacant everywhere.
- Navigating away from the interlocking view while occupying prompts
  **Leave the interlocking?**; confirming leaves the box, cancelling
  keeps the user on the page with the session intact.

#### Takeover and radio

- A signalman can request takeover of a driver's vehicle. The driver
  sees a 15-second countdown and can reject it; if not rejected,
  authority transfers automatically.
- A driver and the active signalman of the relevant interlocking can
  exchange "walkie-talkie" messages using preset phrases; messages are
  delivered to all of the addressee's open sessions and persist for
  10 minutes of replay on reconnect.
- The interlocking view radio panel shows traffic addressed to that
  box and supports sending preset phrases to drivers and (where the
  protocol allows) other interlockings in the layout.

#### Radio Stop

- A driver with at least one drivable vehicle on the layout sees a red
  **Radiostop** button on the throttle overlay's **left toolbar**,
  immediately to the right of the **Fullscreen** toggle. Pressing it
  opens a centred overlay with a red **„Uruchom radiostop”** button and
  a **„Anuluj”** button below it; only **„Uruchom radiostop”** sends
  `system.radioStop`, after which every roster vehicle on **all**
  command stations attached to the layout brakes to a standstill.
- In addition to the roster halt, **every connected driver's
  dead-man's-switch emergency plan is fired** (effect b, §4.6.1a):
  their running scripts stop with reason `"radio_stop"`, and a driver
  whose plan is `release_my_leases` has their outbound leases revoked.
  A connected admin whose plan is `estop_all` is **clamped to
  `stop_my_vehicles`** — Radio Stop never cuts track power.
- The Fullscreen toggle puts the throttle overlay into browser
  fullscreen and back; its icon reflects `document.fullscreenElement`.
- Every open throttle session in the layout (including users who did
  not press the button) plays the radiostop sound (`/sounds/radiostop.ogg`)
  when `system.radioStop` arrives.
- A user with no drive scope (idle signalman, admin without `driver`)
  does not see the button and receives `403` if they craft the WS
  frame manually.
- The audit log records `system.radio_stop` with the triggering user,
  the aggregated list of affected vehicle addresses, and the
  `fired_emergency_plans` list of per-user plans that were run.
- Radio Stop is independent of the walkie-talkie phrase
  `STOP_IMMEDIATELY`: sending that phrase does not brake the layout,
  and Radio Stop does not appear in the interlocking radio panel.
