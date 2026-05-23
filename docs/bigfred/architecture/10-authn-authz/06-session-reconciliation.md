### 7a.6 Session reconciliation on WS connect

When a client (re)connects, the server sends:

1. `auth.me` – the user, the effective role set and the assigned DCC
   pool.
2. `loco.snapshot` for every vehicle/train the user is allowed to see
   (own + currently leased to them + currently overridden by signalman
   takeover, if user is the signalman).
3. The last N minutes of `radio.message` directed at this user.

This is what lets the UI re-render correctly after a refresh or a brief
disconnect without polling REST first.
