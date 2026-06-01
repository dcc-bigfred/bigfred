### §7e.1 Overview & design goals

#### Problem

§5 places the DCC bus inside `loco-server`. `LocoService` keeps a
`map[commandStationID]Station` (§3a.4 rule 5), `Poller` ticks against
every subscribed locomotive address, and the WebSocket Hub fans out
`loco.state` events to subscribed throttles. Everything – auth,
sessions, takeover, radio, scripts, dispatch, polling, station drivers
– runs in one process.

That model is operationally fine for one command station and a dozen
locomotives, but it conflates three concerns that have **very
different blast radii**:

1. **Control plane** — auth, sessions, REST, layout management,
   takeover state machine, radio dispatch, script orchestration,
   audit log. Lives at the *user* layer.
2. **Data plane** — the DCC bus itself: serial / TCP / UDP I/O against
   the Roco Z21 or a LocoNet master, the polling loop, the real-time
   throttle WebSocket events.
3. **Catalogue** — vehicles, trains, functions, templates, layouts.
   Slow-moving SQL state.

A misbehaving serial driver (a USB-to-RS-232 dongle going through a
power cycle, or a LocoNet packet storm) must not be able to take down
the user-facing API. Conversely, a bug in `ScriptService` must not
silently freeze the DCC bus. The two layers are physically different
risk surfaces and deserve **process isolation**.

#### Goals

1. **One DCC connection per `(layout, command station)` daemon.** A
   `dcc-bus` process owns exactly one `commandstation.Station` for one
   `(LayoutID, CommandStationID)` tuple. It is the **single writer** to
   that DCC bus while it is running. Multiple daemons targeting the
   *same* command station from *different* layouts are allowed; they
   share the DCC bus exactly as multiple physical throttles do (§3a.4
   rule 9 — "shared bus" chip).
2. **Layout-scoped throttle visibility.** The daemon subscribes the
   poller to **only** the vehicles attached to its layout's roster
   (`LayoutVehicle` rows for non-system layouts; the full `vehicles`
   table for the system layout). A vehicle that is registered but not
   on this layout is invisible to the daemon's throttle, no matter what
   the catalogue contains.
3. **Re-use, do not re-invent, the security layer.** Every command
   that hits the daemon's WebSocket is gated by exactly the same
   `*SecurityContext` objects from `pkgs/server/security` (§7a.3) that
   `loco-server` uses. The policies stay pure / stateless; only the
   *callers* differ.
4. **Session-aware.** The WS upgrade reads the JWT issued by
   `POST /api/v1/auth/login` (§7a.1, shared secret with `loco-server`)
   and refuses connections whose `layoutId` does not match the
   daemon's `--layout-id`. The daemon mirrors `DriveSession` semantics
   for its own clients (sessionId, heartbeat, emergency plan).
5. **Redis-cached state.** The daemon writes
   `HSET loco:state:<commandStationId> <addr> "{json}"` so that a
   fresh subscriber, a peer `dcc-bus` reading the same bus, or
   `loco-server` answering REST snapshots all converge on the same
   truth (§3a.7 invariants will be tightened to require this).
6. **Reflect external controllers.** The command station is a shared
   controller: physical throttles plugged directly into it can change a
   loco without BigFred's involvement. The daemon watches the bus and
   mirrors those external speed / direction / function changes back into
   the UI — by *subscription/push* (LocoNet shared bus, Z21
   `LAN_SET_BROADCASTFLAGS`), with a *polling* fallback for any future
   driver that cannot push. See §7e.9.
7. **Hot-managed by `loco-server`.** The daemon is spawned, watched
   and restarted by `SupervisordService` (§7d). `loco-server` adds /
   removes `[program:dcc-bus-…]` entries via `UpsertProgram` /
   `RemoveProgram`; supervisord handles crash restart, log rotation
   and graceful shutdown.
8. **Single-binary, single-source-of-truth.** `dcc-bus` is a cobra
   subcommand of the same `loco-server` binary, exactly like
   `scripts-executor` (§7d.4). One Go module, one set of REL
   migrations, one security package, one Redis layout.
9. **Stays Linux/macOS only.** Same as `scripts-executor` (§7d.1
   non-goals).

#### Non-goals (this milestone)

- HTTP/REST surface on `dcc-bus`. The daemon speaks WebSocket only;
  any control / introspection happens via Redis pub/sub or supervisord
  status (§7d.3).
- **Any SQLite access from `dcc-bus`.** The daemon does not open the
  database. Layout roster and command-station connection parameters
  reach the process via Redis snapshots and CLI flags respectively;
  all catalogue mutations stay in `loco-server`.
- Multi-instance arbitration. Exactly one `dcc-bus` per
  `(layoutId, commandStationId)` is enforced by supervisord's program
  uniqueness (each program name is globally unique, §7d.2).
- Cross-host federation. The first cut assumes `loco-server` and every
  `dcc-bus` run on the same host (XDG paths shared, Redis on
  loopback). Hosting `dcc-bus` on a Raspberry Pi physically next to
  the LocoNet master is a later milestone — the protocol does not
  preclude it but the orchestration code initially does.
- Replacing `loco-server`'s WebSocket Hub. The Hub still hosts
  `session.*`, `takeover.*`, `radio.*`, `script.*`, presence,
  `interlocking.*` and `auth.elevationChanged`. Only the throttle
  data plane moves.

#### High-level placement

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Browser (React, single SPA, multiple WS connections)                       │
│                                                                             │
│     ws://host/api/v1/ws            ws://host:<P1>/ws       ws://host:<P2>/ws│
│         ▲                                ▲                       ▲          │
│         │ control plane                  │ throttle              │ throttle │
│         │ (session, takeover,            │ (loco.* for           │ (loco.*  │
│         │  radio, scripts, …)            │  layout L,            │  for     │
│         │                                │  cs C1)               │  L, C2)  │
└─────────┼────────────────────────────────┼───────────────────────┼──────────┘
          │                                │                       │
┌─────────┼────────────────────────────────┼───────────────────────┼──────────┐
│  loco-server (Go, non-root)              │                       │          │
│  ┌──────▼────────────────────────┐       │                       │          │
│  │  HTTP (chi) + Hub             │       │                       │          │
│  │  /api/v1/* + /api/v1/ws       │       │                       │          │
│  │  AuthService, LayoutService,  │       │                       │          │
│  │  TakeoverService, RadioService│       │                       │          │
│  │  ScriptService, PresenceSvc   │       │                       │          │
│  │  SupervisordService           │       │                       │          │
│  └────┬──────────────────────────┘       │                       │          │
│       │ supervises (hot reload)          │                       │          │
│       │  ── adds/removes dcc-bus-X-Y ──  │                       │          │
└───────┼──────────────────────────────────┼───────────────────────┼──────────┘
        │                                  │                       │
        ▼                                  │                       │
┌──────────────────────────────────────┐   │                       │
│  supervisord (Python, child of       │   │                       │
│   loco-server, §7d)                  │   │                       │
│   group: loco                        │   │                       │
│     - scripts-executor               │   │                       │
│   group: dcc-bus                     │   │                       │
│     - dcc-bus-1-2  ◀──port=P1───┐    │   │                       │
│     - dcc-bus-1-3  ◀──port=P2───┼────┘                       │   │
└──────────────────────────────────┼───┼───────────────────────────┼──────────┘
                                   │   │                           │
                                   ▼   ▼                           ▼
       ┌──────────────────────────────────┐  ┌────────────────────────────────┐
       │  dcc-bus (layout=1, cs=2)        │  │  dcc-bus (layout=1, cs=3)      │
       │  ──────────────────────────────  │  │  ──────────────────────────    │
       │  · poll subscribed vehicles      │  │  · poll subscribed vehicles    │
       │  · DCC I/O via                   │  │  · DCC I/O via                 │
       │    commandstation.Station        │  │    commandstation.Station      │
       │  · enforce drive policy on       │  │  · enforce drive policy on     │
       │    every WS action via Redis     │  │    every WS action via Redis   │
       │    roster snapshots              │  │    roster snapshots            │
       │  · write loco:state:cs:2 Redis   │  │  · write loco:state:cs:3 Redis │
       │  · per-daemon dead-man's switch  │  │  · per-daemon dead-man's switch│
       └────────────┬─────────────────────┘  └────────────┬───────────────────┘
                    │                                     │
                    ▼                                     ▼
        ┌────────────────────────┐            ┌────────────────────────┐
        │  Z21 / LocoNet master  │            │  Z21 / LocoNet master  │
        │  (cs id 2)             │            │  (cs id 3)             │
        └────────────────────────┘            └────────────────────────┘

        SQLite (loco-server only) ─── catalogue source of truth
        Shared Redis             ─── state cache, roster snapshots, pub/sub
```

The lozenge `dcc-bus-<layoutId>-<commandStationId>` follows the
supervisord program-naming regex (§7d.2: `^[a-z][a-z0-9_-]*$`) and is
globally unique per `(layout, command station)` tuple.

#### Why a sibling daemon rather than a goroutine

| Aspect | Goroutine inside `loco-server` (§5) | Sibling `dcc-bus` daemon (§7e) |
|---|---|---|
| Crash blast radius | Serial driver panic → whole server | Crashes only the affected layout × cs throttle; supervisord restarts; control plane (login, takeover, scripts) keeps running |
| Hot reload of DCC drivers | Requires restarting `loco-server` (session loss) | `supervisorctl restart dcc-bus-X-Y` — control plane and other layouts untouched |
| Physical placement | Always co-located with `loco-server` | Can be moved off-host later (different binary, network DCC bus) |
| Polling cadence isolation | Shares Go scheduler with HTTP handlers | Independent process; can tune `GOMAXPROCS=1` for predictable latency |
| Multi-station scaling | One `LocoService` map, one polling goroutine per addr | One process per cs, naturally parallel; supervisord aggregates status |
| Operational visibility | One `system.status` payload | Per-cs status row in `supervisorctl status` and in extended `system.status` |
| Code re-use | `LocoService` in-process | `LocoService` migrates to `dcc-bus` largely unchanged (same `Station` interface, same security policies) |

The split is the same playbook that §3a.7 / §7d already apply to
`scripts-executor`: extract a noisy, third-party-driver-heavy concern
out of the user-facing API process, keep the **policy** in Go and
delegate the **mechanism** to supervisord.

#### What `dcc-bus` is NOT

- It is **not a database service.** Catalogue writes go through
  `loco-server`'s REL repositories. The daemon never opens SQLite;
  it consumes pre-built JSON snapshots published by the server on
  Redis (`pkgs/layoutroster`).
- It is **not an authorization authority.** It enforces the same
  `pkgs/server/security` policies as the server but it does not mint
  JWTs, does not extend sudo elevations, does not write audit rows.
  Audit lines for throttle activity continue to be written by
  `loco-server` based on the events the daemon publishes back through
  Redis (see §7e.3 § "audit-fan-in").
- It is **not the script runtime.** Goja still lives in
  `scripts-executor` (§3a.7). Scripts invoke DCC operations via
  `loco-server`, which then publishes a command on the daemon's
  Redis command channel (§7e.6). The dcc-bus is unaware that the
  caller is a script; the same authorization re-checks apply.
- It is **not a discovery service.** Frontends learn the daemon's
  WebSocket URL from `session.opened.availableCommandStations[i].wsUrl`
  (§7e.6); they never scan ports.

#### Why one daemon per `(layout × cs)`, not just per `cs`

A command station is shared *physical hardware*: only one process can
hold the serial port / TCP socket open. The natural cardinality of the
*DCC bus* itself is one daemon **per command station**, not per
layout. So why the `(layout × cs)` key?

1. **The roster of subscribed vehicles is layout-scoped.** A daemon
   polls only the vehicles on its layout's `LayoutVehicle` roster.
   Two layouts sharing the same command station will, in general,
   show different vehicle sets on their respective dashboards.
2. **Authorization scope is layout-scoped.** Signalman role,
   interlocking whitelist, takeover targets, lease visibility, sudo
   elevation: all rooted in the JWT's `layoutId` (§3a.4 rule 1). A
   daemon serving "layout 5 over cs 2" only needs to load layout-5
   context.
3. **Emergency plans are scoped per-daemon.** When a user's session
   on layout A dies, only layout-A's vehicles must stop — not
   everything else running on cs 2 driven by layout-B users. A
   per-`(layout × cs)` daemon naturally has the right blast radius.

The cost is that two daemons may simultaneously open the same
command-station endpoint. The §3a.4 rule 9 "shared bus" semantics
already document this; the **DCC bus itself is shared by the protocol
specification** (multiple throttles drive the same hardware). The two
daemons are the BigFred-side mirror of that physical reality.

For command stations that genuinely cannot tolerate two writers (a
LocoNet serial port the OS can only open once), an operator-side rule
applies: do not attach the same cs to two non-system layouts that have
overlapping vehicle rosters. The system layout always sees every cs
virtually (§3a.4 rule 2), so for the system layout the daemon set is
`{(system_layout, c) : c ∈ command_stations}` and there is exactly one
daemon per command station coming from the system layout side.
