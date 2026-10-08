#!/usr/bin/env pwsh
# go vet + go test, then build bin/smbd.exe (Windows) and bin/smbd (Linux).
$ErrorActionPreference = "Stop"
Push-Location (Split-Path -Parent $MyInvocation.MyCommand.Path)
try {
    Write-Host "vet...";  go vet ./...;  if ($LASTEXITCODE -ne 0) { throw "go vet failed" }
    Write-Host "test..."; go test ./...; if ($LASTEXITCODE -ne 0) { throw "go test failed" }
    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
    $env:CGO_ENABLED = "0"
    Write-Host "build windows/amd64..."
    $env:GOOS = "windows"; $env:GOARCH = "amd64"
    go build -trimpath -o bin/smbd.exe ./cmd/smbd; if ($LASTEXITCODE -ne 0) { throw "windows build failed" }
    Write-Host "build linux/amd64..."
    $env:GOOS = "linux"; $env:GOARCH = "amd64"
    go build -trimpath -o bin/smbd ./cmd/smbd; if ($LASTEXITCODE -ne 0) { throw "linux build failed" }
    Write-Host "built bin/smbd.exe and bin/smbd"
}
finally {
    Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
