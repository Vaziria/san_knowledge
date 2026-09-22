#!/usr/bin/env bash
# Tests and builds bin/knowledge (Linux) and bin/knowledge.exe (Windows).
# The Knowledge UI (internal/ownview/static) is embedded, so no npm step is needed.
#   bash san_knowledge/build.sh
set -euo pipefail
cd "$(dirname "$0")"
go vet ./...
go test ./...
version=$(date +%Y.%m.%d-%H%M)
ldflags="-s -w -X main.version=$version"
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o ../bin/knowledge ./cmd/knowledge
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o ../bin/knowledge.exe ./cmd/knowledge
echo "built bin/knowledge and bin/knowledge.exe ($version)"
