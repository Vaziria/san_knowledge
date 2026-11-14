#!/usr/bin/env pwsh
# Vet, test, then build bin/san_backtest.exe (Windows) and bin/san_backtest (Linux).
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $root
try {
    $version = Get-Date -Format "yyyy.MM.dd-HHmm"

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
    go build -ldflags $ldflags -o bin/san_backtest.exe ./cmd/san_backtest
    if ($LASTEXITCODE -ne 0) { throw "windows build failed" }

    Write-Host "build linux/amd64..."
    $env:GOOS = "linux"; $env:GOARCH = "amd64"
    go build -ldflags $ldflags -o bin/san_backtest ./cmd/san_backtest
    if ($LASTEXITCODE -ne 0) { throw "linux build failed" }

    Write-Host "built bin/san_backtest.exe and bin/san_backtest ($version)"
}
finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
