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

Program while the module is on a **powered LocoNet bus** (command station on),
using one of:

| Tool | Platform |
|------|----------|
| Uhlenbrock **LocoNet-Tool** (bundled with art. **63120**) | Windows |
| **`rb lncv`** (BigFred CLI) | Linux / macOS / Windows |

| LNCV | Set to | Meaning |
|------|--------|---------|
| **2** | **3** | Baud rate **57600** (1=19200, 2=38400, 3=57600, 4=115200) |
| **4** | **1** | **LocoNet Direktmodus** on (transparent / raw stream) |

Reference table from [LocoNet-over-TCP](https://loconetovertcp.sourceforge.net/Interface/) (mode “LocoNet Direktmodus”).

### 4.2.1 `rb lncv` on Linux

The **`rb`** CLI implements the Uhlenbrock LNCV protocol (same message layout as
[JMRI `LncvMessageContents`](https://github.com/JMRI/JMRI/blob/master/java/src/jmri/jmrix/loconet/uhlenbrock/LncvMessageContents.java)).
Use it when LocoNet-Tool is not available.

**Prerequisites**

- Command station **on**; 63120 on **LocoNet-T** (DR5000) or the correct LocoNet
  port for your central — **not** RailSync / booster (B) pins.
- **LocoNet LED** on the 63120 **blinks** on bus activity. A **solid** LocoNet LED
  means no valid bus traffic — fix wiring and power before LNCV programming.
- Host user can open the USB serial device (`dialout` group or udev rule).
- No other program holds the port (JMRI, `xxd`, `minicom`, `dcc-bus`).

**Read module address (CV0)**

```bash
rb lncv get --device /dev/ttyUSB0 --baud 115200 --article 63120 --addr 1 0
```

Expect `1` for a factory-default 63120. Article `63120` is normalised to **6312**
internally.

**Set Direktmodus and 57600 baud**

Writes to the adapter's **own** configuration (CV2, CV4) reconfigure the USB link
and are **not acknowledged** on the wire. Use **`--self-config`**:

```bash
# Still at factory 115200 — set Direktmodus first
rb lncv set --self-config --device /dev/ttyUSB0 --baud 115200 --article 63120 4 1

# Set baud to 57600 (LNCV 2 = 3); reconnect at 57600 for verification
rb lncv set --self-config --device /dev/ttyUSB0 --baud 115200 --article 63120 2 3

rb lncv get --device /dev/ttyUSB0 --baud 57600 --article 63120 2
rb lncv get --device /dev/ttyUSB0 --baud 57600 --article 63120 4
```

On success, `--self-config` prints that the value was **sent** and the adapter
applies it without a LocoNet acknowledge — reconnect at the new baud to verify.

**Useful flags**

| Flag | Default | Purpose |
|------|---------|---------|
| `--device` | from `~/.loco.yaml` | Serial path (`/dev/ttyUSB0`, `/dev/ttyACM0`, `/dev/loconet-63120`) |
| `--baud` | `115200` | Current USB baud (must match LNCV 2) |
| `--article` / `-a` | `6312` | LNCV article (`63120` accepted) |
| `--addr` | `1` | Module address on LocoNet (LNCV 0) |
| `--self-config` | off | Adapter self-configuration (CV2/CV4); no LACK expected |
| `--timeout` | `4` | Response timeout in seconds |
| `-v` / `--debug` | off | Log TX/RX hex |

**Diagnostic messages**

| Message | Likely cause |
|---------|----------------|
| `no bytes received from the adapter` | Dead or unpowered LocoNet bus, wrong port, solid LocoNet LED, or wrong USB baud |
| `adapter saw bus traffic but did not echo our frames` | Module/article not present, or programming not supported on this path |
| `timeout waiting for LNCV write acknowledge` (without `--self-config`) | Bus busy, module rejected write, or session left open — power-cycle 63120 and retry |
| `LNCV write sent to adapter … reconnect with the new settings` (`--self-config`) | Normal for CV2/CV4 — verify after reconnect |

The 63120 handbook (§5) requires **echo-based flow control**: each frame sent over
USB is echoed back from the bus before the next send. `rb lncv` follows this for
programming sessions and always sends **prog-end** so the adapter is not left in
programming mode.

**Limitation (JMRI / handbook):** the 63120's **own** LNCVs are easiest to
program from a **LocoNet throttle** on the bus. Programming via the adapter's USB
port can work for reads and self-config writes, but the link may drop briefly
while CV2/CV4 are applied — plan for a USB reconnect and baud change.

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
| Device | `/dev/loconet-63120`, `/dev/ttyACM0`, or `/dev/ttyUSB0` (depends on USB bridge chip) |
| BigFred URI | `serial:///dev/loconet-63120:57600` |

Identify the device after plug-in:

```bash
ls -l /dev/ttyACM* /dev/ttyUSB* 2>/dev/null
dmesg | tail -20
```

`permission denied` → add user to **`dialout`** or fix udev `GROUP`/`MODE`.

Continue with [§5 BigFred integration](./05-bigfred-integration.md).
