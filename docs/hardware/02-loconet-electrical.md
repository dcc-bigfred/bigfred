# 2. LocoNet electrical & command station wiring

> **Verify against the Digitrax LocoNet Personal Edition (LocoNet PE)
> specification before cutting copper.** The values below reflect community
> practice; the LocoNet PE document is the normative source for line levels,
> timing, and the RJ12 pinout.

## 2.1 The LocoNet bus in one paragraph

LocoNet is a **multi-drop, single data line**, **open-collector** bus running
at a nominal **16,660 baud** (≈ 60 µs/bit) on the wire. Idle = line released
high (pulled up by the master, ~12–15 V domain); a node transmits by **pulling
the line low**. Several nodes can be wired in parallel with standard RJ12 patch
cables; collision detection is done by listening while transmitting.

For this deployment:

1. The **Uhlenbrock 63120** handles wire timing internally — the Pi only sees **57600 baud**
   on USB (LocoBuffer host link), not 16.660 kbaud on the wire.
2. You still need correct **RJ12 wiring** and the **throttle-side** LocoNet port;
   the interface cannot fix a RailSync mis-wire or a dead bus.

## 2.2 RJ12 (6P6C) pinout

LocoNet uses a 6-pin/6-conductor (6P6C) modular connector. Typical assignment
(confirm with a meter on your central):

| Pin | Signal | Notes |
|-----|--------|-------|
| 1 | RailSync − | **Track-level voltage — do not connect to Uhlenbrock 63120 logic** |
| 2 | Ground | Logic ground |
| 3 | **LocoNet data** | Open-collector data line |
| 4 | **LocoNet data** | Same net as pin 3 |
| 5 | Ground | Logic ground |
| 6 | RailSync + | **Track-level — leave unconnected on throttle branch** |

The **Uhlenbrock 63120** needs **pins 2/5 (ground) and 3/4 (data)** only.

## 2.3 Which socket on the command station?

### Digikeijs DR5000 (two LocoNet sockets)

| Port | Purpose | RailSync present? | Use for Uhlenbrock 63120? |
|------|---------|-------------------|----------------|
| **LocoNet-T** (Throttle) | Handhelds, computer interfaces | No (or limited) | **Yes** |
| **LocoNet-B** (Booster) | Boosters, RailSync devices | Yes (track-level) | **No** |

Connect the **Uhlenbrock 63120** to **LocoNet-T** with a standard LocoNet patch cable.

### RailBOX RB1110 / RB1110-Mini

The central has a **LocoNet** RJ12 for handhelds and modules on the layout. For
**BigFred**, use **`z21`** on LAN/WiFi instead — [§7](./07-z21-command-stations.md).
Do not use the Uhlenbrock 63120 on RB1110 for the supported hub path.

### Other centrals

Use the port labelled for **throttles / computer / LocoNet** (not booster /
RailSync-only). When in doubt, check the manual for RailSync on pins 1 and 6.

## 2.4 USB LocoNet on the central (optional alternative)

Some masters (notably **DR5000**) also expose LocoNet on **USB** as a virtual COM
port (LocoBuffer stream). This document standardises on **Uhlenbrock 63120 on the Pi** so
the central's USB stays available and the hub host is always the Pi (§1.6).

## 2.5 Power on the LocoNet side

| Device | LocoNet power |
|--------|----------------|
| **Uhlenbrock 63120** | Powered from the **LocoNet bus** (throttle-class) |
| **62280 (Luisa)** | Optional repeater: **12 V / 500 mA** on secondary segment |

Requirements:

- Command station **on** and driving LocoNet before expecting Uhlenbrock 63120 traffic.
- If the Uhlenbrock 63120 branch is long or heavily loaded, add **62280** between the
  central and the Uhlenbrock 63120 (see [§4.5](./04-uhlenbrock-63120.md#45-optional-62280-luisa-before-uhlenbrock-63120)).

USB powers only the **Uhlenbrock 63120 USB electronics**; LocoNet line drivers still need
a healthy magistrala.

## 2.6 Cabling & topology rules

- Use **6P6C LocoNet patch cables**; daisy-chain or a distribution board.
- Keep stubs short; long unterminated branches degrade edges.
- **Never** wire RailSync into data pins.
- **One** RJ12 from Uhlenbrock 63120 to the LocoNet bus (through Luisa if used).
- **One** BigFred `dcc-bus` opener per Uhlenbrock 63120 USB serial port.

Continue with [§3 Host platform](./03-host-platform.md).
