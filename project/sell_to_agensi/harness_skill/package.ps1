# Builds the Agensi upload: project-brain.zip, one top-level folder project-brain/ holding the skill
# (project-brain/ next to this script) and the current san_knowledge source (text only, no binaries).
#   pwsh project/sell_to_agensi/harness_skill/package.ps1
# Staging is in .build/ (hidden, so the knowledge graph skips the copies). Needs git and Go.
$ErrorActionPreference = 'Stop'
$skill = 'project-brain'
$here = $PSScriptRoot
$repo = (git -C $here rev-parse --show-toplevel).Trim()
$stage = Join-Path $here ".build/$skill"
$zip = Join-Path $here "$skill.zip"

if (Test-Path (Join-Path $here '.build')) { Remove-Item (Join-Path $here '.build') -Recurse -Force }
New-Item -ItemType Directory -Force $stage | Out-Null
Copy-Item (Join-Path $here "$skill/*") $stage -Recurse

# Tracked and new (not ignored) files: leaves out bin/, ui/node_modules and the built UI in dist/ (except .gitkeep).
# The repo's build scripts build Windows+Linux for this repo; SKILL.md has the buyer's build commands instead.
$files = git -C $repo ls-files -co --exclude-standard -- san_knowledge |
    Where-Object { $_ -notmatch '^san_knowledge/build\.(ps1|sh)$' -and (Test-Path (Join-Path $repo $_)) }
foreach ($f in $files) {
    $to = Join-Path $stage $f
    New-Item -ItemType Directory -Force (Split-Path $to) | Out-Null
    Copy-Item (Join-Path $repo $f) $to
}

# Agensi's scan rejects binaries, symlinks and OS junk: refuse to package them.
$bad = Get-ChildItem $stage -Recurse -File | Where-Object {
    $_.Name -in '.DS_Store', 'Thumbs.db' -or $_.LinkType -or
    ([IO.File]::ReadAllBytes($_.FullName) -contains 0)
}
if ($bad) { throw "not text: $($bad.FullName -join ', ')" }

# The packaged source must compile as shipped (the UI is built by the buyer; dist/.gitkeep keeps go:embed happy).
Push-Location (Join-Path $stage 'san_knowledge')
try {
    go vet ./...
    if ($LASTEXITCODE) { throw 'go vet failed on the packaged source' }
} finally { Pop-Location }

if (Test-Path $zip) { Remove-Item $zip }
Add-Type -AssemblyName System.IO.Compression.FileSystem
[IO.Compression.ZipFile]::CreateFromDirectory($stage, $zip, 'Optimal', $true)
$count = (Get-ChildItem $stage -Recurse -File).Count
Write-Host ("packaged {0} files into {1} ({2:N0} KB)" -f $count, $zip, ((Get-Item $zip).Length / 1KB))
