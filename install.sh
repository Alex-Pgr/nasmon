#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$repo_root"

if [[ ! -f go.mod || ! -d cmd/nasmon || ! -d cmd/nasmond ]]; then
  echo "install.sh must be run from the nasmon repository" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required but was not found in PATH" >&2
  exit 1
fi

go_version="$(go env GOVERSION 2>/dev/null || true)"
if [[ "$go_version" =~ ^go([0-9]+)\.([0-9]+) ]]; then
  go_major="${BASH_REMATCH[1]}"
  go_minor="${BASH_REMATCH[2]}"
  if (( go_major < 1 || (go_major == 1 && go_minor < 23) )); then
    echo "Go 1.23 or newer is required; found $go_version" >&2
    exit 1
  fi
else
  echo "cannot determine Go version (got: ${go_version:-unknown}); Go 1.23 or newer is required" >&2
  exit 1
fi

service_user="${NASMON_SERVICE_USER:-${SUDO_USER:-$(id -un)}}"
if [[ -z "$service_user" || ! "$service_user" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "invalid NASMON_SERVICE_USER: $service_user" >&2
  exit 1
fi

if (( EUID == 0 )); then
  as_root() { "$@"; }
else
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required for installation into /usr/local and systemd" >&2
    exit 1
  fi
  as_root() { sudo "$@"; }
fi

build_dir="$(mktemp -d)"
service_tmp="$(mktemp)"
cleanup() {
  rm -rf "$build_dir"
  rm -f "$service_tmp"
}
trap cleanup EXIT

echo "==> Running tests"
go test ./...

echo "==> Building nasmon and nasmond"
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$build_dir/nasmon" ./cmd/nasmon
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$build_dir/nasmond" ./cmd/nasmond

cat >"$service_tmp" <<EOF
[Unit]
Description=NAS Monitor collector daemon
After=network.target

[Service]
Type=simple
User=$service_user
EnvironmentFile=-/etc/nasmon/nasmon.env
RuntimeDirectory=nasmon
RuntimeDirectoryMode=0755
ExecStart=/usr/local/bin/nasmond
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

echo "==> Installing binaries"
as_root install -m 0755 "$build_dir/nasmon" /usr/local/bin/nasmon
as_root install -m 0755 "$build_dir/nasmond" /usr/local/bin/nasmond

echo "==> Installing host configuration example"
as_root install -d -m 0755 /etc/nasmon
as_root install -m 0644 config/nasmon.env.example /etc/nasmon/nasmon.env.example
if [[ ! -e /etc/nasmon/nasmon.env ]]; then
  echo "    optional config: copy /etc/nasmon/nasmon.env.example to /etc/nasmon/nasmon.env"
else
  echo "    preserving existing /etc/nasmon/nasmon.env"
fi

echo "==> Installing systemd service for user $service_user"
as_root install -m 0644 "$service_tmp" /etc/systemd/system/nasmond.service
as_root systemctl daemon-reload
as_root systemctl enable nasmond.service >/dev/null
as_root systemctl restart nasmond.service

if ! as_root systemctl is-active --quiet nasmond.service; then
  echo "nasmond failed to become active" >&2
  as_root systemctl status nasmond.service --no-pager || true
  exit 1
fi

echo "==> Installed successfully"
echo "    nasmon:  /usr/local/bin/nasmon"
echo "    nasmond: /usr/local/bin/nasmond"
echo "    service: active (user: $service_user)"
echo "    config:  /etc/nasmon/nasmon.env (optional, never overwritten)"
