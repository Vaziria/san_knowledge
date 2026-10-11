#!/usr/bin/env bash
# vet, test, and build the san_piano tool. MIDI input works on Windows only,
# so only the Windows binary is built.
set -euo pipefail
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p bin
GOOS=windows GOARCH=amd64 go build -o bin/san_piano.exe ./cmd/san_piano

echo "built bin/san_piano.exe"
