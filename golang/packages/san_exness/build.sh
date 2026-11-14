#!/usr/bin/env bash
# Vet, test, then build bin/san_exness.exe (Windows) and bin/san_exness (Linux).
set -euo pipefail

cd "$(dirname "$0")"

version="$(date +%Y.%m.%d-%H%M)"
ldflags="-X main.version=${version}"

echo "vet..."
go vet ./...

echo "test..."
go test ./...

mkdir -p bin

# No cgo is needed, and with it on the Linux build would look for a Linux C compiler.
export CGO_ENABLED=0

echo "build windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags "${ldflags}" -o bin/san_exness.exe ./cmd/san_exness

echo "build linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o bin/san_exness ./cmd/san_exness

echo "built bin/san_exness.exe and bin/san_exness (${version})"
