package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// ==================== Heartbeat Types ====================

// WorkerHeartbeatReq 心跳请求
type WorkerHeartbeatReq struct {
	WorkerName         string  `json:"workerName"`
	IP                 string  `json:"ip"`
	CpuLoad            float64 `json:"cpuLoad"`
	MemUsed            float64 `json:"memUsed"`
	TaskStartedNumber  int32   `json:"taskStartedNumber"`
	TaskExecutedNumber int32   `json:"taskExecutedNumber"`
	Concurrency        int     `json:"concurrency"`
	IsDaemon           bool    `json:"isDaemon"`
}

// WorkerHeartbeatResp 心跳响应
type WorkerHeartbeatResp struct {
	Code               int    `json:"code"`
	Msg                string `json:"msg"`
	Status             string `json:"status"`
	ManualStopFlag     bool   `json:"manualStopFlag"`
	ManualReloadFlag   bool   `json:"manualReloadFlag"`
	ManualInitEnvFlag  bool   `json:"manualInitEnvFlag"`
	ManualSyncFlag     bool   `json:"manualSyncFlag"`
	DesiredConcurrency int    `json:"desiredConcurrency,omitempty"`
}

// ==================== Heartbeat Handler ====================

// WorkerHeartbeatHandler 心跳接口
// POST /api/v1/worker/heartbeat
func WorkerHeartbeatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerHeartbeatReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerHeartbeatResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.WorkerName == "" {
			httpx.OkJson(w, &WorkerHeartbeatResp{Code: 400, Msg: "workerName不能为空"})
			return
		}

		ctx := r.Context()

		// 直接更新 Worker 状态到 Redis
		workerKey := "cscan:worker:" + req.WorkerName
		workerData := map[string]interface{}{
			"workerName":         req.WorkerName,
			"ip":                 req.IP,
			"cpuLoad":            req.CpuLoad,
			"memUsed":            req.MemUsed,
			"taskStartedNumber":  req.TaskStartedNumber,
			"taskExecutedNumber": req.TaskExecutedNumber,
			"concurrency":        req.Concurrency,
			"isDaemon":           req.IsDaemon,
			"updateTime":         time.Now().Format("2006-01-02 15:04:05"),
			"status":             "online",
		}
		workerJson, _ := json.Marshal(workerData)
		svcCtx.RedisClient.Set(ctx, workerKey, workerJson, 60*time.Second)

		// 添加到 Worker 集合
		svcCtx.RedisClient.SAdd(ctx, "cscan:workers", req.WorkerName)

		// 检查控制命令
		controlKey := "cscan:worker:control:" + req.WorkerName
		controlData, err := svcCtx.RedisClient.Get(ctx, controlKey).Result()

		var manualStop, manualReload, manualInitEnv, manualSync bool
		if err == nil && controlData != "" {
			var control map[string]bool
			if json.Unmarshal([]byte(controlData), &control) == nil {
				manualStop = control["stop"]
				manualReload = control["reload"]
				manualInitEnv = control["initEnv"]
				manualSync = control["sync"]
			}
			svcCtx.RedisClient.Del(ctx, controlKey)
		}

		// 读取期望并发数
		desiredConcurrency := 0
		desiredKey := fmt.Sprintf("cscan:worker:desired_concurrency:%s", req.WorkerName)
		if val, err := svcCtx.RedisClient.Get(ctx, desiredKey).Int(); err == nil && val > 0 {
			desiredConcurrency = val
		}

		httpx.OkJson(w, &WorkerHeartbeatResp{
			Code:               0,
			Msg:                "success",
			Status:             "ok",
			ManualStopFlag:     manualStop,
			ManualReloadFlag:   manualReload,
			ManualInitEnvFlag:  manualInitEnv,
			ManualSyncFlag:     manualSync,
			DesiredConcurrency: desiredConcurrency,
		})
	}
}

// ==================== Offline Types ====================

// WorkerOfflineReq Worker离线通知请求
type WorkerOfflineReq struct {
	WorkerName string `json:"workerName"`
}

// WorkerOfflineResp Worker离线通知响应
type WorkerOfflineResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// ==================== Offline Handler ====================

// WorkerOfflineHandler Worker离线通知接口
// POST /api/v1/worker/offline
// Worker停止时调用此接口，立即删除Redis中的状态数据
func WorkerOfflineHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerOfflineReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerOfflineResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.WorkerName == "" {
			httpx.OkJson(w, &WorkerOfflineResp{Code: 400, Msg: "workerName不能为空"})
			return
		}

		rdb := svcCtx.RedisClient

		// 删除Worker状态数据
		workerKey := fmt.Sprintf("cscan:worker:%s", req.WorkerName)
		rdb.Del(r.Context(), workerKey)

		// 从Worker集合中移除
		rdb.SRem(r.Context(), "cscan:workers", req.WorkerName)

		// 删除控制命令（如果有）
		controlKey := fmt.Sprintf("cscan:worker:control:%s", req.WorkerName)
		rdb.Del(r.Context(), controlKey)

		logx.Infof("[WorkerOffline] Worker %s offline, deleted from Redis", req.WorkerName)

		// 立即恢复该 Worker 处理中的任务（异步执行，避免阻塞 HTTP 响应）
		// 修复：原同步调用 RecoverWorkerTasks 在循环 MongoDB 操作时可能阻塞超过 Worker 的
		// 3s HTTP 超时，导致 Worker 发送 RST → API 进程崩溃 → 8888 端口短暂不可用。
		go func(workerName string) {
			defer func() {
				if r := recover(); r != nil {
					logx.Errorf("[WorkerOffline] panic worker=%s err=%v stack=%s", workerName, r, debug.Stack())
				}
			}()
			recoverCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			recoveredTasks, err := logic.RecoverWorkerTasks(recoverCtx, svcCtx, workerName)
			if err != nil {
				logx.Errorf("[WorkerOffline] Failed to recover tasks for worker %s: %v", workerName, err)
			} else if len(recoveredTasks) > 0 {
				logx.Infof("[WorkerOffline] Worker %s: recovered %d orphaned tasks", workerName, len(recoveredTasks))
			}
		}(req.WorkerName)

		httpx.OkJson(w, &WorkerOfflineResp{
			Code:    0,
			Msg:     "success",
			Success: true,
		})
	}
}
