# 3. Host platform (Raspberry Pi 5)

BigFred runs on the Pi as **`loco-server`** (HTTP + control-plane WebSocket),
**`dcc-bus`** (throttle data plane — LocoNet serial or Z21 UDP), **Redis**, and
process supervision ( **supervisord** on Raspberry Pi OS, **BusyBox `S*` scripts**
on the reference hub image — [§8](./08-hub-os-image.md)).

The host must keep up with **USB serial** or **UDP Z21** traffic and **SQLite
writes** without multi-second stalls.

## 3.1 Bill of materials

| Item | Recommendation |
|------|----------------|
| **Raspberry Pi 5** | **8 GB RAM** (4 GB minimum for one layout + one `dcc-bus`) |
| **Raspberry Pi 5 M.2 HAT+** | Official **M.2 HAT+** for Pi 5 (NVMe) |
| **NVMe SSD** | See [§8.4](./08-hub-os-image.md#84-storage-layout) — e.g. **WD SN740 256 GB** |
| **Rescue microSD** | Endurance 32 GB (Samsung PRO Endurance, SanDisk Max Endurance) — optional |
| **Power supply** | Official **27 W USB-C** PSU for Pi 5 |
| **Cooling** | Active cooler; **fanctl** on reference image (§8.8) |
| **Ethernet** | Gigabit cable to club LAN (preferred over WiFi for many clients) |
| **Uhlenbrock 63120** | **DR5000 path only** — art. **63120** + LocoNet-Tool for LNCV |
| **LocoNet cable** | 6P6C RJ12 to **DR5000 LocoNet-T** |
| **USB cable** | Pi ↔ Uhlenbrock 63120 (DR5000 path only) |

**RB1110 / RB1110-Mini:** same Pi and NVMe; **no Uhlenbrock 63120** — [§7](./07-z21-command-stations.md).

Optional (DR5000 LocoNet): **Uhlenbrock 62280 (Luisa)** (§2).

## 3.2 Operating system choice

| Image | When to use | Documentation |
|-------|-------------|---------------|
| **Hub OS (Buildroot)** | **Production** club hub | **[§8 Hub OS image](./08-hub-os-image.md)** |
| **Raspberry Pi OS 64-bit** | Development, interim bring-up | This section (§3.3–§3.6) |

The reference production image provides **PREEMPT_RT**, **read-only root**,
**`/data` on NVMe**, **CPU isolation**, and **`make image`** automation. Raspberry
Pi OS is acceptable for testing BigFred before the Buildroot image is flashed.

### Raspberry Pi OS on NVMe (interim)

Install **Raspberry Pi OS (64-bit)** with root on **NVMe** via the imaging tool.
Keep microSD only for firmware rescue if needed.

#### Why latency matters

| Workload | Sensitive to slow disk? |
|----------|-------------------------|
| **SQLite** | **Yes** |
| **Logs** | Moderate |
| **Redis** | Low (persistence under `/data` on hub image) |
| **LocoNet / Z21 I/O** | **No** (not disk-bound per packet) |

On Pi OS, use **NVMe root** (not SD daily driver). On the hub image, SQLite and
Redis live on **`/data`** (§8.4).

#### Pi OS `fstab` tuning (RW root)

```fstab
UUID=...  /  ext4  noatime,nodiratime,commit=1  0  1
```

Disable swap or use **zram** only on 8 GB boards.

## 3.3 PREEMPT_RT Linux kernel

**Goal:** reduce scheduling jitter for `dcc-bus` (USB or UDP).

| Deployment | RT kernel |
|------------|-----------|
| **Hub OS image** | Built with `CONFIG_PREEMPT_RT` — [§8.6](./08-hub-os-image.md#86-preempt_rt-kernel) |
| **Raspberry Pi OS** | Install packaged RT kernel if available, else latest kernel + sysctl below |

Pi OS fallback:

```bash
# /etc/sysctl.d/99-bigfred-latency.conf
kernel.sched_rt_runtime_us = -1
vm.swappiness = 10
```

```bash
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
```

On the **hub image**, `isolcpus` / `taskset` in §8.6 replace optional systemd
nice/IO tuning.

## 3.4 USB and serial stability (DR5000 path)

| Practice | Reason |
|----------|--------|
| **Uhlenbrock 63120 on USB 3** | Bandwidth |
| **Short USB cable** | EMI / voltage |
| **udev** stable symlink | §3.5 |
| User in **`dialout`** | Serial permissions |
| **Single opener** of `/dev/loconet-63120` | No `minicom` while `dcc-bus` runs |

## 3.5 udev rule for Uhlenbrock 63120

```bash
udevadm info -a -n /dev/ttyACM0 | grep -E '{idVendor}|{idProduct}|{serial}'
```

```udev
# /etc/udev/rules.d/99-uhlenbrock-63120.rules
SUBSYSTEM=="tty", ATTRS{idVendor}=="xxxx", ATTRS{idProduct}=="yyyy", SYMLINK+="loconet-63120", GROUP="dialout", MODE="0660"
```

On the **hub image**, bake this rule into the Buildroot **overlay** (§8.9).

## 3.6 Build and deploy BigFred

### Cross-compile (both images)

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/loco-server ./pkgs/server
```

Install paths:

| Image | Binaries | Web UI |
|-------|----------|--------|
| **Hub OS** | `/usr/bin/` via Buildroot package | `/usr/share/bigfred/web` (overlay) |
| **Pi OS** | `bin/` + supervisord | `web/dist` per [`15-supervisord`](../bigfred/architecture/15-supervisord/README.md) |

Production flash procedure: [§8.10](./08-hub-os-image.md#810-building-the-image).
Log rotation: [§8.9](./08-hub-os-image.md#89-log-retention-crontab-and-rotation).

Continue with [§4 Uhlenbrock 63120](./04-uhlenbrock-63120.md) or [§8 Hub OS](./08-hub-os-image.md).
