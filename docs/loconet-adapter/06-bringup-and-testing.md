# 6. Bring-up & testing

Staged checks before club traffic. Pick the section for your central.

| Central | Path | Section |
|---------|------|---------|
| **DR5000** | `loconet_serial` + Uhlenbrock 63120 | §6.2–§6.4 |
| **RB1110** / **RB1110-Mini** | `z21` | [§7.6](./07-z21-command-stations.md#76-bring-up-checklist-rb1110--z21) |
| **Other Z21 + LocoNet** | Either (best-effort) | §7 + relevant section below |

## 6.1 Host platform (all paths)

1. **Pi 5** boots from **NVMe** (M.2 HAT+) — hub image (§8) or Pi OS (§3).
2. **PREEMPT_RT** / `isolcpus` per §8.6 or §3.3.
3. **BigFred** deployed; Redis + supervisord per architecture docs.
4. **DR5000 only:** user in **`dialout`**, udev for Uhlenbrock 63120 (§3.5).
5. **RB1110 only:** Pi and central on same LAN; RB1110 IP documented.

## 6.2 Uhlenbrock 63120 (DR5000 only)

1. DR5000 **on**; Uhlenbrock 63120 on **LocoNet-T** (not LocoNet-B).
2. LocoNet LED **blinks** on activity (solid = no bus — fix before LNCV).
3. **LNCV 2 = 3** (57600), **LNCV 4 = 1** (Direktmodus) — LocoNet-Tool or `rb lncv` (§6.2.1).
4. Uhlenbrock 63120 on **USB 3**; `ls -l /dev/loconet-63120` (or `ttyUSB0` / `ttyACM0`).
5. Close serial monitors before `dcc-bus`.

### 6.2.1 LNCV via `rb lncv` (Linux)

Factory USB baud is **115200** until LNCV 2 is changed. Use **`--self-config`**
for adapter CV2/CV4 (no acknowledge on the wire).

```bash
rb lncv get --device /dev/ttyUSB0 --baud 115200 --article 63120 --addr 1 0
rb lncv set --self-config --device /dev/ttyUSB0 --baud 115200 --article 63120 4 1
rb lncv set --self-config --device /dev/ttyUSB0 --baud 115200 --article 63120 2 3
rb lncv get --device /dev/ttyUSB0 --baud 57600 --article 63120 2
rb lncv get --device /dev/ttyUSB0 --baud 57600 --article 63120 4
```

Details: [§4.2.1](./04-uhlenbrock-63120.md#421-rb-lncv-on-linux).

## 6.3 Manual serial sanity (DR5000, optional)

```bash
stty -F /dev/loconet-63120 57600 cs8 -cstopb -parenb raw
timeout 5 xxd /dev/loconet-63120
```

Expect bus traffic; **`83 7C`** is a common valid frame. Garbage at 115200 → LNCV.

## 6.4 BigFred end-to-end — LocoNet (DR5000)

1. `loconet_serial` command station (§5.3).
2. `dcc-bus` starts without `open ...` errors.
3. `setSpeed` moves a loco; physical throttle reflected in UI.
4. Two browser drivers, two locos.

## 6.5 Troubleshooting — LocoNet path

| Symptom | Likely cause | Action |
|---------|--------------|--------|
| No `/dev/ttyACM*` / `ttyUSB*` | USB / PSU | Data cable; 27 W PSU; `dmesg` |
| `permission denied` | dialout / udev | Fix group and rules |
| LocoNet LED **solid**, no blink | No bus traffic | Central on; LocoNet-**T**; RJ12 cable; bus power |
| `rb lncv`: no bytes received | Dead bus or wrong baud | Fix LED/bus; match `--baud` to LNCV 2 |
| `rb lncv get` works once, then silent | Programming session stuck | Power-cycle 63120; `rb lncv` always sends prog-end |
| `rb lncv set` timeout (no `--self-config`) | Module busy / write rejected | Retry; use `--self-config` for adapter CV2/CV4 |
| `xxd` shows nothing (exit 124) | Idle bus **or** dead bus | Normal if central idle; if LED solid → wiring |
| Random hex | LNCV | 57600 + Direktmodus |
| `bad checksum` | Bus / baud | LNCV; 62280; shorter run |
| Commands time out | Central off / wrong port | LocoNet-**T** |
| Works on DR5000 USB, not Uhlenbrock 63120 | T-bus segment | Power/signal on branch |

## 6.6 Troubleshooting — Z21 path

See [§7](./07-z21-command-stations.md): wrong IP, firewall UDP 21105, VLAN,
competing Z21 client (mobile app).

## 6.7 Acceptance checklist

**Host (both)**

- [ ] NVMe partitions per §8.4 (or Pi OS NVMe root); RT / isolcpus documented.
- [ ] Hub image: boot under 10 s, watchdog and fanctl running (§8.11).
- [ ] `rotate-hub-logs` and crontab present (§8.9).

**DR5000 + Uhlenbrock 63120**

- [ ] LNCV 57600 + Direktmodus; stable `/dev/loconet-63120`.
- [ ] LocoNet-**T**; two web drivers on one Uhlenbrock 63120.

**RB1110**

- [ ] Catalogue `z21`; UDP socket open in logs.
- [ ] `setSpeed` + physical throttle reflection + two drivers — §7.6.

**Dual-protocol (unsupported guarantee)**

- [ ] Model, firmware, and chosen kind (`z21` vs `loconet_serial`) recorded in runbook.

Back to the [index](./README.md).
