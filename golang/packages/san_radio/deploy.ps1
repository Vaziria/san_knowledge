#!/usr/bin/env pwsh
# Copy san_radio to a Raspberry Pi over SSH and (re)start it there. It needs
# no sudo:
#   - ffmpeg is the system's when installed; otherwise Debian's packages are
#     fetched with `apt-get download` (checked against the signed package
#     lists, as apt does) and unpacked into ~/san_radio/ffmpeg, which
#     ffmpeg.sh runs;
#   - run.sh keeps san_radio running, and cron starts it at boot (@reboot).
#
#   pwsh deploy.ps1                                   # to the SSH host "raspi"
#   pwsh deploy.ps1 -Video D:\renders\loop.mp4 -Music D:\music
#
# Run build.ps1 first. It copies the program, run.sh, the video
# (assets/loop.mp4), the .wav files the Pi lacks (from assets/) and the
# stream settings (assets/channels.yaml), then restarts the stream: a few
# seconds off the air. On the Pi everything is in ~/san_radio, and the log
# is ~/san_radio/san_radio.log.
param(
    [string]$Target = "raspi",
    [string]$Video = "assets/loop.mp4",
    [string]$Music = "assets",
    [string]$Config = "assets/channels.yaml"
)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Remote runs a shell command on the Pi and returns its output.
function Remote([string]$cmd) {
    $out = ssh -o BatchMode=yes $Target $cmd
    if ($LASTEXITCODE -ne 0) { throw "on ${Target}: '$cmd' failed" }
    return $out
}

# RemoteScript runs a multi-line sh script on the Pi. It goes base64-encoded:
# PowerShell would turn its line endings into CRLF on the way to ssh.
function RemoteScript([string]$script) {
    $b64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($script.Replace("`r`n", "`n")))
    ssh -o BatchMode=yes $Target "echo $b64 | base64 -d | sh"
    if ($LASTEXITCODE -ne 0) { throw "a script on $Target failed" }
}

# Upload sends a file to ~/san_radio/$to under a temporary name, then renames
# it, so the stream never reads half a file.
function Upload([string]$from, [string]$to, [string]$mode = "664") {
    scp -q $from "${Target}:san_radio/.upload.part"
    if ($LASTEXITCODE -ne 0) { throw "copying $from failed" }
    $quoted = "'" + $to.Replace("'", "'\''") + "'"
    Remote "cd ~/san_radio && chmod $mode .upload.part && mv .upload.part $quoted" | Out-Null
}

$arch = "$(Remote 'uname -m')".Trim()
$bin = switch -Regex ($arch) {
    '^aarch64$' { "bin/san_radio-linux-arm64" }
    '^armv[67]' { "bin/san_radio-linux-arm" }
    default { throw "no build for $arch" }
}
if (-not (Test-Path $bin)) { throw "$bin is missing: run build.ps1 first" }
Remote "mkdir -p ~/san_radio/music" | Out-Null

if ("$(Remote 'command -v ffmpeg || { test -x ~/san_radio/ffmpeg/ffmpeg && echo ok; }; true')".Trim() -eq "") {
    Write-Host "fetch ffmpeg on $Target (about 120 MB)..."
    $install = @'
set -e
mkdir -p ~/san_radio/ffmpeg/debs ~/san_radio/ffmpeg/root
cd ~/san_radio/ffmpeg/debs
apt-get download $(apt-get -s install --no-install-recommends ffmpeg 2>/dev/null | awk '/^Inst/ {print $2}')
for d in *.deb; do dpkg -x "$d" ../root; done
cd .. && rm -rf debs
'@
    RemoteScript $install
}
if ("$(Remote 'test -d ~/san_radio/ffmpeg/root && echo yes; true')".Trim() -eq "yes") {
    Upload ffmpeg.sh "ffmpeg/ffmpeg" "755"
}

Write-Host "copy $bin..."
Upload $bin "san_radio.new" "755"
Upload run.sh "run.sh.new" "755"
if ($Video -and (Test-Path $Video)) {
    Write-Host "copy $Video..."
    Upload $Video "loop.mp4.new"
}
if ($Music -and (Test-Path $Music)) {
    $have = @(Remote "ls -1 ~/san_radio/music")
    foreach ($f in Get-ChildItem -LiteralPath $Music -Filter *.wav -File) {
        if ($have -contains $f.Name) { continue }
        Write-Host "copy $($f.Name)..."
        Upload $f.FullName "music/$($f.Name)"
    }
}
if ($Config -and (Test-Path $Config)) {
    # It holds the stream key: readable by the Pi's user only.
    Upload $Config "channels.yaml" "600"
}

# Stop the old one (patterns anchored, so they can't match this command),
# put the new files in place, start run.sh, and have cron start it at boot.
Write-Host "restart on $Target..."
$restart = @'
cd ~/san_radio
pkill -f "^/bin/sh $HOME/san_radio/run.sh"
pkill -x san_radio
for i in $(seq 1 15); do pgrep -x san_radio >/dev/null || break; sleep 1; done
mv san_radio.new san_radio && mv run.sh.new run.sh
test -f loop.mp4.new && mv loop.mp4.new loop.mp4
(crontab -l 2>/dev/null | grep -v "san_radio/run.sh"; echo "@reboot $HOME/san_radio/run.sh") | crontab -
# Without lingering, systemd-logind empties /dev/shm of this user's files
# when their last login ends (an SSH session too), which stops the stream.
loginctl enable-linger 2>/dev/null || echo "warning: loginctl enable-linger failed: an SSH logout will interrupt the stream"
if ! test -f loop.mp4 || ! test -f channels.yaml; then echo "not started: the Pi needs loop.mp4 and channels.yaml"; exit 0; fi
setsid nohup "$HOME/san_radio/run.sh" >/dev/null 2>&1 </dev/null &
sleep 5
tail -5 san_radio.log
'@
RemoteScript $restart
Write-Host "done. The log: ssh $Target tail -f san_radio/san_radio.log"
