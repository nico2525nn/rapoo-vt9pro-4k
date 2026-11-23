#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p build/bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/bin/rapoo-bridge ./cmd/rapoo-bridge
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/bin/rapoo-bridge.exe ./cmd/rapoo-bridge
chmod +x build/bin/rapoo-bridge

file build/bin/rapoo-bridge | grep -q 'x86-64'
file build/bin/rapoo-bridge.exe | grep -q 'x86-64'
echo "sidecars: $(file build/bin/rapoo-bridge build/bin/rapoo-bridge.exe | tr '\n' ' ')"
