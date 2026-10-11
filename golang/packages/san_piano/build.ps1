# vet, test, and build the san_piano tool. MIDI input works on Windows only,
# so only the Windows binary is built.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

New-Item -ItemType Directory -Force bin | Out-Null

$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
go build -o bin/san_piano.exe ./cmd/san_piano
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Remove-Item Env:GOOS, Env:GOARCH
Write-Host 'built bin/san_piano.exe'
