# 7. Z21 connection (RB1110 and dual-protocol centrals)

For some command stations BigFred connects over the **Z21 LAN protocol** (UDP)
instead of **LocoNet serial** via the Uhlenbrock 63120. The Pi 5 host stack
(§3) is the same; you omit the Uhlenbrock 63120 when using Z21.

## 7.1 When to use Z21 vs LocoNet serial

| Command station | Recommended BigFred connection | This document set |
|-----------------|--------------------------------|-------------------|
| **Digikeijs DR5000** | **`loconet_serial`** + Uhlenbrock 63120 on LocoNet-T | §2–§6 (LocoNet path) |
| **RailBOX RB1110** / **RB1110-Mini** | **`z21`** over LAN/WiFi | **This chapter** |
| **Other centrals** with **both** Z21 and LocoNet | Prefer **`z21`** if the vendor documents it; LocoNet via Uhlenbrock 63120 is possible | **Best-effort** — no guarantee (§7.5) |

The RB1110 exposes LocoNet on RJ12 for handhelds and modules, but RailBOX
documents **Z21®** as the primary PC/tablet integration path. BigFred follows
that: use **`z21`**, not `loconet_serial` + Uhlenbrock 63120, for RB1110 hubs.

## 7.2 Data path (RB1110)

```
Browsers ──HTTP/WS──► loco-server (Pi 5)
                          │
                          ▼
                     dcc-bus ──► UDP Z21 (port 21105)
                          │
                     RB1110 (WiFi or Ethernet on club LAN)
                          │
                          └──► DCC ──► layout
```

- **Web → track:** `loco.setSpeed` → Z21 `LAN_X_SET_LOCO_*` → RB1110 → DCC.
- **Track → web:** Z21 **`LAN_X_LOCO_INFO`** push (after broadcast flags) →
  [`z21.go`](../../pkgs/loco/commandstation/z21.go) → subscribed clients.

Driver reference: [`docs/bigfred/protos/z21.md`](../bigfred/protos/z21.md),
[`16-dcc-bus/09-external-state-observation.md`](../bigfred/architecture/16-dcc-bus/09-external-state-observation.md).

## 7.3 Network setup

| Requirement | Detail |
|-------------|--------|
| **Pi and RB1110 on the same LAN** | Prefer **Ethernet** on the Pi; WiFi on the central is fine if stable |
| **Fixed IP for RB1110** | DHCP reservation or static IP in the club runbook |
| **UDP port** | **21105** (default Z21); firewall must allow Pi → central UDP |
| **No Uhlenbrock 63120** | Not used on this path |

Configure the RB1110 network (SSID, IP) with RailBOX tools per the manufacturer
manual before pointing BigFred at it.

## 7.4 BigFred catalogue entry

| Field | Value |
|-------|-------|
| Kind | `z21` |
| URI | `udp://192.168.1.50:21105` (host = RB1110 IP; port optional) |
| Speed steps | `128` (match decoder / central settings) |

Parser ([`driver.go`](../../pkgs/bigfred/dcc-bus/station/driver.go)): `udp://host:port` or
`host:port`; port defaults to **21105**.

Example admin name: `RB1110 (Z21)`.

`dcc-bus` starts with `--station-kind z21` and dials UDP; there is no serial
device to udev-pin.

### Capabilities over Z21

See [`z21.go`](../../pkgs/loco/commandstation/z21.go) — generally broader than
basic LocoNet in BigFred (e.g. more functions, CV access). Hub semantics (many
drivers, takeover, one daemon per command station) are unchanged.

## 7.5 Dual-protocol centrals (best-effort)

Centrals that advertise **both** Z21 LAN and LocoNet (e.g. some Roco Z21
variants with LocoNet ports, club loaned hardware, future models) may work with
BigFred using either:

- **`z21`** — `udp://…:21105`, or
- **`loconet_serial`** — Uhlenbrock 63120 on the LocoNet throttle port (§4).

**There is no official guarantee** until the combination is tested on your layout.
Prefer the connection mode the **manufacturer documents for PC control**; for
RB1110 that is explicitly **Z21**.

Report gaps (missing push, slot limits, accessory commands) as BigFred issues
with the central model and firmware version noted.

## 7.6 Bring-up checklist (RB1110 / Z21)

- [ ] RB1110 powered; track power as required; IP reachable from Pi (`ping`).
- [ ] `nc -u -z` or `socat` smoke test optional; then BigFred catalogue `z21` row.
- [ ] `dcc-bus` log shows `z21 command station: UDP socket open`.
- [ ] Driver session: `setSpeed` moves a loco.
- [ ] Physical throttle change reflected in UI (Z21 broadcast / subscribe path).
- [ ] Second browser driver on another loco.

Troubleshooting: wrong IP, VLAN isolation, firewall blocking UDP 21105, RB1110
WiFi sleep, or another app (mobile RailBOX app) holding the Z21 session — only
one writer per command station (§5.6).

Back to the [index](./README.md).
