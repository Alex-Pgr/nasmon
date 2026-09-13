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

The installer fails fast, runs `go test ./...`, builds static `nasmon` and `nasmond` binaries, installs them into `/usr/local/bin`, installs `/etc/systemd/system/nasmond.service`, runs `systemctl daemon-reload`, enables the service, restarts it, and verifies that it becomes active. Privileged installation steps use `sudo`; the Go build itself runs as the invoking user.

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
systemctl status nasmond --no-pager
cat /run/nasmon/state.json | jq '.version, .written_at'
```
