#!/usr/bin/env bash
set -euo pipefail
task_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
task_root="$(cd -- "$task_dir/../.." && pwd)"
task_arch="${1:-amd64}"
case "$task_arch" in amd64|arm64) ;; *) echo '仅支持 amd64 或 arm64' >&2; exit 2 ;; esac
mkdir -p "$task_dir/bin"
cd "$task_root/backend"
CGO_ENABLED=0 go build -trimpath -o "$task_dir/bin/s44-ws-fixture" ./cmd/s44-ws-fixture
CGO_ENABLED=0 GOOS=linux GOARCH="$task_arch" go build -trimpath -o "$task_dir/bin/s44-ws-fixture-linux" ./cmd/s44-ws-fixture
