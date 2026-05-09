param(
    [string]$Server = 'http://127.0.0.1:8081',
    [string]$Task = 'Please answer in Chinese: summarize 3 trends of China EV industry in 2025.',
    [int]$MaxSteps = 3,
    [int]$PollSeconds = 30
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$base = $Server.TrimEnd('/')

$health = Invoke-RestMethod -UseBasicParsing -Uri "$base/healthz" -TimeoutSec 8
if (-not $health.dev_token) {
    throw 'healthz did not return dev_token'
}

$headers = @{ Authorization = "Bearer $($health.dev_token)" }
$runBody = @{
    task = $Task
    task_type = 'research'
    max_steps = $MaxSteps
    verify_mode = 'off'
} | ConvertTo-Json -Depth 6 -Compress

$run = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$base/api/v1/run" -Headers $headers -ContentType 'application/json' -Body $runBody -TimeoutSec 20
if (-not $run.session_id) {
    throw 'run API did not return session_id'
}

$sessionId = [string]$run.session_id
$inspect = $null
for ($i = 0; $i -lt $PollSeconds; $i++) {
    Start-Sleep -Seconds 1
    $inspect = Invoke-RestMethod -UseBasicParsing -Uri "$base/api/v1/sessions/$sessionId/inspect" -Headers $headers -TimeoutSec 20
    if ($inspect.status -ne 'pending' -and $inspect.status -ne 'running') {
        break
    }
}

[PSCustomObject]@{
    server = $base
    session_id = $sessionId
    status = $inspect.status
    termination_reason = $inspect.termination_reason
    steps = @($inspect.steps).Count
    task = $inspect.task
} | ConvertTo-Json -Depth 8
