package dirscan

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// DirScanAIAnalyzeHandler 单条AI研判
func DirScanAIAnalyzeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanAIAnalyzeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		if req.WorkspaceId == "" {
			req.WorkspaceId = middleware.GetWorkspaceId(r.Context())
		}
		l := logic.NewDirScanLogic(r.Context(), svcCtx)
		resp, err := l.AnalyzeSingle(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// DirScanAIBatchAnalyzeHandler 批量AI研判（异步启动）
func DirScanAIBatchAnalyzeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanAIBatchAnalyzeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		if req.WorkspaceId == "" {
			req.WorkspaceId = middleware.GetWorkspaceId(r.Context())
		}
		l := logic.NewDirScanLogic(r.Context(), svcCtx)
		resp, err := l.BatchAnalyzeAsync(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// DirScanAIBatchProgressHandler 批量研判进度查询
func DirScanAIBatchProgressHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanAIBatchProgressReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		l := logic.NewDirScanLogic(r.Context(), svcCtx)
		resp, err := l.GetBatchProgress(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// DirScanAIStopBatchHandler 停止批量研判
func DirScanAIStopBatchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanAIStopBatchReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		l := logic.NewDirScanLogic(r.Context(), svcCtx)
		err := l.StopBatchTask(req.TaskId)
		if err != nil {
			httpx.OkJson(w, &types.DirScanAIStopBatchResp{Code: 500, Msg: err.Error()})
		} else {
			httpx.OkJson(w, &types.DirScanAIStopBatchResp{Code: 0, Msg: "停止指令已发送"})
		}
	}
}

// DirScanDetailHandler 单条详情（含request/response大字段）
func DirScanDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		if req.WorkspaceId == "" {
			req.WorkspaceId = middleware.GetWorkspaceId(r.Context())
		}
		l := logic.NewDirScanLogic(r.Context(), svcCtx)
		resp, err := l.GetDirScanDetail(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// DirScanListHandlerV2 列表接口（新版，使用logic层，支持AI过滤和投影）
func DirScanListHandlerV2(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanResultListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		if req.WorkspaceId == "" {
			req.WorkspaceId = middleware.GetWorkspaceId(r.Context())
		}
		l := logic.NewDirScanLogic(r.Context(), svcCtx)
		resp, err := l.GetDirScanList(&req)
		if err != nil {
			httpx.OkJson(w, &types.DirScanResultListResp{Code: 500, Msg: err.Error()})
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
