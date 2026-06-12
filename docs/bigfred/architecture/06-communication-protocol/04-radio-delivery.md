### 4.4 Radio – delivery rules

Walkie-talkie radio (this section) is separate from **Radio Stop**
(§4.6): preset phrases such as `STOP_IMMEDIATELY` are point-to-point
messages with no layout-wide braking side effect.

- A radio message addressed to `userId` is delivered to **all** of that
  user's open WebSocket sessions (phone + desktop simultaneously).
- A radio message addressed to `interlockingId` is delivered to the user
  currently occupying that interlocking (via the unique
  `InterlockingSession`), if any.
- All radio messages are persisted in `radio_messages` and can be
  replayed on reconnect for the last N minutes (configurable, default
  10 min) so a brief connection drop does not silently lose traffic.
