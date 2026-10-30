# Puts the trading workflows (README.md) into a running tower: the Trading
# Journal knowledge, with its files copied from journal/ unless they are
# there already, and the six workflows from trading.workflows.json, switched
# off, in a folder named Trading. Nothing trades until "trade tick" is
# switched on.
#
#   pwsh golang/apps/san_tower/workflows/trading/setup.ps1 [-Tower http://127.0.0.1:7477]
param([string]$Tower = "http://127.0.0.1:7477")
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot

function Call($service, $method, $body) {
    $json = $body | ConvertTo-Json -Depth 20 -Compress
    Invoke-RestMethod -Method Post -Uri "$Tower/san.tower.v1.$service/$method" -ContentType "application/json" -Body ([Text.Encoding]::UTF8.GetBytes($json))
}

$names = "trade tick", "trade open", "trade close", "trade protect", "trade lesson", "trade approach"
$have = (Call WorkflowService ListWorkflows @{ allFolders = $true; limit = 1000 }).workflows | Where-Object { $names -contains $_.name }
if ($have) {
    throw "the tower has $(($have | ForEach-Object name) -join ', ') already: rename or delete them first, so the Workflow Tools find the right ones"
}

# The journal: a knowledge whose storage is <data>/trading-journal.
$list = Call KnowledgeService ListKnowledge @{}
$journal = Join-Path $list.dataDir "trading-journal"
if (Test-Path $journal) {
    Write-Host "keeping the journal's files in $journal"
} else {
    Copy-Item -Recurse (Join-Path $here "journal") $journal
    Write-Host "journal files copied to $journal"
}
if ($list.knowledge | Where-Object name -eq "Trading Journal") {
    Write-Host "the Trading Journal knowledge is there"
} else {
    $null = Call KnowledgeService CreateKnowledge @{ name = "Trading Journal"; path = "trading-journal" }
    Write-Host "made the Trading Journal knowledge"
}

$file = Get-Content -Raw -Encoding utf8 (Join-Path $here "trading.workflows.json")
$made = Call WorkflowService ImportWorkflows @{ json = $file }
foreach ($n in $made.notes) { Write-Warning $n }
$folder = (Call WorkflowService ListFolders @{}).folders | Where-Object name -eq "Trading" | Select-Object -First 1
if (-not $folder) { $folder = (Call WorkflowService CreateFolder @{ name = "Trading" }).folder }
$null = Call WorkflowService MoveWorkflows @{ ids = @($made.workflows | ForEach-Object id); folderId = $folder.id }
foreach ($w in $made.workflows) { Write-Host "imported $($w.name) (#$($w.id)), switched off" }
Write-Host "Switch on trade tick when autotrade's /loop has stopped: both would trade the same pairs on one account."
