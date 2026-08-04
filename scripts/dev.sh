#!/bin/bash
# CSCAN 本地开发：一键启动完整本地栈（RPC + API + Web）
# 三个服务均在后台运行，Ctrl+C 可一次性停止全部。
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.." || { echo "[dev] 无法切换到项目根目录"; exit 1; }

export CSCAN_DEV=1

cleanup() {
  echo ""
  echo "[dev] 正在停止所有本地服务 (RPC / API / Web)..."
  # 向当前进程组发送终止信号，覆盖全部后台子进程（go run / npm / vite）
  kill 0 2>/dev/null
  wait 2>/dev/null
  echo "[dev] 已全部停止"
  exit 0
}
trap cleanup INT TERM EXIT

echo "[dev] 启动 RPC 服务..."
go run rpc/task/task.go -f rpc/task/etc/task.yaml &
echo "[dev] 启动 API 服务..."
go run api/cscan.go -f api/etc/cscan.yaml &
echo "[dev] 启动 Web 前端..."
( cd web && { [ -d node_modules ] || npm install; } && npm run dev ) &

echo "[dev] 本地开发栈已启动 (RPC / API / Web)。按 Ctrl+C 停止全部。"
wait
