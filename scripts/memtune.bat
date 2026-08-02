@echo off
REM CSCAN 内存自适应调节工具 - Windows 调度入口
REM 实际逻辑见 scripts/memtune.ps1
setlocal
set "SCRIPT_DIR=%~dp0"
if not exist "%SCRIPT_DIR%memtune.ps1" (
  echo [memtune] 未找到 memtune.ps1
  exit /b 1
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%memtune.ps1" %*
endlocal
