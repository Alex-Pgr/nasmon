# NAS Health Monitor

A lightweight terminal dashboard for a Linux home NAS, rewritten in Go from the earlier Bash monitor.

## Current MVP

- CPU usage and iowait from `/proc/stat`
- CPU temperature from sysfs/hwmon
- RAM and swap from `/proc/meminfo`
- load average and uptime
- interface/IP detection and RX/TX rate
- disk usage for `/`, `/mnt/ssd`, `/mnt/hdd`
- Docker container status
- AMD GPU temperature + VCN state through the existing `nas-gpu-info` helper
- ANSI terminal UI that adapts to terminal width on every refresh
- graceful Ctrl+C/SIGTERM shutdown

The defaults match the current NAS setup (`enp1s0f1`, `/mnt/hdd`) but can be overridden.

## Build

```bash
go build -o nas-monitor .
```

## Run

```bash
./nas-monitor
```

Options:

```bash
./nas-monitor -interval 5s -interface enp1s0f1 -storage /mnt/hdd
```

Environment variables:

- `NAS_INTERFACE` — default interface
- `STORAGE_PATH` — main storage path
- `GPU_INFO_HELPER` — path to the AMD GPU helper, default `/usr/local/sbin/nas-gpu-info`

For GPU metrics the helper must be executable and allowed through passwordless `sudo -n`, as in the Bash version.

## Next steps

This is the first Go baseline. The next pass should restore the richer v16.5 layout/health blocks (SMART/systemd/storage analysis), then split collectors and rendering into packages and add tests.
