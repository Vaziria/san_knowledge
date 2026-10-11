#!/usr/bin/env bash
# vet, test, and build the san_tiktok tool for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/san_tiktok.exe ./cmd/san_tiktok
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/san_tiktok ./cmd/san_tiktok

echo "built bin/san_tiktok.exe and bin/san_tiktok"
