package worker

import (
	"encoding/json"
	"net/http"

	"cscan/api/internal/svc"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// ==================== SubTask Types ====================

// WorkerSubTaskDoneReq 子任务完成请求
type WorkerSubTaskDoneReq struct {
	TaskId      string `json:"taskId"`
	MainTaskId  string `json:"mainTaskId"`
	WorkspaceId string `json:"workspaceId"`
	Phase       string `json:"phase"`
	IsCompleted bool   `json:"isCompleted"`
	IncrAmount  int    `json:"incrAmount"`
}

// WorkerSubTaskDoneResp 子任务完成响应
type WorkerSubTaskDoneResp struct {
	Code         int    `json:"code"`
	Msg          string `json:"msg"`
	Success      bool   `json:"success"`
	SubTaskDone  int32  `json:"subTaskDone"`
	SubTaskCount int32  `json:"subTaskCount"`
	AllDone      bool   `json:"allDone"`
}

// ==================== SubTask Handler ====================

// WorkerSubTaskDoneHandler 子任务进度接口
// POST /api/v1/worker/task/subtask/done
func WorkerSubTaskDoneHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerSubTaskDoneReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerSubTaskDoneResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.TaskId == "" || req.MainTaskId == "" {
			httpx.OkJson(w, &WorkerSubTaskDoneResp{Code: 400, Msg: "taskId和mainTaskId不能为空"})
			return
		}

		result, err := svcCtx.IncrSubTaskDone(r.Context(), req.TaskId, req.MainTaskId, req.WorkspaceId, req.Phase, req.IncrAmount)
		if err != nil {
			logx.Errorf("[WorkerSubTaskDone] IncrSubTaskDone error: %v", err)
			response.Error(w, err)
			return
		}

		httpx.OkJson(w, &WorkerSubTaskDoneResp{
			Code:         0,
			Msg:          result.Message,
			Success:      result.Success,
			SubTaskDone:  result.SubTaskDone,
			SubTaskCount: result.SubTaskCount,
			AllDone:      result.AllDone,
		})
	}
}
