# CSCAN local dev: one-shot launcher for the full local stack
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$ProjectRoot = $ScriptDir
Set-Location $ProjectRoot
$env:CSCAN_DEV = "1"

Write-Host "[dev] Starting dependency stack (MongoDB + Redis)..."
docker-compose -f docker-compose.dev.yaml up -d
if ($LASTEXITCODE -ne 0) {
    Write-Host "[dev] Failed to start dependency stack" -ForegroundColor Red
    exit 1
}

$rpc = Start-Process -FilePath "go" -ArgumentList "run","rpc/task/task.go","-f","rpc/task/etc/task.yaml" -NoNewWindow -PassThru -WorkingDirectory $ProjectRoot
$api = Start-Process -FilePath "go" -ArgumentList "run","api/cscan.go","-f","api/etc/cscan.yaml" -NoNewWindow -PassThru -WorkingDirectory $ProjectRoot
$worker = Start-Process -FilePath "go" -ArgumentList "run","worker/main.go","-s","http://localhost:8888" -NoNewWindow -PassThru -WorkingDirectory $ProjectRoot
$web = Start-Process -FilePath "cmd" -ArgumentList "/c","cd web && (if not exist node_modules call npm install) && npm run dev" -NoNewWindow -PassThru -WorkingDirectory $ProjectRoot

Write-Host "[dev] Local dev stack started (Deps / RPC / API / Worker / Web). Press Ctrl+C to stop all."

try {
    Wait-Process -Id $rpc.Id, $api.Id, $worker.Id, $web.Id
} finally {
    Write-Host "[dev] Stopping all local services..."
    @($rpc, $api, $worker, $web) | ForEach-Object {
        if (-not $_.HasExited) { taskkill /T /F /PID $_.Id 2>$null }
    }
    Write-Host "[dev] Stopping dependency stack and cleaning volumes..."
    docker-compose -f docker-compose.dev.yaml down -v 2>$null
    Write-Host "[dev] All stopped"
}
