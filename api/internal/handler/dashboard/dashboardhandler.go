package dashboard

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// DashboardChangesHandler 工作台变化数据（资产变化 / 风险变化）
func DashboardChangesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DashboardChangesReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewDashboardChangesLogic(r.Context(), svcCtx)
		resp, err := l.DashboardChanges(&req, workspaceId)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
