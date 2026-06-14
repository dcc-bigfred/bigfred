# BigFred hub on Raspberry Pi 5 — reference deployment

This document set describes the **reference production setup** for connecting
**BigFred** to a **DCC command station** and operating it as a **multi-user
throttle hub** on a model railroad.

Hardware is **commercial off-the-shelf** only — no custom LocoNet electronics.

## Officially supported command stations

| Model | Manufacturer | **Recommended** BigFred link | Notes |
|-------|--------------|------------------------------|-------|
| **DR5000** | [Digikeijs](https://www.digikeijs.com/) | **`loconet_serial`** + **Uhlenbrock 63120** on **LocoNet-T** | §2–§6 |
| **RB1110** / **RB1110-Mini** | [RailBOX](https://www.railbox.pl/) | **`z21`** UDP on LAN/WiFi | [§7 Z21](./07-z21-command-stations.md) — **not** Uhlenbrock 63120 |
| **Other centrals** with **Z21 and LocoNet** | various | Usually **`z21`** if vendor documents PC control | **Best-effort** — no guarantee — §7.5 |

Other LocoNet-only masters may work with the **Uhlenbrock 63120** path if they behave as a
LocoNet master; they are outside official support until validated on your layout.

## Two connection paths

### LocoNet serial (DR5000) — §2–§6

```mermaid
flowchart LR
  O[Drivers] -->|HTTP/WS| BF[BigFred Pi 5]
  BF <-->|USB 57600| U6[Uhlenbrock 63120]
  U6 <-->|RJ12 LocoNet| CS[DR5000]
  CS --> DCC[Layout]
```

### Z21 UDP (RB1110) — §7

```mermaid
flowchart LR
  O[Drivers] -->|HTTP/WS| BF[BigFred Pi 5]
  BF <-->|UDP 21105| CS[RB1110]
  CS --> DCC[Layout]
```

## Reference stack (common host)

| Layer | Choice |
|-------|--------|
| **Host** | Raspberry Pi **5** (8 GB RAM recommended) |
| **Storage** | **NVMe SSD** via **M.2 HAT+**; RO `/` + RW `/data` on hub image |
| **OS** | **Buildroot hub image** (production) or Pi OS 64-bit (dev) — [§8](./08-hub-os-image.md) |
| **OS tuning** | **PREEMPT_RT**, CPU isolation, hardware watchdog |
| **To DR5000** | **Uhlenbrock 63120** on **USB 3** → `loconet_serial` @ **57600 8N1** |
| **To RB1110** | **LAN/WiFi** → `z21` @ **UDP 21105** (no Uhlenbrock 63120) |
| **Application** | BigFred `loco-server` + `dcc-bus` |

## Scope

| In scope | Out of scope |
|----------|--------------|
| Pi 5 + hub OS image (Buildroot) | DIY LocoNet interface |
| NVMe layout, RT, fanctl, Alloy | Full Buildroot fork maintenance |
| Uhlenbrock 63120 + LocoNet path (DR5000) | Custom MCU firmware |
| Z21 path (RB1110, dual-protocol notes) | Building a command station |
| BigFred hub + driver limits | Every unsupported central model |
| Bring-up and diagnostics | LCC, XpressNet-only centrals |

## Table of contents

1. [Overview & architecture](./01-overview-and-architecture.md)
2. [LocoNet electrical & wiring](./02-loconet-electrical.md) — DR5000 / Uhlenbrock 63120 path
3. [Host platform (Pi 5)](./03-host-platform.md) — BOM, Pi OS interim path
4. [Uhlenbrock 63120](./04-uhlenbrock-63120.md)
5. [BigFred integration](./05-bigfred-integration.md) — `loconet_serial` and `z21`
6. [Bring-up & testing](./06-bringup-and-testing.md)
7. [Z21 connection](./07-z21-command-stations.md) — RB1110, dual-protocol centrals
8. [Hub OS image](./08-hub-os-image.md) — Buildroot, partitions, RT, `make image`

## Authoritative references

- Drivers: [`loconet_serial.go`](../../pkgs/loco/commandstation/loconet_serial.go),
  [`z21.go`](../../pkgs/loco/commandstation/z21.go),
  [`driver.go`](../../pkgs/bigfred/dcc-bus/station/driver.go).
- Z21 protocol: [`docs/bigfred/protos/z21.md`](../bigfred/protos/z21.md).
- LocoNet protocol: [`docs/bigfred/protos/loconet.md`](../bigfred/protos/loconet.md).
- DCC bus: [`16-dcc-bus/`](../bigfred/architecture/16-dcc-bus/README.md).
- Centrals: [Digikeijs DR5000](https://www.digikeijs.com/),
  [RailBOX RB1110](https://www.railbox.pl/en/products/rb1110).

> LocoNet message formats are © Digitrax; Z21 is © Roco/Fleischmann.
> This documentation is for educational purposes.
