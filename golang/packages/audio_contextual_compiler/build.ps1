# vet, test, and build ctxcompiler for Windows and Linux into bin/.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

New-Item -ItemType Directory -Force bin | Out-Null

$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
go build -o bin/ctxcompiler.exe ./cmd/ctxcompiler
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:GOOS = 'linux'
go build -o bin/ctxcompiler ./cmd/ctxcompiler
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Remove-Item Env:GOOS, Env:GOARCH
Write-Host 'built bin/ctxcompiler.exe and bin/ctxcompiler'
