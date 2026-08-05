<div align="center">
  <img src="images/logo.png" width="80" alt="CSCAN" />
</div>

<div align="center">

**CSCAN - Enterprise Distributed Network Asset Scanning Platform**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-4.5-green)](VERSION)

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
- **Workspace Isolation** - Data filtered by workspace_id, organization/team dimension management
- **Notification Subscription** - Real-time scan result push (DingTalk/Feishu/WeCom/Email/Webhook)

---

## Project Structure

```
cscan/
├── api/              # HTTP API service (go-zero REST :8888)
├── rpc/task/         # gRPC internal service (:9000)
├── worker/           # Distributed worker (WebSocket + 8-phase scan pipeline)
├── internal/         # Internal shared modules
│   ├── model/        # MongoDB models (global collections + workspace_id filter)
│   ├── scanner/      # Scan engines (Naabu/Nmap/Httpx/Nuclei/FFuf/Chromedp)
│   ├── scheduler/    # Task scheduler (Redis queue/chunking/cron/recovery/reverify)
│   └── onlineapi/    # Online asset search (FOFA/Hunter/Quake)
├── pkg/              # Shared packages (xerr/response/executor/notify)
├── rules/            # Default rules & dictionaries (blacklist/fingerprint/HTTP mapping/scan template/weakpass)
├── web/              # Vue 3 frontend (:7777)
├── docker/           # Docker build configs
├── scripts/          # Dev scripts
└── docker-compose*.yaml
```

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

## Resource Allocation (Optional)

Default quotas work out of the box (API/RPC 1G, Worker 2G, Mongo 2G, Redis 512M, Web 512M) — no manual setup needed:
Worker concurrency is auto-derived from the container memory limit (`worker/main.go`, 384MB/tab × 0.6) and then
dynamically throttled at runtime by CPU/memory pressure, so low-spec hosts avoid OOM while high-spec hosts are fully utilized.

To adjust, inject variables in `.env` (full list of overridable variables in `docker-compose.yaml` comments):

```bash
# .env example
CSCAN_WORKER_MEM_LIMIT=3G        # Worker memory limit (default 2G; concurrency derives from it)
CSCAN_WORKER_CPU_LIMIT=4         # Worker CPU limit (default 2)
CSCAN_MONGO_MEM_LIMIT=1G         # Mongo WiredTiger cache limit (default 2G; lower it on small hosts)
CSCAN_API_MEM_LIMIT=768M         # API memory limit (default 1G)
CSCAN_WORKER_CONCURRENCY=4       # Explicit Worker concurrency (default: auto-derived from memory)
```

For standalone Worker probe nodes, download `http://<server>:8888/static/worker-tune.sh`
(`worker-tune.ps1` on Windows) after deploying to the target host — it detects the host specs
and generates an override to match them, then starts the Worker.

---

## Local Development

### One-Click Startup (Recommended)

The full local stack (MongoDB + Redis + RPC + API + Web + Worker) runs as containers built from your local code via `docker-compose.dev.yaml`:

```bash
# Windows PowerShell:
./scripts/dev.ps1
# Linux/macOS / Git Bash:
./scripts/dev.sh
```

Access `http://localhost:7777`, default account `admin / 123456`.

> - The Worker starts with the stack automatically, using a built-in default key (override with `CSCAN_WORKER_KEY` env) — no manual key configuration needed;
> - Re-run the script after code changes to rebuild the affected images;
> - View logs: `docker-compose -f docker-compose.dev.yaml logs -f`; stop: `docker-compose -f docker-compose.dev.yaml down`;
> - Debug a single service (e.g. only the API): `docker-compose -f docker-compose.dev.yaml up -d --build cscan-api`.
---

## License

MIT
<img src="images/wechat.jpg">