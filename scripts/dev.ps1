# CSCAN local dev: one-shot launcher for the full local stack (RPC + API + Web)
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location (Join-Path $ScriptDir "..")
$env:CSCAN_DEV = "1"

$rpc = Start-Process -FilePath "go" -ArgumentList "run","rpc/task/task.go","-f","rpc/task/etc/task.yaml" -NoNewWindow -PassThru
$api = Start-Process -FilePath "go" -ArgumentList "run","api/cscan.go","-f","api/etc/cscan.yaml" -NoNewWindow -PassThru
$web = Start-Process -FilePath "cmd" -ArgumentList "/c","cd web && (if not exist node_modules call npm install) && npm run dev" -NoNewWindow -PassThru

Write-Host "[dev] Local dev stack started (RPC / API / Web). Press Ctrl+C to stop all."

try {
    Wait-Process -Id $rpc.Id, $api.Id, $web.Id
} finally {
    Write-Host "[dev] Stopping all local services..."
    @($rpc, $api, $web) | ForEach-Object {
        if (-not $_.HasExited) { taskkill /T /F /PID $_.Id 2>$null }
    }
    Write-Host "[dev] All stopped"
}
