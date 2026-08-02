package worker

import (
	"net/http"

	"cscan/api/internal/svc"
)

// DockerComposeWorkerHandler 提供 docker-compose-worker.yaml 静态文件
func DockerComposeWorkerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content := `# CSCAN Worker 探针部署
#
# 使用方法（独立 Worker 节点，推荐）:
#   curl -O http://YOUR_SERVER:8888/static/docker-compose-worker.yaml
#   CSCAN_SERVER=http://YOUR_SERVER:8888 CSCAN_KEY=YOUR_KEY \
#     docker-compose -f docker-compose-worker.yaml up -d
#
# 想让配置自动匹配【本机规格】（专用 Worker 节点强烈推荐），改用 worker-tune.sh：
#   curl -O http://YOUR_SERVER:8888/static/worker-tune.sh
#   CSCAN_SERVER=http://YOUR_SERVER:8888 CSCAN_KEY=YOUR_KEY bash worker-tune.sh
#   （同机扩容加 --colocated，仅给小配额避免与主栈争抢）
#
# 与主控同机部署（额外扩容 Worker，需避免容器名/端口冲突）:
#   CSCAN_WORKER_NAME=cscan-worker-extra CSCAN_WORKER_MEM_LIMIT=2G \
#   CSCAN_WORKER_CPU_LIMIT=2 CSCAN_SERVER=... CSCAN_KEY=... \
#     docker-compose -f docker-compose-worker.yaml up -d
#   注意: 本探针默认 network_mode: host，同机部署会占用宿主机端口，
#        请仅在确需本地扩容时如此使用，并调低下方资源上限避免与主栈争抢。
#
# 环境变量:
#   CSCAN_SERVER: API 服务器地址 (必填)
#   CSCAN_KEY: 安装密钥 (必填，从管理后台获取)
#   CSCAN_NAME: Worker 名称 (可选，默认自动生成)
#   CSCAN_WORKER_NAME: 容器名 (可选，同机多 Worker 时用于避免冲突，默认 cscan-worker)
#   CSCAN_CONCURRENCY: 并发数 (可选，留空则 Worker 按自身 cgroup 内存自动推导)
#   CSCAN_WORKER_MEM_LIMIT: 内存硬上限 (可选，默认 2G)
#   CSCAN_WORKER_MEM_RESERVATION: 内存预留 (可选，默认 512M)
#   CSCAN_WORKER_CPU_LIMIT: CPU 上限/天花板，突发可借核 (可选，默认 2)
#   CSCAN_WORKER_CPU_RESERVATION: CPU 保底/地板 (可选，默认 1)

services:
  cscan-worker:
    image: registry.cn-hangzhou.aliyuncs.com/txf7/cscan-worker:latest
    container_name: ${CSCAN_WORKER_NAME:-cscan-worker}
    restart: unless-stopped
    network_mode: host
    deploy:
      resources:
        limits:
          cpus: '${CSCAN_WORKER_CPU_LIMIT:-2}'
          memory: ${CSCAN_WORKER_MEM_LIMIT:-2G}
        reservations:
          cpus: '${CSCAN_WORKER_CPU_RESERVATION:-1}'
          memory: ${CSCAN_WORKER_MEM_RESERVATION:-512M}
    environment:
      - CSCAN_SERVER=${CSCAN_SERVER}
      - CSCAN_KEY=${CSCAN_KEY}
      - CSCAN_NAME=${CSCAN_NAME:-}
      - CSCAN_CONCURRENCY=${CSCAN_CONCURRENCY:-}
`
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=docker-compose-worker.yaml")
		w.Write([]byte(content))
	}
}

// WorkerTuneHandler 提供 worker-tune.sh：在【目标 Worker 机】本地检测其内存/CPU，
// 生成 host 尺寸的 docker-compose.worker.override.yml 并启动，使配置真正匹配该独立服务器。
// （server 无法感知 client 规格，故 sizing 必须发生在 client 侧，思路与主栈 memtune.sh 一致）
func WorkerTuneHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content := `#!/bin/bash
# worker-tune.sh — CSCAN 独立 Worker 探针的本地资源自适应
#
# 解决的问题: /static/docker-compose-worker.yaml 是「通用模板」（固定 2G/2 核），
#   无法感知你部署 Worker 的独立服务器规格。本脚本在【目标 Worker 机】上检测其内存/CPU，
#   生成 host 尺寸的 override 并启动，使配置真正匹配该独立服务器（思路同主栈 memtune.sh）。
#
# 用法:
#   CSCAN_SERVER=http://YOUR_SERVER:8888 CSCAN_KEY=YOUR_KEY \
#     bash worker-tune.sh              # 独立/专用 Worker 节点（默认，按宿主机规格分配）
#   CSCAN_SERVER=... CSCAN_KEY=... bash worker-tune.sh --colocated
#                                    # 与主栈同机扩容，仅给小配额避免争抢
#   bash worker-tune.sh --help
#
# 说明:
#   - 若本地无 docker-compose-worker.yaml 会自动从 CSCAN_SERVER 下载。
#   - 并发数留空，由 Worker 按自身 cgroup 内存自动推导（与主栈一致）。
#   - 同机扩容可用 CSCAN_WORKER_NAME=cscan-worker-extra 改名避免容器/端口冲突。

set -euo pipefail

MODE="standalone"
for a in "$@"; do
  case "$a" in
    --colocated) MODE="colocated" ;;
    --standalone) MODE="standalone" ;;
    -h|--help) echo "用法: CSCAN_SERVER=... CSCAN_KEY=... bash worker-tune.sh [--standalone|--colocated]"; exit 0 ;;
    *) echo "未知参数: $a" >&2; exit 1 ;;
  esac
done

: "${CSCAN_SERVER:?请设置 CSCAN_SERVER（API 服务器地址）}"
: "${CSCAN_KEY:?请设置 CSCAN_KEY（安装密钥）}"
# 透传给 compose 插值（container_name 等用到）
export CSCAN_SERVER CSCAN_KEY CSCAN_NAME CSCAN_CONCURRENCY CSCAN_WORKER_NAME

BASE="docker-compose-worker.yaml"
OVERRIDE="docker-compose.worker.override.yml"

if [ ! -f "$BASE" ]; then
  echo "[worker-tune] 未找到 $BASE，正在从 $CSCAN_SERVER 下载..."
  curl -fsSL "$CSCAN_SERVER/static/docker-compose-worker.yaml" -o "$BASE" \
    || { echo "[worker-tune] 下载失败，请先手动: curl -O $CSCAN_SERVER/static/docker-compose-worker.yaml" >&2; exit 1; }
fi

if [ -r /proc/meminfo ]; then
  HOST_MB=$(awk '/^MemTotal:/{print int($2/1024)}' /proc/meminfo)
else
  HOST_MB=4096
fi
if command -v nproc >/dev/null 2>&1; then HOST_CPU=$(nproc); else HOST_CPU=2; fi

if [ "$MODE" = "colocated" ]; then
  MEM_LIMIT_MB=2048
  MEM_RES_MB=512
  CPU_LIMIT=2
  CPU_RES=1
else
  MEM_LIMIT_MB=$(( HOST_MB * 85 / 100 ))
  MEM_RES_MB=$(( MEM_LIMIT_MB / 4 ))
  CPU_LIMIT=$HOST_CPU
  CPU_RES=$(awk -v c="$HOST_CPU" 'BEGIN{ v=c*0.5; if(v<0.5)v=0.5; printf "%.2f", v }')
fi

echo "[worker-tune] 模式=$MODE  宿主机=${HOST_MB}MB/${HOST_CPU}CPU"
echo "[worker-tune] Worker 配额: mem=${MEM_LIMIT_MB}M res=${MEM_RES_MB}M cpu=${CPU_LIMIT}/${CPU_RES}"

cat > "$OVERRIDE" <<EOF
# 由 worker-tune.sh 自动生成（模式: $MODE），请勿手动编辑。
# 依据宿主机 ${HOST_MB}MB / ${HOST_CPU}CPU 生成，匹配该独立 Worker 服务器规格。
services:
  cscan-worker:
    container_name: ${CSCAN_WORKER_NAME:-cscan-worker}
    deploy:
      resources:
        limits:
          cpus: '${CPU_LIMIT}'
          memory: ${MEM_LIMIT_MB}M
        reservations:
          cpus: '${CPU_RES}'
          memory: ${MEM_RES_MB}M
    environment:
      - CSCAN_SERVER=${CSCAN_SERVER}
      - CSCAN_KEY=${CSCAN_KEY}
      - CSCAN_NAME=${CSCAN_NAME:-}
      - CSCAN_CONCURRENCY=${CSCAN_CONCURRENCY:-}
EOF

# 探测 docker compose 命令（兼容 docker compose 与旧版 docker-compose）
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  echo "[worker-tune] 错误：未检测到 docker compose，请先安装 Docker。" >&2
  exit 1
fi

echo "[worker-tune] 正在启动 Worker（应用 override）..."
$COMPOSE -f "$BASE" -f "$OVERRIDE" up -d
echo "[worker-tune] 完成。查看: $COMPOSE -f $BASE -f $OVERRIDE ps"
`
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=worker-tune.sh")
		w.Write([]byte(content))
	}
}

// WorkerTunePsHandler 提供 worker-tune.ps1（Windows / PowerShell 版），逻辑与 worker-tune.sh 一致：
// 在目标 Worker 机本地检测规格，生成 host 尺寸的 docker-compose.worker.override.yml 并启动。
func WorkerTunePsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content := `# PowerShell - worker-tune.ps1 — CSCAN 独立 Worker 探针的本地资源自适应 (Windows)
# 思路同 worker-tune.sh（server 无法感知 client 规格，sizing 发生在目标机本地）。
#
# 用法:
#   $env:CSCAN_SERVER="http://YOUR_SERVER:8888"; $env:CSCAN_KEY="YOUR_KEY"
#   powershell -NoProfile -ExecutionPolicy Bypass -File worker-tune.ps1
#   # 同机扩容: 追加 --colocated
param()
$ErrorActionPreference = 'Stop'

function Get-ComposeCmd {
  $docker = Get-Command docker -ErrorAction SilentlyContinue
  if ($docker) {
    try { docker compose version 2>$null; if ($LASTEXITCODE -eq 0) { return 'docker compose' } } catch {}
  }
  $dockerCompose = Get-Command docker-compose -ErrorAction SilentlyContinue
  if ($dockerCompose) { return 'docker-compose' }
  Write-Error '未检测到 docker compose，请先安装 Docker Desktop 或 Docker Engine。'; exit 1
}

$Mode = 'standalone'
foreach ($a in $args) {
  if ($a -eq '--colocated') { $Mode = 'colocated' }
  elseif ($a -eq '--standalone') { $Mode = 'standalone' }
  elseif ($a -eq '-h' -or $a -eq '--help') { Write-Host '用法: $env:CSCAN_SERVER=... $env:CSCAN_KEY=... powershell -File worker-tune.ps1 [--standalone|--colocated]'; exit 0 }
  else { Write-Error "未知参数: $a"; exit 1 }
}

if (-not $env:CSCAN_SERVER) { Write-Error '请设置 CSCAN_SERVER（API 服务器地址）'; exit 1 }
if (-not $env:CSCAN_KEY) { Write-Error '请设置 CSCAN_KEY（安装密钥）'; exit 1 }

$Base = 'docker-compose-worker.yaml'
$Override = 'docker-compose.worker.override.yml'

if (-not (Test-Path $Base)) {
  Write-Host "[worker-tune] 未找到 $Base，正在从 $env:CSCAN_SERVER 下载..."
  try { Invoke-WebRequest -Uri "$env:CSCAN_SERVER/static/docker-compose-worker.yaml" -OutFile $Base } catch { Write-Error "下载失败，请先手动下载: $env:CSCAN_SERVER/static/docker-compose-worker.yaml"; exit 1 }
}

try { $HostMb = [int]((Get-CimInstance Win32_PhysicalMemory | Measure-Object Capacity -Sum).Sum / 1MB) } catch { $HostMb = 4096 }
try { $HostCpu = [int](Get-CimInstance Win32_ComputerSystem).NumberOfLogicalProcessors } catch { $HostCpu = 2 }

if ($Mode -eq 'colocated') {
  $MemLimitMb = 2048; $MemResMb = 512; $CpuLimit = 2; $CpuRes = 1
} else {
  $MemLimitMb = [int]($HostMb * 0.85)
  $MemResMb = [int]($MemLimitMb / 4)
  $CpuLimit = $HostCpu
  $CpuRes = [math]::Max(0.5, [double]($HostCpu * 0.5))
}

Write-Host "[worker-tune] 模式=$Mode  宿主机=${HostMb}MB/${HostCpu}CPU"
Write-Host "[worker-tune] Worker 配额: mem=${MemLimitMb}M res=${MemResMb}M cpu=$CpuLimit/$CpuRes"

$workerName = if ($env:CSCAN_WORKER_NAME) { $env:CSCAN_WORKER_NAME } else { 'cscan-worker' }
$server = $env:CSCAN_SERVER; $key = $env:CSCAN_KEY
$name = if ($env:CSCAN_NAME) { $env:CSCAN_NAME } else { '' }
$conc = if ($env:CSCAN_CONCURRENCY) { $env:CSCAN_CONCURRENCY } else { '' }

$overrideContent = @"
# 由 worker-tune.ps1 自动生成（模式: $Mode），请勿手动编辑。
# 依据宿主机 ${HostMb}MB / ${HostCpu}CPU 生成，匹配该独立 Worker 服务器规格。
services:
  cscan-worker:
    container_name: $workerName
    deploy:
      resources:
        limits:
          cpus: '$CpuLimit'
          memory: ${MemLimitMb}M
        reservations:
          cpus: '$CpuRes'
          memory: ${MemResMb}M
    environment:
      - CSCAN_SERVER=$server
      - CSCAN_KEY=$key
      - CSCAN_NAME=$name
      - CSCAN_CONCURRENCY=$conc
"@
Set-Content -Path $Override -Value $overrideContent -Encoding utf8

$cmd = Get-ComposeCmd
Write-Host '[worker-tune] 正在启动 Worker（应用 override）...'
Invoke-Expression "$cmd -f $Base -f $Override up -d"
Write-Host "[worker-tune] 完成。查看: $cmd -f $Base -f $Override ps"
`
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=worker-tune.ps1")
		w.Write([]byte(content))
	}
}
