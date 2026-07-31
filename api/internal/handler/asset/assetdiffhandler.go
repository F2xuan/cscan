package asset

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// AssetDiffListHandler 资产变化快照列表
func AssetDiffListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AssetDiffListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewAssetDiffListLogic(r.Context(), svcCtx)
		resp, err := l.AssetDiffList(&req, workspaceId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

// AssetDiffStatHandler 资产变化快照聚合统计
func AssetDiffStatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AssetDiffStatReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewAssetDiffStatLogic(r.Context(), svcCtx)
		resp, err := l.AssetDiffStat(&req, workspaceId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
