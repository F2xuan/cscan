package vul

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ReverifyConfigSaveHandler 保存弱口令/敏感信息持续复验配置（T3.3 / T3.4）
func ReverifyConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReverifyConfigSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &types.ReverifyConfigSaveResp{Code: 400, Msg: err.Error()})
			return
		}

		l := logic.NewReverifyConfigSaveLogic(r.Context(), svcCtx)
		resp, err := l.ReverifyConfigSave(&req)
		if err != nil {
			httpx.OkJson(w, &types.ReverifyConfigSaveResp{Code: 500, Msg: err.Error()})
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
