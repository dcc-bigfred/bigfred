# 1. Overview & architecture

## 1.1 Roles on the layout

Every deployment has:

| Component | Role |
|-----------|------|
| **Command station** | DCC master, slot server, track power policy |
| **BigFred on Pi 5** | Multi-user hub (HTTP/WS + `dcc-bus`), SQLite, Redis |

Depending on the central, BigFred talks to it through **one** of:

| Link | Hardware | BigFred kind |
|------|----------|--------------|
| **LocoNet serial** | **Uhlenbrock 63120** (USB) on LocoNet | `loconet_serial` |
| **Z21 LAN** | Ethernet/WiFi to the central | `z21` |

The Uhlenbrock 63120 is only used on the **LocoNet** path. It does not replace the command
station — it is a throttle-class LocoNet node whose host process is `dcc-bus`.

## 1.2 Officially supported command stations

| Model | Recommended connection | Official support |
|-------|------------------------|------------------|
| **Digikeijs DR5000** | **`loconet_serial`** + Uhlenbrock 63120 on **LocoNet-T** | Yes — §2–§6 |
| **RailBOX RB1110** / **RB1110-Mini** | **`z21`** UDP (LAN/WiFi) | Yes — [§7](./07-z21-command-stations.md) |
| **Other centrals** with **Z21 and LocoNet** | Prefer **`z21`** when documented; else Uhlenbrock 63120 + LocoNet | **Best-effort only** — no guarantee |

BigFred does not replace vendor mobile apps; it adds a **web throttle hub** on the
same layout.

### RB1110 and Z21

RailBOX positions **Z21®** as the integration path for PC/tablet software. For
RB1110 / RB1110-Mini hubs, **do not** plan on `loconet_serial` + Uhlenbrock 63120 unless you
are explicitly experimenting — use **`z21`** per [§7](./07-z21-command-stations.md).

The RB1110 **LocoNet** RJ12 remains for handhelds and modules; BigFred on Z21
does not require a cable from the Pi to that port.

## 1.3 Data path — LocoNet (DR5000)

```
Browsers ──HTTP/WS──► loco-server (Pi 5)
                          │
                          ▼
                     dcc-bus ──► /dev/loconet-63120 @ 57600 8N1
                          │
                     Uhlenbrock 63120 (USB 3)
                          │  RJ12 LocoNet-T
                     DR5000 ──► DCC ──► layout
```

- **Web → track:** LocoNet `OPC_LOCO_SPD` / `OPC_LOCO_DIRF` via
  [`loconet.go`](../../pkgs/loco/commandstation/loconet.go).
- **Track → web:** Shared LocoNet bus → Uhlenbrock 63120 → `observe()`.

## 1.4 Data path — Z21 (RB1110)

```
Browsers ──HTTP/WS──► loco-server (Pi 5)
                          │
                          ▼
                     dcc-bus ──► udp://<rb1110-ip>:21105
                          │
                     RB1110 (LAN/WiFi) ──► DCC ──► layout
```

- **Web → track:** Z21 `LAN_X_SET_LOCO_*` via [`z21.go`](../../pkgs/loco/commandstation/z21.go).
- **Track → web:** Z21 `LAN_X_LOCO_INFO` broadcast after driver subscribe.

Details: [§7](./07-z21-command-stations.md).

## 1.5 Why BigFred is the hub (not the interface)

Whether LocoNet or Z21, BigFred adds:

- Many **WebSocket clients** on one `loco-server`.
- **Sessions, takeover, leases** above one `dcc-bus` daemon per command station.
- **One writer** to each command station (serial port or UDP session policy).

The Uhlenbrock 63120 or Z21 link only carries station protocol traffic; Pi 5 runs hub logic
and persistence.

## 1.6 Concurrency responsibilities

| Concern | LocoNet path | Z21 path |
|---------|--------------|----------|
| Many drivers, different locos | BigFred sessions | Same |
| LocoNet collisions | Bus + Uhlenbrock 63120 MCU | N/A |
| Z21 broadcast / subscribe | N/A | `Z21Roco` readLoop |
| Slot limits | Command station | Command station |
| Backpressure | USB `cdc_acm`, `rxCh` | UDP socket, observation channel |

## 1.7 Why Pi 5 + SSD + RT (both paths)

| Goal | How |
|------|-----|
| **Reliability** | Vendor interfaces only |
| **Low I/O latency** | NVMe **`/data`** for SQLite/Redis ([§8](./08-hub-os-image.md)) |
| **Scheduling** | PREEMPT_RT, `isolcpus`, `taskset` on hub image |
| **Separation** | Pi in rack; central at layout |
| **Operations** | RO root, watchdog, fanctl, Alloy |

LocoNet path also benefits from stable **USB 3** to the Uhlenbrock 63120. Z21 path needs
stable **LAN** to the central (§7.3). OS image build: [§8](./08-hub-os-image.md).

### DR5000 USB LocoNet (alternative)

The DR5000 can expose LocoNet on **USB** virtual COM. BigFred can use that, but
this set standardises on **Uhlenbrock 63120 on the Pi** so the central's USB stays free.

### Dual-protocol centrals without guarantee

Centrals that offer **both** Z21 and LocoNet may work with either BigFred kind.
Compatibility depends on firmware and how closely the vendor implements each
protocol. Only **DR5000** (LocoNet) and **RB1110** (Z21) are officially supported;
all other dual-protocol combinations are **best-effort** — see [§7.5](./07-z21-command-stations.md#75-dual-protocol-centrals-best-effort).

### Out of scope

Custom LocoNet adapters (RP2040, LocoLinx) are not documented here.

Continue with [§2 LocoNet electrical](./02-loconet-electrical.md) (DR5000 path) or
[§7 Z21](./07-z21-command-stations.md) (RB1110 path).
