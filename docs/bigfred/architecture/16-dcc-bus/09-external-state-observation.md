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
| Z21 (Roco) | UDP | **Yes — implemented** | The driver sends `LAN_SET_BROADCASTFLAGS` (`0x50`) with flags `0x00000001 | 0x00010000`, after which the Z21 pushes unsolicited `LAN_X_LOCO_INFO` (`0xEF`) for **every modified loco** (FW ≥ 1.20) — including changes made by external handsets. The driver was refactored to a single demuxing read loop (the same pattern as LocoNet). |

**Conclusion.** Subscription/push is implemented for **all** current
drivers (LocoNet via the shared bus, Z21 via broadcast flags). The
polling fallback remains in the daemon for any future driver that cannot
push. The capability is expressed as an **optional Go interface** so the
daemon can choose per driver without the `Station` contract growing a
method every driver must stub out.

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
full F0..F31 set at once.

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

#### Z21 driver internals (push)

The Z21 is a single UDP socket. To support push it was refactored to the
same shape as LocoNet:

- A single **read-loop goroutine** owns `conn.Read`. Each UDP datagram is
  split into the concatenated Z21 packets it may carry (each is
  length-prefixed by its little-endian `DataLen`; the station batches
  several `LAN_X_LOCO_INFO` frames into one datagram).
- Every `LAN_X_LOCO_INFO` (`0xEF`) packet — solicited or unsolicited —
  is parsed (address from DB0/DB1, speed/direction from DB3, functions
  from DB4..DB8) and emitted as a `LocoObservation`.
- Synchronous request/response sequences (`ReadCV`, `WriteCV`,
  `GetSpeed`, `ListFunctions`) no longer read the socket directly: they
  set a `syncActive` flag under `ioMu`, write the request, and wait on a
  forwarding channel fed by the read loop. This is the LocoNet pattern,
  so the observer can never steal a reply from an in-flight query.
- **`ObserveStates`** lazily enables the broadcast (`LAN_SET_BROADCASTFLAGS`,
  flags `0x00000001 | 0x00010000`) on first call, so push is off until a
  consumer actually wants it (the standalone CLI never enables it).
- **`SubscribeLocoInfo`** (the optional `LocoInfoSubscriber` capability)
  sends `LAN_X_GET_LOCO_INFO` (§4.1) for a given address. This is the
  crucial part for **older firmware**: flag `0x00010000` ("push for *all*
  modified locos", §2.16) only exists from **FW ≥ 1.24**, and even the
  base flag `0x00000001` delivers `LAN_X_LOCO_INFO` **only for addresses
  the client subscribed to**. So on a pre-1.24 Z21 an external handset
  moving a loco the daemon never queried is invisible unless the daemon
  first subscribes it. `SubscribeLocoInfo` is fire-and-forget — the read
  loop turns the reply into a normal observation.

> **Symptom this fixes.** Drive a loco to speed 18 in BigFred, then stop
> it from an external Z21 app (e.g. `loco speed set -l 4 0` or a phone
> app). The loco stops on the track but BigFred's slider stayed at 18,
> while the external app correctly showed 0 — because that app had
> *subscribed* to the loco and `dcc-bus` had not. Explicit subscription
> makes `dcc-bus` receive the same `LAN_X_LOCO_INFO` and drop to 0.

Two robustness details carried over from the earlier polling work:

- **`ioMu`** still serializes the request/await pairs so two callers
  cannot interleave their sync windows.
- **`ReadInfoTimeout`** (default `1.5s`) bounds `GetSpeed` /
  `ListFunctions` separately from the slow CV programming timeout.

#### Z21 drive encoding — speed, direction, and stop (§4.2 / §4.4)

BigFred's throttle API uses the same speed semantics everywhere:
`0` = normal stop, `1` = emergency stop, `2+` = drive steps (up to the
loco's configured step count). On the wire the Z21 packs speed and
direction into a single byte `DB3 = RVVVVVVV` (§4.2
`LAN_X_SET_LOCO_DRIVE`, §4.4 `LAN_X_LOCO_INFO`):

| Field | Meaning |
|---|---|
| **R** (bit 7) | Direction: `1` = forward, `0` = reverse |
| **V** (bits 0–6) | Speed value; encoding depends on the step mode |

The driver implements this in `encodeLocoDriveDB3` (outbound SET) and
`decodeLocoDriveFromLocoInfo` (inbound INFO / push). Unit tests live in
`pkgs/loco/commandstation/z21_drive_decode_test.go`.

**Direction is always encoded in R — including at stop.** A common
misread of the spec is to treat "Stop" as a direction-neutral pattern
(`0x00` regardless of intent). On the wire `0x00` is `R=0` (reverse) +
`V=0` (stop). Sending that while the UI shows "forward" commands the Z21
to stop *and* flip direction, which is exactly what users saw after
sliding to zero. The correct encodings are:

| Intent | 128-step `DB3` | Notes |
|---|---|---|
| Stop, forward | `0x80` | `R=1`, `V=0` |
| Stop, reverse | `0x00` | `R=0`, `V=0` |
| E-stop, forward | `0x81` | `R=1`, `V=1` |
| E-stop, reverse | `0x01` | `R=0`, `V=1` |
| Drive step *n* ≥ 2 | `R \| (n & 0x7F)` | `R=0x80` when forward |

For 28-step mode the `V` field uses the interleaved `V5` bit (NMRA
S 9.2.1); stop / e-stop still keep direction in **R** — the decoder maps
the interleaved raw values `0/1` and `2/3` to API speeds `0` and `1`
without discarding `R`.

**SET vs. INFO step-mode field — do not mix them up.** The Z21 spec uses
*different* encodings for the speed-step selector on commands vs.
replies:

| Message | Field | DCC 128 steps |
|---|---|---|
| `LAN_X_SET_LOCO_DRIVE` (§4.2) | `DB0` low nibble **S** | **S = 3** → `DB0 = 0x13` |
| `LAN_X_LOCO_INFO` (§4.4) | `DB2` low bits **KKK** | **KKK = 4** → `DB2 = …04` |

`SetSpeed` maps the API value `speedSteps: 128` to **S = 3** when
building SET packets. `encodeLocoDriveDB3` is called with that SET
proto value (`3`). `decodeLocoDriveFromLocoInfo` reads replies with
**KKK = 4**. Sending **S = 4** (the INFO value) on SET is undefined and
caused the Z21 to mis-drive locos (e.g. not stopping when the slider
was at zero).

**Push observations carry direction from R.** Every parsed
`LAN_X_LOCO_INFO` sets `HasForward` from bit 7 of `DB3`. The state feed
(`applyObservation`) merges `forward` normally — there is no special
"ignore direction while stopped" rule; once SET encoding preserves R at
stop, push and poll both report the same direction the throttle last
commanded.

Reference: Roco Z21 LAN protocol (`docs/z21.html`), sections **4.2**
(SET) and **4.4** (INFO).

#### Daemon wiring

`Router.RunStateFeed(ctx)` runs in its own goroutine (started in
`Daemon.Run`). It selects push vs. polling once at startup and feeds both
into `applyObservation`, the reconciler described in §7e.3 ("State feed —
external-throttle visibility"). Both current drivers (LocoNet, Z21)
implement `StateObserver`, so the polling branch only runs for a future
driver that cannot push. The fallback cadence is set with
`--poll-interval-ms` (0 → `750ms` default).

When the driver also implements `LocoInfoSubscriber` (Z21), the push path
starts a **subscription refresh** goroutine (`runSubscriptionRefresh`)
that every `5s` re-subscribes the command station to each address with at
least one live WS subscriber (`Hub.SubscribedAddrs`, filtered by layout
authorization). Re-subscribing is cheap and idempotent; the interval
keeps the subscription alive across the Z21's per-client 16-address FIFO
and client time-out. LocoNet does not implement the capability (the
shared bus shows every packet) so no refresh runs for it.

#### Limitations / future work

- **Z21 firmware.** The "all modified locos" broadcast flag `0x00010000`
  is FW ≥ 1.24 only. To stay firmware-independent the daemon also
  explicitly subscribes each watched loco via `LAN_X_GET_LOCO_INFO`
  (`SubscribeLocoInfo`, refreshed every `5s`), which works under the base
  flag `0x00000001` on all firmware. Locos with no live WS subscriber are
  not subscribed, so a change there is only picked up once someone starts
  watching (or, on FW ≥ 1.24, immediately via the all-locos flag).
- **Z21 broadcast persistence.** Broadcast flags are per-client and lost
  if the Z21 reboots; the daemon sets them once at feed startup. A
  periodic refresh is a possible future hardening.
- **LocoNet function range.** External F0..F8 changes are observed
  (basic slot DIRF/SND). F9+ requires the IMM_PACKET / extended-function
  decode, not implemented (mirrors the existing `SendFn` F0..F8 limit).
- **Polling function range.** The fallback reconciles F0..F28 explicitly
  so an external *off* is detected, not only *on*.