### 10.2 Users, ownership and leases (M2–M3)

- A fresh database can be bootstrapped to a single `admin` user; that
  admin can create additional users and assign them the `driver` role
  via the UI.
- The admin can assign DCC ranges to a driver; an out-of-pool vehicle
  registration is rejected by the server with a clear error.
- The admin can grant the `driver` role temporarily for one hour; after
  exactly one hour the user loses that capability **without any
  manual action**.
- A driver can lease one of their vehicles to another driver for 30
  minutes; the lessee can drive it but cannot edit metadata or write
  CVs. The lease either expires automatically or the owner can revoke it
  early; in both cases the lessee's UI loses driving authority within
  seconds (event-driven, not via UI refresh).
- A driver who owns a train of N ≥ 2 vehicles opens the train control
  view and sees **the same speed slider** as on the single-vehicle
  view (`ThrottleSlider.tsx` is reused unchanged). Dragging that
  slider to step 40 causes **every** member vehicle to roll: each
  member's `loco.state` broadcast lands within one polling interval
  with `speed = 40` and `controlledBy.kind = "train"`, and any member
  marked `Reversed = true` reports `forward` flipped relative to the
  slider's direction toggle so the consist moves rigidly.
- The train slider's `ack` returns a per-member outcome list; if one
  member's decoder fails to acknowledge (simulated by stubbing the
  station to time out on that addr), the ack contains
  `{ addr: X, ok: false, error: "station_timeout" }` for that member
  while every other member rolls. The UI renders a yellow chip on
  the failing member's row; no `system.estop` fires.
- While a train is being driven from a tab, the driver opens a
  second tab and issues an explicit `loco.setSpeed` to one of the
  member addresses. That member's `loco.state` flips `controlledBy`
  to `"driver"`; on the first tab the train slider grays out that
  member's row with a "detached" badge and a one-click re-attach
  button, but does not stop the rest of the consist.
