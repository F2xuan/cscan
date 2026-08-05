# CSCAN local dev: one-click launcher for the full local stack (MongoDB + Redis + RPC + API + Web + Worker)
# All services run as containers built from local code (docker-compose.dev.yaml).
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location (Join-Path $ScriptDir "..")

if (-not (Get-Command docker-compose -ErrorAction SilentlyContinue)) {
    Write-Host "[dev] docker-compose not found. Please install Docker Desktop first." -ForegroundColor Red
    exit 1
}

Write-Host "[dev] Building and starting full local stack (docker-compose.dev.yaml)..."
docker-compose -f docker-compose.dev.yaml up -d --build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`n[dev] Local dev stack is up:"
docker-compose -f docker-compose.dev.yaml ps
Write-Host "`n[dev] Tips:"
Write-Host "  - Web UI:    http://localhost:7777 (admin / 123456)"
Write-Host "  - API:       http://localhost:8888"
Write-Host "  - View logs: docker-compose -f docker-compose.dev.yaml logs -f"
Write-Host "  - Stop all:  docker-compose -f docker-compose.dev.yaml down"
