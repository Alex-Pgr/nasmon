# nasmon

Go rewrite of the Bash `check_health.sh` NAS monitor.

## Architecture

`nasmond` is the single collector process. It runs all hardware/Docker/system collectors, updates one concurrency-safe snapshot, and atomically publishes JSON state to `/run/nasmon/state.json` by default. Any number of `nasmon` TUI clients can read that shared snapshot without repeating privileged polling.

The writer uses a non-blocking `flock` in the state directory, so a second `nasmond` exits instead of creating duplicate collectors. State updates are written to a temporary file in the same directory and published with atomic `rename(2)`, so readers see either the complete previous snapshot or the complete new snapshot and do not need file locks.

`nasmon --standalone` preserves the previous architecture for recovery/debugging: the TUI launches its own collectors and does not depend on `nasmond`.

Collectors still run independently at their own intervals. The renderer performs no hardware or Docker I/O. CPU/RAM/load/network/disk I/O are read directly from `/proc`, `/sys` and Go APIs; disk usage uses `statfs`; Docker uses `/var/run/docker.sock`; only `smartctl`, `hdparm`, `systemctl`, and the existing `nas-gpu-info` helper spawn external processes.

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

It contains a versioned JSON envelope and the latest snapshot. Override the path for both daemon and clients with `NAS_STATE_FILE` if needed. The systemd template uses `RuntimeDirectory=nasmon`, so `/run/nasmon` is created in tmpfs and owned by the service user.

## Environment variables

```text
NAS_INTERFACE=enp1s0f1
GPU_INFO_HELPER=/usr/local/bin/nas-gpu-info
STORAGE_PATH=/mnt/hdd
NAS_STATE_FILE=/run/nasmon/state.json
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

`DISK_QUIET_WINDOW` is the amount of time, in seconds, after the last real block I/O before SMART/temperature polling is suppressed for rotational disks. SSDs are not gated by this quiet window. Disk activity is tracked from `/proc/diskstats`, so the activity check itself does not touch the drive.

## Permissions

The current sudoers rule for `/usr/local/bin/nas-gpu-info` can stay as-is. `nasmond` is the only normal process that performs those privileged polls, so opening multiple TUI sessions no longer multiplies sudo/PAM/journald traffic.

SMART behavior retains `smartctl -n standby,0`, so a sleeping HDD should not be spun up by the monitor. By default HDD temperature polling is every 15 minutes and SMART health polling is every hour while the drive is recently active.

For rotational drives, `hdparm -C` is queried independently every 60 seconds by default. The last known temperature, SMART health, and R/P/U counters remain visible; a blue `SLEEP` suffix is added when the drive reports standby. If direct access fails, `nasmond` falls back to `sudo -n hdparm -C`. When non-interactive sudo reports that authorization is unavailable, further sudo attempts for that device are suppressed for 10 minutes before retrying.

Docker is read through `/var/run/docker.sock`. The user running `nasmond` must have access to that socket, normally through membership in the `docker` group.

## Suggested installation

```bash
sudo install -m 0755 nasmon /usr/local/bin/nasmon
sudo install -m 0755 nasmond /usr/local/bin/nasmond
sudo install -m 0644 nasmond.service.example /etc/systemd/system/nasmond.service
sudo systemctl daemon-reload
sudo systemctl enable --now nasmond
nasmon
```

Check the collector with:

```bash
systemctl status nasmond
cat /run/nasmon/state.json | jq '.version, .written_at'
```
