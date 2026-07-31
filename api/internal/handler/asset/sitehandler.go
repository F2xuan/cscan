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

// SiteListHandler 站点列表
func SiteListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SiteListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewSiteLogic(r.Context(), svcCtx)
		resp, err := l.SiteList(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// SiteStatHandler 站点统计
func SiteStatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewSiteLogic(r.Context(), svcCtx)
		resp, err := l.SiteStat(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// SiteDeleteHandler 删除站点
func SiteDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SiteDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewSiteLogic(r.Context(), svcCtx)
		resp, err := l.SiteDelete(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// SiteBatchDeleteHandler 批量删除站点
func SiteBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SiteBatchDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewSiteLogic(r.Context(), svcCtx)
		resp, err := l.SiteBatchDelete(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// SiteClearHandler 清空站点
func SiteClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewSiteClearLogic(r.Context(), svcCtx)
		resp, err := l.SiteClear(workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
