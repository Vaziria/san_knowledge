#!/usr/bin/env pwsh
# Vet, test, then build bin/san_radio.exe (Windows), bin/san_radio (Linux)
# and bin/san_radio-linux-arm64, bin/san_radio-linux-arm (a Raspberry Pi).
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $root
try {
    $version = if ($env:SAN_RADIO_VERSION) { $env:SAN_RADIO_VERSION } else { Get-Date -Format "yyyy.MM.dd-HHmm" }

    Write-Host "vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed" }

    Write-Host "test..."
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }

    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
    $ldflags = "-X main.version=$version"
    $env:CGO_ENABLED = "0"

    Write-Host "build windows/amd64..."
    $env:GOOS = "windows"; $env:GOARCH = "amd64"
    go build -trimpath -ldflags $ldflags -o bin/san_radio.exe ./cmd/san_radio
    if ($LASTEXITCODE -ne 0) { throw "windows build failed" }

    Write-Host "build linux/amd64..."
    $env:GOOS = "linux"; $env:GOARCH = "amd64"
    go build -trimpath -ldflags $ldflags -o bin/san_radio ./cmd/san_radio
    if ($LASTEXITCODE -ne 0) { throw "linux build failed" }

    # A 64-bit Raspberry Pi OS, and a 32-bit one. ARMv6 runs on every Pi,
    # the Pi 1 and Zero included; ARMv7 would leave those out.
    Write-Host "build linux/arm64..."
    $env:GOARCH = "arm64"
    go build -trimpath -ldflags $ldflags -o bin/san_radio-linux-arm64 ./cmd/san_radio
    if ($LASTEXITCODE -ne 0) { throw "linux/arm64 build failed" }

    Write-Host "build linux/arm (ARMv6)..."
    $env:GOARCH = "arm"; $env:GOARM = "6"
    go build -trimpath -ldflags $ldflags -o bin/san_radio-linux-arm ./cmd/san_radio
    if ($LASTEXITCODE -ne 0) { throw "linux/arm build failed" }

    Write-Host "built bin/san_radio.exe, bin/san_radio, bin/san_radio-linux-arm64 and bin/san_radio-linux-arm ($version)"
}
finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:GOARM -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
