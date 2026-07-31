package report

import (
	"encoding/json"
	"net/http"
	"net/url"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
)

func ReportPeriodicExportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReportPeriodicExportReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(&types.BaseResp{Code: 400, Msg: "参数错误"})
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		l := logic.NewReportPeriodicExportLogic(r.Context(), svcCtx)
		data, filename, err := l.PeriodicExport(&req, workspaceId)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(&types.BaseResp{Code: 500, Msg: err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
		w.Write(data)
	}
}
