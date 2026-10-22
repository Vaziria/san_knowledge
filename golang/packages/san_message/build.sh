#!/usr/bin/env bash
# Vet, test, then build bin/san_message.exe (Windows) and bin/san_message (Linux).
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
GOOS=windows GOARCH=amd64 go build -ldflags "${ldflags}" -o bin/san_message.exe ./cmd/san_message

echo "build linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o bin/san_message ./cmd/san_message

echo "built bin/san_message.exe and bin/san_message (${version})"
