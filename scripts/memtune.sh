#!/bin/bash
#
# memtune.sh — CSCAN 内存自适应调节工具
#
# 解决 compose 中内存上限硬编码、高配机资源利用不足、低配机易 OOM 的问题：
#   1. 配置化   : 内存/CPU 上限改由 docker-compose.yaml 的 ${VAR:-默认} 注入；
#                本工具生成 docker-compose.override.yml 落地具体配额。
#   2. 自动计算 : 按宿主机可用物理内存自动分级（tiny/small/medium/large/xlarge）
#                或精确计算各服务配额，高配机多分、低配机保底，避免 OOM。
#   3. 弹性调整 : 支持按实例规格（命名画像 / 指定内存）分配；Worker 并发随内存自适应。
#   4. 运行时热更新 : `update` 子命令通过 `docker update` 调整运行中容器上限，无需重启。
#   5. 监控与保护 : `monitor` 轮询内存占用，超限告警并可在 Worker 侧自动降并发保护。
#
# 用法:
#   ./memtune.sh detect                       # 检测宿主机并给出推荐画像
#   ./memtune.sh plan [auto|tiny|small|medium|large|xlarge|<RAM>]   # 仅计算并预览，不落盘
#   ./memtune.sh apply [auto|tiny|small|medium|large|xlarge|<RAM>]   # 生成 override 并重建服务
#   ./memtune.sh update                       # 运行时热更新内存/CPU 上限（不重启）
#   ./memtune.sh monitor [--once] [--interval=N] [--threshold=N] [--auto-protect]
#   ./memtune.sh status                       # 查看当前配额与实际占用
#   ./memtune.sh reset                        # 移除 override，回退到 compose 默认上限
#
# 说明: 生成的 docker-compose.override.yml 会被 `docker compose up -d` 自动合并。

set -euo pipefail

# ---------- 路径与文件 ----------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 向上查找包含 docker-compose.yaml 的项目根目录（兼容在子目录或其它位置运行本脚本）
find_project_dir() {
  local d="$SCRIPT_DIR"
  while [ "$d" != "/" ]; do
    if [ -f "$d/docker-compose.yaml" ] || [ -f "$d/docker-compose.yml" ]; then
      echo "$d"; return
    fi
    d=$(dirname "$d")
  done
  echo "$(cd "$SCRIPT_DIR/.." && pwd)"
}
PROJECT_DIR="$(find_project_dir)"
OVERRIDE="$PROJECT_DIR/docker-compose.override.yml"
STATE="$PROJECT_DIR/.memtune.state"

# 字面的 "$" 字符：用于在 unquoted heredoc 中原样写入 ${VAR:-}，
# 避免被 shell 在生成 override 时提前展开（否则写进文件会变成空值，丢失 .env 覆盖能力）
DOL='$'

# ---------- 颜色 ----------
c_info()  { echo -e "\033[32m[memtune]\033[0m $*"; }
c_warn()  { echo -e "\033[33m[memtune]\033[0m $*"; }
c_err()   { echo -e "\033[31m[memtune]\033[0m $*"; }
c_step()  { echo -e "\033[36m[memtune]\033[0m $*"; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

# ---------- Docker Compose 命令探测 ----------
detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
  elif command_exists docker-compose; then
    COMPOSE_CMD="docker-compose"
  else
    c_err "未检测到 docker compose，请先安装 Docker。"
    exit 1
  fi
}

# 必须在含 docker-compose.yaml 的项目根目录运行
require_compose_file() {
  if [ ! -f "$PROJECT_DIR/docker-compose.yaml" ] && [ ! -f "$PROJECT_DIR/docker-compose.yml" ]; then
    c_err "未在 $PROJECT_DIR 找到 docker-compose.yaml。"
    c_err "请在 cscan 项目根目录（即包含 docker-compose.yaml 的目录）运行本脚本。"
    exit 1
  fi
}

# 组装 compose -f 参数：基础文件 + 已存在的 override
compose_args() {
  local args="-f docker-compose.yaml"
  if [ -f "$OVERRIDE" ]; then
    args="$args -f docker-compose.override.yml"
  fi
  echo "$args"
}

# ---------- 宿主机资源探测 ----------
host_ram_mb() {
  if [ -r /proc/meminfo ]; then
    awk '/^MemTotal:/{print int($2/1024)}' /proc/meminfo
  elif command_exists sysctl; then
    local b; b=$(sysctl -n hw.memsize 2>/dev/null || echo 0)
    echo $(( b / 1048576 ))
  else
    echo 0
  fi
}

host_cpus() {
  if command_exists nproc; then
    nproc
  elif command_exists sysctl; then
    sysctl -n hw.ncpu 2>/dev/null || echo 1
  else
    echo 1
  fi
}

# 将 "4G"/"4096"/"4096M"/"8g" 解析为 MB
to_mb() {
  local s=${1,,}
  if [[ $s == *g ]]; then echo $(( ${s%g} * 1024 ));
  elif [[ $s == *m ]]; then echo "${s%m}";
  else echo "$s"; fi
}

# MB -> compose 内存串 (512M / 2G)
mb_to_compose() {
  local mb=$1
  if [ "$mb" -ge 1024 ] && [ $((mb % 1024)) -eq 0 ]; then
    echo "$((mb / 1024))G"
  else
    echo "${mb}M"
  fi
}

# CPU 预留 = 上限的一个小比例（避免小机过度预留），下限 0.5
res_cpu() {
  awk -v v="$1" 'BEGIN{ r=v/4; if(r<0.5)r=0.5; printf "%.2f", r }'
}

# 内存储备(OS+Docker 开销) 与预算
host_reserve_mb() {
  awk -v mb="$1" 'BEGIN{r=mb*0.12; if(r<1024)r=1024; if(r>4096)r=4096; print int(r)}'
}

# ---------- 画像选择 ----------
tier_for_ram() {
  local mb=$1
  if [ "$mb" -lt 3072 ]; then echo tiny
  elif [ "$mb" -lt 5120 ]; then echo small
  elif [ "$mb" -lt 10240 ]; then echo medium
  elif [ "$mb" -lt 20480 ]; then echo large
  else echo xlarge
  fi
}

# 命名画像写全局变量 P_*
load_preset() {
  case $1 in
    tiny)
      P_REDIS_LIM=256;  P_REDIS_RES=64;   P_REDIS_CPU=1
      P_MONGO_LIM=1024; P_MONGO_RES=256;  P_MONGO_CPU=1
      P_API_LIM=512;    P_API_RES=128;    P_API_CPU=1
      P_RPC_LIM=512;    P_RPC_RES=128;    P_RPC_CPU=1
      P_WEB_LIM=256;    P_WEB_RES=64;     P_WEB_CPU=1
      P_WORKER_LIM=1024;P_WORKER_RES=256; P_WORKER_CPU=1; P_WORKER_CONC=1
      ;;
    small)
      P_REDIS_LIM=512;  P_REDIS_RES=128;  P_REDIS_CPU=2
      P_MONGO_LIM=2048; P_MONGO_RES=512;  P_MONGO_CPU=2
      P_API_LIM=1024;   P_API_RES=256;    P_API_CPU=2
      P_RPC_LIM=1024;   P_RPC_RES=256;    P_RPC_CPU=2
      P_WEB_LIM=512;    P_WEB_RES=128;    P_WEB_CPU=2
      P_WORKER_LIM=2048;P_WORKER_RES=512; P_WORKER_CPU=2; P_WORKER_CONC=2
      ;;
    medium)
      P_REDIS_LIM=512;  P_REDIS_RES=128;  P_REDIS_CPU=3
      P_MONGO_LIM=3072; P_MONGO_RES=768;  P_MONGO_CPU=3
      P_API_LIM=1536;   P_API_RES=384;    P_API_CPU=3
      P_RPC_LIM=1536;   P_RPC_RES=384;    P_RPC_CPU=3
      P_WEB_LIM=512;    P_WEB_RES=128;    P_WEB_CPU=3
      P_WORKER_LIM=3072;P_WORKER_RES=768; P_WORKER_CPU=3; P_WORKER_CONC=4
      ;;
    large)
      P_REDIS_LIM=1024; P_REDIS_RES=256;  P_REDIS_CPU=4
      P_MONGO_LIM=5120; P_MONGO_RES=1280; P_MONGO_CPU=4
      P_API_LIM=2048;   P_API_RES=512;    P_API_CPU=4
      P_RPC_LIM=2048;   P_RPC_RES=512;    P_RPC_CPU=4
      P_WEB_LIM=1024;   P_WEB_RES=256;    P_WEB_CPU=4
      P_WORKER_LIM=4096;P_WORKER_RES=1024;P_WORKER_CPU=4; P_WORKER_CONC=6
      ;;
    xlarge)
      P_REDIS_LIM=2048; P_REDIS_RES=512;  P_REDIS_CPU=6
      P_MONGO_LIM=8192; P_MONGO_RES=2048; P_MONGO_CPU=6
      P_API_LIM=3072;   P_API_RES=768;    P_API_CPU=6
      P_RPC_LIM=3072;   P_RPC_RES=768;    P_RPC_CPU=6
      P_WEB_LIM=1024;   P_WEB_RES=256;    P_WEB_CPU=6
      P_WORKER_LIM=6144;P_WORKER_RES=1536;P_WORKER_CPU=6; P_WORKER_CONC=10
      ;;
    *) c_err "未知画像: $1 (可选: tiny/small/medium/large/xlarge)"; exit 1 ;;
  esac
}

# 精确计算：依据宿主机内存/CPU 推导各服务配额
compute_alloc() {
  local mb=$1 cpus=$2
  local reserve; reserve=$(host_reserve_mb "$mb")
  # awk 输出 15 个值: 6 个内存(MB) + 6 个 CPU + 1 个并发
  read -r P_REDIS_LIM P_MONGO_LIM P_API_LIM P_RPC_LIM P_WEB_LIM P_WORKER_LIM \
          P_REDIS_CPU P_MONGO_CPU P_API_CPU P_RPC_CPU P_WEB_CPU P_WORKER_CPU \
          P_WORKER_CONC <<< "$(
    awk -v mb="$mb" -v cpus="$cpus" '
    BEGIN {
      split("1 6 2.5 2.5 1 6", w, " ");
      mins[1]=256; mins[2]=1024; mins[3]=512; mins[4]=512; mins[5]=256; mins[6]=1024;
      sum=19;
      r=mb*0.12; if(r<1024)r=1024; if(r>4096)r=4096; reserve=int(r);
      budget=mb-reserve;
      for(i=1;i<=6;i++){ a[i]=int(budget*0.9*w[i]/sum); if(a[i]<mins[i]) a[i]=mins[i]; }
      total=a[1]+a[2]+a[3]+a[4]+a[5]+a[6];
      if(total>budget){ scale=budget/total; for(i=1;i<=6;i++){ a[i]=int(a[i]*scale); if(a[i]<mins[i])a[i]=mins[i]; } }
      for(i=1;i<=6;i++){ c[i]=a[i]/total*cpus; if(c[i]<0.5)c[i]=0.5; if(c[i]>cpus)c[i]=cpus; }
      conc=int(a[6]*0.6/384); if(conc<1)conc=1; if(conc>16)conc=16;
      printf "%d %d %d %d %d %d %.2f %.2f %.2f %.2f %.2f %.2f %d",
             a[1],a[2],a[3],a[4],a[5],a[6], c[1],c[2],c[3],c[4],c[5],c[6], conc;
    }')"
  # 低配无法容纳保底值时回退到 small 画像
  local total=$(( P_REDIS_LIM + P_MONGO_LIM + P_API_LIM + P_RPC_LIM + P_WEB_LIM + P_WORKER_LIM ))
  local budget=$(( mb - reserve ))
  if [ "$total" -gt "$budget" ]; then
    c_warn "可用内存不足以承载自动计算配额，回退到 small 画像（建议宿主机 ≥4G）。"
    load_preset small
  fi
  # 预留(Reservation) = 上限的一半，下限 64M
  P_REDIS_RES=$(( P_REDIS_LIM / 2  > 64 ? P_REDIS_LIM / 2  : 64 ))
  P_MONGO_RES=$(( P_MONGO_LIM / 2  > 64 ? P_MONGO_LIM / 2  : 64 ))
  P_API_RES=$(( P_API_LIM / 2     > 64 ? P_API_LIM / 2     : 64 ))
  P_RPC_RES=$(( P_RPC_LIM / 2     > 64 ? P_RPC_LIM / 2     : 64 ))
  P_WEB_RES=$(( P_WEB_LIM / 2     > 64 ? P_WEB_LIM / 2     : 64 ))
  P_WORKER_RES=$(( P_WORKER_LIM / 2 > 64 ? P_WORKER_LIM / 2 : 64 ))
}

# 依据目标解析画像：auto/RAM -> 计算或分级；命名 -> 直接取
resolve_profile() {
  local target=${1:-auto}
  local mb cpu
  mb=$(host_ram_mb); cpu=$(host_cpus)
  P_SOURCE=""
  case "$target" in
    auto)
      P_SOURCE="auto(宿主机 ${mb}MB)"
      if [ "$mb" -lt 5120 ]; then
        load_preset "$(tier_for_ram "$mb")"
        P_PROFILE="$(tier_for_ram "$mb")"
        [ "$mb" -lt 3072 ] && c_warn "内存低于 3G，已采用最保守画像；强烈建议 ≥4G。"
      else
        compute_alloc "$mb" "$cpu"
        P_PROFILE="computed"
      fi
      ;;
    tiny|small|medium|large|xlarge)
      load_preset "$target"
      P_PROFILE="$target"
      P_SOURCE="named($target)"
      ;;
    *)
      # 视为显式内存规格
      mb=$(to_mb "$target")
      if [ "$mb" -lt 512 ]; then c_err "内存规格过小: ${target}"; exit 1; fi
      P_SOURCE="ram(${target})"
      if [ "$mb" -lt 5120 ]; then
        load_preset "$(tier_for_ram "$mb")"
        P_PROFILE="$(tier_for_ram "$mb")"
      else
        compute_alloc "$mb" "$cpu"
        P_PROFILE="computed"
      fi
      ;;
  esac
  HOST_MB=$mb; HOST_CPUS=$cpu
  # CPU 上限不超过宿主机核数
  P_REDIS_CPU=$(awk -v v="$P_REDIS_CPU" -v m="$cpu" 'BEGIN{print (v>m?m:v)}')
  P_MONGO_CPU=$(awk -v v="$P_MONGO_CPU" -v m="$cpu" 'BEGIN{print (v>m?m:v)}')
  P_API_CPU=$(awk -v v="$P_API_CPU" -v m="$cpu" 'BEGIN{print (v>m?m:v)}')
  P_RPC_CPU=$(awk -v v="$P_RPC_CPU" -v m="$cpu" 'BEGIN{print (v>m?m:v)}')
  P_WEB_CPU=$(awk -v v="$P_WEB_CPU" -v m="$cpu" 'BEGIN{print (v>m?m:v)}')
  P_WORKER_CPU=$(awk -v v="$P_WORKER_CPU" -v m="$cpu" 'BEGIN{print (v>m?m:v)}')
}

# ---------- 落盘: override + state ----------
write_override() {
  local ts; ts=$(date '+%Y-%m-%d %H:%M:%S')
  cat > "$OVERRIDE" <<EOF
# 由 scripts/memtune.sh 自动生成，请勿手动编辑。
# 当前内存画像: ${P_PROFILE} (来源: ${P_SOURCE})
# 宿主机内存: ${HOST_MB}MB / CPU: ${HOST_CPUS}   生成时间: ${ts}
# 说明: 本文件被 docker compose 自动合并，覆盖 docker-compose.yaml 中的默认上限。

services:
  redis:
    deploy:
      resources:
        limits:
          memory: $(mb_to_compose "$P_REDIS_LIM")
        reservations:
          memory: $(mb_to_compose "$P_REDIS_RES")

  mongodb:
    deploy:
      resources:
        limits:
          memory: $(mb_to_compose "$P_MONGO_LIM")
        reservations:
          memory: $(mb_to_compose "$P_MONGO_RES")

  cscan-api:
    deploy:
      resources:
        limits:
          cpus: '${P_API_CPU}'
          memory: $(mb_to_compose "$P_API_LIM")
        reservations:
          cpus: '$(res_cpu "$P_API_CPU")'
          memory: $(mb_to_compose "$P_API_RES")
    environment:
      # 留空默认 -> Go 运行时按 cgroup 自动推导 GOMAXPROCS / GOMEMLIMIT（含运行时热更新后自适配）
      # 若用户在 .env 显式设置 CSCAN_API_GOMAXPROCS / CSCAN_API_GOMEMLIMIT 则优先生效
      # 注意：此处 ${DOL} 渲染为字面 "$"，使 compose 在 up 时重新从 .env 读取，而非写死空值
      - GOMAXPROCS=${DOL}{CSCAN_API_GOMAXPROCS:-}
      - GOMEMLIMIT=${DOL}{CSCAN_API_GOMEMLIMIT:-}

  cscan-rpc:
    deploy:
      resources:
        limits:
          cpus: '${P_RPC_CPU}'
          memory: $(mb_to_compose "$P_RPC_LIM")
        reservations:
          cpus: '$(res_cpu "$P_RPC_CPU")'
          memory: $(mb_to_compose "$P_RPC_RES")
    environment:
      - GOMAXPROCS=${DOL}{CSCAN_RPC_GOMAXPROCS:-}
      - GOMEMLIMIT=${DOL}{CSCAN_RPC_GOMEMLIMIT:-}

  cscan-web:
    deploy:
      resources:
        limits:
          cpus: '${P_WEB_CPU}'
          memory: $(mb_to_compose "$P_WEB_LIM")
        reservations:
          cpus: '$(res_cpu "$P_WEB_CPU")'
          memory: $(mb_to_compose "$P_WEB_RES")

  cscan-worker:
    deploy:
      resources:
        limits:
          cpus: '${P_WORKER_CPU}'
          memory: $(mb_to_compose "$P_WORKER_LIM")
        reservations:
          cpus: '$(res_cpu "$P_WORKER_CPU")'
          memory: $(mb_to_compose "$P_WORKER_RES")
    environment:
      - GOMAXPROCS=${DOL}{CSCAN_WORKER_GOMAXPROCS:-}
      - GOMEMLIMIT=${DOL}{CSCAN_WORKER_GOMEMLIMIT:-}
      - CSCAN_CONCURRENCY=${P_WORKER_CONC}
EOF
}

write_state() {
  cat > "$STATE" <<EOF
# memtune 运行时状态（供 update/monitor/status 读取，可 source）
PROFILE=${P_PROFILE}
SOURCE=${P_SOURCE}
HOST_MB=${HOST_MB}
HOST_CPUS=${HOST_CPUS}
REDIS_CONTAINER=cscan_redis
REDIS_LIMIT_MB=${P_REDIS_LIM}
REDIS_RES_MB=${P_REDIS_RES}
REDIS_CPU=${P_REDIS_CPU}
MONGO_CONTAINER=cscan_mongodb
MONGO_LIMIT_MB=${P_MONGO_LIM}
MONGO_RES_MB=${P_MONGO_RES}
MONGO_CPU=${P_MONGO_CPU}
API_CONTAINER=cscan_api
API_LIMIT_MB=${P_API_LIM}
API_RES_MB=${P_API_RES}
API_CPU=${P_API_CPU}
RPC_CONTAINER=cscan_rpc
RPC_LIMIT_MB=${P_RPC_LIM}
RPC_RES_MB=${P_RPC_RES}
RPC_CPU=${P_RPC_CPU}
WEB_CONTAINER=cscan_web
WEB_LIMIT_MB=${P_WEB_LIM}
WEB_RES_MB=${P_WEB_RES}
WEB_CPU=${P_WEB_CPU}
WORKER_CONTAINER=cscan_worker
WORKER_LIMIT_MB=${P_WORKER_LIM}
WORKER_RES_MB=${P_WORKER_RES}
WORKER_CPU=${P_WORKER_CPU}
WORKER_CONCURRENCY=${P_WORKER_CONC}
EOF
}

# 从 state 重建 override（用于运行时调整 Worker 并发后）
regen_override_from_state() {
  # shellcheck disable=SC1090
  source "$STATE"
  P_PROFILE=$PROFILE; P_SOURCE=$SOURCE; HOST_MB=$HOST_MB; HOST_CPUS=$HOST_CPUS
  P_REDIS_LIM=$REDIS_LIMIT_MB;   P_REDIS_RES=$REDIS_RES_MB;   P_REDIS_CPU=$REDIS_CPU
  P_MONGO_LIM=$MONGO_LIMIT_MB;   P_MONGO_RES=$MONGO_RES_MB;   P_MONGO_CPU=$MONGO_CPU
  P_API_LIM=$API_LIMIT_MB;       P_API_RES=$API_RES_MB;       P_API_CPU=$API_CPU
  P_RPC_LIM=$RPC_LIMIT_MB;       P_RPC_RES=$RPC_RES_MB;       P_RPC_CPU=$RPC_CPU
  P_WEB_LIM=$WEB_LIMIT_MB;       P_WEB_RES=$WEB_RES_MB;       P_WEB_CPU=$WEB_CPU
  P_WORKER_LIM=$WORKER_LIMIT_MB; P_WORKER_RES=$WORKER_RES_MB; P_WORKER_CPU=$WORKER_CPU
  P_WORKER_CONC=$WORKER_CONCURRENCY
  write_override
}

# ---------- 展示 ----------
show_plan() {
  c_step "内存画像: ${P_PROFILE}  (来源: ${P_SOURCE})"
  c_step "宿主机: ${HOST_MB}MB RAM / ${HOST_CPUS} CPU"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "服务" "上限" "预留" "CPU" "并发"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "----" "----" "----" "---" "----"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "redis"   "$(mb_to_compose "$P_REDIS_LIM")"  "$(mb_to_compose "$P_REDIS_RES")"  "$P_REDIS_CPU"  "-"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "mongodb" "$(mb_to_compose "$P_MONGO_LIM")" "$(mb_to_compose "$P_MONGO_RES")" "$P_MONGO_CPU" "-"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "api"     "$(mb_to_compose "$P_API_LIM")"   "$(mb_to_compose "$P_API_RES")"   "$P_API_CPU"   "-"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "rpc"     "$(mb_to_compose "$P_RPC_LIM")"   "$(mb_to_compose "$P_RPC_RES")"   "$P_RPC_CPU"   "-"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "web"     "$(mb_to_compose "$P_WEB_LIM")"   "$(mb_to_compose "$P_WEB_RES")"   "$P_WEB_CPU"   "-"
  printf "%-10s %-10s %-10s %-8s %-8s\n" "worker"  "$(mb_to_compose "$P_WORKER_LIM")" "$(mb_to_compose "$P_WORKER_RES")" "$P_WORKER_CPU" "$P_WORKER_CONC"
}

# ---------- 子命令 ----------
cmd_detect() {
  local mb cpu rec
  mb=$(host_ram_mb); cpu=$(host_cpus)
  rec=$(tier_for_ram "$mb")
  c_info "宿主机内存: ${mb}MB / CPU: ${cpu}"
  c_info "推荐内存画像: ${rec}"
  c_info "可用命令: ./memtune.sh apply auto   # 按当前内存自动应用"
}

cmd_plan() {
  resolve_profile "${1:-auto}"
  show_plan
}

cmd_apply() {
  detect_compose
  require_compose_file
  resolve_profile "${1:-auto}"
  show_plan
  write_override
  write_state
  c_info "已生成 $OVERRIDE，正在重建服务以应用新配额..."
  ( cd "$PROJECT_DIR" && $COMPOSE_CMD $(compose_args) up -d )
  c_info "应用完成。运行 ./memtune.sh status 查看实际占用。"
}

cmd_update() {
  detect_compose
  if [ ! -f "$STATE" ]; then
    c_err "未找到 $STATE，请先执行 ./memtune.sh apply。"
    exit 1
  fi
  # shellcheck disable=SC1090
  source "$STATE"
  c_step "运行时热更新内存/CPU 上限（不重启容器）..."
  local ok=0 fail=0
  for svc in REDIS MONGO API RPC WEB WORKER; do
    local container limit cpu
    container=$(eval echo "\${${svc}_CONTAINER}")
    limit=$(eval echo "\${${svc}_LIMIT_MB}")
    cpu=$(eval echo "\${${svc}_CPU}")
    if docker update --memory "${limit}M" --cpus "$cpu" "$container" >/dev/null 2>&1; then
      c_info "  $container -> mem=$(mb_to_compose "$limit") cpu=$cpu"
      ok=$((ok+1))
    else
      c_warn "  $container 热更新失败（cgroup 可能不支持，需重建: ./memtune.sh apply）"
      fail=$((fail+1))
    fi
  done
  c_info "热更新完成: 成功 $ok / 失败 $fail。Go 服务将按新 cgroup 自动调整 GOMAXPROCS/GOMEMLIMIT。"
}

cmd_status() {
  detect_compose
  c_step "CSCAN 内存状态"
  if [ -f "$STATE" ]; then
    # shellcheck disable=SC1090
    source "$STATE"
    c_info "当前画像: ${PROFILE} (${SOURCE}) | 宿主机 ${HOST_MB}MB / ${HOST_CPUS}CPU"
  else
    c_warn "未检测到 memtune 状态文件，显示 compose 默认上限。"
  fi
  echo "---------------------------------------------------------------------------"
  printf "%-14s %-10s %-12s %-8s\n" "容器" "配额" "当前占用" "使用率"
  echo "---------------------------------------------------------------------------"
  for c in cscan_redis cscan_mongodb cscan_api cscan_rpc cscan_web cscan_worker; do
    local lim usage pct
    lim=$(docker inspect -f '{{.HostConfig.Memory}}' "$c" 2>/dev/null || echo 0)
    if [ "$lim" = "0" ]; then continue; fi
    local line
    line=$(docker stats --no-stream --format "{{.MemUsage}}|{{.MemPerc}}" "$c" 2>/dev/null || echo "n/a|n/a")
    usage=$(echo "$line" | cut -d'|' -f1)
    pct=$(echo "$line" | cut -d'|' -f2)
    printf "%-14s %-10s %-12s %-8s\n" "$c" "$(mb_to_compose $((lim/1024/1024)))" "$usage" "$pct"
  done
  echo "---------------------------------------------------------------------------"
}

cmd_monitor() {
  detect_compose
  local once=0 interval=30 threshold=85 autoprotect=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --once) once=1 ;;
      --interval=*) interval=${1#*=} ;;
      --threshold=*) threshold=${1#*=} ;;
      --auto-protect) autoprotect=1 ;;
      *) c_err "未知参数: $1"; exit 1 ;;
    esac
    shift
  done
  if [ ! -f "$STATE" ]; then
    c_err "未找到 $STATE，请先执行 ./memtune.sh apply。"
    exit 1
  fi
  # shellcheck disable=SC1090
  source "$STATE"
  c_info "开始监控 (阈值=${threshold}%${autoprotect:+, 自动保护开启})。Ctrl+C 退出。"
  while true; do
    local ts; ts=$(date '+%H:%M:%S')
    for svc in REDIS MONGO API RPC WEB WORKER; do
      local container limit
      container=$(eval echo "\${${svc}_CONTAINER}")
      limit=$(eval echo "\${${svc}_LIMIT_MB}")
      local pct raw
      raw=$(docker stats --no-stream --format "{{.MemPerc}}" "$container" 2>/dev/null || echo "0.00%")
      pct=$(echo "$raw" | tr -d '%' | awk '{printf "%.0f", $1}')
      if [ "${pct:-0}" -ge "$threshold" ]; then
        c_err "[${ts}] 超限 ${container}: ${raw} (上限 $(mb_to_compose "$limit"))"
        send_alert "$container" "$raw"
        if [ -n "$autoprotect" ] && [ "$svc" = "WORKER" ]; then
          protect_worker
        fi
      else
        c_info "[${ts}] OK   ${container}: ${raw}"
      fi
    done
    # 宿主机可用内存过低也告警
    if [ -r /proc/meminfo ]; then
      local avail tot pct
      avail=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)
      tot=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
      pct=$(awk -v a="$avail" -v t="$tot" 'BEGIN{print int(a/t*100)}')
      if [ "$pct" -lt 5 ]; then
        c_err "[${ts}] 宿主机可用内存仅 ${pct}%，存在整体 OOM 风险！"
      fi
    fi
    [ "$once" -eq 1 ] && break
    sleep "$interval"
  done
}

protect_worker() {
  if [ "$WORKER_CONCURRENCY" -le 1 ]; then
    c_warn "  Worker 并发已为最小值 1，无法继续降低；尝试重启 Worker 释放内存。"
    ( cd "$PROJECT_DIR" && $COMPOSE_CMD restart cscan-worker ) || true
    return
  fi
  local new=$(( WORKER_CONCURRENCY - 1 ))
  c_warn "  自动保护: 降低 Worker 并发 ${WORKER_CONCURRENCY} -> ${new} 并重建 Worker。"
  sed -i "s/^WORKER_CONCURRENCY=.*/WORKER_CONCURRENCY=${new}/" "$STATE"
  regen_override_from_state
  ( cd "$PROJECT_DIR" && $COMPOSE_CMD $(compose_args) up -d cscan_worker ) || true
  WORKER_CONCURRENCY=$new
  c_info "  Worker 并发已调整为 ${new}。"
}

send_alert() {
  local container=$1 pct=$2
  local webhook=${CSCAN_ALERT_WEBHOOK:-}
  [ -z "$webhook" ] && return
  command_exists curl || return
  curl -s -m 5 -X POST "$webhook" \
    -H 'Content-Type: application/json' \
    -d "{\"text\":\"[cscan] 内存超限: ${container} 使用率 ${pct}\"}" >/dev/null 2>&1 || true
}

cmd_reset() {
  detect_compose
  require_compose_file
  if [ -f "$OVERRIDE" ]; then
    rm -f "$OVERRIDE"
    c_info "已删除 $OVERRIDE"
  fi
  if [ -f "$STATE" ]; then
    rm -f "$STATE"
    c_info "已删除 $STATE"
  fi
  c_info "正在以 compose 默认上限重建服务..."
  ( cd "$PROJECT_DIR" && $COMPOSE_CMD -f docker-compose.yaml up -d )
  c_info "已回退到默认内存上限。"
}

# ---------- 入口 ----------
case "${1:-help}" in
  detect)    cmd_detect ;;
  plan)      shift; cmd_plan "$@" ;;
  apply)     shift; cmd_apply "$@" ;;
  update)    cmd_update ;;
  status)    cmd_status ;;
  monitor)   shift; cmd_monitor "$@" ;;
  reset)     cmd_reset ;;
  help|-h|--help|"")
    echo "CSCAN 内存自适应调节工具 memtune.sh"
    echo ""
    echo "用法:"
    echo "  detect                                          检测宿主机并给出推荐画像"
    echo "  plan [auto|tiny|small|medium|large|xlarge|<RAM>] 预览配额(不落盘)"
    echo "  apply [auto|tiny|small|medium|large|xlarge|<RAM>] 生成 override 并重建"
    echo "  update                                          运行时热更新内存/CPU上限(不重启)"
    echo "  monitor [--once] [--interval=N] [--threshold=N] [--auto-protect]"
    echo "  status                                          查看当前配额与实际占用"
    echo "  reset                                           移除 override, 回退默认上限"
    ;;
  *) c_err "未知命令: $1"; exit 1 ;;
esac
