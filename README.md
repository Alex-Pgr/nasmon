# nasmon

Go rewrite of the Bash `check_health.sh` NAS monitor.

## Architecture

`nasmond` is the single collector process. It runs all hardware/Docker/system collectors, updates one concurrency-safe snapshot, and atomically publishes JSON state to `/run/nasmon/state.json` by default. Any number of `nasmon` TUI clients can read that shared snapshot without repeating privileged polling.

The writer uses a non-blocking `flock` in the state directory, so a second `nasmond` exits instead of creating duplicate collectors. State updates are written to a temporary file in the same directory and published with atomic `rename(2)`, so readers see either the complete previous snapshot or the complete new snapshot and do not need file locks.

`nasmon --standalone` preserves the previous architecture for recovery/debugging: the TUI launches its own collectors and does not depend on `nasmond`.

Collectors still run independently at their own intervals. The renderer performs no hardware or Docker I/O. CPU/RAM/load/network/disk I/O are read directly from `/proc`, `/sys` and Go APIs; disk usage uses `statfs`; Docker uses `/var/run/docker.sock`; only `smartctl`, `hdparm`, `systemctl`, and an optional GPU helper spawn external processes.

## Portable defaults

The repository no longer assumes a particular NAS layout. With no host configuration:

- network interface is selected from the Linux default route;
- GPU helper collection is disabled;
- Storage Analysis uses `/`;
- Disk Usage contains `/`;
- shared state is `/run/nasmon/state.json`.

Docker, `smartctl`, `hdparm`, and the GPU helper are optional capabilities. If they are unavailable, the rest of the monitor continues to run.

Host-specific settings belong in `/etc/nasmon/nasmon.env`, not in the source tree. Both `nasmond` and `nasmon` load that file directly, so a client launched from a normal shell uses the same state path and timing as the daemon. Process environment variables override values from the file. `config/nasmon.env.example` documents the available values.

## Build

Go 1.23 or newer is required when building from source.

```bash
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o nasmon ./cmd/nasmon
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o nasmond ./cmd/nasmond
```

Normal operation uses the daemon plus one or more clients:

```bash
nasmond
nasmon
```

For recovery/debugging without the daemon:

```bash
nasmon --standalone
```

The optional positional number controls the TUI refresh/read interval (and the collector interval in standalone mode):

```bash
nasmon 5
nasmon --standalone 5
```

## ARM SBC support

The supported ARM target is **64-bit Linux (`arm64` / `aarch64`)** on modern Raspberry Pi and Orange Pi systems. The intended deployments are:

- Raspberry Pi 4 and Raspberry Pi 5 running 64-bit Raspberry Pi OS;
- Orange Pi boards running 64-bit Armbian or Ubuntu;
- other 64-bit Linux SBCs with standard `/proc`, `/sys`, systemd, and Linux block/network interfaces may work, but are not specifically validated.

CI cross-builds both binaries for Linux `arm64`. It also keeps an ARMv7 cross-build as a regression check, but old 32-bit Raspberry Pi systems are not part of the supported target set and are best-effort only.

CPU temperature prefers the x86 `coretemp`/`k10temp` hwmon drivers when present, then uses Linux thermal zones whose type identifies a CPU/SoC/package sensor. This covers the common Raspberry Pi and Orange Pi thermal-zone layout without accidentally treating a GPU/DDR sensor as CPU temperature. Generic hwmon remains the final fallback.

microSD/eMMC devices such as `mmcblk0` remain visible for disk usage, identity, and I/O, but are not sent to `smartctl`, because Linux MMC devices normally do not implement ATA/NVMe SMART. SATA/USB-SATA/NVMe devices continue to use the regular SMART pipeline.

### ARM deployment checklist

For a first install on a Raspberry Pi, Orange Pi, or another ARM SBC, start with the portable defaults and avoid creating `/etc/nasmon/nasmon.env` unless the host actually needs overrides. This validates that default-route network selection and generic storage discovery work correctly on the board.

Check the OS, architecture, and Go toolchain before installation:

```bash
uname -a
uname -m
cat /etc/os-release
go version
```

For supported systems, `uname -m` should normally report `aarch64`. Building from source requires Go 1.23 or newer.

Optional disk-health tools can be installed on Debian/Ubuntu/Raspberry Pi OS/Armbian hosts with:

```bash
sudo apt update && sudo apt install -y smartmontools hdparm
```

Docker is optional and should only be installed if the host uses it.

Install nasmon from the repository:

```bash
git clone https://github.com/Alex-Pgr/nas_monitoring.git
cd nas_monitoring
./install.sh
```

Immediately after installation, run:

```bash
nasmon doctor
systemctl status nasmond --no-pager
```

`WARN` is expected for optional integrations that are not installed or not used, such as Docker, `smartctl`, `hdparm`, or a GPU helper. `FAIL` indicates a required host/configuration problem that should be fixed before treating the installation as healthy.

Inspect block devices and ARM thermal zones:

```bash
lsblk -o NAME,TYPE,SIZE,FSTYPE,MOUNTPOINTS
for z in /sys/class/thermal/thermal_zone*; do echo "== $z =="; cat "$z/type" 2>/dev/null; cat "$z/temp" 2>/dev/null; done
```

On Raspberry Pi or Orange Pi systems using microSD/eMMC, devices such as `mmcblk0` should remain visible in disk usage and I/O metrics, but they should not cause the SMART or disk-temperature collectors to report an error merely because MMC media does not support ATA/NVMe SMART.

Finally, verify the published daemon state and open the TUI:

```bash
cat /run/nasmon/state.json | jq '{written_at, cpu_temp:.snapshot.cpu_temp_c, disks:.snapshot.disk_health}'
nasmon
```

For a first real-board smoke test, the key things to verify are: CPU temperature is plausible, the network interface is selected without `NAS_INTERFACE`, `mmcblk*` storage is shown correctly, and there are no false SMART errors from the system microSD/eMMC device.

## Doctor

Run the built-in host diagnostics after installation, after changing configuration, or when bringing nasmon to a new machine:

```bash
nasmon doctor
```

The doctor checks the Linux/CPU architecture, selected config file, configured storage and disk paths, network interface/default route, `smartctl`, `hdparm`, `systemctl`, the optional GPU helper, Docker socket access, and freshness/readability of the daemon state file. Optional capabilities are reported as `WARN`; broken required host configuration such as a missing configured storage path or network interface is reported as `FAIL` and makes the command exit non-zero.

This command is intentionally useful before all optional integrations are installed, so a minimal host can still be considered runnable with warnings.

## Shared state

The default state file is:

```text
/run/nasmon/state.json
```

It contains a versioned JSON envelope and the latest snapshot. Override the path for both daemon and clients with `NAS_STATE_FILE` if needed. The systemd unit uses `RuntimeDirectory=nasmon`, so `/run/nasmon` is created in tmpfs and owned by the service user.

## Host configuration

Create `/etc/nasmon/nasmon.env` only for values that differ from the portable defaults. For example:

```bash
NAS_INTERFACE=enp1s0f1
GPU_INFO_HELPER=/usr/local/bin/nas-gpu-info
STORAGE_PATH=/mnt/hdd
DISK_PATHS=/,/mnt/ssd,/mnt/hdd
```

`DISK_PATHS` is a comma-separated list of mounted paths shown in the Disk Usage section. Root is displayed first, configured mounts follow in `DISK_PATHS` order, and any additional autodetected block-device mounts are listed alphabetically. An empty or missing value falls back to `/`.

Available environment variables:

```text
NAS_INTERFACE=             # empty = default-route autodetection
GPU_INFO_HELPER=           # empty = disabled
STORAGE_PATH=/             # highlighted filesystem
DISK_PATHS=/               # comma-separated mount paths
NAS_STATE_FILE=/run/nasmon/state.json
MAIN_INTERVAL=2
GPU_INTERVAL=10
DOCKER_INTERVAL=30
DISK_LAYOUT_INTERVAL=15
DISK_POWER_INTERVAL=60
DISK_TEMP_INTERVAL=900
DISK_QUIET_WINDOW=300
SMART_INTERVAL=3600
SYSTEMD_INTERVAL=30
IP_INTERVAL=60
NAS_FORCE_COLS=...
NAS_FORCE_ROWS=...
NAS_MONITOR_ONESHOT=1
```

`MAIN_INTERVAL` defaults to 2 seconds. A positional CLI interval overrides it for `nasmon`.

For development or alternate packaging, `NASMON_CONFIG_FILE=/path/to/file` selects a different env-style config file. The default remains `/etc/nasmon/nasmon.env`.

`DISK_QUIET_WINDOW` is the amount of time, in seconds, after the last real block I/O before SMART/temperature polling is suppressed for rotational disks. SSDs are not gated by this quiet window. Disk activity is tracked from `/proc/diskstats`, so the activity check itself does not touch the drive.

## Permissions and optional integrations

If a GPU helper is configured and requires privilege, grant only that helper the needed sudo permission. `nasmond` is the only normal process that performs those polls, so opening multiple TUI sessions does not multiply sudo/PAM/journald traffic.

SMART behavior retains `smartctl -n standby,0`, so a sleeping HDD should not be spun up by the monitor. By default HDD temperature polling is every 15 minutes and SMART health polling is every hour while the drive is recently active. Systems without `smartctl` simply do not get SMART data.

For rotational drives, `hdparm -C` is queried independently every 60 seconds by default. The last known temperature, SMART health, and R/P/U counters remain visible; a blue `SLEEP` suffix is added when the drive reports standby. If direct access fails, `nasmond` falls back to `sudo -n hdparm -C`. When non-interactive sudo reports that authorization is unavailable, further sudo attempts for that device are suppressed for 10 minutes before retrying. Systems without `hdparm` continue without power-state data.

Docker is read through `/var/run/docker.sock`. To show Docker data, the user running `nasmond` must have access to that socket, normally through membership in the `docker` group. Docker itself is not required for the daemon to start.

## Installation and upgrades

Run the installer as your normal login user from the repository root:

```bash
./install.sh
```

The installer fails fast, requires Go 1.23 or newer, runs `go test ./...`, builds static `nasmon` and `nasmond` binaries for the host architecture, installs them into `/usr/local/bin`, installs `/etc/systemd/system/nasmond.service`, runs `systemctl daemon-reload`, enables the service, restarts it, and verifies that it becomes active. Privileged installation steps use `sudo`; the Go build itself runs as the invoking user.

It also installs the current configuration template as:

```text
/etc/nasmon/nasmon.env.example
```

An existing `/etc/nasmon/nasmon.env` is never overwritten during upgrades. To configure a host for the first time:

```bash
sudo cp /etc/nasmon/nasmon.env.example /etc/nasmon/nasmon.env
sudo editor /etc/nasmon/nasmon.env
sudo systemctl restart nasmond
```

By default the systemd service runs as the invoking user. Override that explicitly when needed:

```bash
NASMON_SERVICE_USER=monitoring ./install.sh
```

For later upgrades:

```bash
git pull --ff-only && ./install.sh
```

Because the script uses `set -euo pipefail`, a failed test or build stops before installed binaries are replaced. The same script is safe to run repeatedly; existing binaries and the systemd unit are updated in place and `nasmond` is restarted only after successful tests/builds.

Manual installation remains possible using `nasmond.service.example`, but `install.sh` is the preferred path. Replace the `YOUR_USER` placeholder before installing the example unit manually.

Check the collector with:

```bash
nasmon doctor
systemctl status nasmond --no-pager
cat /run/nasmon/state.json | jq '.version, .written_at'
```
