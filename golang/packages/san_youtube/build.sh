#!/usr/bin/env bash
# vet, test, and build the san_youtube tool for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/san_youtube.exe ./cmd/san_youtube
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/san_youtube ./cmd/san_youtube

echo "built bin/san_youtube.exe and bin/san_youtube"
