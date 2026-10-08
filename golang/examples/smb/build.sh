#!/usr/bin/env bash
# go vet + go test, then build bin/smbd(.exe) for Windows and Linux.
set -euo pipefail
cd "$(dirname "$0")"
echo "vet...";  go vet ./...
echo "test..."; go test ./...
mkdir -p bin
echo "build windows/amd64..."; GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/smbd.exe ./cmd/smbd
echo "build linux/amd64...";   GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/smbd     ./cmd/smbd
echo "built bin/smbd.exe and bin/smbd"
