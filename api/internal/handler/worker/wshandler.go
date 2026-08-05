package worker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"cscan/api/internal/svc"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/zeromicro/go-zero/core/logx"
)

// ==================== WebSocket Message Types ====================

const (
	WSTypeAuth           = "AUTH"            // 认证请求
	WSTypeAuthOK         = "AUTH_OK"         // 认证成功
	WSTypeAuthFail       = "AUTH_FAIL"       // 认证失败
	WSTypePing           = "PING"            // 心跳请求
	WSTypePong           = "PONG"            // 心跳响应
	WSTypeLog            = "LOG"             // 日志消息
	WSTypeLogBatch       = "LOG_BATCH"       // 批量日志消息
	WSTypeControl        = "CONTROL"         // 控制信号
	WSTypeLogSyncReq     = "LOG_SYNC_REQ"    // API 请求 Worker 同步日志
	WSTypeLogSyncResp    = "LOG_SYNC_RESP"   // Worker 返回同步日志数据
	WSTypeLogSyncAck     = "LOG_SYNC_ACK"    // API 确认日志已写入文件
)

// WSMessage WebSocket消息结构
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// AuthPayload 认证消息载荷
type AuthPayload struct {
	WorkerName string `json:"workerName"`
	InstallKey string `json:"installKey"`
}

// LogPayload 日志消息载荷
type LogPayload struct {
	TaskId    string `json:"taskId"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// LogBatchPayload 批量日志消息载荷
type LogBatchPayload struct {
	Logs []LogPayload `json:"logs"`
}

// LogSyncReqPayload 日志同步请求载荷（API → Worker）
type LogSyncReqPayload struct {
	Filename string `json:"filename"`
	Offset   int64  `json:"offset"`
}

// LogSyncRespPayload 日志同步响应载荷（Worker → API）
type LogSyncRespPayload struct {
	Filename  string                   `json:"filename"`
	Logs      []svc.WorkerLogEntry     `json:"logs"`
	NewOffset int64                    `json:"newOffset"`
	HasMore   bool                     `json:"hasMore"`
	NextFile  string                   `json:"nextFile"`
}

// LogSyncAckPayload 日志同步确认载荷（API → Worker）
type LogSyncAckPayload struct {
	Filename string `json:"filename"`
	Offset   int64  `json:"offset"`
}

// ControlPayload 控制信号载荷
type ControlPayload struct {
	TaskId string `json:"taskId"`
	Action string `json:"action"` // STOP, PAUSE, RESUME
}

// ==================== Worker Connection ====================

// WorkerConnection 单个Worker的WebSocket连接
type WorkerConnection struct {
	conn            net.Conn
	workerName      string
	svcCtx          *svc.ServiceContext
	sendChan        chan []byte
	closeChan       chan struct{}
	closeOnce       sync.Once
	lastPing        time.Time
	mu              sync.RWMutex

	// 日志同步游标（记录已成功写入文件的日志位置）
	syncCursorMu sync.Mutex
	syncCursor   LogSyncReqPayload // {filename, offset}
	syncRespChan chan *LogSyncRespPayload // 同步响应通道

	// 日志同步完成信号（供 TriggerLogSyncAndWait 等待本轮同步落盘，用于刷新时立即拉取最新日志）
	syncDone chan struct{}
}

// NewWorkerConnection 创建新的Worker连接
func NewWorkerConnection(conn net.Conn, workerName string, svcCtx *svc.ServiceContext) *WorkerConnection {
	return &WorkerConnection{
		conn:         conn,
		workerName:   workerName,
		svcCtx:       svcCtx,
		sendChan:     make(chan []byte, 256),
		closeChan:    make(chan struct{}),
		lastPing:     time.Now(),
		syncRespChan: make(chan *LogSyncRespPayload, 8),
		syncDone:     make(chan struct{}, 1),
	}
}

// GetWorkerName 获取Worker名称（并发安全）
func (wc *WorkerConnection) GetWorkerName() string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.workerName
}

// SetWorkerName 设置Worker名称（并发安全），供 RenameConnection 等场景使用
func (wc *WorkerConnection) SetWorkerName(name string) {
	wc.mu.Lock()
	wc.workerName = name
	wc.mu.Unlock()
}

// Send 发送消息到Worker
func (wc *WorkerConnection) Send(msg *WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case wc.sendChan <- data:
		return nil
	case <-wc.closeChan:
		return ErrConnectionClosed
	default:
		return ErrSendBufferFull
	}
}

// Close 关闭连接
func (wc *WorkerConnection) Close() {
	wc.closeOnce.Do(func() {
		close(wc.closeChan)
	})
}

// isClosed 检查连接是否已关闭（非阻塞）
func (wc *WorkerConnection) isClosed() bool {
	select {
	case <-wc.closeChan:
		return true
	default:
		return false
	}
}

// UpdateLastPing 更新最后心跳时间
func (wc *WorkerConnection) UpdateLastPing() {
	wc.mu.Lock()
	wc.lastPing = time.Now()
	wc.mu.Unlock()
}

// GetLastPing 获取最后心跳时间
func (wc *WorkerConnection) GetLastPing() time.Time {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.lastPing
}

// ==================== File Operation Methods ====================

// ==================== WebSocket Handler ====================

// WorkerWSHandler WebSocket处理器
type WorkerWSHandler struct {
	svcCtx         *svc.ServiceContext
	connections    sync.Map // workerName -> *WorkerConnection
}

// 错误定义
var (
	ErrConnectionClosed = &WSError{Code: 1000, Message: "connection closed"}
	ErrSendBufferFull   = &WSError{Code: 1001, Message: "send buffer full"}
	ErrAuthFailed       = &WSError{Code: 1002, Message: "authentication failed"}
	ErrInvalidMessage   = &WSError{Code: 1003, Message: "invalid message"}
)

type WSError struct {
	Code    int
	Message string
}

func (e *WSError) Error() string {
	return e.Message
}

// NewWorkerWSHandler 创建WebSocket处理器
func NewWorkerWSHandler(svcCtx *svc.ServiceContext) *WorkerWSHandler {
	h := &WorkerWSHandler{
		svcCtx: svcCtx,
	}

	// 启动 Worker 控制命令订阅
	go h.subscribeWorkerControl()

	return h
}

// subscribeWorkerControl 订阅 Worker 控制命令频道
// 修复 C-09：原实现 for msg := range ch 在 Redis 连接断开时 channel 关闭，
// goroutine 直接退出，后续所有 worker 控制命令（stop/restart/rename）永久失效。
// 现增加断线重连+指数退避。
func (h *WorkerWSHandler) subscribeWorkerControl() {
	ctx := context.Background()
	const maxBackoff = 30 * time.Second
	backoff := time.Second

	for {
		pubsub := h.svcCtx.RedisClient.Subscribe(ctx, "cscan:worker:control")
		ch := pubsub.Channel()

		// 等待订阅确认
		if _, err := pubsub.Receive(ctx); err != nil {
			logx.Errorf("[WorkerWS] Subscribe cscan:worker:control failed: %v, retry in %v", err, backoff)
			pubsub.Close()
			h.sleepCtx(ctx, backoff)
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}
		backoff = time.Second
		logx.Info("[WorkerWS] Subscribed to worker control channel")

	consumeLoop:
		for msg := range ch {
			if msg == nil {
				break consumeLoop
			}
			h.handleWorkerControlMessage(msg.Payload)
		}

		logx.Errorf("[WorkerWS] Worker control subscription disconnected, reconnecting in %v...", backoff)
		pubsub.Close()
		h.sleepCtx(ctx, backoff)
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

// handleWorkerControlMessage 处理单条 worker 控制命令
func (h *WorkerWSHandler) handleWorkerControlMessage(payloadStr string) {
	// 解析控制命令
	var cmd struct {
		Action      string `json:"action"`
		WorkerName  string `json:"workerName"`
		NewName     string `json:"newName,omitempty"`
		Concurrency int    `json:"concurrency,omitempty"`
	}
	if err := json.Unmarshal([]byte(payloadStr), &cmd); err != nil {
		logx.Errorf("[WorkerWS] Invalid control command: %v", err)
		return
	}

	logx.Infof("[WorkerWS] Received control command: action=%s, worker=%s", cmd.Action, cmd.WorkerName)

	// 获取 Worker 连接
	conn, ok := h.GetConnection(cmd.WorkerName)
	if !ok {
		logx.Infof("[WorkerWS] Worker %s not connected, skipping control command", cmd.WorkerName)
		return
	}

	// 构造并发送控制消息
	var payload []byte
	switch cmd.Action {
	case "stop":
		payload, _ = json.Marshal(map[string]interface{}{
			"action": "WORKER_STOP",
		})
	case "restart":
		payload, _ = json.Marshal(map[string]interface{}{
			"action": "WORKER_RESTART",
		})
	case "rename":
		payload, _ = json.Marshal(map[string]interface{}{
			"action":  "WORKER_RENAME",
			"newName": cmd.NewName,
		})
		// 同时更新服务端的连接映射
		if cmd.NewName != "" && cmd.NewName != cmd.WorkerName {
			h.RenameConnection(cmd.WorkerName, cmd.NewName)
		}
	case "setConcurrency":
		payload, _ = json.Marshal(map[string]interface{}{
			"action":      "WORKER_SET_CONCURRENCY",
			"concurrency": cmd.Concurrency,
		})
	default:
		logx.Infof("[WorkerWS] Unknown control action: %s", cmd.Action)
		return
	}

	// 发送控制消息给 Worker
	if err := conn.Send(&WSMessage{
		Type:    WSTypeControl,
		Payload: payload,
	}); err != nil {
		logx.Errorf("[WorkerWS] Failed to send control command to %s: %v", cmd.WorkerName, err)
	} else {
		logx.Infof("[WorkerWS] Sent control command to %s: %s", cmd.WorkerName, cmd.Action)
	}
}

// sleepCtx 可被 ctx 取消的 sleep
func (h *WorkerWSHandler) sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// nextBackoff 指数退避，上限为 max
func nextBackoff(cur, max time.Duration) time.Duration {
	next := cur * 2
	if next > max {
		next = max
	}
	return next
}

// GetConnection 获取Worker连接
func (h *WorkerWSHandler) GetConnection(workerName string) (*WorkerConnection, bool) {
	if conn, ok := h.connections.Load(workerName); ok {
		return conn.(*WorkerConnection), true
	}
	return nil, false
}

// RenameConnection 重命名Worker连接映射
// 当Worker被重命名时，需要同步更新WebSocket连接映射的key
func (h *WorkerWSHandler) RenameConnection(oldName, newName string) {
	if oldName == newName || newName == "" {
		return
	}

	// 获取旧连接
	if conn, ok := h.connections.Load(oldName); ok {
		workerConn := conn.(*WorkerConnection)
		// 更新连接的workerName（并发安全，可能正被 readPump/心跳/任务回调读取）
		workerConn.SetWorkerName(newName)
		// 存储到新key
		h.connections.Store(newName, workerConn)
		// 删除旧key
		h.connections.Delete(oldName)

		logx.Infof("[WorkerWS] Connection renamed: %s -> %s", oldName, newName)
	}
}

// ==================== HTTP Handler ====================

// WorkerWSEndpointHandler WebSocket端点处理器
// GET /api/v1/worker/ws
func WorkerWSEndpointHandler(svcCtx *svc.ServiceContext, wsHandler *WorkerWSHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 修复 C-26：升级前校验 Origin，防止 CSWSH
		// Worker 是非浏览器客户端，Origin 通常为空，validateWSOrigin 允许空 Origin
		if !validateWSOrigin(r) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		// 升级HTTP连接为WebSocket
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			logx.Errorf("[WorkerWS] Failed to upgrade connection: %v", err)
			return
		}

		// 创建连接上下文
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// 处理连接
		handleWebSocketConnection(ctx, conn, svcCtx, wsHandler)
	}
}

// validateWSOrigin 校验 WebSocket 升级请求的 Origin 头，防止 CSWSH 攻击
// 修复 C-26：原 UpgradeHTTP 调用未校验 Origin，浏览器发起的跨站请求可自动携带 cookie/token，
// 存在跨站 WebSocket 劫持风险。
// 策略：
//   - Origin 为空：允许（非浏览器客户端如 Worker、curl 不发送 Origin）
//   - Origin 存在：必须与请求 Host 同源（scheme+host+port 一致）
func validateWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器客户端，由后续 install key / JWT 认证负责
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := parsed.Hostname()
	originPort := parsed.Port()
	if originHost == "" {
		return false
	}

	// 解析请求 Host
	reqHost := r.Host
	if h, p, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = h
		reqPort := p
		// 同源比较：host 一致，且端口一致（或都为默认端口）
		return isSameHostPort(originHost, originPort, reqHost, reqPort)
	}
	// 请求 Host 无端口（默认 80/443）
	return isSameHostPort(originHost, originPort, reqHost, "")
}

// isSameHostPort 比较 host:port 是否同源（考虑默认端口）
func isSameHostPort(originHost, originPort, reqHost, reqPort string) bool {
	if originHost != reqHost {
		return false
	}
	// 实际部署中前端与 API 同源时端口必然一致；
	// 任一端口为空表示使用默认端口，视为同源
	return originPort == reqPort || originPort == "" || reqPort == ""
}

// handleWebSocketConnection 处理WebSocket连接
func handleWebSocketConnection(ctx context.Context, conn net.Conn, svcCtx *svc.ServiceContext, wsHandler *WorkerWSHandler) {
	defer conn.Close()

	// 等待认证消息（超时30秒）
	authCtx, authCancel := context.WithTimeout(ctx, 30*time.Second)
	defer authCancel()

	workerName, err := waitForAuth(authCtx, conn, svcCtx)
	if err != nil {
		logx.Errorf("[WorkerWS] Authentication failed: %v", err)
		sendAuthFail(conn, err.Error())
		return
	}

	// 认证成功，发送AUTH_OK
	sendAuthOK(conn)
	logx.Infof("[WorkerWS] Worker authenticated: %s", workerName)

	// 创建Worker连接
	wc := NewWorkerConnection(conn, workerName, svcCtx)

	// 检查是否已有同名连接，如果有则关闭旧连接
	if oldConn, ok := wsHandler.connections.Load(workerName); ok {
		if old, ok := oldConn.(*WorkerConnection); ok {
			old.Close()
		}
	}

	// 注册连接
	wsHandler.connections.Store(workerName, wc)
	defer func() {
		wsHandler.connections.Delete(workerName)
		wc.Close()
		logx.Infof("[WorkerWS] Worker disconnected: %s", workerName)
	}()

	// 启动控制信号订阅
	go subscribeControlSignals(ctx, wc, svcCtx)

	// 启动发送协程
	go writePump(ctx, conn, wc)

	// 启动心跳检测
	go heartbeatChecker(ctx, wc)

	// 启动日志同步循环（定期从 Worker 拉取日志写入文件）
	go startLogSyncLoop(ctx, wc, svcCtx)

	// 连接建立后立即触发一次日志同步
	TriggerLogSync(wc)

	// 主循环：读取消息
	readPump(ctx, conn, wc, svcCtx)
}

// ==================== Authentication ====================

// waitForAuth 等待认证消息
func waitForAuth(ctx context.Context, conn net.Conn, svcCtx *svc.ServiceContext) (string, error) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	// 读取认证消息
	data, _, err := wsutil.ReadClientData(conn)
	if err != nil {
		return "", err
	}

	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return "", ErrInvalidMessage
	}

	if msg.Type != WSTypeAuth {
		return "", ErrAuthFailed
	}

	var authPayload AuthPayload
	if err := json.Unmarshal(msg.Payload, &authPayload); err != nil {
		return "", ErrInvalidMessage
	}

	// 验证Install Key
	if err := validateInstallKey(ctx, svcCtx, authPayload.InstallKey); err != nil {
		return "", err
	}

	if authPayload.WorkerName == "" {
		return "", ErrAuthFailed
	}

	return authPayload.WorkerName, nil
}

// validateInstallKey 验证Install Key
// 双密钥接受：环境变量 CSCAN_WORKER_KEY（默认 Worker）或 Redis install_key（手动探针）。
// 基础设施故障返回 503 错误，避免 Worker 误判密钥无效。
func validateInstallKey(ctx context.Context, svcCtx *svc.ServiceContext, installKey string) error {
	if installKey == "" {
		return ErrAuthFailed
	}

	// 双密钥校验（环境变量默认密钥 或 Redis install_key）
	valid, infraError := svcCtx.ValidateWorkerKey(ctx, installKey)
	if infraError {
		logx.Errorf("[WorkerWS] Auth service unavailable during key validation")
		return &WSError{Code: 1004, Message: "认证服务暂时不可用"}
	}
	if !valid {
		logx.Errorf("[WorkerWS] Invalid install key attempt")
		return ErrAuthFailed
	}

	return nil
}

// sendAuthOK 发送认证成功消息
func sendAuthOK(conn io.Writer) {
	msg := &WSMessage{Type: WSTypeAuthOK}
	data, _ := json.Marshal(msg)
	wsutil.WriteServerMessage(conn, ws.OpText, data)
}

// sendAuthFail 发送认证失败消息
func sendAuthFail(conn io.Writer, reason string) {
	payload, _ := json.Marshal(map[string]string{"reason": reason})
	msg := &WSMessage{Type: WSTypeAuthFail, Payload: payload}
	data, _ := json.Marshal(msg)
	wsutil.WriteServerMessage(conn, ws.OpText, data)
}

// ==================== Message Pumps ====================

// readPump 读取消息循环
func readPump(ctx context.Context, conn net.Conn, wc *WorkerConnection, svcCtx *svc.ServiceContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-wc.closeChan:
			return
		default:
		}

		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		data, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			logx.Errorf("[WorkerWS] Read error for %s: %v", wc.GetWorkerName(), err)
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			logx.Errorf("[WorkerWS] Invalid message from %s: %v, data: %s", wc.GetWorkerName(), err, string(data))
			continue
		}

		// 调试：打印收到的消息类型
		logx.Infof("[WorkerWS] Received message from %s: type=%s", wc.GetWorkerName(), msg.Type)

		// 路由消息
		handleMessage(ctx, wc, svcCtx, &msg)
	}
}

// writePump 发送消息循环
func writePump(ctx context.Context, conn net.Conn, wc *WorkerConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wc.closeChan:
			return
		case data := <-wc.sendChan:
			if err := wsutil.WriteServerMessage(conn, ws.OpText, data); err != nil {
				logx.Errorf("[WorkerWS] Write error for %s: %v", wc.GetWorkerName(), err)
				return
			}
		case <-ticker.C:
			// 发送PING保活
			msg := &WSMessage{Type: WSTypePing}
			data, _ := json.Marshal(msg)
			if err := wsutil.WriteServerMessage(conn, ws.OpText, data); err != nil {
				logx.Errorf("[WorkerWS] Ping error for %s: %v", wc.GetWorkerName(), err)
				return
			}
		}
	}
}

// heartbeatChecker 心跳检测
func heartbeatChecker(ctx context.Context, wc *WorkerConnection) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wc.closeChan:
			return
		case <-ticker.C:
			// 检查最后心跳时间，超过90秒未收到心跳则断开
			if time.Since(wc.GetLastPing()) > 90*time.Second {
				logx.Infof("[WorkerWS] Heartbeat timeout for %s", wc.GetWorkerName())
				wc.Close()
				return
			}
		}
	}
}

// ==================== Message Routing ====================

// handleMessage 处理消息路由
func handleMessage(ctx context.Context, wc *WorkerConnection, svcCtx *svc.ServiceContext, msg *WSMessage) {
	switch msg.Type {
	case WSTypePing:
		handlePing(wc)
	case WSTypePong:
		handlePong(wc)
	case WSTypeLog:
		handleLog(ctx, wc, svcCtx, msg.Payload)
	case WSTypeLogBatch:
		handleLogBatch(ctx, wc, svcCtx, msg.Payload)
	case WSTypeLogSyncResp:
		// Worker 返回的日志同步数据
		handleLogSyncResp(ctx, wc, svcCtx, msg.Payload)
	default:
		logx.Infof("[WorkerWS] Unknown message type from %s: %s", wc.GetWorkerName(), msg.Type)
	}
}

// handlePing 处理PING消息
func handlePing(wc *WorkerConnection) {
	wc.UpdateLastPing()
	// 发送PONG响应
	wc.Send(&WSMessage{Type: WSTypePong})
}

// handlePong 处理PONG消息
func handlePong(wc *WorkerConnection) {
	wc.UpdateLastPing()
}

// handleLog 处理单条日志消息（向后兼容：旧版 Worker 仍发送 LOG 消息）
// 新版 Worker 通过游标同步协议传输日志，不再发送 LOG 消息
// 保留此 handler 将旧版消息写入文件，避免日志丢失
// 修复 H-4：Worker 字段强制使用已认证连接名，忽略上报 payload 中可能伪造的名称
func handleLog(ctx context.Context, wc *WorkerConnection, svcCtx *svc.ServiceContext, payload json.RawMessage) {
	var logPayload LogPayload
	if err := json.Unmarshal(payload, &logPayload); err != nil {
		logx.Errorf("[WorkerWS] Invalid log payload from %s: %v", wc.GetWorkerName(), err)
		return
	}

	if logPayload.Timestamp == 0 {
		logPayload.Timestamp = time.Now().UnixMilli()
	}

	// 写入文件（兼容旧版 Worker）
	if svcCtx.WorkerLogWriter != nil {
		svcCtx.WorkerLogWriter.Write(svc.WorkerLogEntry{
			Ts:     time.UnixMilli(logPayload.Timestamp).Local().Format("2006-01-02T15:04:05.000-07:00"),
			Level:  logPayload.Level,
			Worker: wc.GetWorkerName(),
			TaskId: logPayload.TaskId,
			Msg:    logPayload.Message,
		})
	}
}

// handleLogBatch 处理批量日志消息（向后兼容）
func handleLogBatch(ctx context.Context, wc *WorkerConnection, svcCtx *svc.ServiceContext, payload json.RawMessage) {
	var batchPayload LogBatchPayload
	if err := json.Unmarshal(payload, &batchPayload); err != nil {
		logx.Errorf("[WorkerWS] Invalid log batch payload from %s: %v", wc.GetWorkerName(), err)
		return
	}

	if svcCtx.WorkerLogWriter != nil {
		entries := make([]svc.WorkerLogEntry, 0, len(batchPayload.Logs))
		for _, logPayload := range batchPayload.Logs {
			if logPayload.Timestamp == 0 {
				logPayload.Timestamp = time.Now().UnixMilli()
			}
			entries = append(entries, svc.WorkerLogEntry{
				Ts:     time.UnixMilli(logPayload.Timestamp).Local().Format("2006-01-02T15:04:05.000-07:00"),
				Level:  logPayload.Level,
				Worker: wc.GetWorkerName(),
				TaskId: logPayload.TaskId,
				Msg:    logPayload.Message,
			})
		}
		svcCtx.WorkerLogWriter.WriteBatch(entries)
	}
}

// handleLogSyncResp 处理 Worker 返回的日志同步数据
// 修复 H-2：等待日志真正写入磁盘并 Sync 成功后才发送 ACK，避免 API 崩溃导致日志丢失。
// Worker 只有收到 ACK 才会推进本地持久化游标，因此 ACK 必须作为"已持久化"的承诺。
func handleLogSyncResp(ctx context.Context, wc *WorkerConnection, svcCtx *svc.ServiceContext, payload json.RawMessage) {
	var resp LogSyncRespPayload
	if err := json.Unmarshal(payload, &resp); err != nil {
		logx.Errorf("[WorkerWS] Invalid log sync response from %s: %v", wc.GetWorkerName(), err)
		return
	}

	// 修复 H-4：强制覆盖 Worker 名为已认证连接名，防止恶意 Worker 通过上报数据伪造 Worker 字段穿越路径
	authedName := wc.GetWorkerName()
	for i := range resp.Logs {
		resp.Logs[i].Worker = authedName
	}

	if len(resp.Logs) == 0 {
		// 没有新日志，直接 ACK 推进游标
		sendLogSyncAck(wc, resp.Filename, resp.NewOffset)
		wc.syncCursorMu.Lock()
		wc.syncCursor = LogSyncReqPayload{
			Filename: resp.Filename,
			Offset:   resp.NewOffset,
		}
		wc.syncCursorMu.Unlock()
		if resp.HasMore && resp.NextFile != "" {
			wc.syncCursorMu.Lock()
			wc.syncCursor = LogSyncReqPayload{Filename: resp.NextFile, Offset: 0}
			wc.syncCursorMu.Unlock()
		}
		wc.notifySyncDone()
		return
	}

	// 同步写入文件并等待 Sync 落盘
	var writeErr error
	if svcCtx.WorkerLogWriter != nil {
		writeErr = svcCtx.WorkerLogWriter.SyncWriteBatch(resp.Logs)
	}
	if writeErr != nil {
		// 写入失败：不发送 ACK，Worker 会在下次同步时重发同一批日志（at-least-once）
		logx.Errorf("[WorkerWS] Failed to sync-write logs from %s: %v, will NOT ack (worker will retry)",
			wc.GetWorkerName(), writeErr)
		return
	}

	// 写入并 Sync 成功后才发送 ACK 给 Worker，更新其本地持久化游标
	sendLogSyncAck(wc, resp.Filename, resp.NewOffset)

	// 更新 API 端的内存游标
	wc.syncCursorMu.Lock()
	wc.syncCursor = LogSyncReqPayload{
		Filename: resp.Filename,
		Offset:   resp.NewOffset,
	}
	wc.syncCursorMu.Unlock()

	// 如果还有更多数据且需要切换到下一个文件（跨日），更新游标
	// NextFile 非空表示当前文件已读完，需要切换到下一个文件从头开始
	if resp.HasMore && resp.NextFile != "" {
		wc.syncCursorMu.Lock()
		wc.syncCursor = LogSyncReqPayload{Filename: resp.NextFile, Offset: 0}
		wc.syncCursorMu.Unlock()
	}
	wc.notifySyncDone()
}

// sendLogSyncAck 发送日志同步确认给 Worker
func sendLogSyncAck(wc *WorkerConnection, filename string, offset int64) {
	ack := LogSyncAckPayload{
		Filename: filename,
		Offset:   offset,
	}
	payloadData, _ := json.Marshal(ack)
	wc.Send(&WSMessage{
		Type:    WSTypeLogSyncAck,
		Payload: payloadData,
	})
}

// TriggerLogSync 触发一次日志同步（用户点击刷新按钮时调用）
// 向 Worker 发送 LOG_SYNC_REQ，请求从当前游标位置开始的新日志
func TriggerLogSync(wc *WorkerConnection) {
	wc.syncCursorMu.Lock()
	cursor := wc.syncCursor
	wc.syncCursorMu.Unlock()

	req := LogSyncReqPayload{
		Filename: cursor.Filename,
		Offset:   cursor.Offset,
	}
	payloadData, _ := json.Marshal(req)
	wc.Send(&WSMessage{
		Type:    WSTypeLogSyncReq,
		Payload: payloadData,
	})
}

// notifySyncDone 非阻塞地通知等待方"本轮日志同步已完成并落盘"
func (wc *WorkerConnection) notifySyncDone() {
	select {
	case wc.syncDone <- struct{}{}:
	default:
	}
}

// TriggerLogSyncAndWait 触发一次日志同步，并等待本轮同步完成（或超时/连接关闭）
// 用于用户点击"刷新"时立即拉取 Worker 最新日志，避免仅依赖后台 5s 轮询导致刷新看不到最新内容
func (wc *WorkerConnection) TriggerLogSyncAndWait(timeout time.Duration) {
	// 先清空可能残留的完成信号，确保等待的是本轮同步
	for {
		select {
		case <-wc.syncDone:
			continue
		default:
		}
		break
	}
	TriggerLogSync(wc)
	select {
	case <-wc.syncDone:
	case <-time.After(timeout):
	case <-wc.closeChan:
	}
}

// SyncAllAndWait 触发所有已连接 Worker 的日志同步并等待完成（或超时）
// 任务日志刷新时调用：确保读取前各 Worker 已把最新日志拉取到 API 本地文件
func (h *WorkerWSHandler) SyncAllAndWait(timeout time.Duration) {
	var wg sync.WaitGroup
	h.connections.Range(func(key, value interface{}) bool {
		wc, ok := value.(*WorkerConnection)
		if !ok {
			return true
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			wc.TriggerLogSyncAndWait(timeout)
		}()
		return true
	})
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// startLogSyncLoop 启动日志同步循环（每个 Worker 连接一个 goroutine）
// 定期向 Worker 请求新日志，写入文件
func startLogSyncLoop(ctx context.Context, wc *WorkerConnection, svcCtx *svc.ServiceContext) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wc.closeChan:
			return
		case <-ticker.C:
			if wc.isClosed() {
				return
			}
			TriggerLogSync(wc)
		}
	}
}

// ==================== Control Signal Subscription ====================

// subscribeControlSignals 订阅Redis控制信号
// 修复 C-09：原实现 channel 关闭时直接 return，Redis 重连后该 Worker 不再收到任务控制信号
// （STOP/PAUSE/RESUME），导致任务无法取消。现增加断线重连+指数退避。
func subscribeControlSignals(ctx context.Context, wc *WorkerConnection, svcCtx *svc.ServiceContext) {
	const maxBackoff = 30 * time.Second
	backoff := time.Second

	for {
		if ctx.Err() != nil || wc.isClosed() {
			return
		}

		// 使用模式订阅所有任务控制信号
		pubsub := svcCtx.RedisClient.PSubscribe(ctx, "cscan:task:ctrl:*")
		ch := pubsub.Channel()

		// 等待订阅确认
		if _, err := pubsub.Receive(ctx); err != nil {
			if ctx.Err() != nil || wc.isClosed() {
				pubsub.Close()
				return
			}
			logx.Errorf("[WorkerWS] PSubscribe cscan:task:ctrl:* failed for %s: %v, retry in %v",
				wc.GetWorkerName(), err, backoff)
			pubsub.Close()
			controlSleepCtx(ctx, wc, backoff)
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}
		backoff = time.Second

	consumeLoop:
		for {
			select {
			case <-ctx.Done():
				pubsub.Close()
				return
			case <-wc.closeChan:
				pubsub.Close()
				return
			case msg, ok := <-ch:
				// 通道关闭（连接断开/错误）：退出内层循环，外层重连
				if !ok || msg == nil {
					logx.Errorf("[WorkerWS] Task control subscription closed for %s, reconnecting",
						wc.GetWorkerName())
					pubsub.Close()
					break consumeLoop
				}
				// 解析频道名获取taskId
				// 频道格式: cscan:task:ctrl:{taskId}
				taskId := extractTaskIdFromChannel(msg.Channel)
				if taskId == "" {
					continue
				}

				// 转发控制信号给Worker
				action := msg.Payload // STOP, PAUSE, RESUME
				payload, _ := json.Marshal(&ControlPayload{
					TaskId: taskId,
					Action: action,
				})

				wc.Send(&WSMessage{
					Type:    WSTypeControl,
					Payload: payload,
				})

				logx.Infof("[WorkerWS] Forwarded control signal to %s: taskId=%s, action=%s",
					wc.GetWorkerName(), taskId, action)
			}
		}

		// 断线退避后重连
		controlSleepCtx(ctx, wc, backoff)
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

// controlSleepCtx 可被 ctx / wc.closeChan 取消的 sleep
func controlSleepCtx(ctx context.Context, wc *WorkerConnection, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-wc.closeChan:
	case <-t.C:
	}
}

// extractTaskIdFromChannel 从频道名提取taskId
func extractTaskIdFromChannel(channel string) string {
	// 频道格式: cscan:task:ctrl:{taskId}
	const prefix = "cscan:task:ctrl:"
	if len(channel) > len(prefix) {
		return channel[len(prefix):]
	}
	return ""
}

// ==================== Terminal Output Handling ====================

