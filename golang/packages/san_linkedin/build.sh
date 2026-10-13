#!/usr/bin/env bash
# vet, test, and build the san_linkedin tool for both platforms.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/san_linkedin.exe ./cmd/san_linkedin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/san_linkedin ./cmd/san_linkedin

echo "built bin/san_linkedin.exe and bin/san_linkedin"
