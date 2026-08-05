#!/bin/bash
# CSCAN 本地开发：一键启动完整本地栈（MongoDB + Redis + RPC + API + Web + Worker）
# 全部服务经 docker-compose.dev.yaml 容器化运行（本地代码自动构建）。
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.." || { echo "[dev] 无法切换到项目根目录"; exit 1; }

command -v docker-compose >/dev/null 2>&1 || { echo "[dev] docker-compose 未安装，请先安装 Docker Desktop"; exit 1; }

echo "[dev] 构建并启动完整本地栈（docker-compose.dev.yaml）..."
docker-compose -f docker-compose.dev.yaml up -d --build

echo ""
echo "[dev] 本地开发栈已启动："
docker-compose -f docker-compose.dev.yaml ps
echo ""
echo "[dev] 提示："
echo "  - Web UI:   http://localhost:7777（默认账号 admin / 123456）"
echo "  - API:      http://localhost:8888"
echo "  - 查看日志: docker-compose -f docker-compose.dev.yaml logs -f"
echo "  - 停止全部: docker-compose -f docker-compose.dev.yaml down"
