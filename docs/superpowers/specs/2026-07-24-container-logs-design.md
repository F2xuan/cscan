# Container Logs Viewer — Design Spec

**Date:** 2026-07-24
**Route:** `http://localhost:7777/worker-logs` (page replaced)
**Goal:** Show Docker container logs for all cscan services (api/rpc/web/worker/redis/mongodb) on one page.

## 1. Context

Existing `/worker-logs` shows Worker-daemon runtime logs pushed over WebSocket. The user wants to replace it with a unified view of **Docker container logs** pulled from the host Docker daemon. Redis and MongoDB do not emit log files, so `docker logs` is the only uniform source.

Currently only `cscan_mongodb_dev` and `cscan_redis_dev` containers are running locally. Full stack runs via `docker-compose.yaml` (api/rpc/web/worker/mongodb/redis).

## 2. Decisions (confirmed with user)

| Decision | Choice |
|----------|--------|
| Log source | Docker container real-time logs, not persisted |
| Page integration | Replace existing `/worker-logs` page |
| Container scope | Auto-discover cscan-related containers |
| Backend collection | Docker SDK + `client.ContainerLogs` with `Follow:true` |
| Delivery channel | SSE (Server-Sent Events) |
| Frontend interaction | Single container selection (left list, right stream) |
| Auth | JWT + Admin (reuse `ConsoleAuthMiddleware`), SSE supports `?token=` query |

## 3. Architecture

```
[WorkerLogs.vue]
   │
   ├─ GET  /api/v1/container/list           → container list (POST admin)
   └─ EventSource /api/v1/container/logs/stream?name=...&token=...&tail=1000
                │
                ▼
        [ContainerLogStreamHandler]
                │
                ▼
        [Docker SDK client]
                │
                ▼
        docker.sock / npipe
                │
                ▼
        Target container stdout/stderr  (follow)
```

Each browser tab opens one SSE per active container. Switching containers closes the old SSE and opens a new one. No DB storage — logs are only kept in memory in the browser (capped at 2000 lines).

## 4. Backend

### 4.1 New dependency

`github.com/docker/docker/client` (and transitively `github.com/docker/docker/api/types`).

Initialize a single `*client.Client` in `ServiceContext` via `client.NewClientWithOpts(client.FromEnv)`. Default socket resolution:
- Linux: `unix:///var/run/docker.sock`
- Windows: `npipe:////./pipe/docker_engine`
- Honors `DOCKER_HOST` env var when set.

### 4.2 Config

Add to `api/etc/cscan.yaml`:

```yaml
Docker:
  Host: ""                              # optional; empty = use DOCKER_HOST env or default
  ContainerPrefix: "cscan"              # name starts with cscan- or cscan_
  ImageRegistry: "registry.cn-hangzhou.aliyuncs.com/txf7/"
  ExtraNames:
    - cscan_mongodb_dev
    - cscan_redis_dev
```

Add `DockerConfig` struct to `api/internal/config/config.go` and `Docker DockerConfig \`json:",optional"\`` to `Config`.

### 4.3 ServiceContext

Add `DockerClient *client.Client` field. Initialize in `NewServiceContext`. On error, log warning and leave nil — handlers return `503` when Docker is unavailable.

Introduce a small wrapper `DockerService` in `api/internal/svc/docker_service.go`:

```go
type DockerService struct {
    cli    *client.Client
    prefix string
    registry string
    extraNames map[string]struct{}
}

func (s *DockerService) ListCscanContainers(ctx context.Context) ([]ContainerInfo, error)
```

### 4.4 Routes

Register under console-auth block (JWT + Admin), extending the pattern at `routes.go:444-477`:

- `POST /api/v1/container/list` — list containers
- `GET  /api/v1/container/logs/stream` — SSE stream (token in query; no Authorization header because EventSource cannot set one)

SSE handler authenticates by reusing `validateJWTToken` (already in `worker/terminalhandler.go:703`). After token validation, manually enforce `role == admin || superadmin` (same logic as `WorkerTerminalWSHandlerWithAuth`).

### 4.5 Handlers

`api/internal/handler/container/`:
- `listhandler.go` — `ContainerListHandler(svcCtx)`
- `logstreamhandler.go` — `ContainerLogStreamHandler(svcCtx)`

Logic files (go-zero convention):
- `api/internal/logic/containerlistlogic.go`
- `api/internal/logic/containerlogstreamlogic.go`

### 4.6 SSE protocol

Response headers:
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

Event format:
```
data: {"ts":"2026-07-24T10:11:12.345Z","stream":"stdout","line":"..."}\n
\n
```

Terminal sentinel on exit:
```
event: end
data: {"reason":"container stopped or client disconnected"}\n
\n
```

Error event:
```
event: error
data: {"msg":"..."}\n
\n
```

### 4.7 Log reader

```go
reader, err := cli.ContainerLogs(ctx, name, types.ContainerLogsOptions{
    ShowStdout: true,
    ShowStderr: true,
    Follow:     true,
    Tail:       tail,       // "1000" default
    Since:      since,      // optional RFC3339
    Timestamps: true,       // docker prepends RFC3339 ts
})
```

Docker's streamed log format prefixes each 8-byte header (`streamType byte` + 7 bytes big-endian length) before the payload. For `Follow:true` we must use `stdcopy.StdCopy` to demultiplex stdout/stderr. Parse the leading RFC3339 timestamp (docker's `Timestamps:true` format is `2026-07-24T10:11:12.345678901Z `) and emit it as `ts`.

Read loop:
1. `bufio.Scanner` over `stdcopy.NewReader` at 1 MiB buffer
2. For each line: strip the leading docker timestamp, parse stream type, emit SSE `data:` line
3. Stop when `ctx.Done()` fires (client disconnected) or reader returns `io.EOF` (container stopped → emit `event: end`)

Must flush after each event: use `http.Flusher`. Set `r.Context()` as the cancellation source so closing the browser kills the docker follow goroutine.

### 4.8 Container discovery

`ContainerListOptions{All: true}` then filter by:
- Name matches `^cscan[-_]` OR
- Image matches `^<ImageRegistry>` OR
- Name in `ExtraNames`

Return shape:
```json
{
  "code": 0, "msg": "success",
  "data": {
    "list": [
      {"name":"cscan-api","image":"...","state":"running","status":"Up 9 hours","ports":[...]},
      ...
    ]
  }
}
```

### 4.9 Fallback (non-SSE consumers)

Add `POST /api/v1/container/logs/fetch` returning last N lines as JSON; used by export and when SSE is blocked. Implementation reuses `ContainerLogsOptions{Follow:false, Tail:N, Timestamps:true}` then parses with the same stdcopy split.

### 4.10 Export

Reuse existing `/worker/logs/export` pattern: client sends desired `format` (txt/json/csv) and server returns a blob of the current in-memory snapshot would require persisting. Instead, simplify: server fetches last `limit` lines (e.g. 5000) via `logs/fetch` and returns as a blob; no streaming export.

## 5. Frontend

### 5.1 Replacement of `web/src/views/WorkerLogs.vue`

Keep route `/worker-logs`. Replace the template and script:

**Layout** (two-column inside the card):
- Left column (240px): container list
  - `el-menu` vertical, each item shows a status dot (green=running, gray=exited), container name, state text
  - Refresh button — re-fetch `/container/list`
- Right column: log viewer
  - Top filter bar: search box, stream filter (stdout/stderr/all), pause/resume button, clear button, export dropdown
  - Stats row: container name, streamed lines count, connection state
  - Log area: monospace, auto-scroll toggle, 2000-line ring buffer
  - Empty state when no container selected

### 5.2 SSE client

```js
function openStream(containerName) {
  if (eventSource) eventSource.close()
  const token = userStore.token
  const url = `${baseURL}/container/logs/stream?name=${encodeURIComponent(containerName)}&token=${encodeURIComponent(token)}&tail=1000`
  eventSource = new EventSource(url)
  eventSource.onmessage = (e) => { pushLog(JSON.parse(e.data)) }
  eventSource.addEventListener('end', () => eventSource.close())
  eventSource.addEventListener('error', () => { /* reconnect with backoff */ })
  eventSource.onerror = () => { /* mark disconnected */ }
}
```

On unmount or container switch: call `eventSource.close()`.

### 5.3 New API module

`web/src/api/container.js`:
```js
import request from '@/api/request'
export const listContainers = () => request.post('/container/list')
export const fetchContainerLogs = (data) => request.post('/container/logs/fetch', data)
```

### 5.4 i18n

Add keys under `container.*` in `web/src/i18n/locales/zh-CN.js` and `en-US.js` (names, streams, pause/resume, clear, export, empty states, connection state).

### 5.5 Existing worker runtime logs

Per user choice "replace existing page": the `/worker/logs/history`、`/worker/logs/clear`、`/worker/logs/export`、`/worker/logs/stream` endpoints remain in the backend (still used by the worker daemon) but are no longer called from the UI. The Worker's own log upload behavior is unchanged.

## 6. Files

### New (backend)
- `api/internal/handler/container/containerhandler.go` (or split two files)
- `api/internal/handler/container/listhandler.go`
- `api/internal/handler/container/logstreamhandler.go`
- `api/internal/logic/containerlistlogic.go`
- `api/internal/logic/containerlogstreamlogic.go`
- `api/internal/svc/docker_service.go`
- `api/internal/types/container.go` (types, or append to `types.go`)

### Modified (backend)
- `api/internal/handler/routes.go` — register new routes under console-auth block
- `api/internal/svc/servicecontext.go` — add DockerClient + DockerService
- `api/internal/config/config.go` — DockerConfig struct
- `api/etc/cscan.yaml` — Docker config block
- `go.mod` / `go.sum` — add `github.com/docker/docker`

### New (frontend)
- `web/src/api/container.js`

### Modified (frontend)
- `web/src/views/WorkerLogs.vue` — full rewrite
- `web/src/i18n/locales/zh-CN.js`
- `web/src/i18n/locales/en-US.js`

## 7. Security & Operations

- **Socket exposure**: API container must mount `/var/run/docker.sock:/var/run/docker.sock` (Linux) or `//./pipe/docker_engine://./pipe/docker_engine` (Windows). Document in README.
- **Auth**: Admin-only. SSE cannot send `Authorization` header — token in query string. Tokens are short-lived JWTs (24h). Log URIs are not cached by intermediaries (`Cache-Control: no-store` plus the existing `X-Accel-Buffering: no`).
- **Audit**: Existing `ConsoleAuthMiddleware` already records all accesses to the Redis audit stream; new routes inherit that.
- **Resource**: One active follow per browser; closing tab cancels. No goroutine leak because reader respects `ctx.Done()`.
- **Sensitive data**: Container logs may contain secrets if the app prints them. Same risk as `docker logs` on the host; acceptable for admin-only role.

## 8. Testing

- **Unit**: `docker_service.go` container filter — table test with sample `types.Container` slice
- **Unit**: SSE line parser — feed a `stdcopy` payload, assert JSON events
- **Manual**: Run `docker-compose.dev.yaml` (mongodb + redis), open page, select each container, verify live log output. Stop a container and verify `event: end` arrives.
- No integration tests against a live Docker daemon (CI lacks Docker).

## 9. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Windows `npipe` path | Docker SDK resolves automatically; document in README if user runs API outside Docker |
| Docker socket not mounted | `DockerClient == nil` → handler returns 503 with explanatory msg |
| Huge log bursts overrun buffer | 1 MiB scanner buffer, client-side 2000-line ring buffer |
| EventSource auto-reconnect duplicates tail | Send `since` = last seen docker timestamp on reconnect |
| Token in URL exposed in nginx access logs | Already the case for `/worker/console/terminal` — acceptable |
| Docker daemon version mismatch | SDK uses `client.FromEnv` + `api-version` auto-negotiation |

## 10. Out of Scope

- Multi-container merged stream (could add later as a "follow all" toggle)
- Historical log search beyond `tail` (would require persisting to MongoDB)
- Worker runtime log page — fully replaced; if a user later wants both, add tabs.

