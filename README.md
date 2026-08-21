# nasmon

Go rewrite of the Bash `check_health.sh` NAS monitor.

## Architecture

- collectors run independently at their own intervals;
- collectors update one concurrency-safe snapshot;
- renderer reads the snapshot only and never performs I/O;
- resize uses `SIGWINCH` + `ioctl`, so there is no 1-second `stty` polling;
- CPU/RAM/load/network/disk I/O/temperature are read directly from `/proc`, `/sys` and Go's network API;
- disk usage uses `statfs`, not `df`;
- Docker uses `/var/run/docker.sock`, not the `docker` CLI;
- only `smartctl`, `hdparm`, `systemctl`, and the existing `nas-gpu-info` helper still spawn external processes.

This structure is intentionally ready for future views/tabs: collectors and model do not depend on the renderer.

## Build

```bash
cd nasmon-go
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o nasmon ./cmd/nasmon
```

Run:

```bash
./nasmon
```

or set the main refresh interval:

```bash
./nasmon 5
```

## Environment variables

The existing names are preserved where practical:

```text
NAS_INTERFACE=enp1s0f1
GPU_INFO_HELPER=/usr/local/bin/nas-gpu-info
STORAGE_PATH=/mnt/hdd
GPU_INTERVAL=10
DOCKER_INTERVAL=30
DISK_LAYOUT_INTERVAL=15
DISK_TEMP_INTERVAL=900
DISK_QUIET_WINDOW=300
SMART_INTERVAL=3600
SYSTEMD_INTERVAL=30
IP_INTERVAL=60
NAS_FORCE_COLS=...
NAS_FORCE_ROWS=...
NAS_MONITOR_ONESHOT=1
```

`MAIN_INTERVAL` defaults to 5 seconds, but the positional CLI argument has priority.

`DISK_QUIET_WINDOW` is the amount of time, in seconds, after the last real block I/O before SMART/temperature polling is suppressed for rotational disks. SSDs are not gated by this quiet window. Disk activity is tracked from `/proc/diskstats`, so the activity check itself does not touch the drive.

## Permissions

The current sudoers rule for `/usr/local/bin/nas-gpu-info` can stay as-is.

SMART behavior retains `smartctl -n standby,0`, so a sleeping HDD should not be spun up by the monitor. In addition, rotational HDDs are no longer queried after the quiet window has elapsed, which avoids the monitor itself interfering with a configured spindown timer. By default HDD temperature polling is every 15 minutes and SMART health polling is every hour while the drive is recently active.

For rotational drives, `hdparm -C` is used on the normal disk-layout interval to query ATA power state without spinning the disk up. The last known temperature, SMART health, and R/P/U counters remain visible; a blue `SLEEP` suffix is added when the drive reports standby. If the user cannot issue the power-state ioctl directly, `nasmon` falls back to `sudo -n hdparm -C`.

Docker is read through `/var/run/docker.sock`. The user running `nasmon` must have access to that socket (normally membership in the `docker` group, which is already required for unprivileged `docker ps`).

## Suggested installation

```bash
sudo install -m 0755 nasmon /usr/local/bin/nasmon
nasmon
```

Keep the Bash version alongside it during the first few days so values can be compared before replacing the old command permanently.
