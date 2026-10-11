#!/usr/bin/env bash
# vet, test, and build the spike for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/spike.exe ./cmd/spike
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/spike ./cmd/spike

echo "built bin/spike.exe and bin/spike"
