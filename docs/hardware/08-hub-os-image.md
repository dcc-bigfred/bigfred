# 8. Hub OS image (Buildroot)

This chapter specifies the **reference operating system image** for a
**Raspberry Pi 5** running BigFred as a layout hub. It is derived from the
hub OS build plan and targets **deterministic latency**, **read-only root**, and
**fully automated image builds** — not a general-purpose Raspberry Pi OS desktop.

For a quicker lab setup you may still use **Raspberry Pi OS (64-bit)** on NVMe
(§3.2); treat that as development or interim bring-up. Production clubs should
plan on the image described here.

## 8.1 Design goals

| Goal | How |
|------|-----|
| Fast boot | Target **under 10 s** to hub services |
| Low jitter for `dcc-bus` | **PREEMPT_RT**, CPU isolation, `taskset` |
| Power-loss tolerance | **RW `/data`** on ext4; **RO `/`**; SQLite WAL |
| No moving parts on root | **NVMe** for `/` and `/data`; SD only for rescue |
| Simple operations | **BusyBox init**, no systemd, no containers |
| Reproducible builds | **Buildroot** + `make image` |

## 8.2 Software stack on the image

| Component | Role on hub |
|-----------|-------------|
| **Linux PREEMPT_RT** | aarch64, musl toolchain in Buildroot |
| **BusyBox** | `init`, `rcS`, `S*` boot scripts |
| **Redis** | Loopback cache / pub-sub for BigFred ([`16-dcc-bus`](../bigfred/architecture/16-dcc-bus/README.md)) |
| **SQLite3** | `loco-server` persistence |
| **Dropbear** | SSH administration |
| **Grafana Alloy** | Metrics/logs shipping |
| **htop** | On-device diagnostics |
| **Hardware watchdog** | Reboot on hang / kernel panic |
| **fanctl** | Pi 5 active cooler by temperature |
| **BigFred** | `loco-server` + `dcc-bus` (+ static `web/dist`) |

On Raspberry Pi OS deployments, **supervisord** fills a similar process-supervision
role ([`15-supervisord`](../bigfred/architecture/15-supervisord/README.md)). On the
reference image, **BusyBox `S*` scripts** start and stop services instead — same
logical stack, different init integration.

## 8.3 Boot sequence

```text
PID 1: BusyBox init
  ↓
/etc/init.d/rcS
  ↓
S05-cron       # BusyBox crond (reads /etc/crontabs/root)
S10-mount      # NVMe partitions, RO root remount
S15-network    # static IP or dhclient
S20-sysctl     # RT / latency tunables
S30-redis
S40-alloy
S50-fanctl
S60-bigfred    # loco-server + dcc-bus (hub)
S90-dropbear
```

`S60-bigfred` is the production name for the plan’s `S60-loconet` script — it
starts the Go hub binaries, not a separate product.

## 8.4 Storage layout

### NVMe SSD (primary)

| Device | Mount | FS | Mode |
|--------|-------|-----|------|
| `/dev/nvme0n1p1` | `/boot` | FAT32 | RW (firmware / kernel) |
| `/dev/nvme0n1p2` | `/` | ext4 | **read-only** |
| `/dev/nvme0n1p3` | `/data` | ext4 | **read-write** |

### microSD (rescue only)

Use a small **endurance** card for emergency boot or imaging — **not** for daily
SQLite/Redis I/O.

| Model (examples) |
|------------------|
| Samsung PRO Endurance 32 GB |
| SanDisk Max Endurance 32 GB |

### NVMe hardware (examples)

| Tier | Model |
|------|-------|
| Recommended | **WD SN740 256 GB** NVMe |
| Alternatives | Samsung PM991a, Micron 2450, WD SN530 |

Minimum **128 GB** if you retain logs and metrics on `/data` for months.

### `/etc/fstab` (reference)

```fstab
/dev/nvme0n1p2  /       ext4  ro,noatime           0  1
/dev/nvme0n1p3  /data   ext4  rw,noatime           0  2

tmpfs           /tmp      tmpfs  defaults,size=64m   0  0
tmpfs           /var/log  tmpfs  defaults,size=64m   0  0
tmpfs           /var/run  tmpfs  defaults,size=16m   0  0
```

Volatile logs on `tmpfs` avoid wearing the root partition; long-term logs go
through Alloy off-box.

### BigFred data paths

| Data | Path |
|------|------|
| SQLite (`loco-server`) | `/data/sqlite/loconet.db` (name is historical; holds BigFred DB) |
| Redis persistence | `/data/redis/` |
| Alloy state | `/data/alloy/` |
| Rotated file logs | `/data/logs/` (see §8.9) |

SQLite pragmas (set at application open or migration):

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
```

## 8.5 Buildroot configuration

### Target

- **Board:** Raspberry Pi 5
- **Toolchain:** aarch64, **musl** libc
- **Init:** BusyBox

### Packages (defconfig baseline)

| Package | Purpose |
|---------|---------|
| `busybox` | init, core utilities |
| `dropbear` | SSH |
| `sqlite` | embedded DB |
| `redis` | in-memory / pub-sub |
| `htop` | diagnostics |
| `iproute2`, `ethtool` | networking |
| `curl` | HTTP health checks, debugging, fetching configs |
| `netcat` (`nc`) | TCP/UDP probes (e.g. Z21 port **21105**, Redis **6379**) |
| `dhclient` | optional DHCP |
| `watchdog` | daemon + kernel WDT |
| **BusyBox `crond`** | nightly log rotation (§8.9) |
| **Grafana Alloy** | custom Buildroot package or prebuilt binary in overlay |
| **BigFred** | custom package installing `loco-server`, `dcc-bus`, `web/dist` |

Kernel built from **`kernel/`** tree in the image repository with RT options below.

## 8.6 PREEMPT_RT kernel

Required options (fragment — enable in `kernel/config`):

```text
CONFIG_PREEMPT_RT=y
CONFIG_HIGH_RES_TIMERS=y
CONFIG_HZ_1000=y
CONFIG_WATCHDOG=y
CONFIG_BCM2835_WDT=y
```

### `cmdline.txt` (boot partition)

Isolate hub CPUs from IRQ housekeeping:

```text
isolcpus=2,3
nohz_full=2,3
rcu_nocbs=2,3
irqaffinity=0,1
rcu_nocb_poll
```

### CPU layout

| CPUs | Workload |
|------|----------|
| **0–1** | IRQs, Redis, Alloy, SSH, fanctl, DHCP |
| **2–3** | **`dcc-bus`** and **`loco-server`** (Go) |

Start hub processes pinned:

```bash
taskset -c 2,3 /usr/bin/loco-server …
taskset -c 2,3 /usr/bin/dcc-bus …
```

Pin auxiliary daemons to 0–1:

```bash
taskset -c 0,1 redis-server /etc/redis/redis.conf
taskset -c 0,1 alloy run /etc/alloy/config.alloy
```

Exact flags and config paths belong in the `S*` scripts and Buildroot overlay.

### Sysctl (`S20-sysctl`)

Align with §3.3 where applicable:

```text
kernel.sched_rt_runtime_us = -1
vm.swappiness = 10
```

Use **`performance`** cpufreq governor when the driver exposes it.

## 8.7 Networking

Default pattern: **static IP** on `eth0` for predictable Z21 URIs and club DNS.

| Field | Example |
|-------|---------|
| IP | `192.168.10.10` |
| Mask | `255.255.255.0` |
| Gateway | `192.168.10.1` |

```bash
ip link set eth0 up
ip addr add 192.168.10.10/24 dev eth0
ip route add default via 192.168.10.1
```

Optional DHCP in `S15-network`:

```bash
dhclient eth0
```

Document the chosen address in the club runbook; RB1110 `z21` URIs use this host
only as the **client** — the central keeps its own IP (§7.3).

## 8.8 Watchdog and cooling

### Watchdog

Device: `/dev/watchdog`. Policy:

- Reboot if userspace stops petting within timeout.
- Reboot on **kernel panic** when configured.

Hub scripts should pet the watchdog only when `loco-server` and `dcc-bus` are healthy.

### Fan control (`fanctl`)

Daemon: `/usr/bin/fanctl` (started in `S50-fanctl`).

| SoC temperature | Fan |
|-----------------|-----|
| below 45 °C | OFF |
| 45–60 °C | LOW |
| 60–70 °C | MEDIUM |
| above 70 °C | HIGH |

## 8.9 Log retention, crontab, and rotation

### Where logs live

| Location | Lifetime | Contents |
|----------|----------|----------|
| `/var/log` (tmpfs) | Lost on reboot | BusyBox, kernel ring buffer copies, ephemeral boot messages |
| `/data/logs/` | Persistent, rotated | BigFred, Redis, Alloy file tails, `rotate-hub-logs` archives |

Point application loggers at **`/data/logs/<service>/`**, for example:

```text
/data/logs/bigfred/loco-server.log
/data/logs/bigfred/dcc-bus.log
/data/logs/redis/redis.log
/data/logs/alloy/alloy.log
```

Alloy still ships telemetry off-box; local files are for **SSH debugging** when the
network is down.

### `rotate-hub-logs` script

Install from the Buildroot overlay (example path):

```text
/usr/sbin/rotate-hub-logs
```

The script should:

1. **Rotate** files in `/data/logs/*/*.log` (copytruncate or `mv` + `gzip`).
2. **Compress** archives older than one day (`*.log.1` → `*.log.1.gz`).
3. **Delete** `*.gz` older than **14 days** (tune per club retention policy).
4. **Cap** total size under `/data/logs` (e.g. stop deleting oldest `.gz` until
   usage is below **512 MiB**).
5. Exit non-zero only on errors worth alerting (Alloy can scrape a counter later).

Skeleton (implement in `loconet-os/scripts/rotate-hub-logs`):

```sh
#!/bin/sh
# rotate-hub-logs — hub log rotation (BusyBox ash)
LOGROOT=/data/logs
RETENTION_DAYS=14
MAX_BYTES=$((512 * 1024 * 1024))

for dir in "$LOGROOT"/*; do
  [ -d "$dir" ] || continue
  for f in "$dir"/*.log; do
    [ -f "$f" ] || continue
    # size-triggered rotate (e.g. > 10 MiB)
    if [ "$(stat -c%s "$f" 2>/dev/null || echo 0)" -gt 10485760 ]; then
      ts=$(date +%Y%m%d%H%M%S)
      cp "$f" "$f.$ts" && : > "$f"
      gzip -9 "$f.$ts"
    fi
  done
  find "$dir" -name '*.gz' -mtime +"$RETENTION_DAYS" -delete
done

# enforce MAX_BYTES on $LOGROOT (oldest .gz first)
# …
```

Make executable in overlay: `chmod 755 overlays/usr/sbin/rotate-hub-logs`.

### Crontab

Enable **BusyBox `crond`** in defconfig (`BR2_PACKAGE_BUSYBOX_CONFIG_CROND=y`).
Start it from **`S05-cron`** before services that append logs:

```sh
#!/bin/sh
case "$1" in
  start)
    mkdir -p /etc/crontabs
    crond -c /etc/crontabs
    ;;
  stop)
    killall crond 2>/dev/null
    ;;
esac
```

Root crontab in overlay — **`etc/crontabs/root`**:

```cron
# m h  dom mon dow  command
0  3  *   *   *     /usr/sbin/rotate-hub-logs
15 3  *   *   *     /usr/sbin/rotate-hub-logs
```

- **`03:00`** — daily rotation after typical club hours.
- **`03:15`** — second pass catches logs written during the first run (optional).

For manual test after flash:

```bash
/usr/sbin/rotate-hub-logs
ls -la /data/logs/bigfred/
```

Do **not** run rotation on **CPU 2–3**; `crond` stays on **0–1** with other
housekeeping (§8.6).

## 8.10 Building the image

Target repository layout (separate **`loconet-os/`** tree — not yet part of this
monorepo):

```text
loconet-os/
├── buildroot/      # Buildroot tree or external tree pointer
├── configs/        # defconfig, kernel fragments, cmdline
├── overlays/       # etc/fstab, crontabs, init.d, usr/sbin/rotate-hub-logs
├── kernel/         # RT patches / config for Pi 5
├── scripts/        # rotate-hub-logs source, post-image, flash-nvme
└── Makefile
```

Build:

```bash
make image
```

Artifact (example):

```text
output/images/sdcard.img
```

Flash to **NVMe** (or SD for rescue) with `dd` or the project’s `scripts/flash-nvme.sh`
once provided. First boot expands or verifies `/data` if the Makefile includes
post-install hooks.

### Installing BigFred binaries

Cross-compile from this repository (`CGO_ENABLED=0`, `GOARCH=arm64`) and install
into the Buildroot overlay or package:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o overlay/usr/bin/loco-server ./pkgs/bigfred/server
# dcc-bus binary path per your build layout
```

Bundle `web/dist` under `/usr/share/bigfred/web` (or path configured in
`loco-server`).

## 8.11 Verification after flash

1. Power-on; confirm boot **under 10 s** to listening HTTP port.
2. `ssh` via Dropbear; check `taskset` / CPU affinity in `ps`.
3. `redis-cli ping`; SQLite file on `/data/sqlite/`.
4. Attach Uhlenbrock 63120 (DR5000) or configure `z21` URI — §6, §7.
5. Load-shed test: sustained `dcc-bus` traffic while watching `htop` and frame loss (§4.4).
6. `rotate-hub-logs` by hand; confirm `.gz` under `/data/logs/` and `crontab -l` via BusyBox.

## 8.12 Raspberry Pi OS vs reference image

| Topic | Reference image (this chapter) | Raspberry Pi OS (§3) |
|-------|----------------------------------|----------------------|
| Init | BusyBox `S*` scripts | systemd |
| Root FS | RO `/`, RW `/data` | RW `/` on NVMe |
| RT | Built-in PREEMPT_RT | Package or manual kernel |
| Process layout | `taskset`, isolcpus | Optional nice/systemd |
| Build | `make image` | Imaging tool + apt |

Use Pi OS for **development**; ship **Buildroot image** for club production.

Back to [§3 Host platform](./03-host-platform.md) · [index](./README.md).
