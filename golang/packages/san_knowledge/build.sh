#!/usr/bin/env bash
# Builds the Knowledge UI (ui/, React + TypeScript + Chakra UI, into internal/ownview/dist, which the binary embeds),
# then tests and builds bin/knowledge (Linux) and bin/knowledge.exe (Windows). Needs Node.js.
#   bash golang/packages/san_knowledge/build.sh
set -euo pipefail
cd "$(dirname "$0")"
(cd ui && { [ -d node_modules ] || npm ci --no-audit --no-fund; } && npm run build)
go vet ./...
go test ./...
version=$(date +%Y.%m.%d-%H%M)
ldflags="-s -w -X main.version=$version"
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o ../../../bin/knowledge ./cmd/knowledge
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o ../../../bin/knowledge.exe ./cmd/knowledge
echo "built bin/knowledge and bin/knowledge.exe ($version)"
