package asset

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// IPListHandler IP列表
func IPListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.IPListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewIPLogic(r.Context(), svcCtx)
		resp, err := l.IPList(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// IPStatHandler IP统计
func IPStatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewIPLogic(r.Context(), svcCtx)
		resp, err := l.IPStat(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// IPDeleteHandler 删除IP
func IPDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.IPDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewIPLogic(r.Context(), svcCtx)
		resp, err := l.IPDelete(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// IPBatchDeleteHandler 批量删除IP
func IPBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.IPBatchDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewIPLogic(r.Context(), svcCtx)
		resp, err := l.IPBatchDelete(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// IPClearHandler 清空 IP
func IPClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewIPClearLogic(r.Context(), svcCtx)
		resp, err := l.IPClear(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
