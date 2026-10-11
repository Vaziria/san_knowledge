# Builds the Knowledge UI (ui/, React + TypeScript + Chakra UI, into internal/ownview/dist, which the binary embeds),
# then tests and builds bin/knowledge.exe (Windows) and bin/knowledge (Linux). Needs Node.js.
#   pwsh san_knowledge/build.ps1
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    Push-Location ui
    try {
        if (-not (Test-Path node_modules)) { npm ci --no-audit --no-fund; if ($LASTEXITCODE) { throw 'npm ci failed' } }
        npm run build
        if ($LASTEXITCODE) { throw 'UI build failed' }
    } finally { Pop-Location }
    go vet ./...
    if ($LASTEXITCODE) { throw 'go vet failed' }
    go test ./...
    if ($LASTEXITCODE) { throw 'tests failed' }
    $version = Get-Date -Format 'yyyy.MM.dd-HHmm'
    $ldflags = "-s -w -X main.version=$version"
    $env:GOARCH = 'amd64'
    $env:GOOS = 'windows'
    go build -trimpath -ldflags $ldflags -o ../bin/knowledge.exe ./cmd/knowledge
    if ($LASTEXITCODE) { throw 'go build (windows) failed' }
    $env:GOOS = 'linux'
    $env:CGO_ENABLED = '0'  # with gcc on PATH, cgo would try to build runtime/cgo for linux and fail
    go build -trimpath -ldflags $ldflags -o ../bin/knowledge ./cmd/knowledge
    if ($LASTEXITCODE) { throw 'go build (linux) failed' }
    Write-Host "built bin/knowledge.exe and bin/knowledge ($version)"
} finally { Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue; Pop-Location }
