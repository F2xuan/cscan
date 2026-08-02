<div align="center">
  <img src="images/logo.png" width="80" alt="CSCAN" />
</div>

<div align="center">

**CSCAN-企业级分布式网络资产扫描平台**

[![Go](https://img.shields.io/badge/Go-1.25.7-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-4.2-green)](VERSION)

[中文](README.md) · [English](README_EN.md)

</div>

---

<table width="100%">
  <tr>
    <td align="center"><b>控制台</b></td>
    <td align="center"><b>资产检索</b></td>
    <td align="center"><b>指纹管理</b></td>
    <td align="center"><b>漏洞库</b></td>
    <td align="center"><b>节点监控</b></td>
    <td align="center"><b>通知订阅</b></td>
  </tr>
  <tr>
    <td align="center"><img src="images/dashboard.png"></td>
    <td align="center"><img src="images/filter.png"></td>
    <td align="center"><img src="images/finger.png"></td>
    <td align="center"><img src="images/poc.png"></td>
    <td align="center"><img src="images/worker.png"></td>
    <td align="center"><img src="images/notice.png"></td>
  </tr>
</table>

---
## 功能特性
### 核心能力

- **分布式架构** - Master/Worker 分离，支持多节点弹性扩缩容
- **流水线编排** - 扫描阶段自动串联，前序结果自动传递给后续阶段
- **弱口令字典管理** - 内置默认字典，支持自定义字典增删改查、导入导出
- **定时任务** - Cron 表达式驱动的周期性扫描任务
- **资产分组** - 按域名自动聚合资产，实时反映任务状态
- **多工作空间** - 租户级数据隔离，支持组织/团队维度管理
- **通知订阅** - 扫描结果实时推送（钉钉/飞书/企业微信/邮件/Webhook）

---

## 快速开始

```bash
# 克隆项目
git clone https://github.com/tangxiaofeng7/cscan.git
cd cscan

# Linux/macOS：直接以 bash 运行即可
bash cscan.sh

# Windows
.\cscan.bat
```

> 访问 `https://ip:7777`，默认账号 `admin / 123456`
>
> 注：内存自动调优由 `cscan.sh` 内部调用 `scripts/memtune.sh` 完成（通过 `bash` 解释执行，**无需对 memtune.sh 执行 chmod +x**）。选择「1. 一键安装」或「6. 启动服务」时会自动按宿主机内存生成 `docker-compose.override.yml` 并应用。
---

## 内存自适应调优（自动资源配额）

CSCAN 已取消 docker-compose 中内存上限的硬编码，改为「可配置 + 按需自动计算」：

- **安装/启动自动调优**：首次执行 `./cscan.sh install`（或 `./cscan.sh start`）且项目下尚无 `docker-compose.override.yml` 时，会按宿主机物理内存自动生成资源配额并重建服务，无需手动干预。
- **手动调优（统一入口）**：`cscan.sh memtune <子命令>`，等价于 `scripts/memtune.sh`，但通过项目管理脚本统一调度，避免单独运行 `memtune.sh` 时的目录/路径冲突。

子命令一览：

| 命令 | 作用 |
| --- | --- |
| `cscan.sh memtune detect` | 检测宿主机内存并给出推荐画像 |
| `cscan.sh memtune plan [auto\|tiny\|small\|medium\|large\|xlarge\|<RAM>]` | 仅预览配额，不落盘 |
| `cscan.sh memtune apply [同上]` | 生成 `docker-compose.override.yml` 并重建服务 |
| `cscan.sh memtune update` | 运行时热更新容器内存/CPU 上限（`docker update`，无需重启） |
| `cscan.sh memtune monitor [--auto-protect]` | 轮询占用，超限告警，可选自动降低 Worker 并发保护 |
| `cscan.sh memtune status` | 查看当前配额与实际占用 |
| `cscan.sh memtune reset` | 移除 override，回退到 compose 默认上限 |

> 若需在 `.env` 中手动固定 Go 运行时参数，可设置 `CSCAN_API_GOMAXPROCS` / `CSCAN_API_GOMEMLIMIT`（rpc、worker 同理），优先级高于自动推导。

完整设计说明见 [`MEMORY_AUTOTUNE.md`](./MEMORY_AUTOTUNE.md)。

---

## 本地开发

```bash
# 1. 启动依赖
docker-compose -f docker-compose.dev.yaml up -d

# 2. 启动服务
go run rpc/task/task.go -f rpc/task/etc/task.yaml


# 3. 开发模式豁免（自动生成随机 secret，仅限本地调试）
# Windows powershell
$env:CSCAN_DEV=1
# Windows cmd
# set CSCAN_DEV=1
# linux & mac
# export CSCAN_DEV=1
go run api/cscan.go -f api/etc/cscan.yaml

# 4. 启动前端
cd web ; npm install ; npm run dev

# 5. 启动 Worker
go run cmd/worker/main.go -k <install_key> -s http://localhost:8888
```
---

## License

MIT
<img src="images/wechat.jpg">