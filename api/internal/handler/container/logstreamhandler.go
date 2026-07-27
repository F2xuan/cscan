package container

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// containerLogStreamHandler SSE 推送实时容器日志
// GET /api/v1/container/logs/stream?name=xxx&token=xxx&tail=1000&since=...
// EventSource 不能设置 Authorization 头,token 通过查询字符串传递,与终端 WS 路由保持一致
func containerLogStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.DockerService == nil {
			writeSSEError(w, "docker service unavailable")
			return
		}
		name := r.URL.Query().Get("name")
		if name == "" {
			writeSSEError(w, "name is required")
			return
		}
		tail := r.URL.Query().Get("tail")
		if tail == "" {
			tail = "1000"
		}
		since := r.URL.Query().Get("since")

		// SSE 响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeSSEError(w, "streaming unsupported")
			return
		}
		flusher.Flush()

		// 所有对 w 的写入串行化,避免心跳协程与主循环并发写
		var writeMu sync.Mutex
		writeSSE := func(prefix, body string) {
			writeMu.Lock()
			_, _ = fmt.Fprintf(w, "%s%s\n\n", prefix, body)
			flusher.Flush()
			writeMu.Unlock()
		}

		// 心跳: 每 15s 推送一个注释行,防止中间代理断开空闲连接
		heartbeatCtx, heartbeatCancel := context.WithCancel(r.Context())
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer func() {
				ticker.Stop()
				close(done)
			}()
			for {
				select {
				case <-heartbeatCtx.Done():
					return
				case <-ticker.C:
					writeSSE(":heartbeat\n", "")
				}
			}
		}()

		// 主循环结束后停止心跳并等待协程退出,避免协程在 handler 返回后仍写 w
		defer func() {
			heartbeatCancel()
			<-done
		}()

		onLine := func(line svc.ContainerLogLine) {
			data, _ := json.Marshal(types.ContainerLogLine{
				Stream: line.Stream,
				TS:     line.TS,
				Line:   line.Line,
			})
			writeSSE("data: ", string(data))
		}
		onEnd := func(reason string) {
			writeSSE("event: end\ndata: ", string(mustJSON(reason)))
		}

		err := svcCtx.DockerService.StreamLogs(r.Context(), name, tail, since, onLine, onEnd)
		if err != nil && err != context.Canceled {
			logx.Errorf("[ContainerLogsStream] name=%s err=%v", name, err)
			writeSSE("event: error\ndata: ", string(mustJSON("docker logs stream failed")))
		}
	}
}

// ContainerLogStreamHandler 带认证的 SSE 入口(JWT + Admin)
func ContainerLogStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				token = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if token == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		claims, err := middleware.ValidateJWTToken(token, svcCtx.Config.Auth.AccessSecret)
		if err != nil {
			logx.Errorf("[ContainerLogsStream] invalid token: %v", err)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		role, _ := claims["role"].(string)
		if role != "admin" && role != "superadmin" {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		containerLogStreamHandler(svcCtx).ServeHTTP(w, r)
	}
}

// containerMergedLogStreamHandler 合并多个容器日志到同一个 SSE 流
// GET /api/v1/container/logs/stream/merged?names=container1,container2&token=xxx&tail=1000&since=...
func containerMergedLogStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.DockerService == nil {
			writeSSEError(w, "docker service unavailable")
			return
		}
		namesParam := r.URL.Query().Get("names")
		if namesParam == "" {
			writeSSEError(w, "names is required")
			return
		}
		names := strings.Split(namesParam, ",")
		// 去空
		filtered := make([]string, 0, len(names))
		for _, n := range names {
			if n = strings.TrimSpace(n); n != "" {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) == 0 {
			writeSSEError(w, "no valid container names")
			return
		}

		tail := r.URL.Query().Get("tail")
		if tail == "" {
			tail = "1000"
		}
		since := r.URL.Query().Get("since")

		// SSE 响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeSSEError(w, "streaming unsupported")
			return
		}
		flusher.Flush()

		var writeMu sync.Mutex
		writeSSE := func(prefix, body string) {
			writeMu.Lock()
			_, _ = fmt.Fprintf(w, "%s%s\n\n", prefix, body)
			flusher.Flush()
			writeMu.Unlock()
		}

		// 心跳
		heartbeatCtx, heartbeatCancel := context.WithCancel(r.Context())
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer func() {
				ticker.Stop()
				close(done)
			}()
			for {
				select {
				case <-heartbeatCtx.Done():
					return
				case <-ticker.C:
					writeSSE(":heartbeat\n", "")
				}
			}
		}()

		defer func() {
			heartbeatCancel()
			<-done
		}()

		// 日志行 channel
		type taggedLine struct {
			line      svc.ContainerLogLine
			container string
		}
		lineCh := make(chan taggedLine, 256)

		// 为每个容器启动独立的 StreamLogs goroutine
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		var wg sync.WaitGroup
		for _, name := range filtered {
			wg.Add(1)
			go func(containerName string) {
				defer wg.Done()
				onLine := func(line svc.ContainerLogLine) {
					select {
					case lineCh <- taggedLine{line: line, container: containerName}:
					case <-ctx.Done():
					}
				}
				onEnd := func(reason string) {
					// 单个容器结束不关闭整个流
					data, _ := json.Marshal(map[string]string{
						"event":     "container_end",
						"container": containerName,
						"reason":    reason,
					})
					writeSSE("event: container_end\ndata: ", string(data))
				}
				if err := svcCtx.DockerService.StreamLogs(ctx, containerName, tail, since, onLine, onEnd); err != nil && err != context.Canceled {
					logx.Errorf("[MergedLogsStream] container=%s err=%v", containerName, err)
				}
			}(name)
		}

		// 等待所有容器结束的 goroutine，关闭 lineCh
		go func() {
			wg.Wait()
			close(lineCh)
		}()

		// 主循环：从 lineCh 读取并推送到 SSE
		for tagged := range lineCh {
			data, _ := json.Marshal(types.ContainerLogLine{
				Stream:    tagged.line.Stream,
				TS:        tagged.line.TS,
				Line:      tagged.line.Line,
				Container: tagged.container,
			})
			writeSSE("data: ", string(data))
		}

		// 所有容器已结束
		writeSSE("event: end\ndata: ", string(mustJSON("all containers stopped")))
	}
}

// ContainerMergedLogStreamHandler 带认证的合并流 SSE 入口
func ContainerMergedLogStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				token = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if token == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		claims, err := middleware.ValidateJWTToken(token, svcCtx.Config.Auth.AccessSecret)
		if err != nil {
			logx.Errorf("[MergedLogsStream] invalid token: %v", err)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		role, _ := claims["role"].(string)
		if role != "admin" && role != "superadmin" {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		containerMergedLogStreamHandler(svcCtx).ServeHTTP(w, r)
	}
}

func writeSSEError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", mustJSON(msg))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func mustJSON(v string) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`"json error"`)
	}
	return b
}
