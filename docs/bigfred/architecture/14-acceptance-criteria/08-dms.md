### 10.6 Drive session & dead-man's switch (cuts across M1–M4)

- A driver who has set a vehicle's speed to 60 and then **closes the
  browser tab** loses control: within the configured grace period
  (default 5 s) the server issues `SetSpeed(0)` on that vehicle. This
  is independently observable on the command station's diagnostic
  output, not only in the UI.
- A driver opens the app on phone **and** desktop simultaneously,
  drives a vehicle, then closes the phone. The vehicle keeps moving:
  no emergency action fires because the desktop session is still
  alive.
- A driver suffers a momentary network blip lasting less than the
  grace period. The client reconnects, sends back `sessionId`, the
  pending emergency timer is cancelled; the vehicle keeps moving at
  its previous speed without any visible glitch.
- A driver who configured their emergency plan to `release_my_leases`
  loses connectivity. After the grace period, their leased-out
  vehicles are returned to their owners **and** the driver's own
  vehicles are stopped.
- Restarting the Go backend triggers a global e-stop on the command
  station as part of startup; once stations come back, drivers must
  re-issue speed commands.
- The user's *other* open sessions, plus any signalman currently
  controlling one of the affected vehicles via takeover, receive a
  `session.emergencyExecuted` event with the list of affected
  addresses, so all UIs converge to the same state without polling.
