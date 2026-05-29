### §7e.9 External state observation (subscription vs. polling)

#### Why

A command station is **shared hardware** (§3a.4 rule 9). The Z21 or
LocoNet master that this daemon owns can be driven simultaneously by:

- BigFred throttles (via this `dcc-bus`),
- BigFred scripts / train fan-out / takeover (via the command channel,
  §7e.3),
- **external physical throttles** plugged directly into the command
  station — a Roco multiMaus on the Z21, a hand-held on the LocoNet bus.

The last case is invisible to BigFred unless the daemon actively watches
the bus. This subsection specifies how `dcc-bus` reflects those external
speed / direction / function changes into the throttle UI, and the
driver-capability research behind the two implementations.

#### Driver-capability research

The question: can `pkgs/loco/commandstation` learn about state changes
it did not author, ideally by **subscription/push** rather than polling?

| Driver | Transport | Can observe external changes? | Mechanism |
|---|---|---|---|
| LocoNet serial | RS-232 / USB UART | **Yes — natively** | LocoNet is a *shared bus*: every device sees every `OPC_LOCO_SPD` (`0xA0`), `OPC_LOCO_DIRF` (`0xA1`), `OPC_LOCO_SND` (`0xA2`) and slot-read (`OPC_SL_RD_DATA`, `0xE7`) packet. The driver already runs a read-loop goroutine; it just had to demux unsolicited traffic. |
| LocoNet TCP | LoconetOverTcp | **Yes — natively** | Same shared-bus semantics; the `RECEIVE …` lines carry every other throttle's packets. |
| Z21 (Roco) | UDP | **Yes — in principle** | The Z21 LAN protocol supports `LAN_SET_BROADCASTFLAGS` (`0x50`) + per-loco `LAN_X_GET_LOCO_INFO` subscription, after which the station pushes unsolicited `LAN_X_LOCO_INFO` (`0xEF`) on any change. **Not implemented yet**: the current driver does synchronous request/response reads on the single UDP socket, so adding a push reader needs a demuxing refactor. Until then the Z21 uses the polling fallback. |

**Conclusion.** Subscription is the right model where the transport
supports it cheaply (LocoNet, today). Where it does not (Z21, today), we
fall back to polling. The capability is expressed as an **optional Go
interface** so the daemon can choose per driver without the `Station`
contract growing a method every driver must stub out.

#### Capability contract (`pkgs/loco/commandstation`)

```go
// Optional: implemented by drivers that can push state changes.
type StateObserver interface {
    ObserveStates() <-chan LocoObservation
}

// A (possibly partial) state change observed on the bus. Only fields
// whose Has* flag is set are meaningful; the consumer merges the delta
// onto the last known snapshot. Speed is in the same units GetSpeed
// returns.
type LocoObservation struct {
    Addr       LocoAddr
    HasSpeed   bool
    Speed      uint8
    HasForward bool
    Forward    bool
    Functions  map[int]bool // function -> on, for the bits this update carries
}
```

Partial updates matter: a LocoNet `OPC_LOCO_SPD` carries only speed,
`OPC_LOCO_DIRF` carries direction + F0..F4, `OPC_LOCO_SND` carries
F5..F8; a slot read carries all of it. The Z21 `LAN_X_LOCO_INFO` (and
the polling fallback that emulates it) carries speed + direction + the
full function set at once.

Callers MUST type-assert and degrade gracefully:

```go
if obs, ok := station.(commandstation.StateObserver); ok {
    // consume obs.ObserveStates()
} else {
    // poll GetSpeed / ListFunctions
}
```

#### LocoNet driver internals (push)

`LocoNet` now has a single **dispatch goroutine** that owns the
transport's receive channel. For every packet it:

1. Updates the slot/dirf/snd caches and the reverse `slot → addr` map
   (needed to attribute slot-keyed `SPD`/`DIRF`/`SND` traffic).
2. Emits a `LocoObservation`.
3. Forwards the packet to the request/response waiter **only while a
   synchronous sequence is in flight** (a `syncActive` flag set around
   `ensureSlot` / `querySlot` under the existing request mutex). This
   keeps unsolicited bus traffic from piling up in the waiter channel
   while nobody is requesting, and prevents the observer from stealing a
   response packet from an in-flight `GetSpeed`.

#### Z21 driver internals (polling fallback, made safe)

The Z21 driver gained two robustness changes required to be polled
safely from a background goroutine while WS clients write concurrently:

- **`ioMu`** serializes every request/response sequence. The Z21 is one
  UDP socket; concurrent readers would split datagrams between
  goroutines and corrupt each other's responses.
- **`ReadInfoTimeout`** (default `1.5s`) bounds the `LAN_X_GET_LOCO_INFO`
  reads (`GetSpeed` / `ListFunctions`) separately from the slow CV
  programming timeout. The poll loop therefore cannot stall the shared
  socket — and with it every throttle write — for the full 10s CV
  timeout when a loco is unresponsive.

#### Daemon wiring

`Router.RunStateFeed(ctx)` runs in its own goroutine (started in
`Daemon.Run`). It selects push vs. polling once at startup and feeds
both into `applyObservation`, the reconciler described in §7e.3 ("State
feed — external-throttle visibility"). The polling cadence is set with
`--poll-interval-ms` (0 → `750ms` default).

#### Limitations / future work

- **Z21 push.** Implementing `StateObserver` for the Z21 via
  `LAN_SET_BROADCASTFLAGS` would remove the poll latency and the
  per-subscriber DCC traffic. It requires refactoring the Z21 driver to
  a single demuxing reader (the same pattern LocoNet now uses). Tracked
  as a follow-up; the interface is already in place.
- **LocoNet function range.** External F0..F8 changes are observed
  (basic slot DIRF/SND). F9+ requires the IMM_PACKET / extended-function
  decode, not implemented (mirrors the existing `SendFn` F0..F8 limit).
- **Polling function range.** The fallback reconciles F0..F28 explicitly
  so an external *off* is detected, not only *on*.