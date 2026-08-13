package vul

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ReverifyConfigGetHandler 获取弱口令/敏感信息持续复验配置（T3.3 / T3.4）
func ReverifyConfigGetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReverifyConfigGetReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &types.ReverifyConfigGetResp{Code: 400, Msg: err.Error()})
			return
		}

		l := logic.NewReverifyConfigGetLogic(r.Context(), svcCtx)
		resp, err := l.ReverifyConfigGet(&req)
		if err != nil {
			httpx.OkJson(w, &types.ReverifyConfigGetResp{Code: 500, Msg: err.Error()})
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
