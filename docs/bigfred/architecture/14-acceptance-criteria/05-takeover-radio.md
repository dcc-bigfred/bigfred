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

#### Interlocking two-panel view

- The staffed interlocking view shows a **two-panel** work area: a
  fixed-width, scrollable **radio chat** on the left and a fixed-width,
  scrollable, **searchable vehicle/train roster** on the right.
- The chat shows every message exchanged with **every driver** in the
  layout (group-chat), ordered by time, each line formatted
  `({driverLogin}) {vehicle/train name}: {translated phrase}`.
- Each chat line has a **„Odpowiedz"** icon pinned to a fixed position on
  the right; the chat text wraps without moving the icon. Pressing it
  opens a **searchable phrase table** (not a free-text field) addressed
  to that driver in the same vehicle/train context.
- The roster panel marks targets currently **in motion**
  (`loco.state.speed != 0`), filters rows from the header search box, and
  exposes per-row **Radio**, **Stop („Zatrzymaj skład")** and **Przejęcie
  kontroli** actions.
- **„Zatrzymaj skład"** brakes **only** the selected vehicle/train
  (`system.estopTarget`), never the whole layout.
- The interlocking view exposes a **command-station picker behind a cog
  button** (same control as the throttle, §6.3b); a signalman must pick a
  command station before driving a taken-over target, and the pick fires
  `session.setCommandStation`.
- A **Radio Stop** button is shown **above the two panels**, with the
  same red radio-handset icon as the throttle plus the **text label
  „Radio stop"**; confirming dispatches `system.radioStop` (layout-wide).

#### Takeover (15 s window → 5-min self-lease)

- A signalman can request takeover of a driver's vehicle or train. The
  driver sees a **15-second countdown** and can reject it.
- If the driver does **not** reject within the window, the takeover is
  **granted automatically**: a **5-minute lease** of the target is issued
  to the signalman, the driver's **throttle session for that target
  ends**, the driver is **redirected to the dashboard**, and the target
  **disappears from the driver's throttle picker** for the lease
  duration.
- The signalman drives the target in a **closable throttle overlay**
  opened over the interlocking view; the overlay can only be **closed
  when the target's speed is 0**, and closing releases the takeover.
- The takeover (and its lease) ends on the **earliest** of: the 5-minute
  lease expiry, the signalman releasing it, or the signalman leaving the
  box. On release the target **reappears in the driver's throttle
  picker**.
- The signalman becomes able to drive the target because `loco-server`
  adds them to the target's `controllerUserIds` in the republished
  `allowed_vehicles` snapshot (§7e.3); **`dcc-bus` has no lease/takeover
  concept** and only checks set membership. On release the signalman is
  removed from `controllerUserIds` and a `setSpeed` from them is rejected
  `not_authorized` on the next command.

#### Radio (walkie-talkie)

- A driver opens the throttle **radio** overlay (top: searchable
  interlocking picker; bottom: searchable phrase table) and sends a
  phrase **in the context of the vehicle/train they are driving**.
- A radio message is delivered to **all** of the addressee's open
  sessions; the signalman sees it in the group-chat panel, the driver as
  an on-screen alert popup, and the driver's throttle **chat icon lights
  red** until the chat overlay is opened.
- The **sender** hears `/sounds/interlockings/radio-sent.ogg`; the
  **receiver** hears `/sounds/interlockings/{phrase}.ogg`.
- Radio messages are stored **only in Redis** and **expire after ~4
  hours** (default TTL); they are replayed from Redis on reconnect /
  chat-panel mount and there is **no SQLite radio table**.
- Every radio message references exactly one **target** (user XOR
  interlocking) and exactly one **context** (vehicle XOR train).

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
- A **signalman** (even idle — not occupying a box, not mid-takeover)
  may trigger Radio Stop, both from the throttle overlay and from the
  **„Radio stop" button above the interlocking panels** (§6.3d).
- A user who is **neither** a signalman **nor** has any drive scope
  (e.g. an `admin` without `driver`/`signalman`) does not see the button
  and receives `403` if they craft the WS frame manually.
- The audit log records `system.radio_stop` with the triggering user,
  the aggregated list of affected vehicle addresses, and the
  `fired_emergency_plans` list of per-user plans that were run.
- Radio Stop is independent of the walkie-talkie phrase
  `STOP_IMMEDIATELY`: sending that phrase does not brake the layout,
  and Radio Stop does not appear in the interlocking radio panel.
