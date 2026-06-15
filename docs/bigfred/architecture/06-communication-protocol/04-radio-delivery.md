### 4.4 Radio – delivery rules

Walkie-talkie radio (this section) is separate from **Radio Stop**
(§4.6): preset phrases such as `STOP_IMMEDIATELY` are point-to-point
messages with no layout-wide braking side effect.

#### 4.4.1 Message shape

Every radio message carries, on top of the closed-vocabulary `phrase`
and an optional free-text `note`:

- a **target** — exactly one of `toUserId` (a driver) or
  `toInterlockingId` (a signal box);
- a **context** — exactly one of `vehicleId` or `trainId`. The context
  is **required**: a radio message is always *about* a specific vehicle
  or train (both variants are selectable in the UI). The context drives
  the chat line label `({driverLogin}) {vehicle or train name}` and lets
  the signalman correlate traffic with the roster panel (§6.3d).

#### 4.4.2 Delivery

- A radio message addressed to `userId` is delivered to **all** of that
  user's open WebSocket sessions (phone + desktop simultaneously).
- A radio message addressed to `interlockingId` is delivered to the user
  currently occupying that interlocking (via the unique
  `InterlockingSession`), if any.
- On delivery the recipient sees an on-screen **alert-style popup** in
  the throttle view (driver) or a new line in the chat panel (signalman),
  and the chat affordance lights up (red unread badge on the driver's
  throttle chat icon until the chat overlay is opened, §6.3b).

#### 4.4.3 Visibility (group-chat scoping)

The same persisted traffic is projected differently per role:

- **Signalman, in the interlocking view** — sees **all** messages
  exchanged with **all** drivers in the layout in a single chat panel
  (group-chat style), ordered by time.
- **Driver, in the throttle view** — sees **only their own** messages
  (driver→signalman) and the messages directed **at them**
  (signalman→driver). A driver never sees traffic addressed to other
  drivers.

#### 4.4.4 Storage (Redis-only, 4-hour TTL)

Radio messages are **not** persisted in SQLite. They live **only in
Redis** and **expire after a default 4 hours** (configurable). This is
the single source of truth for the chat history and the reconnect
replay:

- write path: `RadioService.Send` appends the message to the relevant
  Redis conversation stream(s) with `EXPIRE` set to the configured TTL
  (default `4h`);
- read / replay path: on chat-panel mount or WS reconnect the client
  fetches the recent window from Redis (via the REST replay endpoint
  §4.1 or a `radio.replay` WS round trip), so a brief connection drop
  does not silently lose traffic. Anything older than the TTL is gone by
  design — radio is operational chatter, not an audit trail.

Suggested key layout (final names settled during implementation):

- `bigfred:radio:layout:<L>:interlocking:<I>` — the signalman's
  group-chat stream for one interlocking (everything that box
  sent/received), TTL 4h;
- `bigfred:radio:layout:<L>:user:<U>` — a driver's personal stream
  (their own messages + messages directed at them), TTL 4h.

A single `radio.send` fans the message into both the addressed party's
stream and the sender's own stream so each side replays a consistent
view.

#### 4.4.5 Sounds

Sound assets live under `web/public/sounds/interlockings/`:

- the **sender** plays `/sounds/interlockings/radio-sent.ogg` on a
  successful `radio.send` ack;
- the **receiver** plays `/sounds/interlockings/{phrase}.ogg`, i.e. one
  asset per `RadioPhrase` value (e.g.
  `/sounds/interlockings/entry_permitted.ogg`). A missing asset falls
  back to a generic chime; playback is best-effort (autoplay may be
  blocked until the first user gesture, like the radiostop alarm in
  §4.6.3).
