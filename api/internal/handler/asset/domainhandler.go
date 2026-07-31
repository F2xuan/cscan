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

// DomainListHandler 域名列表
func DomainListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DomainListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewDomainLogic(r.Context(), svcCtx)
		resp, err := l.DomainList(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DomainStatHandler 域名统计
func DomainStatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewDomainLogic(r.Context(), svcCtx)
		resp, err := l.DomainStat(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DomainDeleteHandler 删除域名
func DomainDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DomainDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewDomainLogic(r.Context(), svcCtx)
		resp, err := l.DomainDelete(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DomainBatchDeleteHandler 批量删除域名
func DomainBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DomainBatchDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewDomainLogic(r.Context(), svcCtx)
		resp, err := l.DomainBatchDelete(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DomainClearHandler 清空域名
func DomainClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewDomainClearLogic(r.Context(), svcCtx)
		resp, err := l.DomainClear(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
