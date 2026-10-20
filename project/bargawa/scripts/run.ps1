<#
  Jobs for the Ramabargawa Blender project. Everything runs headless; logs go to _logs\.
  project\bargawa\scripts\run.ps1 <command> [name] [options]      (help: run.ps1 help)
#>
param(
  [Parameter(Position = 0)][string]$Command = "help",
  [Parameter(Position = 1)][string]$Name,
  [string]$Out,                 # place: write a test copy here instead
  [string]$Blend,               # views: use this .blend instead of the place's
  [int]$Pct = 35,               # views: still size in % of 1920x1080
  [string]$Only,                # views: "pendapa,huts"
  [switch]$Building,            # views: <name> is a building file (blender\buildings\<name>.blend)
  [switch]$Object,              # views: <name> is an object file (blender\objects\<name>.blend)
  [switch]$Force                # go on even if the file is open in a Blender window
)
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Blender = if ($env:BLENDER) { $env:BLENDER } else { "D:\blender\blender.exe" }
$Logs = Join-Path $Root "_logs"; New-Item -ItemType Directory -Force $Logs | Out-Null

function Show-Help {
  @"
Ramabargawa Blender jobs (headless). Logs: _logs\<name>.<job>.log

  run.ps1 fetch
        download the Poly Haven textures listed in scripts\common\polyhaven.py into blender\assets\polyhaven\ (kept if there)
  run.ps1 object <name> [-Out <file.blend>] [-Force]
        build blender\objects\<name>.blend from scripts\objects\<name>.py (stone). Buildings link it: rebuild them after.
  run.ps1 building <name> [-Out <file.blend>] [-Force]
        build blender\buildings\<name>.blend from scripts\buildings\<name>.py (pendopo). Places link it: rebuild the place after.
  run.ps1 place <name> [-Out <file.blend>] [-Force]
        build blender\places\<name>.blend from scripts\places\<name>.py (jatisrana). Refuses while the file is open in Blender.
  run.ps1 views <name> [-Building | -Object] [-Pct 35] [-Only "pendapa,huts"] [-Blend <file>]
        review stills (temporary cameras, nothing saved) + a contact sheet -> renders\checks\<name>\
        -Building: <name> is a building file, seen from round the origin; -Object: an object file, with its own views
"@ | Write-Host
}

function Assert-Closed([string]$file) {
  $full = [IO.Path]::GetFullPath($file).ToLower()
  $p = @(Get-Process blender -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowTitle -and $_.MainWindowTitle.ToLower().Contains($full) })
  if ($p.Count -gt 0 -and -not $Force) {
    Write-Host "STOP: $file is open in Blender (PID $($p[0].Id)). Close it first (or -Force, then File > Revert there without saving)." -ForegroundColor Red
    exit 2
  }
}
function Invoke-Blender([string[]]$argv, [string]$log, [hashtable]$envs = @{}) {
  foreach ($k in $envs.Keys) { Set-Item "env:$k" $envs[$k] }
  $quoted = $argv | ForEach-Object { if ($_ -match '\s') { '"' + $_ + '"' } else { $_ } }
  try {
    $p = Start-Process -FilePath $Blender -ArgumentList (@("--python-exit-code", "1") + $quoted) -NoNewWindow -Wait -PassThru `
         -RedirectStandardOutput $log -RedirectStandardError "$log.err"
  } finally { foreach ($k in $envs.Keys) { Remove-Item "env:$k" -ErrorAction SilentlyContinue } }
  Get-Content $log, "$log.err" | Where-Object { $_ -match '^(STEP|PLACE|BUILDING|OBJECT|SHOT|SHEET|CHECK|FETCH|DONE)|Traceback|Error' }
  if ($p.ExitCode -ne 0 -or (Select-String -Path $log, "$log.err" -Pattern 'Traceback' -Quiet)) {
    Write-Host "FAILED (exit $($p.ExitCode)) - see $log" -ForegroundColor Red; exit 1
  }
}
function Need-Name { if (-not $Name) { Write-Host "Which place? e.g. run.ps1 $Command jatisrana" -ForegroundColor Red; exit 2 } }

switch ($Command) {
  "fetch" {
    Invoke-Blender @("-b", "--factory-startup", "--python", (Join-Path $PSScriptRoot "tools\polyhaven.py")) (Join-Path $Logs "polyhaven.fetch.log")
  }
  "object" {
    Need-Name
    $script = Join-Path $PSScriptRoot "objects\$Name.py"
    if (-not (Test-Path $script)) { Write-Host "No scripts\objects\$Name.py" -ForegroundColor Red; exit 2 }
    if ($Out) { $target = [IO.Path]::GetFullPath($Out); $envs = @{ OBJECT_OUT = $target } }
    else { $target = Join-Path $Root "blender\objects\$Name.blend"; Assert-Closed $target; $envs = @{} }
    Invoke-Blender @("-b", "--factory-startup", "--python", $script) (Join-Path $Logs "$Name.object.log") $envs
  }
  "building" {
    Need-Name
    $script = Join-Path $PSScriptRoot "buildings\$Name.py"
    if (-not (Test-Path $script)) { Write-Host "No scripts\buildings\$Name.py" -ForegroundColor Red; exit 2 }
    if ($Out) { $target = [IO.Path]::GetFullPath($Out); $envs = @{ BUILDING_OUT = $target } }
    else { $target = Join-Path $Root "blender\buildings\$Name.blend"; Assert-Closed $target; $envs = @{} }
    Invoke-Blender @("-b", "--factory-startup", "--python", $script) (Join-Path $Logs "$Name.building.log") $envs
  }
  "place" {
    Need-Name
    $script = Join-Path $PSScriptRoot "places\$Name.py"
    if (-not (Test-Path $script)) { Write-Host "No scripts\places\$Name.py" -ForegroundColor Red; exit 2 }
    if ($Out) { $target = [IO.Path]::GetFullPath($Out); $envs = @{ PLACE_OUT = $target } }
    else { $target = Join-Path $Root "blender\places\$Name.blend"; Assert-Closed $target; $envs = @{} }
    Invoke-Blender @("-b", "--factory-startup", "--python", $script) (Join-Path $Logs "$Name.place.log") $envs
  }
  "views" {
    Need-Name
    $kind = if ($Building) { "buildings" } elseif ($Object) { "objects" } else { "places" }
    $file = if ($Blend) { [IO.Path]::GetFullPath($Blend) } else { Join-Path $Root "blender\$kind\$Name.blend" }
    $envs = @{ VIEWS_OUT = (Join-Path $Root "renders\checks\$Name"); VIEWS_PCT = "$Pct" }
    if ($Only) { $envs["VIEWS_ONLY"] = $Only }
    if ($Building) { $envs["VIEWS_SET"] = "building" }
    if ($Object) { $envs["VIEWS_SET"] = $Name }
    Invoke-Blender @("-b", $file, "--python", (Join-Path $PSScriptRoot "tools\views.py")) (Join-Path $Logs "$Name.views.log") $envs
  }
  default { Show-Help }
}
