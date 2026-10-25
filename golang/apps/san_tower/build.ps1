#!/usr/bin/env pwsh
# Build the UI (typescripts/san_tower_frontend, into internal/web/dist, which
# the binary embeds), vet, test, then build bin/san_tower.exe (Windows),
# bin/san_tower (Linux) and bin/san_tower-linux-arm64 (Raspberry Pi). Needs
# Node.js.
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $root
try {
    # The release pipeline gives the tag (v1.0.0); a local build the date.
    $version = if ($env:SAN_TOWER_VERSION) { $env:SAN_TOWER_VERSION } else { Get-Date -Format "yyyy.MM.dd-HHmm" }

    Write-Host "ui..."
    Push-Location ../../../typescripts/san_tower_frontend
    try {
        if (-not (Test-Path node_modules)) { npm ci --no-audit --no-fund; if ($LASTEXITCODE -ne 0) { throw "npm ci failed" } }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "UI build failed" }
    }
    finally { Pop-Location }

    Write-Host "vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed" }

    Write-Host "test..."
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }

    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
    $ldflags = "-X main.version=$version"
    # No cgo is needed, and with it on the Linux build would look for a Linux C compiler.
    $env:CGO_ENABLED = "0"

    Write-Host "build windows/amd64..."
    $env:GOOS = "windows"; $env:GOARCH = "amd64"
    go build -ldflags $ldflags -o bin/san_tower.exe ./cmd/san_tower
    if ($LASTEXITCODE -ne 0) { throw "windows build failed" }

    Write-Host "build linux/amd64..."
    $env:GOOS = "linux"; $env:GOARCH = "amd64"
    go build -ldflags $ldflags -o bin/san_tower ./cmd/san_tower
    if ($LASTEXITCODE -ne 0) { throw "linux build failed" }

    # 64-bit ARM Linux: the Raspberry Pi (the user's, 2026-10-26).
    Write-Host "build linux/arm64..."
    $env:GOOS = "linux"; $env:GOARCH = "arm64"
    go build -ldflags $ldflags -o bin/san_tower-linux-arm64 ./cmd/san_tower
    if ($LASTEXITCODE -ne 0) { throw "linux/arm64 build failed" }

    Write-Host "built bin/san_tower.exe, bin/san_tower and bin/san_tower-linux-arm64 ($version)"
}
finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
