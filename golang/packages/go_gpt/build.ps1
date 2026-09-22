# vet, test, and build the spike for both platforms.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

New-Item -ItemType Directory -Force bin | Out-Null

$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
go build -o bin/spike.exe ./cmd/spike
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:GOOS = 'linux'; $env:GOARCH = 'amd64'
go build -o bin/spike ./cmd/spike
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Remove-Item Env:GOOS, Env:GOARCH
Write-Host 'built bin/spike.exe and bin/spike'
