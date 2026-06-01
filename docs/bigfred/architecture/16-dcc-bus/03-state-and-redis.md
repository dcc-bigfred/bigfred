### §7e.3 State & Redis cache

#### Inputs

`dcc-bus` consumes three kinds of input and produces one kind of
output:

```
                   ┌──────────────────────────────────────────────────┐
                   │                  dcc-bus                          │
                   │                                                  │
       Redis ─────►│  roster snapshots (allowed_vehicles,             │
       GET+sub     │  defined_trains) + loco:state cache              │
                   │                                                  │
       Redis ─────►│  pub/sub: server-initiated cmds + roster updates │
       pub/sub     │                                                  │
                   │                                                  │
       command  ──►│  WebSocket frontend clients                      │
       station     │  (loco.* and system.estop)                       │
       (DCC,        │  (connection params from CLI --station-*)        │
       bidir)     ──┴──►  state cache + audit events                  │
                   │                                                  │
                   └────────┬─────────────────────────────────────────┘
                            │
                            ▼
                          Redis (string keys + pub/sub)
                            │
                            ▼
                   loco-server (SQLite catalogue, snapshot publisher)
                   browsers     (snapshot on subscribe)
```

Catalogue truth lives in **`loco-server`'s SQLite**. The daemon never
queries it. Instead `loco-server` builds JSON snapshots and publishes
them on Redis (types in `pkgs/layoutroster`).

#### Layout roster snapshots

For each layout `L` the server maintains two Redis **string keys** and
mirrors the same JSON on matching **pub/sub channels** (`SET` +
`PUBLISH` in one pipeline so subscribers always match `GET`):

| Key / channel | JSON root | Purpose |
|---|---|---|
| `bigfred:layout:<L>:allowed_vehicles` | `{ layoutId, updatedAt, vehicles[] }` | Drivable vehicles on the layout roster. Each entry: `vehicleId`, `addr`, `ownerUserId`, `controllerUserIds[]`. Dummies (no DCC address) are omitted. |
| `bigfred:layout:<L>:defined_trains` | `{ layoutId, updatedAt, trains[] }` | Trains on the layout roster with ordered `members[]` (`vehicleId`, `position`, `reversed`, optional `addr`). |

**Daemon boot:** `GET` both keys (may be missing → empty roster until
the server publishes).

**Runtime:** `SUBSCRIBE` to both channels; each message replaces the
in-memory cache (`Router.ApplyAllowedVehicles` /
`ApplyDefinedTrains`).

**Publisher:** `LayoutVehicleService.SyncLayoutRosterToRedis` after
layout roster mutations, vehicle catalogue updates affecting roster
trains, train catalogue changes, and once per layout at
`loco-server` bootstrap.

`loco.subscribe` gates on membership in `allowed_vehicles`.
`loco.setSpeed` / `loco.toggleFn` additionally require
`session.userId ∈ controllerUserIds` for that address (today the server
publishes `[ownerUserId]`; leases/takeovers will extend the slice).

#### Per-vehicle invalidation channels (planned)

Finer-grained channels from the original design (`bigfred:vehicle:<id>:lease`,
`takeover`, `functions`, …) remain **future work**. Until then, any
catalogue change that affects driving rights flows through a full
roster snapshot on the two layout keys above.

#### State feed — external-throttle visibility

The command station this daemon owns is **only one of several possible
controllers** for any given loco: a physical handheld throttle plugged
straight into the Z21 or onto the LocoNet bus can change a loco's speed,
direction or functions without BigFred ever issuing the command. To keep
the throttle UI honest, `dcc-bus` runs a **state feed** that mirrors
whatever it observes on the bus back into the `loco:state` cache and out
to WS clients. See §7e.9 for the driver-capability research that drives
the two implementations below.

The feed (`Router.RunStateFeed`) picks one of two strategies at startup
depending on whether the driver implements the optional
`commandstation.StateObserver` capability:

- **Push** (LocoNet serial / TCP **and** Z21). The driver demultiplexes
  its receive stream and emits a `LocoObservation` per change; the feed
  consumes that channel in real time. LocoNet is a shared bus, so every
  `OPC_LOCO_SPD` / `OPC_LOCO_DIRF` / `OPC_LOCO_SND` / `OPC_SL_RD_DATA`
  packet is visible; the Z21 pushes `LAN_X_LOCO_INFO` after the driver
  enables `LAN_SET_BROADCASTFLAGS` (see §7e.9). Both surface changes
  authored by external handsets.
- **Polling fallback** (any future driver without push). The feed ticks
  at `--poll-interval-ms` (default `750ms`) and, for every address with
  **at least one live WS subscriber**, issues `Station.GetSpeed` +
  `Station.ListFunctions`. Addresses nobody is watching are skipped to
  avoid useless DCC traffic.

Both strategies funnel into the same reconciler (`applyObservation`).
For Z21, speed/direction on the wire and the SET-vs-INFO step-mode
asymmetry are documented in §7e.9 ("Z21 drive encoding").

1. Merge the (possibly partial) observation onto the last cached
   snapshot.
2. **Only** store + fan when the merged state actually changes. This
   change-guard also collapses the echo of BigFred's own writes (those
   already wrote Redis with their real `source` / `controlledByUserId`),
   so in-app driver attribution survives while genuine external changes
   still surface.
3. Write the snapshot to Redis (`loco:state:<layoutId>:<addr>`, TTL
   refreshed) and fan `loco.state` out to every subscriber of `addr` via
   the in-memory Hub (and, through `StoreState`, onto the
   `dcc-bus:evt:<L>:<C>` event channel).

An externally-driven change carries `controlledByUserId: 0` (nobody in
BigFred owns it) and `source: "external"` (push) or `source: "poller"`
(poll-detected drift). The frontend treats both as authoritative and
applies them rather than suppressing them as its own optimistic echo.
The feed also keeps the per-`(addr, fn)` `fnCache` honest after an
external function change, so the next in-app toggle is not wrongly
collapsed as a no-op.

`controlledBy` for in-app writes is still computed inside the daemon
from the most recent `setSpeed` / `toggleFn` caller, pub/sub takeover
state, and explicit re-broadcasts from `loco-server` (see §7e.5).

#### Redis key layout

| Key | Type | Owner | Purpose |
|---|---|---|---|
| `loco:state:<layoutId>:<addr>` | string | `dcc-bus` writes | Per-loco JSON snapshot (`LocoStatePayload`). TTL refreshed on change. |
| `dcc-bus:ports` | hash | `loco-server` | `<layoutId>:<csId>` → `<port>` allocation table; persisted across server restarts. |
| `dcc-bus:<L>:<C>:status` | string | `dcc-bus` | One of `starting` \| `running` \| `draining` \| `degraded`; consumed by `loco-server` for the `system.status` event. |
| `dcc-bus:<L>:<C>:sessions` | hash | `dcc-bus` | `<sessionId>` → `<openedAt,unix>`; lets `loco-server` and the operator inspect active throttles per daemon. |
| `dcc-bus:cmd:<L>:<C>` | pub/sub channel | `loco-server` publishes, `dcc-bus` consumes | Server-initiated DCC commands (scripts, dead-man, takeover-release fan-out, train-level fan-out). See "Command channel" below. |
| `dcc-bus:evt:<L>:<C>` | pub/sub channel | `dcc-bus` publishes, `loco-server` consumes | Outbound throttle events that `loco-server` needs to mirror onto its own WS (cross-tab fan-out, audit fan-in). See "Event channel" below. |
| `bigfred:layout:<L>:allowed_vehicles` | string + pub/sub | `loco-server` publishes | Full drivable-vehicle roster for layout `L`. |
| `bigfred:layout:<L>:defined_trains` | string + pub/sub | `loco-server` publishes | Full train roster for layout `L`. |
| `bigfred:layout:<L>:emergency:<userId>` | pub/sub channel | `loco-server` publishes | Cross-process dead-man's switch fan-out (§7e.5, planned). |

All keys share the Redis instance configured by `--redis-addr` on
`loco-server` and `dcc-bus`. Roster snapshot keys have no TTL;
`loco:state` entries use a short TTL refreshed on each write.
Pub/sub channels are ephemeral by definition.

#### Snapshot on subscribe

When a frontend issues `loco.subscribe { addr }` to the daemon, the
WS handler:

1. Validates authorization (§7e.5: `CanDriveLoco` / read access).
2. Adds the client to the in-memory Hub for `addr`.
3. Reads `HGET loco:state:<csId> <addr>` from Redis; if present,
   immediately emits `loco.state {…}` to that single client. If
   absent, fires a one-shot `Station.GetSpeed(addr)` against the DCC
   bus and broadcasts the result.

This preserves §5.3's promise — "the UI doesn't wait for the poller"
— and stays inside the daemon's own state caches.

#### Command channel (server → daemon)

Throttle write operations originated by frontends arrive directly on
the daemon's WebSocket. Throttle operations originated by **anything
else inside `loco-server`** (scripts, train-wide `train.setSpeed`,
takeover release `SetSpeed(0)`, dead-man's switch fan-out) reach the
daemon via the `dcc-bus:cmd:<L>:<C>` Redis pub/sub channel.

Payload envelope:

```json
{
  "type": "loco.setSpeed" | "loco.toggleFn" | "system.estop",
  "id":   "uuid (for ack via dcc-bus:evt:<L>:<C>)",
  "actor": {
    "userId":   42,
    "source":   "frontend" | "script" | "deadman" | "takeover" | "train",
    "sessionId":"optional; for cross-tab attribution"
  },
  "payload": { "addr": 3, "speed": 64, "forward": true }
}
```

The daemon:

1. **Re-checks the policy** for the `actor.userId` even though
   `loco-server` already evaluated it. Policy evaluation is pure
   and cheap; the duplicate check eliminates a "TOCTOU" between the
   server's decision and the daemon's command write.
2. Invokes the matching `Station` method.
3. Updates the Redis state hash (same path as a frontend write).
4. Publishes the resulting `loco.state` event on both:
   - the in-memory Hub (for connected WS clients on this daemon),
   - the `dcc-bus:evt:<L>:<C>` Redis channel (for `loco-server`'s
     cross-tab fan-out and audit fan-in).

The `source` discriminator is preserved end-to-end so the broadcast
`loco.state.controlledBy` correctly reads `"signalman"` for takeover
writes, `"train"` for train fan-out, `"driver"` for direct writes,
etc. (§4.2 enum).

#### Event channel (daemon → server)

Conversely, every DCC state change observed by the daemon (including
events caused by an external physical throttle the daemon polls but
did not author) is published on `dcc-bus:evt:<L>:<C>`. The server's
`LocoEventConsumer` (lives in `loco-server`, listens on this channel)
mirrors the event onto the server WS for clients who are subscribed
**there** (not the typical throttle client, but e.g. an MCP SSE
session, or the dashboard for some read-only widget). It also writes
audit rows if the event is takeover-relevant (e.g. logs `vehicle.taken_over`).

Throttle audit lines (a driver pressed setSpeed at 11:42:13) are
**not** added to the audit log by default — that would balloon the
table. Only takeover state transitions, emergency-plan executions and
script invocations are audited (existing rule, §3a.5).

#### State reconciliation across daemons

When two daemons share the same command station (different layouts,
same cs), they both poll the bus and they both write to
`loco:state:<csId>`. Conflicting writes:

- For `speed` / `forward`, the **latest** `updatedAt` wins
  (server-side timestamp). Both daemons see the same DCC bus, so the
  read-back via `GetSpeed` converges quickly.
- For `functions`, each daemon polls `ListFunctions` independently;
  the latest `updatedAt` again wins.
- `controlledBy` is daemon-local: each daemon writes only when it
  observed the operation. A cross-daemon takeover does not propagate
  into the other daemon's `controlledBy` because that field is
  scoped to the daemon's own session graph; the cross-bus chip (§3a.4
  rule 9) is the UI element that communicates "another daemon is
  also driving this cs".

#### In-memory state inside the daemon

| Structure | Source | Notes |
|---|---|---|
| `allowed map[addr]` | `allowed_vehicles` snapshot | Gates `loco.subscribe` and estop-all scope. |
| `byAddr map[addr] → AllowedVehicle` | same snapshot | `controllerUserIds` for drive commands. |
| `trains []DefinedTrain` | `defined_trains` snapshot | Train fan-out / future `train.setSpeed` via server command channel. |
| `fnCache` per `(addr, fn)` | local | Avoids duplicate DCC function packets. |
| WS session | JWT at upgrade | `userId`, subscribed addresses, dead-man targets. |

No SQLite connection. No per-row catalogue LRU — the roster maps are
replaced wholesale on each snapshot message.
