package vul

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ReverifyHandler 单条/批量漏洞复验（人工触发，worker 执行复测）
// POST /api/v1/vul/reverify
func ReverifyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.VulReverifyReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &types.VulReverifyResp{Code: 400, Msg: err.Error()})
			return
		}

		l := logic.NewVulReverifyLogic(r.Context(), svcCtx)
		resp, err := l.VulReverify(&req)
		if err != nil {
			httpx.OkJson(w, &types.VulReverifyResp{Code: 500, Msg: err.Error()})
			return
		}
		httpx.OkJson(w, resp)
	}
}
