#!/usr/bin/env bash
# Build the UI (typescripts/san_tower_frontend, into internal/web/dist, which
# the binary embeds), vet, test, then build bin/san_tower.exe (Windows) and
# bin/san_tower (Linux). Needs Node.js.
set -euo pipefail

cd "$(dirname "$0")"

version="$(date +%Y.%m.%d-%H%M)"
ldflags="-X main.version=${version}"

echo "ui..."
(
  cd ../../../typescripts/san_tower_frontend
  [ -d node_modules ] || npm ci --no-audit --no-fund
  npm run build
)

echo "vet..."
go vet ./...

echo "test..."
go test ./...

mkdir -p bin

# No cgo is needed, and with it on the Linux build would look for a Linux C compiler.
export CGO_ENABLED=0

echo "build windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags "${ldflags}" -o bin/san_tower.exe ./cmd/san_tower

echo "build linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o bin/san_tower ./cmd/san_tower

echo "built bin/san_tower.exe and bin/san_tower (${version})"
