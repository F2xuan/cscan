@echo off
REM CSCAN local dev: one-shot launcher for the full local stack (RPC + API + Web)
REM Actual logic lives in scripts/dev.ps1
setlocal
set "SCRIPT_DIR=%~dp0"
if not exist "%SCRIPT_DIR%dev.ps1" (
  echo [dev] dev.ps1 not found
  exit /b 1
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%dev.ps1" %*
endlocal
