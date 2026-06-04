# 4. Uhlenbrock 63120 USB-LocoNet interface

The **Uhlenbrock 63120** is a commercial **USB ↔ LocoNet** bridge with an internal
microcontroller that handles **LocoNet bit timing** on the wire. The host (Pi 5)
sees a **USB CDC serial port** carrying **LocoNet frames** — the same class of
stream as a Digitrax LocoBuffer-USB when configured correctly.

- [Uhlenbrock product information](https://www.uhlenbrock.de/de_DE/produkte/loconet/I000C6F6-001.htm)
- [LocoNet-over-TCP interface comparison](https://loconetovertcp.sourceforge.net/Interface/)

## 4.1 Physical connection

```
Command station LocoNet ── RJ12 ── [optional 62280 Luisa] ── RJ12 ── Uhlenbrock 63120 ── USB 3 ── Pi 5
```

| Rule | Detail |
|------|--------|
| **DR5000:** use **LocoNet-T** | Not LocoNet-B (RailSync / booster port) — §2.3 |
| **RB1110:** | Use **`z21`** (§7), not this adapter |
| **Do not** connect RailSync pins 1 & 6 to the Uhlenbrock 63120 | Data + ground only |
| **Uhlenbrock 63120 is bus-powered** on LocoNet | Central must be **on**; weak bus → **62280** |
| **One** BigFred process opens the USB serial port | No parallel JMRI on the same `/dev/ttyACM*` |

## 4.2 LNCV configuration (required for BigFred)

Factory defaults are often **115200 baud** and **filtered** mode. BigFred needs
**57600 8N1** and a **transparent** stream (LocoNet **Direktmodus**).

Program via Uhlenbrock **LocoNet-Tool** (included with art. **63120**) while the
module is on the powered LocoNet bus.

| LNCV | Set to | Meaning |
|------|--------|---------|
| **2** | **3** | Baud rate **57600** (1=19200, 2=38400, 3=57600, 4=115200) |
| **4** | **1** | **LocoNet Direktmodus** on (transparent / raw stream) |

Reference table from [LocoNet-over-TCP](https://loconetovertcp.sourceforge.net/Interface/) (mode “LocoNet Direktmodus”).

### Mode comparison

| LNCV 4 | Name | BigFred |
|--------|------|---------|
| **1** | LocoNet Direktmodus | **Use this** — raw frames to/from USB |
| **0** | Only valid messages | Filters like LocoBuffer-USB; may hide rare traffic |

Wrong baud or wrong mode produces **garbage or missing frames** in BigFred — not
a Pi 5 defect.

## 4.3 Protocol contract with BigFred

BigFred `loconet_serial` expects:

1. **57600**, 8 data bits, no parity, 1 stop bit.
2. **Raw bytes** per LocoNet message (opcode … checksum inclusive).
3. **No** ASCII `SEND` / `RECEIVE` lines (that is `loconet_tcp` / LbServer).

Checksum: XOR of all bytes including checksum byte = **`0xFF`**
([`loconet_proto.go`](../../pkgs/loco/commandstation/loconet_proto.go)).

Example idle frame: `83 7C`.

## 4.4 LocoNet frame loss — real or not?

**Yes, it can happen**, but with **correct LNCV** and a healthy bus it should be
**uncommon** on Pi 5 + Uhlenbrock 63120 for normal throttle hub load.

### Where frames are lost

```text
LocoNet bus (collisions, weak signal)
        ↕ Uhlenbrock 63120 MCU
USB cdc_acm kernel buffer
        ↕
BigFred readLoop → rxCh (depth 64) → dispatch
```

| Layer | Typical cause on this setup |
|-------|---------------------------|
| **LocoNet bus** | Too many devices, poor power, long unrefreshed segment — **not Pi-specific** |
| **Uhlenbrock 63120** | Extreme bus traffic vs USB bandwidth; wrong mode filters packets |
| **USB / Pi** | `readLoop` blocked while `rxCh` full (64 packets) — rare at throttle rates |
| **BigFred** | **Bad checksum** dropped intentionally; full `obsCh` drops UI updates only |

Pi 5 USB 3 is **not** the weak link versus older SBCs. **Misconfigured LNCV** is
the most common self-inflicted issue.

### Symptoms

- `loconet serial: dropping packet (bad checksum)` in logs
- `timeout waiting for slot` on `SetSpeed`
- Throttle UI lagging while bus is busy

### Mitigations

1. LNCV **57600** + **Direktmodus** (§4.2).
2. **62280 (Luisa)** on long or heavily loaded LocoNet branches.
3. Short **USB** cable; **performance** governor / RT kernel (§3).
4. Avoid opening the serial port with other tools while `dcc-bus` runs.
5. On **DR5000**, compare once with **USB LocoNet** on the central — if stable
   there but not via Uhlenbrock 63120, suspect the **T-bus segment** to the Uhlenbrock 63120, not the Pi.

## 4.5 Optional: 62280 (Luisa) before Uhlenbrock 63120

If the Uhlenbrock 63120 sits on a **long** LocoNet run with many modules, put **62280**
between the command station and the Uhlenbrock 63120 branch:

- Regenerates signals
- **12 V / 500 mA** for the secondary segment
- Galvanic isolation — short on secondary does not kill primary T bus

BigFred behaviour is the **same**; Luisa improves **power and signal**, not the
hub protocol.

## 4.6 Linux device path

After udev (§3.5):

| Field | Value |
|-------|-------|
| Device | `/dev/loconet-63120` or `/dev/ttyACM0` |
| BigFred URI | `serial:///dev/loconet-63120:57600` |

Continue with [§5 BigFred integration](./05-bigfred-integration.md).
