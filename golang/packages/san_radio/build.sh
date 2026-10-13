#!/usr/bin/env bash
# Vet, test, then build bin/san_radio.exe (Windows), bin/san_radio (Linux)
# and bin/san_radio-linux-arm64, bin/san_radio-linux-arm (a Raspberry Pi).
set -euo pipefail

cd "$(dirname "$0")"
version="${SAN_RADIO_VERSION:-$(date +%Y.%m.%d-%H%M)}"

echo "vet..."
go vet ./...

echo "test..."
go test ./...

mkdir -p bin
ldflags="-X main.version=${version}"
export CGO_ENABLED=0

echo "build windows/amd64..."
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "${ldflags}" -o bin/san_radio.exe ./cmd/san_radio

echo "build linux/amd64..."
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "${ldflags}" -o bin/san_radio ./cmd/san_radio

# A 64-bit Raspberry Pi OS, and a 32-bit one. ARMv6 runs on every Pi,
# the Pi 1 and Zero included; ARMv7 would leave those out.
echo "build linux/arm64..."
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "${ldflags}" -o bin/san_radio-linux-arm64 ./cmd/san_radio

echo "build linux/arm (ARMv6)..."
GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags "${ldflags}" -o bin/san_radio-linux-arm ./cmd/san_radio

echo "built bin/san_radio.exe, bin/san_radio, bin/san_radio-linux-arm64 and bin/san_radio-linux-arm (${version})"
