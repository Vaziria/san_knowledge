#!/usr/bin/env bash
# vet, test, and build the san_medium tool for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/san_medium.exe ./cmd/san_medium
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/san_medium ./cmd/san_medium

echo "built bin/san_medium.exe and bin/san_medium"
