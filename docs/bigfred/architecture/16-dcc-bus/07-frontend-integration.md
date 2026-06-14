### §7e.7 Frontend integration

The throttle (§6.3b) is rendered as a full-screen overlay above the
existing `AppShell`. The §7e split changes **where** the UI gets its
real-time data from, not the visual surface.

#### Two WebSockets

```
   browser
   ├── ws://host/api/v1/ws            (loco-server)
   │     - session.opened             (carries availableCommandStations + wsUrls)
   │     - session.setCommandStation  (client → server, picks the cs)
   │     - session.commandStationChanged (server → client, carries new wsUrl)
   │     - takeover.*, radio.*, script.*, presence, …
   │
   └── ws://host:<port>/ws?token=<jwt> (dcc-bus, picked cs)
         - dcc-bus.opened             (sessionId, sharedBus, layoutId, csId)
         - loco.subscribe / unsubscribe / setSpeed / toggleFn / system.estop / ping
         - loco.state, loco.error, vehicle.functionsChanged, system.status,
           session.warning, session.emergencyExecuted, pong, ack
```

Both connections are managed by the existing `useSocket` hook
parameterised by URL; the throttle overlay is the **only** place that
opens the second one. The Zustand store has two namespaces:

```ts
// web/src/ws/store.ts (extended)
type ThrottleState = {
  controlWs: WsClient | null;       // /api/v1/ws to loco-server
  dccBusWs:  WsClient | null;       // ws://host:port/ws to current dcc-bus
  dccBusUrl: string | null;
  dccBusOpened: { sessionId: string; layoutId: number; commandStationId: number } | null;
  // existing fields:
  locos: Record<number, LocoState>;
  // …
};
```

#### Lifecycle

1. **Login.** The user enters login, PIN, layout in the form. The
   server issues a JWT pinned to `(userId, layoutId)`. The SPA opens
   `controlWs`.

2. **`session.opened` received.** The SPA stores
   `availableCommandStations`. Most carry `wsUrl == null` at first —
   the dcc-bus has not been spawned yet.

3. **User opens the throttle overlay** (§6.3b). The overlay renders
   the command-station dropdown from `availableCommandStations`. The
   slider, function buttons, and script row are disabled with a
   "Pick a command station to begin driving" placeholder.

4. **User picks a cs.** The overlay dispatches
   `session.setCommandStation { commandStationId }` on `controlWs`.
   The server:
   - Authorizes via `LayoutSecurityContext.CanSetSessionCommandStation`.
   - Calls `DccBusService.EnsureRunning(L, C)`.
   - **Lazy spawn UX.** If the daemon needs to be spawned (no existing
     `dcc-bus-<L>-<C>` program in supervisord), the server immediately
     emits an interim event
     `session.commandStationChanged { commandStationId, wsUrl: null, status: "starting", reason: "spawning" }`
     so the SPA can render the placeholder
     **"Potrzebuję chwili, pierwsze połączenie z centralką…"**
     (i18n key `throttle:csStatus.spawning`). It then waits up to
     `dcc-bus startSecs + dial timeout` (≈ 10 s) for the daemon to
     reach RUNNING and accept a WS dial.
   - On success: replies `ack { ok:true }` on the original
     `session.setCommandStation` envelope **and** broadcasts the
     final `session.commandStationChanged { commandStationId, wsUrl, status: "running" }`
     to every concurrent session of the user.
   - On failure: replies `ack { ok:false, error:"dcc_bus_unavailable" }`
     (or `error:"no_dcc_bus_ports_available"` when the pool is
     exhausted); the SPA flips the placeholder to a red banner
     mapped from `throttle:errors.dcc_bus_unavailable`.

5. **SPA opens `dccBusWs`** at the received `wsUrl`. The two
   WebSockets are now both live.

6. **Snapshot.** The dcc-bus emits `dcc-bus.opened`. The SPA reads
   `sharedBus` and shows the chip if true. The SPA then issues
   `loco.subscribe { addr }` on `dccBusWs` for each vehicle the user
   has opened in the overlay (initially zero — see "vehicle picker"
   below).

7. **Driving.** Every slider move sends `loco.setSpeed` on `dccBusWs`.
   The `loco.state` push events update the Zustand `locos` map; the
   slider is re-rendered.

8. **Switching cs.** The user picks a different command station from
   the dropdown. The SPA:
   - Sends `session.setCommandStation { commandStationId: newC }` on
     `controlWs`.
   - The server runs the existing emergency-plan-on-old-cs path
     (§3a.4 rule 4) by publishing onto the **old** daemon's command
     channel; the old daemon stops the user's drive targets.
   - The server emits `session.commandStationChanged { wsUrl: newUrl }`.
   - The SPA closes the old `dccBusWs`, opens a new one against the
     new URL, and re-subscribes.

9. **Logout.** The SPA closes both WS connections in order: data
   plane first, control plane second. The server's emergency-plan
   logic does not fire because logout cleanly clears `DriveTargets`.

10. **dcc-bus crash / restart.** The data-plane WS drops with
    `ECONNREFUSED`. The SPA retries with exponential backoff and
    re-subscribes on success. The control plane never blinks; the
    user sees a transient `loco.error { code:"command_station_disconnected" }`
    in the overlay and a "Reconnecting…" toast until the daemon is
    back.

#### Vehicle picker inside the overlay

The throttle overlay is **multi-vehicle**: it lets the driver swipe /
tab between the vehicles they currently have authority on
(owned + leased + takeover-held). On open it queries
`GET /api/v1/vehicles?layout={L}&driveable=true` (existing REST,
filtered to the active layout) and renders a horizontal scroll-snap
strip of vehicle cards. Tapping a card subscribes to that addr on
`dccBusWs` (lazy — only the currently-focused vehicle is subscribed,
others are unsubscribed to keep the poller's working set small).

Trains: the strip also lists trains the user has authority on. Tapping
a train subscribes to **every member's** addr via `train.subscribe`
on `controlWs` (the server fans the per-member subscribes onto the
appropriate dcc-bus command channels). The slider in train mode
emits `train.setSpeed` on `controlWs`, not on `dccBusWs`, because
train fan-out is server-side (§7e.4).

#### Throttle component tree

```
<ThrottleOverlay>          // full-screen layer above AppShell (§6.3b)
  <ThrottleHeader>          // command-station dropdown, sharedBus chip, close button
    <CommandStationPicker />
    <SharedBusChip />       // visible iff dcc-bus.opened.sharedBus === true
    <SudoIndicator />       // existing
    <DeadmanIndicator />    // displays grace countdown when session.warning arrives
  </ThrottleHeader>

  <VehicleStrip>            // horizontal scroll-snap row of cards
    {driveableVehicles.map(v => <VehicleCard addr={v.dccAddress} {...v} />)}
    {driveableTrains.map(t  => <TrainCard   trainId={t.id} {...t} />)}
  </VehicleStrip>

  <ThrottlePanel>
    <ThrottleSlider value={speed} forward={fwd}
                    onValueChange={onSetSpeed}
                    onDirectionChange={onSetDir} />
    <EmergencyStopButton onClick={onSystemEstop} />
    <DirectionToggle />
    <FunctionButtons   vehicle={v} state={fnState} />
    <ScriptButtons     vehicle={v} />                // unchanged, on controlWs
    <ScriptConsole     vehicle={v} />                // unchanged
    {takeover && <TakeoverBanner state={takeover} />}
  </ThrottlePanel>
</ThrottleOverlay>
```

The Slider, FunctionButtons, ScriptButtons and ScriptConsole are the
same components specified in §6.3 / §6.3a / §6.7; the only change is
that `<FunctionButtons>` for a single-vehicle view dispatches
`loco.toggleFn` on **`dccBusWs`** (data plane) while `<ScriptButtons>`
still goes through `controlWs` (control plane). Picking the right
socket is encapsulated in two helper hooks:

```ts
// web/src/ws/hooks.ts
export function useDataPlane()    { /* returns send(env) bound to dccBusWs */ }
export function useControlPlane() { /* returns send(env) bound to controlWs */ }
```

Components call whichever they need; no component knows about ports.

#### Multi-tab behaviour

Two browser tabs of the same user with the same JWT:

- Both open `controlWs` (existing rule: § 4.5.1 N concurrent sessions).
- Both open `dccBusWs` to the same daemon when the user picks the
  same cs in each tab. The daemon allocates two
  `daemonSession`s with different `sessionId`s.
- A slider move on tab A produces `loco.setSpeed` from daemonSession
  A; the daemon broadcasts `loco.state` to **every** subscriber of
  that addr, including tab B. Tab B updates its slider position.
- Closing tab A drops one daemonSession; tab B keeps driving until
  it too closes, at which point the per-daemon emergency plan fires
  (§7e.5).

#### Required i18n keys

A new namespace `pl/throttle.json` (and `en/throttle.json`) hosts the
overlay strings. Initial coverage:

```json
{
  "title": "Throttle",
  "pickCs": "Wybierz centralkę",
  "csStatus": {
    "running": "Połączona",
    "starting": "Uruchamianie…",
    "spawning": "Potrzebuję chwili, pierwsze połączenie z centralką…",
    "stopped": "Niepołączona",
    "draining": "Kończy pracę",
    "degraded": "Błąd"
  },
  "sharedBus": "Współdzielona magistrala DCC",
  "emergencyStop": "Stop awaryjny",
  "deadman": {
    "warning": "Brak aktywności — automatyczny stop za {{seconds}} s",
    "executed": "Wykonano plan awaryjny ({{action}})"
  },
  "errors": {
    "command_station_disconnected": "Utracono połączenie z centralką",
    "vehicle_not_on_layout": "Ten pojazd nie znajduje się na obecnej makiecie",
    "vehicle_is_dummy": "Pojazd nie posiada adresu DCC",
    "not_authorized_to_drive": "Brak uprawnień do sterowania tym pojazdem",
    "taken_over": "Pojazd przejęty przez nastawniczego",
    "lease_expired": "Wypożyczenie wygasło",
    "dcc_bus_unavailable": "Demon dcc-bus niedostępny — spróbuj ponownie za chwilę",
    "no_dcc_bus_ports_available": "Brak wolnego portu dla nowej centralki — zatrzymaj nieużywaną"
  }
}
```

Following §7c, the daemon does **not** ship localized strings; only
machine codes (`loco.error.code` values) travel the wire.

#### Reusing `pkgs/bigfred/server` types

`tygo` (§ Tech stack) already generates TS types from `pkgs/bigfred/server/ws`
Go structs. The new envelope types
(`DccBusOpened`, `LocoError`, `SystemStatus`, …) live in
`pkgs/bigfred/dcc-bus/ws/protocol.go` and are exported from the same `tygo`
file so the frontend gets them automatically.

#### Acceptance summary (UX-side)

- A vehicle slider opens at 0, snaps to 0 when emergency fires, and
  is read-only when `controlledBy.kind == "signalman"`.
- Switching the command-station dropdown waits at most 10 s
  before either resuming control on the new station or surfacing
  `dcc_bus_unavailable`.
- A network blip on the data plane shows a transient banner but
  the AppBar, sudo padlock, layout switcher and language menu stay
  reactive.
- The same vehicle controlled from two tabs converges within one
  poll interval of any slider move.
