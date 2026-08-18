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
- only `smartctl`, `systemctl`, and the existing `nas-gpu-info` helper still spawn external processes.

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
DISK_TEMP_INTERVAL=30
SMART_INTERVAL=600
SYSTEMD_INTERVAL=30
IP_INTERVAL=60
NAS_FORCE_COLS=...
NAS_FORCE_ROWS=...
NAS_MONITOR_ONESHOT=1
```

`MAIN_INTERVAL` defaults to 5 seconds, but the positional CLI argument has priority.

## Permissions

The current sudoers rule for `/usr/local/bin/nas-gpu-info` can stay as-is.

SMART behavior retains `smartctl -n standby,0`, so a sleeping HDD should not be spun up by the monitor.

Docker is read through `/var/run/docker.sock`. The user running `nasmon` must have access to that socket (normally membership in the `docker` group, which is already required for unprivileged `docker ps`).

## Suggested installation

```bash
sudo install -m 0755 nasmon /usr/local/bin/nasmon
nasmon
```

Keep the Bash version alongside it during the first few days so values can be compared before replacing the old command permanently.
