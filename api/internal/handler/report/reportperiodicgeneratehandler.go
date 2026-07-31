package report

import (
	"encoding/json"
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
)

func ReportPeriodicGenerateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReportPeriodicGenerateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(&types.ReportPeriodicGenerateResp{Code: 400, Msg: "参数错误"})
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewReportPeriodicGenerateLogic(r.Context(), svcCtx)
		resp, _ := l.PeriodicGenerate(&req, workspaceId)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
