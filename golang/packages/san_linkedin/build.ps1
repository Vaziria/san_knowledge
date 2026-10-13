# vet, test, and build the san_linkedin tool for both platforms.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

New-Item -ItemType Directory -Force bin | Out-Null

# Pure Go: without this, a global CGO_ENABLED=1 makes the Linux cross build
# look for a C compiler for Linux.
$env:CGO_ENABLED = '0'

$env:GOOS = 'windows'; $env:GOARCH = 'amd64'
go build -o bin/san_linkedin.exe ./cmd/san_linkedin
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:GOOS = 'linux'; $env:GOARCH = 'amd64'
go build -o bin/san_linkedin ./cmd/san_linkedin
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
Write-Host 'built bin/san_linkedin.exe and bin/san_linkedin'
