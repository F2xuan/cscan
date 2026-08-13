package task

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// TaskQuickCreateHandler 一键扫描 + 智能模板推荐（T4.1）
func TaskQuickCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TaskQuickCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &types.TaskQuickCreateResp{Code: 400, Msg: err.Error()})
			return
		}

		l := logic.NewTaskQuickCreateLogic(r.Context(), svcCtx)
		resp, err := l.TaskQuickCreate(&req)
		if err != nil {
			httpx.OkJson(w, &types.TaskQuickCreateResp{Code: 500, Msg: err.Error()})
			return
		}
		httpx.OkJson(w, resp)
	}
}
