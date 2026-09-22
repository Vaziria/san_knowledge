# Tests and builds bin/knowledge.exe (Windows) and bin/knowledge (Linux). The Knowledge UI (internal/ownview/static)
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
    $ldflags = "-s -w -X main.version=$version"
    $env:GOARCH = 'amd64'
    $env:GOOS = 'windows'
    go build -trimpath -ldflags $ldflags -o ../bin/knowledge.exe ./cmd/knowledge
    if ($LASTEXITCODE) { throw 'go build (windows) failed' }
    $env:GOOS = 'linux'
    go build -trimpath -ldflags $ldflags -o ../bin/knowledge ./cmd/knowledge
    if ($LASTEXITCODE) { throw 'go build (linux) failed' }
    Write-Host "built bin/knowledge.exe and bin/knowledge ($version)"
} finally { Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue; Pop-Location }
