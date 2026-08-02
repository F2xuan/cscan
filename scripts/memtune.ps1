# memtune.ps1 - CSCAN 内存自适应调节工具 (Windows / PowerShell)
# 与 scripts/memtune.sh 功能对齐：配置化、按可用内存自动计算、弹性调整、
# 运行时热更新(docker update)、监控与超限保护。
# 由 memtune.bat 调度：powershell -NoProfile -ExecutionPolicy Bypass -File memtune.ps1 <args>

param()

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# 向上查找包含 docker-compose.yaml 的项目根目录（兼容在子目录或其它位置运行本脚本）
function Find-ProjectDir {
  $d = $ScriptDir
  while ($true) {
    if (-not $d) { break }
    if (Test-Path (Join-Path $d 'docker-compose.yaml')) { return $d }
    if (Test-Path (Join-Path $d 'docker-compose.yml')) { return $d }
    $parent = Split-Path -Parent $d
    if ($parent -eq $d -or -not $parent) { break }
    $d = $parent
  }
  return (Split-Path -Parent $ScriptDir)
}
$ProjectDir = Find-ProjectDir
$Override = Join-Path $ProjectDir 'docker-compose.override.yml'
$State = Join-Path $ProjectDir '.memtune.state'

function Require-ComposeFile {
  if (-not (Test-Path (Join-Path $ProjectDir 'docker-compose.yaml')) -and -not (Test-Path (Join-Path $ProjectDir 'docker-compose.yml'))) {
    Err "未在 $ProjectDir 找到 docker-compose.yaml。"
    Err '请在 cscan 项目根目录（即包含 docker-compose.yaml 的目录）运行本脚本。'
    exit 1
  }
}

function ComposeArgs {
  $a = '-f docker-compose.yaml'
  if (Test-Path $Override) { $a += ' -f docker-compose.override.yml' }
  return $a
}

function Info($m){ Write-Host "[memtune] $m" -ForegroundColor Green }
function Warn($m){ Write-Host "[memtune] $m" -ForegroundColor Yellow }
function Err($m){ Write-Host "[memtune] $m" -ForegroundColor Red }
function Step($m){ Write-Host "[memtune] $m" -ForegroundColor Cyan }

function Get-ComposeCmd {
  if (docker compose version 2>$null) { return 'docker compose' }
  if (Get-Command docker-compose -ErrorAction SilentlyContinue) { return 'docker-compose' }
  Err '未检测到 docker compose，请先安装 Docker。'; exit 1
}

function Get-HostRamMb {
  try {
    $b = (Get-CimInstance Win32_PhysicalMemory | Measure-Object Capacity -Sum).Sum
    return [int]($b / 1MB)
  } catch { return 0 }
}

function Get-HostCpus {
  try { return [int](Get-CimInstance Win32_ComputerSystem).NumberOfLogicalProcessors } catch { return 1 }
}

function ToMb($s) {
  $s = $s.ToString().ToLower()
  if ($s.EndsWith('g')) { return [int]($s.Substring(0, $s.Length-1)) * 1024 }
  if ($s.EndsWith('m')) { return [int]($s.Substring(0, $s.Length-1)) }
  return [int]$s
}

function MbToCompose($mb) {
  if ($mb -ge 1024 -and $mb % 1024 -eq 0) { return ($mb / 1024).ToString() + 'G' }
  return $mb.ToString() + 'M'
}

function TierForRam($mb) {
  if ($mb -lt 3072) { return 'tiny' }
  if ($mb -lt 5120) { return 'small' }
  if ($mb -lt 10240) { return 'medium' }
  if ($mb -lt 20480) { return 'large' }
  return 'xlarge'
}

# 全局配额变量
$P = @{}

function LoadPreset($tier) {
  switch ($tier) {
    'tiny'   { $P = @{ redis_lim=256;  redis_res=64;  redis_cpu=1; mongo_lim=1024; mongo_res=256; mongo_cpu=1; api_lim=512;  api_res=128; api_cpu=1; rpc_lim=512;  rpc_res=128; rpc_cpu=1; web_lim=256;  web_res=64;  web_cpu=1; worker_lim=1024; worker_res=256; worker_cpu=1; worker_conc=1 } }
    'small'  { $P = @{ redis_lim=512;  redis_res=128; redis_cpu=2; mongo_lim=2048; mongo_res=512; mongo_cpu=2; api_lim=1024; api_res=256; api_cpu=2; rpc_lim=1024; rpc_res=256; rpc_cpu=2; web_lim=512;  web_res=128; web_cpu=2; worker_lim=2048; worker_res=512; worker_cpu=2; worker_conc=2 } }
    'medium' { $P = @{ redis_lim=512;  redis_res=128; redis_cpu=3; mongo_lim=3072; mongo_res=768; mongo_cpu=3; api_lim=1536; api_res=384; api_cpu=3; rpc_lim=1536; rpc_res=384; rpc_cpu=3; web_lim=512;  web_res=128; web_cpu=3; worker_lim=3072; worker_res=768; worker_cpu=3; worker_conc=4 } }
    'large'  { $P = @{ redis_lim=1024; redis_res=256; redis_cpu=4; mongo_lim=5120; mongo_res=1280;mongo_cpu=4; api_lim=2048; api_res=512; api_cpu=4; rpc_lim=2048; rpc_res=512; rpc_cpu=4; web_lim=1024; web_res=256; web_cpu=4; worker_lim=4096; worker_res=1024;worker_cpu=4; worker_conc=6 } }
    'xlarge' { $P = @{ redis_lim=2048; redis_res=512; redis_cpu=6; mongo_lim=8192; mongo_res=2048;mongo_cpu=6; api_lim=3072; api_res=768; api_cpu=6; rpc_lim=3072; rpc_res=768; rpc_cpu=6; web_lim=1024; web_res=256; web_cpu=6; worker_lim=6144; worker_res=1536;worker_cpu=6; worker_conc=10 } }
    default  { Err "未知画像: $tier"; exit 1 }
  }
}

function ComputeAlloc($mb, $cpus) {
  $reserve = [math]::Max(1024, [math]::Min(4096, [int]($mb * 0.12)))
  $budget = $mb - $reserve
  $w = @(1, 6, 2.5, 2.5, 1, 6)
  $mins = @(256, 1024, 512, 512, 256, 1024)
  $sum = 19
  $a = @(0)*6
  for ($i=0; $i -lt 6; $i++) {
    $a[$i] = [int]($budget * 0.9 * $w[$i] / $sum)
    if ($a[$i] -lt $mins[$i]) { $a[$i] = $mins[$i] }
  }
  $total = ($a | Measure-Object -Sum).Sum
  if ($total -gt $budget) {
    $scale = $budget / $total
    for ($i=0; $i -lt 6; $i++) {
      $a[$i] = [int]($a[$i] * $scale)
      if ($a[$i] -lt $mins[$i]) { $a[$i] = $mins[$i] }
    }
  }
  $c = @(0)*6
  for ($i=0; $i -lt 6; $i++) {
    $cv = $a[$i] / $total * $cpus
    if ($cv -lt 0.5) { $cv = 0.5 }
    if ($cv -gt $cpus) { $cv = $cpus }
    $c[$i] = [math]::Max(1, [int]($cv + 0.5))
  }
  $conc = [math]::Max(1, [math]::Min(16, [int]($a[5] * 0.6 / 384)))
  $P = @{
    redis_lim=$a[0]; redis_res=[int]($a[0]/2); redis_cpu=$c[0];
    mongo_lim=$a[1]; mongo_res=[int]($a[1]/2); mongo_cpu=$c[1];
    api_lim=$a[2];   api_res=[int]($a[2]/2);   api_cpu=$c[2];
    rpc_lim=$a[3];   rpc_res=[int]($a[3]/2);   rpc_cpu=$c[3];
    web_lim=$a[4];   web_res=[int]($a[4]/2);   web_cpu=$c[4];
    worker_lim=$a[5];worker_res=[int]($a[5]/2);worker_cpu=$c[5]; worker_conc=$conc
  }
  # 低配无法容纳时回退 small
  $tt = $a[0]+$a[1]+$a[2]+$a[3]+$a[4]+$a[5]
  if ($tt -gt $budget) { Warn '可用内存不足以承载自动计算配额，回退到 small 画像（建议宿主机 ≥4G）。'; LoadPreset 'small' }
}

function ResolveProfile($target) {
  $mb = Get-HostRamMb; $cpus = Get-HostCpus
  $global:HostMb = $mb; $global:HostCpus = $cpus
  if (-not $target -or $target -eq 'auto') {
    $global:ProfileSource = "auto(宿主机 ${mb}MB)"
    if ($mb -lt 5120) { $t = TierForRam $mb; LoadPreset $t; $global:ProfileName = $t; if ($mb -lt 3072) { Warn '内存低于 3G，已采用最保守画像；强烈建议 ≥4G。' } }
    else { ComputeAlloc $mb $cpus; $global:ProfileName = 'computed' }
  } elseif (@('tiny','small','medium','large','xlarge') -contains $target) {
    LoadPreset $target; $global:ProfileName = $target; $global:ProfileSource = "named($target)"
  } else {
    $m = ToMb $target
    if ($m -lt 512) { Err "内存规格过小: $target"; exit 1 }
    $global:ProfileSource = "ram($target)"
    if ($m -lt 5120) { $t = TierForRam $m; LoadPreset $t; $global:ProfileName = $t }
    else { ComputeAlloc $m $cpus; $global:ProfileName = 'computed' }
  }
  # CPU 不超过宿主机核数
  foreach ($k in @('redis_cpu','mongo_cpu','api_cpu','rpc_cpu','web_cpu','worker_cpu')) {
    if ($P[$k] -gt $cpus) { $P[$k] = $cpus }
  }
}

function WriteOverride {
  $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
  $sb = @"
# 由 scripts/memtune.ps1 自动生成，请勿手动编辑。
# 当前内存画像: $($global:ProfileName) (来源: $($global:ProfileSource))
# 宿主机内存: $($global:HostMb)MB / CPU: $($global:HostCpus)   生成时间: $ts
# 说明: 本文件被 docker compose 自动合并，覆盖 docker-compose.yaml 中的默认上限。

services:
  redis:
    deploy:
      resources:
        limits:
          memory: $(MbToCompose $P['redis_lim'])
        reservations:
          memory: $(MbToCompose $P['redis_res'])

  mongodb:
    deploy:
      resources:
        limits:
          memory: $(MbToCompose $P['mongo_lim'])
        reservations:
          memory: $(MbToCompose $P['mongo_res'])

  cscan-api:
    deploy:
      resources:
        limits:
          cpus: '$($P['api_cpu'])'
          memory: $(MbToCompose $P['api_lim'])
        reservations:
          cpus: '$(([math]::Max(0.5, [double]$P['api_cpu']/4)).ToString('0.00'))'
          memory: $(MbToCompose $P['api_res'])
    environment:
      - GOMAXPROCS=`${CSCAN_API_GOMAXPROCS:-}
      - GOMEMLIMIT=`${CSCAN_API_GOMEMLIMIT:-}

  cscan-rpc:
    deploy:
      resources:
        limits:
          cpus: '$($P['rpc_cpu'])'
          memory: $(MbToCompose $P['rpc_lim'])
        reservations:
          cpus: '$(([math]::Max(0.5, [double]$P['rpc_cpu']/4)).ToString('0.00'))'
          memory: $(MbToCompose $P['rpc_res'])
    environment:
      - GOMAXPROCS=`${CSCAN_RPC_GOMAXPROCS:-}
      - GOMEMLIMIT=`${CSCAN_RPC_GOMEMLIMIT:-}

  cscan-web:
    deploy:
      resources:
        limits:
          cpus: '$($P['web_cpu'])'
          memory: $(MbToCompose $P['web_lim'])
        reservations:
          cpus: '$(([math]::Max(0.5, [double]$P['web_cpu']/4)).ToString('0.00'))'
          memory: $(MbToCompose $P['web_res'])

  cscan-worker:
    deploy:
      resources:
        limits:
          cpus: '$($P['worker_cpu'])'
          memory: $(MbToCompose $P['worker_lim'])
        reservations:
          cpus: '$(([math]::Max(0.5, [double]$P['worker_cpu']/4)).ToString('0.00'))'
          memory: $(MbToCompose $P['worker_res'])
    environment:
      - GOMAXPROCS=`${CSCAN_WORKER_GOMAXPROCS:-}
      - GOMEMLIMIT=`${CSCAN_WORKER_GOMEMLIMIT:-}
      - CSCAN_CONCURRENCY=$($P['worker_conc'])
"@
  Set-Content -Path $Override -Value $sb -Encoding utf8
}

function WriteState {
  $lines = @(
    "PROFILE=$($global:ProfileName)",
    "SOURCE=$($global:ProfileSource)",
    "HOST_MB=$($global:HostMb)",
    "HOST_CPUS=$($global:HostCpus)",
    "REDIS_CONTAINER=cscan_redis",
    "REDIS_LIMIT_MB=$($P['redis_lim'])", "REDIS_RES_MB=$($P['redis_res'])", "REDIS_CPU=$($P['redis_cpu'])",
    "MONGO_CONTAINER=cscan_mongodb",
    "MONGO_LIMIT_MB=$($P['mongo_lim'])", "MONGO_RES_MB=$($P['mongo_res'])", "MONGO_CPU=$($P['mongo_cpu'])",
    "API_CONTAINER=cscan_api",
    "API_LIMIT_MB=$($P['api_lim'])", "API_RES_MB=$($P['api_res'])", "API_CPU=$($P['api_cpu'])",
    "RPC_CONTAINER=cscan_rpc",
    "RPC_LIMIT_MB=$($P['rpc_lim'])", "RPC_RES_MB=$($P['rpc_res'])", "RPC_CPU=$($P['rpc_cpu'])",
    "WEB_CONTAINER=cscan_web",
    "WEB_LIMIT_MB=$($P['web_lim'])", "WEB_RES_MB=$($P['web_res'])", "WEB_CPU=$($P['web_cpu'])",
    "WORKER_CONTAINER=cscan_worker",
    "WORKER_LIMIT_MB=$($P['worker_lim'])", "WORKER_RES_MB=$($P['worker_res'])", "WORKER_CPU=$($P['worker_cpu'])",
    "WORKER_CONCURRENCY=$($P['worker_conc'])"
  )
  Set-Content -Path $State -Value $lines -Encoding ascii
}

function LoadState {
  if (-not (Test-Path $State)) { Err "未找到 $State，请先执行 apply。"; exit 1 }
  $h = @{}
  foreach ($line in (Get-Content $State)) {
    if ($line -match '^([A-Z_]+)=(.*)$') { $h[$Matches[1]] = $Matches[2] }
  }
  return $h
}

function ShowPlan {
  Step "内存画像: $($global:ProfileName)  (来源: $($global:ProfileSource))"
  Step "宿主机: $($global:HostMb)MB RAM / $($global:HostCpus) CPU"
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f '服务','上限','预留','CPU','并发'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f '----','----','----','---','----'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f 'redis',  (MbToCompose $P['redis_lim']),  (MbToCompose $P['redis_res']),  $P['redis_cpu'],  '-'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f 'mongodb',(MbToCompose $P['mongo_lim']), (MbToCompose $P['mongo_res']), $P['mongo_cpu'], '-'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f 'api',    (MbToCompose $P['api_lim']),    (MbToCompose $P['api_res']),    $P['api_cpu'],    '-'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f 'rpc',    (MbToCompose $P['rpc_lim']),    (MbToCompose $P['rpc_res']),    $P['rpc_cpu'],    '-'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f 'web',    (MbToCompose $P['web_lim']),    (MbToCompose $P['web_res']),    $P['web_cpu'],    '-'
  '{0,-10}{1,-10}{2,-10}{3,-8}{4,-8}' -f 'worker', (MbToCompose $P['worker_lim']), (MbToCompose $P['worker_res']), $P['worker_cpu'], $P['worker_conc']
}

function Cmd-Detect {
  Info "宿主机内存: $(Get-HostRamMb)MB / CPU: $(Get-HostCpus)"
  Info "推荐内存画像: $(TierForRam (Get-HostRamMb))"
  Info '可用命令: .\memtune.bat apply auto'
}

function Cmd-Plan($t) { ResolveProfile $t; ShowPlan }

function Cmd-Apply($t) {
  $cmd = Get-ComposeCmd
  Require-ComposeFile
  ResolveProfile $t; ShowPlan
  WriteOverride; WriteState
  Info "已生成 $Override，正在重建服务以应用新配额..."
  Push-Location $ProjectDir
  try { Invoke-Expression "$cmd $(ComposeArgs) up -d" } finally { Pop-Location }
  Info '应用完成。运行 .\memtune.bat status 查看实际占用。'
}

function Cmd-Update {
  $cmd = Get-ComposeCmd
  $h = LoadState
  Step '运行时热更新内存/CPU 上限（不重启容器）...'
  $ok = 0; $fail = 0
  foreach ($svc in @('REDIS','MONGO','API','RPC','WEB','WORKER')) {
    $container = $h["${svc}_CONTAINER"]
    $limit = $h["${svc}_LIMIT_MB"]
    $cpu = $h["${svc}_CPU"]
    try {
      docker update --memory "$($limit)M" --cpus $cpu $container 2>$null
      Info "  $container -> mem=$(MbToCompose $limit) cpu=$cpu"; $ok++
    } catch {
      Warn "  $container 热更新失败（cgroup 可能不支持，需重建: .\memtune.bat apply）"; $fail++
    }
  }
  Info "热更新完成: 成功 $ok / 失败 $fail。Go 服务将按新 cgroup 自动调整 GOMAXPROCS/GOMEMLIMIT。"
}

function Cmd-Status {
  $cmd = Get-ComposeCmd
  Step 'CSCAN 内存状态'
  if (Test-Path $State) {
    $h = LoadState
    Info "当前画像: $($h['PROFILE']) ($($h['SOURCE'])) | 宿主机 $($h['HOST_MB'])MB / $($h['HOST_CPUS'])CPU"
  } else { Warn '未检测到 memtune 状态文件，显示 compose 默认上限。' }
  foreach ($c in @('cscan_redis','cscan_mongodb','cscan_api','cscan_rpc','cscan_web','cscan_worker')) {
    try {
      $lim = docker inspect -f '{{.HostConfig.Memory}}' $c 2>$null
      if (-not $lim -or $lim -eq '0') { continue }
      $stat = docker stats --no-stream --format '{{.MemUsage}}|{{.MemPerc}}' $c 2>$null
      $parts = $stat -split '\|'
      $limMb = [int]($lim / 1024 / 1024)
      '{0,-14}{1,-10}{2,-14}{3,-8}' -f $c, (MbToCompose $limMb), $parts[0], $parts[1]
    } catch {}
  }
}

function Cmd-Monitor($args) {
  $cmd = Get-ComposeCmd
  $once = $false; $interval = 30; $threshold = 85; $autoprotect = $false
  foreach ($a in $args) {
    if ($a -eq '--once') { $once = $true }
    elseif ($a -like '--interval=*') { $interval = $a.Split('=')[1] }
    elseif ($a -like '--threshold=*') { $threshold = $a.Split('=')[1] }
    elseif ($a -eq '--auto-protect') { $autoprotect = $true }
  }
  $h = LoadState
  Info "开始监控 (阈值=${threshold}%$(if($autoprotect){', 自动保护开启'}))。Ctrl+C 退出。"
  while ($true) {
    $ts = Get-Date -Format 'HH:mm:ss'
    foreach ($svc in @('REDIS','MONGO','API','RPC','WEB','WORKER')) {
      $container = $h["${svc}_CONTAINER"]
      $limit = $h["${svc}_LIMIT_MB"]
      $raw = docker stats --no-stream --format '{{.MemPerc}}' $container 2>$null
      $pct = [int]($raw -replace '%','')
      if ($pct -ge $threshold) {
        Err "[$ts] 超限 $container : $raw (上限 $(MbToCompose $limit))"
        if ($autoprotect -and $svc -eq 'WORKER') { Protect-Worker $h }
      } else {
        Info "[$ts] OK   $container : $raw"
      }
    }
    if ($once) { break }
    Start-Sleep -Seconds $interval
  }
}

function Protect-Worker($h) {
  $cur = [int]$h['WORKER_CONCURRENCY']
  if ($cur -le 1) {
    Warn '  Worker 并发已为最小值 1，尝试重启 Worker 释放内存。'
    Push-Location $ProjectDir; try { Invoke-Expression "$(Get-ComposeCmd) restart cscan_worker" } finally { Pop-Location }
    return
  }
  $new = $cur - 1
  Warn "  自动保护: 降低 Worker 并发 $cur -> $new 并重建 Worker。"
  (Get-Content $State) -replace '^WORKER_CONCURRENCY=.*', "WORKER_CONCURRENCY=$new" | Set-Content $State
  $h['WORKER_CONCURRENCY'] = $new
  WriteOverrideFromState $h
  Push-Location $ProjectDir; try { Invoke-Expression "$(Get-ComposeCmd) $(ComposeArgs) up -d cscan_worker" } finally { Pop-Location }
  Info "  Worker 并发已调整为 $new。"
}

function WriteOverrideFromState($h) {
  $P2 = @{
    redis_lim=[int]$h['REDIS_LIMIT_MB']; redis_res=[int]$h['REDIS_RES_MB']; redis_cpu=$h['REDIS_CPU'];
    mongo_lim=[int]$h['MONGO_LIMIT_MB']; mongo_res=[int]$h['MONGO_RES_MB']; mongo_cpu=$h['MONGO_CPU'];
    api_lim=[int]$h['API_LIMIT_MB']; api_res=[int]$h['API_RES_MB']; api_cpu=$h['API_CPU'];
    rpc_lim=[int]$h['RPC_LIMIT_MB']; rpc_res=[int]$h['RPC_RES_MB']; rpc_cpu=$h['RPC_CPU'];
    web_lim=[int]$h['WEB_LIMIT_MB']; web_res=[int]$h['WEB_RES_MB']; web_cpu=$h['WEB_CPU'];
    worker_lim=[int]$h['WORKER_LIMIT_MB']; worker_res=[int]$h['WORKER_RES_MB']; worker_cpu=$h['WORKER_CPU']; worker_conc=$h['WORKER_CONCURRENCY']
  }
  $global:ProfileName = $h['PROFILE']; $global:ProfileSource = $h['SOURCE']
  $global:HostMb = $h['HOST_MB']; $global:HostCpus = $h['HOST_CPUS']
  $P = $P2
  WriteOverride
}

function Cmd-Reset {
  $cmd = Get-ComposeCmd
  Require-ComposeFile
  if (Test-Path $Override) { Remove-Item $Override; Info "已删除 $Override" }
  if (Test-Path $State) { Remove-Item $State; Info "已删除 $State" }
  Info '正在以 compose 默认上限重建服务...'
  Push-Location $ProjectDir; try { Invoke-Expression "$cmd -f docker-compose.yaml up -d" } finally { Pop-Location }
  Info '已回退到默认内存上限。'
}

# ---------- 入口 ----------
switch ($args[0]) {
  'detect' { Cmd-Detect }
  'plan'   { Cmd-Plan $args[1] }
  'apply'  { Cmd-Apply $args[1] }
  'update' { Cmd-Update }
  'status' { Cmd-Status }
  'monitor'{ Cmd-Monitor $args[1..($args.Length-1)] }
  'reset'  { Cmd-Reset }
  default {
    Write-Host 'CSCAN 内存自适应调节工具 memtune.ps1'
    Write-Host ''
    Write-Host '用法:'
    Write-Host '  detect                                          检测宿主机并给出推荐画像'
    Write-Host '  plan [auto|tiny|small|medium|large|xlarge|<RAM>] 预览配额(不落盘)'
    Write-Host '  apply [auto|tiny|small|medium|large|xlarge|<RAM>] 生成 override 并重建'
    Write-Host '  update                                          运行时热更新内存/CPU上限(不重启)'
    Write-Host '  monitor [--once] [--interval=N] [--threshold=N] [--auto-protect]'
    Write-Host '  status                                          查看当前配额与实际占用'
    Write-Host '  reset                                           移除 override, 回退默认上限'
  }
}
