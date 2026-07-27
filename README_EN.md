<div align="center">
  <img src="images/logo.png" width="80" alt="CSCAN" />
</div>


<div align="center">

**CSCAN-Enterprise Distributed Network Asset Scanning Platform**

[![Go](https://img.shields.io/badge/Go-1.25.7-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-3.5-green)](VERSION)

[中文](README.md) · [English](README_EN.md)

</div>

---

| Dashboard | Asset Filter | Fingerprint | Vulnerability | Nodes Monitor | Notification |
|:---:|:---:|:---:|:---:|:---:|:---:|
| <img src="images/dashboard.png" width="200"> | <img src="images/filter.png" width="200"> | <img src="images/finger.png" width="200"> | <img src="images/poc.png" width="200"> | <img src="images/worker.png" width="200"> | <img src="images/notice.png" width="200"> |

---

## Features
### Core Capabilities

- **Distributed Architecture** - Master/Worker separation, multi-node elastic scaling
- **Pipeline Orchestration** - Scan phases automatically chained, results passed to subsequent phases
- **Weakpass Dict Management** - Built-in default dictionaries, custom dict CRUD, import/export
- **Cron Tasks** - Cron expression-driven periodic scanning tasks
- **Asset Grouping** - Auto-aggregate assets by domain, real-time task status reflection
- **Multi-Workspace** - Tenant-level data isolation, organization/team dimension management
- **Notification** - Real-time scan result push (DingTalk/Feishu/WeCom/Email/Webhook)

---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/tangxiaofeng7/cscan.git
cd cscan

# Linux/macOS
chmod +x cscan.sh && ./cscan.sh

# Windows
.\cscan.bat
```

> Access `https://ip:7777`, default account `admin / 123456`
>
> Worker nodes must be deployed before executing scans

---

## Local Development

```bash
# 1. Start dependencies
docker-compose -f docker-compose.dev.yaml up -d

# 2. Set JWT secret environment variable (required, otherwise API refuses to start)
export CSCAN_JWT_SECRET="$(openssl rand -hex 32)"
# Or use dev mode bypass (auto-generates random secret, local debug only)
# export CSCAN_DEV=1

# 3. Start services
go run rpc/task/task.go -f rpc/task/etc/task.yaml
go run api/cscan.go -f api/etc/cscan.yaml

# 4. Start frontend
cd web ; npm install ; npm run dev

# 5. Start Worker
go run cmd/worker/main.go -k <install_key> -s http://localhost:8888
```

---

## Worker Deployment

```bash
# Linux
./cscan-worker -k <install_key> -s http://<api_host>:8888

# Windows
cscan-worker.exe -k <install_key> -s http://<api_host>:8888
```

---

## License

MIT
