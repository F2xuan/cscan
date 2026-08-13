package jsfinder

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// JSFinderDetailHandler 获取单条 JSFinder 结果详情（含 request/response/curl_command）
func JSFinderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		resp, err := l.GetJSFinderDetail(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
