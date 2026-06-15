### 4.3 Takeover state machine

```
                 takeover.request                    timer (15 s)
   (idle) ───────────────────────────────────► (pending) ────────────────► (granted)
                                          │
                              ┌───────────┴───────────┐
                              ▼                       ▼
                       takeover.reject          takeover.cancel
                          (driver)                 (signalman)
                              │                       │
                              ▼                       ▼
                         (rejected)              (cancelled)

   (granted) ─── 5-min lease expires / signalman releases / leaves box ──► (released → idle)
```

The state machine lives in `TakeoverService` and is persisted in the
`takeover_requests` table for auditing. The 15 s window is driven by a
`time.AfterFunc` keyed by `RequestID`; if the server restarts mid-window
the request is re-loaded and either auto-granted (if `AutoGrantAt` has
already passed) or rescheduled.

The driver keeps the **15-second reject window**: while `pending`, a
`takeover.reject` cancels the request. If the driver does **not** react
within the window, the request is **granted automatically** (the
signalman never needs to click "grant").

#### Grant effect – 5-minute self-lease + driver eviction

On transition to `granted` (auto after the timer, never early):

1. `TakeoverService` creates a **self-lease** of the target to the
   signalman — a `VehicleLease` / `TrainLease` row with
   `FromUserID = owner`, `ToUserID = signalman`,
   `ExpiresAt = now + TakeoverLeaseDuration` (**5 minutes**). This reuses
   the existing lease machinery so the signalman's driving authority,
   audit and dead-man's-switch contract are identical to any other
   lessee.
2. The affected **driver's throttle session for that target ends**: the
   server emits `takeover.granted` to the driver, whose client shows a
   message ("Twoja sesja Throttle zakończyła się z powodu przejęcia
   składu") and **redirects them to the dashboard**. The target
   **disappears from the driver's throttle vehicle/train picker** for
   the lease duration (the resolved drive scope excludes targets leased
   away). This is stronger than the previous "read-only telemetry"
   behaviour — the driver leaves the throttle entirely.
3. The **signalman** drives the target from a **closable throttle
   overlay** opened inside the interlocking view (§6.3d). The overlay can
   only be **closed when the target's speed is 0**; closing it releases
   the takeover.

#### Release

The takeover (and its lease) ends on the **earliest** of:

- the **5-minute lease expiry** (the lease janitor revokes it and emits
  `takeover.released`);
- the signalman explicitly releasing it (closing the throttle overlay at
  speed 0, or a release control);
- the signalman **leaving the interlocking** (or being displaced).

On release the lease row is revoked, `takeover.released` is broadcast,
and the target **reappears in the original driver's throttle picker**.
The driver can re-enter throttle mode and resume driving.
