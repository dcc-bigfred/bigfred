# Uhlenbrock 63120 USB-LocoNet interface

Reference for the **Uhlenbrock USB-LocoNet interface** (art. **63120**) in the
context of the BigFred hub. Manufacturer handbook:
[Bes63120e.pdf (English)](https://www.uhlenbrock.de/de_DE/service/download/handbook/en/Bes63120e.pdf).

Product page:
[Uhlenbrock I000C6F6-001](https://www.uhlenbrock.de/de_DE/produkte/loconet/I000C6F6-001.htm).

BigFred terminology: *command station* / *centralka* — see
[`00-terminology.md`](../architecture/00-terminology.md).

---

## 1. Role in BigFred

The 63120 is a **USB ↔ LocoNet bridge**. Its internal microcontroller handles
**LocoNet bit timing** on the wire; the Pi host sees a **USB serial port** carrying
LocoNet frames. BigFred uses it on the **Digikeijs DR5000** path:

```
loco-server / dcc-bus ──► /dev/loconet-63120 @ 57600 8N1
                              │
                         Uhlenbrock 63120 (USB)
                              │
                         RJ12 ──► command station LocoNet-T (e.g. DR5000)
```

| Field | BigFred value |
|-------|---------------|
| Command station kind | `loconet_serial` |
| Connection URI | `serial:///dev/loconet-63120:57600` |

The 63120 does **not** replace the command station — it is a **throttle-class**
LocoNet node whose host process is `dcc-bus`. It is **not** used with
**RailBOX RB1110** (use **`z21`** on LAN instead).

Deep-dive on wiring, udev, and frame loss:
[`loconet-adapter/04-uhlenbrock-63120.md`](../../loconet-adapter/04-uhlenbrock-63120.md).

---

## 2. Product variants

| Art. no. | Contents |
|----------|----------|
| **63120** | USB-LocoNet interface + **LocoNet-Tool** software + USB and LocoNet cables + handbook |
| **63130** | USB-LocoNet interface **without** LocoNet-Tool |
| **61070** | Replacement USB cable |

LocoNet-Tool is required to program **LNCV** values on the interface itself (§5)
and to configure Uhlenbrock LocoNet modules (feedback, switches, LISSY, …).

---

## 3. Characteristics (manufacturer)

From the Uhlenbrock handbook:

| Feature | Detail |
|---------|--------|
| **Galvanic isolation** | Between PC (USB) and LocoNet |
| **Power** | From **LocoNet bus** and/or **USB** |
| **USB serial baud rates** | **19200**, **38400**, **57600**, **115200** |
| **LocoNet programming ID** | Part number **63120**; default module address **1** |
| **Warranty** | 2 years (damage from misuse voids claim) |

### 3.1 Compatible command stations (handbook)

The interface connects LocoNet-capable centrals **without a built-in PC port**,
including:

| Manufacturer | Examples |
|--------------|----------|
| **Uhlenbrock** | Intellibox, DAISY, Track-Control, IntelliLight |
| **Märklin** | 6021 + 6021 infrared & LocoNet adapter |
| **Fleischmann** | Twin-Center, ProfiBoss, LokBoss |
| **PIKO** | Power Box |
| **Digitrax** | Centrals without PC interface |

**BigFred official path:** [**Digikeijs DR5000**](../commandstations/dr5000.md) on
**LocoNet-T**. Other LocoNet masters may work **best-effort**.

**Limitation (manufacturer):** feedback from **s88 modules** wired through Märklin
Memory/Interface units **cannot** be forwarded to the PC through this interface.

### 3.2 Supported PC software (handbook)

Any program supporting the **LocoNet protocol** for layout control — e.g. iTrain,
RocRail, Koploper, TrainController, DecoderPro, WinDigipet, **JMRI** (63120Adapter
since 4.21.4).

LocoNet-Tool registration: for use with **Intellibox**, **Twin-Center**, or **PIKO
Power Box**, register serial numbers at [uhlenbrock.de](https://www.uhlenbrock.de/)
(LocoNet-Tool requirement).

---

## 4. Operating modes

The 63120 has two USB↔LocoNet modes, selected by **LNCV 4**:

| LNCV 4 | Name (handbook) | Behaviour |
|--------|-----------------|-----------|
| **0** | Only valid messages | **Factory default.** PC sends only validated LocoNet messages; the interface **controls bus traffic** toward LocoNet. All bytes from LocoNet are passed to the PC. Similar to LocoBuffer-USB filtered mode. |
| **1** | LocoNet Direct mode | Each byte is sent to LocoNet **without interface flow control** — raw transparent stream. Handbook §6 states this **should be used only at 19200 baud**; community practice and BigFred use **57600** successfully when the host driver manages pacing (see §5). |

Mode **(b)** in the handbook overview matches **LNCV 4 = 0**; mode **(a)** matches
**LNCV 4 = 1** (direct bytes; handbook ties direct transfer to 19200 in §6).

**BigFred requires LNCV 4 = 1** (LocoNet Direktmodus) so `dcc-bus` receives **raw
LocoNet frames** (opcode … checksum), not a filtered subset.

---

## 5. LNCV configuration

Program while the module is on a **powered LocoNet bus** (command station on),
using **LocoNet-Tool** or a LocoNet throttle with LNCV programming (handbook:
“configure by the Intellibox using LocoNet programming”).

| LNCV | Description | Factory default | BigFred |
|------|-------------|-----------------|---------|
| **0** | Module address | **1** | leave **1** |
| **1** | Software version | read-only | — |
| **2** | USB baud rate: 1=19200, 2=38400, **3=57600**, 4=115200 | **4** (115200) | **3** (57600) |
| **3** | Unknown — **do not modify** | 0 | do not modify |
| **4** | LocoNet Direct mode: 0=off, **1=on** | **0** (off) | **1** (on) |

Reference also:
[LocoNet-over-TCP interface comparison](https://loconetovertcp.sourceforge.net/Interface/).

Wrong **LNCV 2** or **LNCV 4** produces garbage, missing frames, or checksum
errors in `dcc-bus` logs — not a Pi defect.

---

## 6. LED indicators

| LED | Meaning |
|-----|---------|
| **USB** | Lights when the interface is ready on the USB bus |
| **LocoNet** | Flashes briefly on each LocoNet packet activity |

---

## 7. Physical installation

### 7.1 Cabling

```
Command station LocoNet-T ── RJ12 ── [optional 62280 Luisa] ── RJ12 ── 63120 ── USB ── Pi
```

| Rule | Detail |
|------|--------|
| **DR5000** | **LocoNet-T** only — not LocoNet-B (RailSync / booster port) |
| **RJ12** | Standard 6P6C LocoNet cable; data on pins 2–5; **never** RailSync pins 1 & 6 to the 63120 |
| **Bus power** | 63120 is **LocoNet bus-powered**; central must be **on** before traffic |
| **Long / loaded bus** | Optional **Uhlenbrock 62280 (Luisa)** repeater: 12 V / 500 mA, signal regeneration |
| **One serial opener** | Only one process (e.g. `dcc-bus`) may open the USB device |

Pinout and topology: [`loconet-adapter/02-loconet-electrical.md`](../../loconet-adapter/02-loconet-electrical.md).

### 7.2 Driver and Linux (BigFred host)

On **Linux** (Raspberry Pi 5 hub image), the device typically enumerates as a
**USB serial port** (`/dev/ttyACM*`) — often via the kernel **cdc_acm** or a
USB-UART bridge driver, depending on hardware revision. No Uhlenbrock Windows
installer is required.

**Install driver before first USB plug-in** (handbook) — on Linux, verify with
`dmesg` after connect.

Pin a stable device path with **udev** (example placeholders — read VID/PID from
your unit):

```bash
udevadm info -a -n /dev/ttyACM0 | grep -E '{idVendor}|{idProduct}|{serial}'
```

```udev
# /etc/udev/rules.d/99-uhlenbrock-63120.rules
SUBSYSTEM=="tty", ATTRS{idVendor}=="xxxx", ATTRS{idProduct}=="yyyy", \
  SYMLINK+="loconet-63120", GROUP="dialout", MODE="0660"
```

Add the `dcc-bus` service user to group **`dialout`**. Hub image: bake the rule
into the Buildroot overlay ([`loconet-adapter/03-host-platform.md` §3.5](../../loconet-adapter/03-host-platform.md)).

Use a **short USB 3** cable to the Pi; avoid sharing the port with other high-bandwidth
devices during bring-up.

---

## 8. PC ↔ LocoNet protocol (handbook §5)

Uhlenbrock documents an **expert-level** host protocol for reliable communication:

1. **Send over USB**, then **wait to receive the same message echoed** from LocoNet
   before sending the next message.
2. Process other received messages while waiting.
3. **LACK (Long Acknowledge)** handling: for messages that can be followed by LACK,
   set a flag after send/receive; if the next message is LACK, process it as the
   response; otherwise clear the flag.
4. **Do not** rely on fire-and-forget sends without flow control — causes errors,
   especially at **115200** baud where flow control cannot slow the link.

BigFred's [`loconet.go`](../../../pkgs/loco/commandstation/loconet.go) driver
implements LocoNet request/response sequencing and checksum validation on the serial
stream. Configure the 63120 for **transparent raw frames** (§5) so observations
from other throttles on the bus reach `dcc-bus`.

---

## 9. BigFred integration

### 9.1 Protocol contract

`loconet_serial` expects:

1. **57600 8N1** (8 data bits, no parity, 1 stop bit).
2. **Raw bytes** per LocoNet message (opcode through checksum inclusive).
3. **No** ASCII `SEND` / `RECEIVE` lines (that is `loconet_tcp` / LBServer).

Checksum: XOR of all bytes including checksum byte = **`0xFF`**
([`loconet_proto.go`](../../../pkgs/loco/commandstation/loconet_proto.go)). Example
idle frame: `83 7C`.

### 9.2 Catalogue and daemon

| Step | Action |
|------|--------|
| 1 | Command station row: e.g. `DR5000 (LocoNet)`, kind `loconet_serial`, URI `serial:///dev/loconet-63120:57600`, speed steps `128` |
| 2 | Attach to layout (`POST /api/v1/layouts/{id}/command-stations`) |
| 3 | Supervisord starts `dcc-bus-<layout>-<station>` |

Example CLI:

```bash
dcc-bus \
  --station-kind loconet_serial \
  --station-uri "serial:///dev/loconet-63120:57600" \
  --speed-steps 128 \
  …
```

### 9.3 Capabilities over LocoNet (via 63120)

| Feature | Supported |
|---------|-----------|
| Speed / direction | yes |
| Functions **F0–F8** | yes |
| Functions **F9+** | no |
| Observe handheld / other throttle changes | yes (shared LocoNet bus) |
| CV / programming track | no |
| Accessory decoders | not via current BigFred driver scope |

External observation:
[`16-dcc-bus/09-external-state-observation.md`](../architecture/16-dcc-bus/09-external-state-observation.md).

---

## 10. LocoNet-Tool (bundled software)

**LocoNet-Tool** (art. 63120) provides:

- **LNCV programming** for Uhlenbrock LocoNet modules (feedback, switches, displays, LISSY).
- **LocoNet Monitor** — watch and analyse bus traffic (useful when debugging LNCV or
  automatic layout logic).

Configure the **63120's own LNCVs** here (§5) while connected to a powered LocoNet
segment. On Windows-only workshop PCs this is the supported path; on Linux, LNCV
changes may require LocoNet-Tool under Wine, a Windows VM, or programming from a
LocoNet throttle per handbook.

---

## 11. Bring-up checklist

- [ ] 63120 on **LocoNet-T** (DR5000); central powered; LocoNet LED flashes on activity.
- [ ] LNCV **2 = 3** (57600), **LNCV 4 = 1** (Direct mode) verified.
- [ ] USB connected to Pi; `/dev/loconet-63120` (or `ttyACM*`) present; user in **`dialout`**.
- [ ] No other program holds the serial port (JMRI, minicom, LocoNet-Tool on same path).
- [ ] BigFred catalogue: `loconet_serial`, URI `serial:///dev/loconet-63120:57600`.
- [ ] `dcc-bus` log: no sustained `bad checksum` / `timeout waiting for slot`.
- [ ] `setSpeed` moves a loco; second browser session on another address works.
- [ ] Handheld change on LocoNet visible in web UI.

**Common issues:** factory defaults (115200 + filtered mode), wrong LocoNet port
(B vs T), weak bus power (add **62280**), parallel serial port users, USB autosuspend.

Optional sanity check ([`loconet-adapter/06-bringup` §6.3](../../loconet-adapter/06-bringup-and-testing.md)):

```bash
stty -F /dev/loconet-63120 57600 cs8 -cstopb -parenb raw
timeout 5 xxd /dev/loconet-63120
```

---

## 12. Related documents

| Document | Topic |
|----------|-------|
| [`DR5000`](../commandstations/dr5000.md) | Command station paired with 63120 |
| [`loconet-adapter/04`](../../loconet-adapter/04-uhlenbrock-63120.md) | Frame loss, Luisa, udev |
| [`loconet-adapter/05`](../../loconet-adapter/05-bigfred-integration.md) | Catalogue integration |
| [`16-dcc-bus`](../architecture/16-dcc-bus/README.md) | Daemon, sessions |
| [Handbook PDF](https://www.uhlenbrock.de/de_DE/service/download/handbook/en/Bes63120e.pdf) | Uhlenbrock Bes63120e (English) |

Back to the [devices index](./README.md).
