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
$Godot = if ($env:GODOT) { $env:GODOT } else { Join-Path $Root "tools\godot\Godot_v4.7.2-stable_win64_console.exe" }
$Game = Join-Path $Root "game"
$Logs = Join-Path $Root "_logs"; New-Item -ItemType Directory -Force $Logs | Out-Null

function Show-Help {
  @"
Ramabargawa Blender jobs (headless). Logs: _logs\<name>.<job>.log

  run.ps1 fetch
        download the Poly Haven textures listed in scripts\common\polyhaven.py into blender\assets\polyhaven\ (kept if there)
  run.ps1 texture <family>|all [-Out <file.blend>] [-Force]
        build blender\textures\<family>.blend (wood, stone, plants, ground, metal, palette) from common\textures.py: the texture
        library every other file links its materials from. Build it first; rebuild objects, buildings and places after a change.
  run.ps1 register
        register blender\textures\ as the asset library "Bargawa Textures" in Blender's preferences (once; Blender closed)
  run.ps1 object <name> [-Out <file.blend>] [-Force]
        build blender\objects\<name>.blend from scripts\objects\<name>.py (stone, soko, damar, fire). Buildings link it: rebuild them after.
  run.ps1 building <name> [-Out <file.blend>] [-Force]
        build blender\buildings\<name>.blend from scripts\buildings\<name>.py (pendopo). Places link it: rebuild the place after.
  run.ps1 place <name> [-Out <file.blend>] [-Force]
        build blender\places\<name>.blend from scripts\places\<name>.py (jatisrana). Refuses while the file is open in Blender.
  run.ps1 views <name> [-Building | -Object] [-Pct 35] [-Only "pendapa,huts"] [-Blend <file>]
        review stills (temporary cameras, nothing saved) + a contact sheet -> renders\checks\<name>\
        -Building: <name> is a building file, seen from round the origin; -Object: an object file, with its own views
  run.ps1 proportions <name>
        proportion and size check of a place (scripts\tools\proportions.py; reads it, saves nothing): headroom along the ways
        people walk, stair rises and treads, passage widths, every object's size -> renders\checks\<name>\proportions.json
  run.ps1 game <name>
        the game's copy of a place (scripts\game\<name>.py, reads blender\places\<name>.blend, saves nothing in it):
        game\assets\export\<name>\<name>.glb + .json for the Godot project in game\ (common\game.py); then Godot imports it

The game (Godot 4.7.2 in tools\godot, README "The game"; the Makefile wraps these: make play, make check ...):
  run.ps1 play                  play it (imports first if Godot's cache is missing)
  run.ps1 import                headless import of the game's assets
  run.ps1 check                 import, then a short headless run; prints errors only
  run.ps1 measure               walk the set route: _logs\game_measure.json, renders\checks\game\*.png
  run.ps1 editor                open the Godot editor
  run.ps1 clean-cache           delete game\.godot (the next import rebuilds it)
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
  Get-Content $log, "$log.err" | Where-Object { $_ -match '^(STEP|PLACE|BUILDING|OBJECT|TEXTURE|SHOT|SHEET|CHECK|FETCH|DONE)|Traceback|Error' }
  if ($p.ExitCode -ne 0 -or (Select-String -Path $log, "$log.err" -Pattern 'Traceback' -Quiet)) {
    Write-Host "FAILED (exit $($p.ExitCode)) - see $log" -ForegroundColor Red; exit 1
  }
}
function Need-Name { if (-not $Name) { Write-Host "Which place? e.g. run.ps1 $Command jatisrana" -ForegroundColor Red; exit 2 } }
function Need-Godot {
  if (-not (Test-Path $Godot)) { Write-Host "Godot 4.7.2 is missing: download it into tools\godot\ (README.md, 'The game')." -ForegroundColor Red; exit 2 }
}
function Import-Game {
  # Godot's headless import; only its errors and warnings are shown.
  & $Godot --path $Game --headless --import 2>&1 | ForEach-Object { "$_" } | Where-Object { $_ -match 'ERROR|WARNING' }
}

switch ($Command) {
  "fetch" {
    Invoke-Blender @("-b", "--factory-startup", "--python", (Join-Path $PSScriptRoot "tools\polyhaven.py")) (Join-Path $Logs "polyhaven.fetch.log")
  }
  "texture" {
    Need-Name
    $families = @("wood", "stone", "plants", "ground", "metal", "palette")
    $list = if ($Name -eq "all") { $families } elseif ($families -contains $Name) { @($Name) } else {
      Write-Host "No texture family $Name (one of: $($families -join ', '), all)" -ForegroundColor Red; exit 2 }
    foreach ($f in $list) {
      if ($Out) { $target = [IO.Path]::GetFullPath($Out); $envs = @{ TEXTURE_FAMILY = $f; TEXTURE_OUT = $target } }
      else { $target = Join-Path $Root "blender\textures\$f.blend"; Assert-Closed $target; $envs = @{ TEXTURE_FAMILY = $f } }
      Invoke-Blender @("-b", "--factory-startup", "--python", (Join-Path $PSScriptRoot "textures\build.py")) (Join-Path $Logs "$f.texture.log") $envs
    }
  }
  "register" {
    $open = @(Get-Process blender -ErrorAction SilentlyContinue)
    if ($open.Count -gt 0 -and -not $Force) {
      Write-Host "STOP: Blender is open (PID $($open[0].Id)). Close it first: on exit it saves its own preferences and would drop the library." -ForegroundColor Red
      exit 2
    }
    Invoke-Blender @("-b", "--python", (Join-Path $PSScriptRoot "tools\register_library.py")) (Join-Path $Logs "register.log")
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
  "proportions" {
    Need-Name
    $file = Join-Path $Root "blender\places\$Name.blend"
    Invoke-Blender @("-b", $file, "--python", (Join-Path $PSScriptRoot "tools\proportions.py")) (Join-Path $Logs "$Name.proportions.log")
    Get-Content (Join-Path $Logs "$Name.proportions.log") | Where-Object { $_ -match '^PROP' }
  }
  "game" {
    Need-Name
    $script = Join-Path $PSScriptRoot "game\$Name.py"
    if (-not (Test-Path $script)) { Write-Host "No scripts\game\$Name.py" -ForegroundColor Red; exit 2 }
    $file = Join-Path $Root "blender\places\$Name.blend"
    if (-not (Test-Path $file)) { Write-Host "No blender\places\$Name.blend: run.ps1 place $Name first" -ForegroundColor Red; exit 2 }
    Invoke-Blender @("-b", $file, "--python", $script) (Join-Path $Logs "$Name.game.log")
    if (Test-Path $Godot) { Import-Game; Write-Host "IMPORTED into game\" } else { Write-Host "Godot missing: import skipped" -ForegroundColor Yellow }
  }
  "play" {
    Need-Godot
    if (-not (Test-Path (Join-Path $Game ".godot\imported"))) { Import-Game }
    & $Godot --path $Game
  }
  "import" { Need-Godot; Import-Game; Write-Host "IMPORTED" }
  "check" {
    Need-Godot; Import-Game
    $e = & $Godot --path $Game --headless --quit-after 120 2>&1 | ForEach-Object { "$_" } | Where-Object { $_ -match 'ERROR|WARNING' }
    if ($e) { $e; exit 1 } else { Write-Host "OK: no errors" }
  }
  "measure" {
    Need-Godot
    & $Godot --path $Game -- --measure 2>&1 | ForEach-Object { "$_" } | Where-Object { $_ -match 'ERROR|SCRIPT' }
    Get-Content (Join-Path $Logs "game_measure.json")
  }
  "editor" { Need-Godot; Start-Process -FilePath ($Godot -replace '_console\.exe$', '.exe') -ArgumentList @("--path", $Game, "--editor") }
  "clean-cache" { Remove-Item -Recurse -Force (Join-Path $Game ".godot") -ErrorAction SilentlyContinue; Write-Host "removed game\.godot" }
  default { Show-Help }
}
