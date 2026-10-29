#!/usr/bin/env bash
# vet, test, and build ctxcompiler for Linux and Windows into bin/.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
GOOS=linux GOARCH=amd64 go build -o bin/ctxcompiler ./cmd/ctxcompiler
GOOS=windows GOARCH=amd64 go build -o bin/ctxcompiler.exe ./cmd/ctxcompiler
echo 'built bin/ctxcompiler and bin/ctxcompiler.exe'
