Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$noteRoot = Join-Path $repoRoot '.sisyphus/notepads/general-agent-harness-go'

$files = @(
    'decisions.md',
    'issues.md',
    'problems.md'
)

$requiredSections = @('## Template', '## Historical', '## Current', '## Verification')
$entryPattern = '^### Entry (?<id>(H|C|V)-\d{3})\s*$'
$changeRefPattern = '^\s*- \*\*Change Ref\*\*:\s*(?<ref>CHG-\d{8}-\d{3})\s*$'
$chgHeaderPattern = '^###\s+(?<ref>CHG-\d{8}-\d{3})\s*$'

$changeLogPath = Join-Path $noteRoot 'change-log.md'
if (-not (Test-Path $changeLogPath)) {
    throw "Missing change-log file: $changeLogPath"
}

$changeLogLines = Get-Content -LiteralPath $changeLogPath
$knownRefs = New-Object System.Collections.Generic.HashSet[string]
foreach ($line in $changeLogLines) {
    if ($line -match $chgHeaderPattern) {
        [void]$knownRefs.Add($Matches['ref'])
    }
}

if ($knownRefs.Count -eq 0) {
    throw "No CHG entries found in change-log.md"
}

$errors = New-Object System.Collections.Generic.List[string]

foreach ($file in $files) {
    $path = Join-Path $noteRoot $file
    if (-not (Test-Path $path)) {
        $errors.Add("Missing notepad file: $path")
        continue
    }

    $lines = Get-Content -LiteralPath $path
    $text = [string]::Join([Environment]::NewLine, $lines)

    foreach ($section in $requiredSections) {
        if ($text -notmatch [regex]::Escape($section)) {
            $errors.Add("$file missing section: $section")
        }
    }

    $entryIds = New-Object System.Collections.Generic.List[string]
    $pendingEntry = $null
    $pendingRefFound = $false

    foreach ($line in $lines) {
        if ($line -match $entryPattern) {
            if ($null -ne $pendingEntry -and -not $pendingRefFound) {
                $errors.Add("$file entry $pendingEntry missing Change Ref")
            }
            $pendingEntry = $Matches['id']
            $pendingRefFound = $false
            $entryIds.Add($pendingEntry)
            continue
        }

        if ($null -ne $pendingEntry -and $line -match $changeRefPattern) {
            $ref = $Matches['ref']
            if (-not $knownRefs.Contains($ref)) {
                $errors.Add("$file entry $pendingEntry references unknown Change Ref $ref")
            }
            $pendingRefFound = $true
        }
    }

    if ($null -ne $pendingEntry -and -not $pendingRefFound) {
        $errors.Add("$file entry $pendingEntry missing Change Ref")
    }

    $duplicateIds = $entryIds | Group-Object | Where-Object { $_.Count -gt 1 }
    foreach ($dup in $duplicateIds) {
        $errors.Add("$file has duplicate Entry ID: $($dup.Name)")
    }
}

if ($errors.Count -gt 0) {
    Write-Host 'Notepad governance check failed:' -ForegroundColor Red
    foreach ($err in $errors) {
        Write-Host " - $err" -ForegroundColor Red
    }
    exit 1
}

Write-Host 'Notepad governance check passed.' -ForegroundColor Green
exit 0
