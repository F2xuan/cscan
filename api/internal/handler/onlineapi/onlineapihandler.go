package onlineapi

import (
	"context"
	"net/http"
	"time"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// OnlineSearchHandler 在线API搜索
func OnlineSearchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OnlineSearchReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewOnlineAPILogic(r.Context(), svcCtx)
		resp, err := l.Search(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// OnlineImportHandler 在线API导入（异步，返回taskId）
func OnlineImportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OnlineImportReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())

		// 复制资产数据，避免请求结束后数据被回收
		assets := make([]types.OnlineSearchResult, len(req.Assets))
		copy(assets, req.Assets)
		req.Assets = assets

		// 生成taskId并初始化任务状态（原子写入，避免竞态）
		taskId := logic.SubmitImportTask(req.Platform, "current", len(req.Assets))

		// 立即返回taskId
		httpx.OkJson(w, &types.OnlineImportTaskSubmitResp{
			Code:   0,
			Msg:    "导入任务已提交，后台处理中",
			TaskId: taskId,
		})

		// 异步执行导入
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					logx.Errorf("OnlineImport async panic: %v", rec)
					if v, ok := logic.GetOnlineImportTaskState(taskId); ok {
						v.Status = "failed"
						v.ErrorMsg = "导入过程发生异常"
						v.EndTime = time.Now()
					}
				}
			}()

			bgCtx := context.Background()
			l := logic.NewOnlineAPILogic(bgCtx, svcCtx)
			state, _ := logic.GetOnlineImportTaskState(taskId)

			resp, err := l.Import(&req, workspaceId, state)
			if err != nil {
				logx.Errorf("OnlineImport async failed: %v", err)
				state.Status = "failed"
				state.ErrorMsg = err.Error()
				state.EndTime = time.Now()
				return
			}
			if resp.Code != 0 {
				logx.Errorf("OnlineImport async error: code=%d msg=%s", resp.Code, resp.Msg)
				state.Status = "failed"
				state.ErrorMsg = resp.Msg
				state.EndTime = time.Now()
				return
			}

			state.Status = "completed"
			state.EndTime = time.Now()
			logx.Infof("OnlineImport async success: %s", resp.Msg)
		}()
	}
}

// OnlineImportProgressHandler 查询导入任务进度
func OnlineImportProgressHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OnlineImportTaskProgressReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewOnlineAPILogic(r.Context(), svcCtx)
		resp, err := l.GetImportTaskProgress(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// OnlineImportResultHandler 查询导入任务结果
func OnlineImportResultHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OnlineImportTaskResultReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewOnlineAPILogic(r.Context(), svcCtx)
		resp, err := l.GetImportTaskResult(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// APIConfigListHandler API配置列表
func APIConfigListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewOnlineAPILogic(r.Context(), svcCtx)
		resp, err := l.ConfigList(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// APIConfigSaveHandler 保存API配置
func APIConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.APIConfigSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewOnlineAPILogic(r.Context(), svcCtx)
		resp, err := l.ConfigSave(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// OnlineImportAllHandler 导入全部资产（异步，返回taskId）
func OnlineImportAllHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OnlineImportAllReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())

		// 生成taskId并初始化任务状态（原子写入，避免竞态）
		taskId := logic.SubmitImportTask(req.Platform, "all", 0)

		// 立即返回taskId
		httpx.OkJson(w, &types.OnlineImportTaskSubmitResp{
			Code:   0,
			Msg:    "导入任务已提交，后台处理中",
			TaskId: taskId,
		})

		// 异步执行导入
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					logx.Errorf("OnlineImportAll async panic: %v", rec)
					if v, ok := logic.GetOnlineImportTaskState(taskId); ok {
						v.Status = "failed"
						v.ErrorMsg = "导入过程发生异常"
						v.EndTime = time.Now()
					}
				}
			}()

			bgCtx := context.Background()
			l := logic.NewOnlineAPILogic(bgCtx, svcCtx)
			state, _ := logic.GetOnlineImportTaskState(taskId)

			resp, err := l.ImportAll(&req, workspaceId, state)
			if err != nil {
				logx.Errorf("OnlineImportAll async failed: %v", err)
				state.Status = "failed"
				state.ErrorMsg = err.Error()
				state.EndTime = time.Now()
				return
			}
			if resp.Code != 0 {
				logx.Errorf("OnlineImportAll async error: code=%d msg=%s", resp.Code, resp.Msg)
				state.Status = "failed"
				state.ErrorMsg = resp.Msg
				state.EndTime = time.Now()
				return
			}

			state.Status = "completed"
			state.TotalFetched = resp.TotalFetched
			state.TotalPages = resp.TotalPages
			state.EndTime = time.Now()
			logx.Infof("OnlineImportAll async success: fetched=%d import=%d pages=%d",
				resp.TotalFetched, resp.TotalImport, resp.TotalPages)
		}()
	}
}

