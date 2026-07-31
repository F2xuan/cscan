package vul

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ReverifyRunNowHandler 立即触发弱口令复验（T3.3）
func ReverifyRunNowHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReverifyRunNowReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &types.ReverifyRunNowResp{Code: 400, Msg: err.Error()})
			return
		}
		if req.WorkspaceId == "" {
			req.WorkspaceId = middleware.GetWorkspaceId(r.Context())
		}

		l := logic.NewReverifyRunNowLogic(r.Context(), svcCtx)
		resp, err := l.ReverifyRunNow(&req)
		if err != nil {
			httpx.OkJson(w, &types.ReverifyRunNowResp{Code: 500, Msg: err.Error()})
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
