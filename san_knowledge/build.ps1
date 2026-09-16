# Tests and builds bin/knowledge.exe. The Knowledge UI (internal/ownview/static)
# is plain HTML/CSS/JS embedded in the binary, so no npm step is needed.
#   pwsh san_knowledge/build.ps1
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    go vet ./...
    if ($LASTEXITCODE) { throw 'go vet failed' }
    go test ./...
    if ($LASTEXITCODE) { throw 'tests failed' }
    $version = Get-Date -Format 'yyyy.MM.dd-HHmm'
    go build -trimpath -ldflags "-s -w -X main.version=$version" -o ../bin/knowledge.exe ./cmd/knowledge
    if ($LASTEXITCODE) { throw 'go build failed' }
    Write-Host "built $(Resolve-Path ../bin/knowledge.exe) ($version)"
} finally { Pop-Location }
