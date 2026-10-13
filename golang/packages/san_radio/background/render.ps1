#!/usr/bin/env pwsh
# Render background/terminal.html into assets/loop.mp4: a screenshot with
# headless Edge, then 10 seconds of it as H.264 at 30 fps. A still picture
# is light for the Raspberry Pi to decode.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Push-Location $root
try {
    $edge = "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
    if (-not (Test-Path $edge)) { $edge = "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe" }
    $html = (Resolve-Path background/terminal.html).Path
    $png = Join-Path ([IO.Path]::GetTempPath()) "san_radio-terminal.png"
    # The virtual time budget lets the font load and the canvas draw first.
    & $edge --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=1 `
        --allow-file-access-from-files --virtual-time-budget=3000 `
        --window-size=1280,720 "--screenshot=$png" "file:///$($html.Replace('\', '/'))" 2>$null | Out-Null
    if (-not (Test-Path $png)) { throw "Edge made no screenshot" }

    New-Item -ItemType Directory -Force -Path assets | Out-Null
    ffmpeg -hide_banner -loglevel error -y -loop 1 -framerate 30 -i $png -t 10 `
        -vf "scale=1280:720,format=yuv420p" -c:v libx264 -preset slow -tune stillimage -crf 18 -g 60 assets/loop.mp4
    if ($LASTEXITCODE -ne 0) { throw "ffmpeg failed" }
    Remove-Item $png
    Write-Host "rendered assets/loop.mp4"
}
finally {
    Pop-Location
}
