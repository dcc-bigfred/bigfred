### §7e.8 Acceptance criteria

To be copied into `14-acceptance-criteria/` when the milestone is
scheduled.

1. **Bootstrap.** Starting `loco-server` from a clean install creates
   the supervisord groups `loco` (with `scripts-executor` per §7d) and
   `dcc-bus` (empty until a session selects a command station). No
   `dcc-bus-*` process exists in `ps` until step 3.
2. **Session-driven spawn.** A driver logs in, picks a layout with
   one attached command station, opens the throttle overlay and
   selects that command station from the dropdown. Within 10 s a
   `dcc-bus-<layoutId>-<commandStationId>` process is RUNNING in
   `supervisorctl status` AND a `loco-server` WS event
   `session.commandStationChanged { wsUrl: "ws://…" }` reaches the
   client.
3. **Direct throttle write.** Moving the slider with no other client
   connected produces a single `loco.setSpeed` on the daemon's WS
   followed by a `loco.state` push within one poll interval. The DCC
   bus carries the matching `XPN_SET_LOCO_DRIVE` (Z21) or
   `OPC_LOCO_SPD` (LocoNet) packet — verifiable with the existing
   command-station test harness.
4. **External throttle convergence.** A physical throttle on the same
   command station changes the speed of a subscribed loco. The
   browser slider converges to the external value within one poll
   interval (= `--poll-interval`, default 200 ms).
5. **Two tabs, two daemons converge.** The same user opens two
   browser tabs to the same layout and the same command station. A
   slider move on tab A appears on tab B within one round trip; the
   daemon reports two distinct `sessionId`s on Redis
   `dcc-bus:<L>:<C>:sessions`.
6. **`SIGTERM` to `loco-server` stops every daemon.** Sending
   `SIGTERM` to `loco-server` causes `SupervisordService.Stop` to
   shut down every `dcc-bus-*` program (each drains its WS clients
   per §7e.2). After 30 s of grace, `ps` lists no `dcc-bus` and no
   `supervisord` processes owned by the operator UID.
7. **Daemon crash isolation.** `kill -9 $(pgrep -f dcc-bus-1-2)`
   terminates only the affected daemon. supervisord respawns it
   within `startsecs` (1 s) due to `autorestart=true`; throttles
   connected to it receive `loco.error { code:"command_station_disconnected" }`
   then reconnect on `RUNNING`. **Throttles on other command
   stations stay live throughout.**
8. **Authorization re-check.** A lessee drives a vehicle through the
   daemon. The owner revokes the lease via REST. The next
   `loco.setSpeed` from the lessee returns `loco.error { code:"not_authorized_to_drive" }`;
   the lessee's slider freezes. Verifiable because the daemon does
   not store its own lease state — it re-reads SQLite on every
   command.
9. **Takeover propagation.** A signalman issues
   `takeover.request` on `loco-server`'s WS; after the 15 s window
   the granted takeover causes the affected driver's WS
   subscription on `dcc-bus` to flip to
   `loco.state { controlledBy: { kind:"signalman" } }`, and any
   subsequent driver `loco.setSpeed` is denied with `taken_over`.
   The signalman's own throttle on the same dcc-bus can now drive.
10. **Dead-man's switch on the daemon.** The driver locks the
    phone; after `--heartbeat-grace` (default 5 s) the daemon emits
    `session.warning` and, on no recovery, runs `SetSpeed(0)` on
    every drive target. **Vehicles on a different command station
    of the same user keep running** (per-daemon scope).
11. **Dead-man's switch on `loco-server`.** With the `loco-server`
    WS dropping (e.g. network glitch) but the daemon WS still
    alive, the dead-man's switch on the daemon does **not** fire;
    however `ScriptService.StopAllForUser` runs as before because
    `loco-server`'s emergency path completed (existing §4.5).
12. **Script DCC writes still gated.** A user's running script calls
    `setSpeed`. The path is: `scripts-executor → loco-server →
    LocoServiceDriver → DccBusService.PublishCommand → dcc-bus → DCC bus`.
    Authorization is enforced in `loco-server` AND re-enforced in
    `dcc-bus`. If the user's lease has just expired, both denials
    fire and the script receives the same `not_authorized_to_drive`
    error it would have received pre-daemon.
13. **Train fan-out.** `train.setSpeed` on `loco-server`'s WS
    produces one publish on each affected member's `(L, C)`
    command channel; daemons write each member sequentially per
    Reversed-flip rules (§4.2); the consolidated WS `ack` aggregates
    per-member outcomes exactly as before.
14. **Roster invalidation.** Adding a vehicle to the layout's
    roster (`POST /api/v1/layouts/{id}/vehicles`) publishes
    `bigfred:layout:<L>:vehicles`; within 100 ms the daemon's
    interesting set widens, the poller picks up the new addr (on
    its next subscriber), and existing throttle pages can
    `loco.subscribe` without restart.
15. **Hot reload of `dcc-bus` programs.** Attaching a new command
    station to a non-system layout, then logging in and selecting
    it, causes `SupervisordService.UpsertProgram` and
    `supervisorctl reread` + `update` to add the new entry — **no
    full supervisord daemon restart** (verifiable: PID of the
    supervisord process stays stable across the operation).
16. **Port pool exhaustion.** When `[9200, 9299]` is fully allocated,
    `session.setCommandStation` ack returns
    `ack { ok:false, error:"no_dcc_bus_ports_available" }`; the
    server logs a warning, and operator action (widen `--dcc-bus-port-max`)
    re-enables it.
17. **JWT mismatch.** A WS upgrade to `dcc-bus-1-2` with a JWT
    pinned to a different `layoutId` is closed with HTTP 403; the
    daemon's stderr contains a structured `jwt_layout_mismatch` log.
18. **Audit fan-in.** A `system.estop` from a driver appears in
    `loco-server`'s audit log as `session.emergency_executed`,
    even though the WS frame never touched `loco-server`'s WS.
    Source attribution (`Metadata.source = "dcc_bus_estop"`) is
    present.
19. **Cold restart of `loco-server` preserves daemons.** Stopping
    just `loco-server` (not supervisord) leaves the `dcc-bus-*`
    programs running. Restarting `loco-server` reads
    `dcc-bus:ports` from Redis, re-publishes the same desired state
    to supervisord, and the WS sessions continue (the `controlWs`
    on the browser reconnects with backoff; the `dccBusWs` never
    blinks).
20. **Cold restart of supervisord re-creates daemons.** Killing
    supervisord with `SIGKILL` is followed by `loco-server`'s
    `tryRespawnDaemon` (§7d.3) bringing it back. supervisord's
    `[program:dcc-bus-…]` entries from the rendered config restart
    every daemon; throttles reconnect.

These criteria are observable from outside the system (curl, ps,
browser devtools, `supervisorctl status`) and do not require
inspecting Go internals — same standard as §10 today.
