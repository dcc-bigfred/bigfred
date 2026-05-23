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

   (granted) ─── signalman leaves interlocking / clicks release ──► (released → idle)
```

The state machine lives in `TakeoverService` and is persisted in the
`takeover_requests` table for auditing. The 15 s window is driven by a
`time.AfterFunc` keyed by `RequestID`; if the server restarts mid-window
the request is re-loaded and either auto-granted (if `AutoGrantAt` has
already passed) or rescheduled.
