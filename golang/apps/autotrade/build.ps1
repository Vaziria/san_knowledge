# vet, test, and build the autotrade tool for both platforms.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

New-Item -ItemType Directory -Force bin | Out-Null

$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
go build -o bin/autotrade.exe ./cmd/autotrade
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:GOOS = 'linux'; $env:GOARCH = 'amd64'
go build -o bin/autotrade ./cmd/autotrade
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Remove-Item Env:GOOS, Env:GOARCH
Write-Host 'built bin/autotrade.exe and bin/autotrade'
