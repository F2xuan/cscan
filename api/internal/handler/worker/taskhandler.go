package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// ==================== Worker Task Types ====================

// WorkerTaskCheckReq 任务拉取请求
type WorkerTaskCheckReq struct {
	WorkerName string `json:"workerName"`
}

// WorkerTaskCheckResp 任务拉取响应
type WorkerTaskCheckResp struct {
	Code        int    `json:"code"`
	Msg         string `json:"msg"`
	IsExist     bool   `json:"isExist"`
	IsFinished  bool   `json:"isFinished"`
	TaskId      string `json:"taskId"`
	MainTaskId  string `json:"mainTaskId"`
	WorkspaceId string `json:"workspaceId"`
	Config      string `json:"config"`
}

// WorkerTaskUpdateReq 任务状态更新请求
type WorkerTaskUpdateReq struct {
	TaskId   string `json:"taskId"`
	State    string `json:"state"` // started, success, failure, paused
	Worker   string `json:"worker"`
	Result   string `json:"result"`
	Progress int    `json:"progress"` // 0-100
	Phase    string `json:"phase"`    // 当前阶段描述
}

// WorkerTaskUpdateResp 任务状态更新响应
type WorkerTaskUpdateResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// ==================== Task Check Handler ====================

// WorkerTaskCheckHandler 任务拉取接口
// POST /api/v1/worker/task/check
func WorkerTaskCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerTaskCheckReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerTaskCheckResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.WorkerName == "" {
			httpx.OkJson(w, &WorkerTaskCheckResp{Code: 400, Msg: "workerName不能为空"})
			return
		}

		checkCtx, checkCancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer checkCancel()

		result, err := svcCtx.CheckTask(checkCtx, req.WorkerName)
		if err != nil {
			response.Error(w, err)
			return
		}

		httpx.OkJson(w, &WorkerTaskCheckResp{
			Code:        0,
			Msg:         "success",
			IsExist:     result.IsExist,
			IsFinished:  result.IsFinished,
			TaskId:      result.TaskId,
			MainTaskId:  result.MainTaskId,
			WorkspaceId: result.WorkspaceId,
			Config:      result.Config,
		})
	}
}

// ==================== Task Update Handler ====================

// WorkerTaskUpdateHandler 任务状态更新接口
// POST /api/v1/worker/task/update
func WorkerTaskUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerTaskUpdateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerTaskUpdateResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.TaskId == "" {
			httpx.OkJson(w, &WorkerTaskUpdateResp{Code: 400, Msg: "taskId不能为空"})
			return
		}

		err := svcCtx.UpdateTask(r.Context(), req.TaskId, req.State, req.Worker, req.Result, req.Phase)
		if err != nil {
			logx.Errorf("[WorkerTaskUpdate] error: %v", err)
			response.Error(w, err)
			return
		}

		httpx.OkJson(w, &WorkerTaskUpdateResp{
			Code:    0,
			Msg:     "Task status updated",
			Success: true,
		})
	}
}

// ==================== Task Control Handler ====================

// WorkerTaskControlReq 任务控制信号请求
type WorkerTaskControlReq struct {
	WorkerName string   `json:"workerName"`
	TaskIds    []string `json:"taskIds"` // 当前正在执行的任务ID列表
}

// TaskControlSignal 单个任务的控制信号
type TaskControlSignal struct {
	TaskId string `json:"taskId"`
	Action string `json:"action"` // STOP, PAUSE, RESUME
}

// WorkerTaskControlResp 任务控制信号响应
type WorkerTaskControlResp struct {
	Code    int                 `json:"code"`
	Msg     string              `json:"msg"`
	Success bool                `json:"success"`
	Signals []TaskControlSignal `json:"signals"`
}

// WorkerTaskControlHandler 任务控制信号轮询接口
// POST /api/v1/worker/task/control
// 用于WebSocket不可用时的HTTP轮询回退
func WorkerTaskControlHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerTaskControlReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerTaskControlResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if len(req.TaskIds) == 0 {
			httpx.OkJson(w, &WorkerTaskControlResp{
				Code:    0,
				Msg:     "success",
				Success: true,
				Signals: []TaskControlSignal{},
			})
			return
		}

		// 从Redis检查每个任务的控制信号
		var signals []TaskControlSignal
		ctx := r.Context()

		for _, taskId := range req.TaskIds {
			// 检查Redis中是否有该任务的控制信号
			// 控制信号存储在 cscan:task:ctrl:{taskId} 键中
			ctrlKey := "cscan:task:ctrl:" + taskId
			action, err := svcCtx.RedisClient.Get(ctx, ctrlKey).Result()
			if err == nil && action != "" {
				signals = append(signals, TaskControlSignal{
					TaskId: taskId,
					Action: action,
				})
			}
		}

		httpx.OkJson(w, &WorkerTaskControlResp{
			Code:    0,
			Msg:     "success",
			Success: true,
			Signals: signals,
		})
	}
}

// ==================== Task Recovery Handler ====================

// WorkerTaskRecoveryReq 任务恢复请求
type WorkerTaskRecoveryReq struct {
	WorkerName string `json:"workerName"`
}

// RecoveredTaskInfo 恢复的任务信息
type RecoveredTaskInfo struct {
	TaskId      string `json:"taskId"`
	MainTaskId  string `json:"mainTaskId"`
	WorkspaceId string `json:"workspaceId"`
	Status      string `json:"status"`
	StartTime   string `json:"startTime"`
}

// WorkerTaskRecoveryResp 任务恢复响应
type WorkerTaskRecoveryResp struct {
	Code           int                 `json:"code"`
	Msg            string              `json:"msg"`
	Success        bool                `json:"success"`
	RecoveredTasks []RecoveredTaskInfo `json:"recoveredTasks"`
	RecoveredCount int                 `json:"recoveredCount"`
}

// WorkerTaskRecoveryHandler Worker 启动时的任务恢复接口
// POST /api/v1/worker/task/recovery
// 当 Worker 重新启动时调用，恢复该 Worker 之前未完成的任务
func WorkerTaskRecoveryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerTaskRecoveryReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerTaskRecoveryResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.WorkerName == "" {
			httpx.OkJson(w, &WorkerTaskRecoveryResp{Code: 400, Msg: "workerName不能为空"})
			return
		}

		logx.Infof("[WorkerTaskRecovery] Worker %s requesting task recovery", req.WorkerName)

		ctx := r.Context()

		// 1. 清理该 Worker 在 processing 集合中的任务记录
		logic.CleanupStaleProcessingTasks(ctx, svcCtx, req.WorkerName)

		// 2. 恢复超时的孤儿任务 (5分钟认为超时)
		recoveredTasksInfo, err := logic.RecoverOrphanedTasks(ctx, svcCtx, 5*time.Minute)
		if err != nil {
			httpx.OkJson(w, &WorkerTaskRecoveryResp{
				Code:    500,
				Msg:     "获取工作空间失败",
				Success: false,
			})
			return
		}

		// 转换为响应数据结构
		var recoveredTasks []RecoveredTaskInfo
		for _, v := range recoveredTasksInfo {
			recoveredTasks = append(recoveredTasks, RecoveredTaskInfo{
				TaskId:      v.TaskId,
				MainTaskId:  v.MainTaskId,
				WorkspaceId: v.WorkspaceId,
				Status:      v.Status,
				StartTime:   v.StartTime,
			})
		}

		if len(recoveredTasks) > 0 {
			logx.Infof("[WorkerTaskRecovery] Worker %s recovered %d tasks", req.WorkerName, len(recoveredTasks))
		}

		httpx.OkJson(w, &WorkerTaskRecoveryResp{
			Code:           0,
			Msg:            "success",
			Success:        true,
			RecoveredTasks: recoveredTasks,
			RecoveredCount: len(recoveredTasks),
		})
	}
}
