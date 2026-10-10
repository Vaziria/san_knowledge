#!/usr/bin/env bash
# vet, test, and build the autotrade tool for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
GOOS=windows GOARCH=amd64 go build -o bin/autotrade.exe ./cmd/autotrade
GOOS=linux GOARCH=amd64 go build -o bin/autotrade ./cmd/autotrade

echo "built bin/autotrade.exe and bin/autotrade"
