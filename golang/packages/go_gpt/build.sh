#!/usr/bin/env bash
# vet, test, and build the spike for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
GOOS=windows GOARCH=amd64 go build -o bin/spike.exe ./cmd/spike
GOOS=linux GOARCH=amd64 go build -o bin/spike ./cmd/spike

echo "built bin/spike.exe and bin/spike"
