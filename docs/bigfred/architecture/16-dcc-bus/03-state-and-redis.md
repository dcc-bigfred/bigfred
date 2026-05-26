### §7e.3 State & Redis cache

#### Inputs

`dcc-bus` consumes three kinds of input and produces one kind of
output:

```
                   ┌──────────────────────────────────────────────────┐
                   │                  dcc-bus                          │
                   │                                                  │
       SQLite ────►│  catalogue: Vehicle, VehicleFunction, Train,     │
       (read-     │              LayoutVehicle, VehicleLease, …      │
       only)       │                                                  │
                   │                                                  │
       Redis ─────►│  pub/sub: invalidations + server-initiated cmds  │
       pub/sub     │                                                  │
                   │                                                  │
       command  ──►│  WebSocket frontend clients                      │
       station     │  (loco.* and system.estop)                       │
       (DCC,        │                                                  │
       bidir)     ──┴──►  state cache + audit events                  │
                   │                                                  │
                   └────────┬─────────────────────────────────────────┘
                            │
                            ▼
                          Redis (hash + pub/sub)
                            │
                            ▼
                   loco-server (audit log, snapshot serving)
                   peer dcc-bus  (cross-bus reconciliation)
                   browsers     (snapshot on subscribe)
```

#### Vehicle subscription set

The daemon's *interesting set* is the union of:

1. `Vehicle` rows from `LayoutVehicle` where `LayoutID == --layout-id`
   **and** `Vehicle.DCCAddress IS NOT NULL` (dummies are skipped — they
   never reach DCC, §3a.3 invariants).
2. *(when `Layout.IsSystem == true`)* every `Vehicle` row in the
   global catalogue with a non-NULL DCC address.

The interesting set is loaded into memory on boot and refreshed
incrementally via Redis pub/sub events published by `loco-server`:

| Channel | Payload | Daemon reaction |
|---|---|---|
| `bigfred:layout:<L>:vehicles` | `{ action:"added"\|"removed", vehicleAddr, vehicleId }` | Subscribe / unsubscribe the address with the poller and the WS Hub fan-out. |
| `bigfred:vehicle:<id>:functions` | `{ vehicleId, addr }` | Invalidate function definition cache; broadcast `vehicle.functionsChanged { addr }` to WS subscribers. |
| `bigfred:vehicle:<id>:invalidated` | `{ vehicleId, addr }` | Re-read the vehicle row from SQLite (covers rename, kind change, DCC-address change, deletion). |
| `bigfred:vehicle:<id>:lease` | `{ vehicleId, leaseState }` | Invalidate the cached lease for the address; next authorization re-checks the policy. |
| `bigfred:vehicle:<id>:takeover` | `{ vehicleId, state }` | Same as above. Also push an updated `loco.state { controlledBy }` to subscribed clients. |
| `bigfred:layout:<L>:roster_full_invalidate` | – | Re-read the entire roster from SQLite. Emergency catch-all; used by `loco-server` after restoring from backup. |

The pub/sub layer is *advisory*: every authorization decision **also**
re-reads the relevant row from SQLite. Redis is the latency
optimisation; SQLite is the source of truth.

#### Poller

A single poller goroutine ticks at `--poll-interval` and, for every
address in the *interesting set with at least one WS subscriber*,
issues `Station.GetSpeed(addr)` and `Station.ListFunctions(addr)` (the
existing `commandstation.Station` API). For each address it:

1. Compares against the last cached value.
2. If changed, writes the new value to Redis
   (`HSET loco:state:<csId> <addr> "{json}"`) and publishes to the
   in-memory bus.
3. The WS Hub fans the event out as `loco.state { addr, speed, forward,
   functions, updatedAt, controlledBy }` to every subscriber of `addr`.

The poller skips addresses with **no current subscribers** to avoid
useless DCC traffic. As soon as the first subscribe arrives for an
address, the poller adds it to its rotation and issues an immediate
`GetSpeed` to populate the snapshot.

`controlledBy` is computed inside the daemon from:

- the most recent `setSpeed` / `toggleFn` caller (in-memory),
- pub/sub-delivered takeover state,
- explicit re-broadcasts from `loco-server` (see §7e.5).

Polling errors (`commandstation.Station` returning `error`) are
counted; after `N` consecutive errors on an address (default `3`),
the daemon emits `loco.error { addr, code:"poll_failed", message }`
and marks the address as **unsynced** in Redis (`HSET
loco:state:<csId>:meta <addr>:unsynced "1"`). The next successful
poll clears the flag.

#### Redis key layout

| Key | Type | Owner | Purpose |
|---|---|---|---|
| `loco:state:<csId>` | hash | `dcc-bus` writes, `loco-server` and peer daemons read | `<addr>` → `{speed,forward,functions,controlledBy,updatedAt}` JSON. Latest known truth from the DCC bus. |
| `loco:state:<csId>:meta` | hash | `dcc-bus` | `<addr>:unsynced` → `"1"` while polling is failing. Also `<addr>:lastPolledAt`. |
| `dcc-bus:ports` | hash | `loco-server` | `<layoutId>:<csId>` → `<port>` allocation table; persisted across server restarts. |
| `dcc-bus:<L>:<C>:status` | string | `dcc-bus` | One of `starting` \| `running` \| `draining` \| `degraded`; consumed by `loco-server` for the `system.status` event. |
| `dcc-bus:<L>:<C>:sessions` | hash | `dcc-bus` | `<sessionId>` → `<openedAt,unix>`; lets `loco-server` and the operator inspect active throttles per daemon. |
| `dcc-bus:cmd:<L>:<C>` | pub/sub channel | `loco-server` publishes, `dcc-bus` consumes | Server-initiated DCC commands (scripts, dead-man, takeover-release fan-out, train-level fan-out). See "Command channel" below. |
| `dcc-bus:evt:<L>:<C>` | pub/sub channel | `dcc-bus` publishes, `loco-server` consumes | Outbound throttle events that `loco-server` needs to mirror onto its own WS (cross-tab fan-out, audit fan-in). See "Event channel" below. |
| `bigfred:layout:<L>:vehicles` | pub/sub channel | `loco-server` publishes | Roster mutations. |
| `bigfred:vehicle:<id>:*` | pub/sub channel | `loco-server` publishes | Per-vehicle invalidations (functions, lease, takeover, definition). |
| `bigfred:layout:<L>:emergency:<userId>` | pub/sub channel | `loco-server` publishes | Cross-process dead-man's switch fan-out (§7e.5). |

All keys live in the `--redis-db` index. TTL is `0` (forever) for
state hashes; pub/sub channels are ephemeral by definition.

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

#### Memory caches inside the daemon

| Cache | Refresh trigger | Eviction policy |
|---|---|---|
| `Vehicle` by `(layoutId, addr)` | pub/sub: `vehicle:invalidated`; falls back to SQLite read on miss | LRU, 4096 entries |
| `VehicleLease` (active) by `vehicleId` | pub/sub: `vehicle:lease`; falls back to SQLite read | LRU, 4096 |
| `TakeoverRequest` (active) by `(vehicleId)` and `(trainId)` | pub/sub: `vehicle:takeover` | LRU, 4096 |
| `VehicleFunction` definitions by `vehicleId` | pub/sub: `vehicle:functions` | LRU, 4096 |
| `User` by `userId` | none — read fresh per WS upgrade, kept until WS closes | per-WS-session |
| `Layout` row | none — read once at boot | per-daemon |

A cache miss reads SQLite; SQLite-WAL allows the read while
`loco-server` writes. Refreshes are advisory only — the
authorization re-check is always against the freshly resolved object.
