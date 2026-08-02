<div align="center">
  <img src="images/logo.png" width="80" alt="CSCAN" />
</div>

<div align="center">

**CSCAN - Enterprise Distributed Network Asset Scanning Platform**

[![Go](https://img.shields.io/badge/Go-1.25.7-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-4.3-green)](VERSION)

[中文](README.md) · [English](README_EN.md)

</div>

---

<table width="100%">
  <tr>
    <td align="center"><b>Dashboard</b></td>
    <td align="center"><b>Asset Search</b></td>
    <td align="center"><b>Fingerprint</b></td>
    <td align="center"><b>Vulnerability</b></td>
    <td align="center"><b>Node Monitor</b></td>
    <td align="center"><b>Notification</b></td>
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

## Features
### Core Capabilities

- **Distributed Architecture** - Master/Worker separation, multi-node elastic scaling
- **Pipeline Orchestration** - Scan phases automatically chained, results passed to subsequent phases
- **Weak Password Dictionary Management** - Built-in default dictionaries, custom dict CRUD, import/export
- **Cron Tasks** - Cron expression-driven periodic scanning tasks
- **Asset Grouping** - Auto-aggregate assets by domain, real-time task status reflection
- **Multi-Workspace** - Tenant-level data isolation, organization/team dimension management
- **Notification Subscription** - Real-time scan result push (DingTalk/Feishu/WeCom/Email/Webhook)

---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/tangxiaofeng7/cscan.git
cd cscan

# Linux/macOS: run via bash
bash cscan.sh

# Windows
.\cscan.bat
```

> Access `https://ip:7777`, default account `admin / 123456`
---

## Local Development

```bash
# 1. Start dependencies
docker-compose -f docker-compose.dev.yaml up -d

# 2. Start services
go run rpc/task/task.go -f rpc/task/etc/task.yaml


# 3. Dev mode bypass (auto-generates random secret, local debug only)
# Windows powershell
$env:CSCAN_DEV=1
# Windows cmd
# set CSCAN_DEV=1
# linux & mac
# export CSCAN_DEV=1
go run api/cscan.go -f api/etc/cscan.yaml

# 4. Start frontend
cd web ; npm install ; npm run dev

# 5. Start Worker
go run cmd/worker/main.go -k <install_key> -s http://localhost:8888
```
---

## License

MIT
<img src="images/wechat.jpg">