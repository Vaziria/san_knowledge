# One-time setup: fetches the Vosk native library and an English model, and
# puts libvosk where both the linker and the loader find it without any CGO_*
# environment variables, so plain "go run ." works.
#
#   pwsh experimental/transcript/setup.ps1
$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

$runtime = 'https://github.com/alphacep/vosk-api/releases/download/v0.3.45/vosk-win64-0.3.45.zip'
$models  = @{ 'en-us' = 'https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip' }

function Get-Zip($url, $dest) {
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName() + '.zip')
    Write-Host "downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
    Expand-Archive -Path $tmp -DestinationPath $dest -Force
    Remove-Item $tmp
}

# The Go binding's own cgo directives look for the header and library in
# <module>/../src. Vendoring puts the module inside this tree, so that path
# becomes vendor/github.com/alphacep/vosk-api/src and we can populate it.
Push-Location $root
try { go mod vendor } finally { Pop-Location }
$vendorSrc = Join-Path $root 'vendor\github.com\alphacep\vosk-api\src'

if (-not (Test-Path (Join-Path $root 'libvosk.dll'))) {
    $staging = Join-Path $root '.vosk-staging'
    Get-Zip $runtime $staging
    New-Item -ItemType Directory -Force -Path $vendorSrc | Out-Null
    # Header goes where the binding expects to include it from.
    Get-ChildItem $staging -Recurse -Filter 'vosk_api.h' |
        ForEach-Object { Copy-Item $_.FullName $vendorSrc -Force }
    # The DLL goes beside these sources: cgoflags.go links against it with
    # -L${SRCDIR}, and Windows finds it at run time via the working directory.
    Get-ChildItem $staging -Recurse -Filter 'libvosk.dll' |
        ForEach-Object { Copy-Item $_.FullName $root -Force }
    Remove-Item $staging -Recurse -Force
    Write-Host "libvosk -> $root"
} else {
    Write-Host "libvosk already present"
}

foreach ($name in $models.Keys) {
    $target = Join-Path $root "models\$name"
    if (Test-Path (Join-Path $target 'am')) { Write-Host "model $name already present"; continue }
    $staging = Join-Path $root '.model-staging'
    Get-Zip $models[$name] $staging
    # The archives wrap everything in a vosk-model-* directory; lift it out.
    $inner = Get-ChildItem $staging -Directory | Select-Object -First 1
    New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
    if (Test-Path $target) { Remove-Item $target -Recurse -Force }
    Move-Item $inner.FullName $target
    Remove-Item $staging -Recurse -Force
    Write-Host "model $name -> $target"
}

# Indonesian. AlphaCephei publishes no Indonesian model, but bookbot-kids ship a
# complete one (Apache-2.0) inside a Flutter repository, so it is fetched with a
# sparse checkout rather than downloaded as an archive. Expect a few minutes.
$idTarget = Join-Path $root 'models\id'
if (Test-Path (Join-Path $idTarget 'am')) {
    Write-Host "model id already present"
} elseif (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Host "skipping the Indonesian model: git is not on PATH"
} else {
    $repo = 'https://github.com/bookbot-kids/speech-recognizer-bahasa-indonesian.git'
    $inner = 'speech_recognizer/android/app/src/main/assets/model-id-id'
    $staging = Join-Path $root '.id-staging'
    if (Test-Path $staging) { Remove-Item $staging -Recurse -Force }

    Write-Host "cloning the Indonesian model (this takes a few minutes)"
    git clone --filter=blob:none --sparse --depth 1 $repo $staging
    git -C $staging sparse-checkout set $inner

    $src = Join-Path $staging ($inner -replace '/', '\')
    if (-not (Test-Path (Join-Path $src 'am'))) { throw "clone did not produce $src" }
    New-Item -ItemType Directory -Force -Path (Split-Path $idTarget) | Out-Null
    Move-Item $src $idTarget
    # Git marks objects read-only, so a plain delete fails here.
    Get-ChildItem $staging -Recurse -Force | ForEach-Object { $_.Attributes = 'Normal' }
    Remove-Item $staging -Recurse -Force
    Write-Host "model id -> $idTarget"
}

Write-Host ""
Write-Host "done:  cd experimental/transcript; go run ."
