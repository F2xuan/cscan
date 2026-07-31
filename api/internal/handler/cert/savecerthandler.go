package cert

import (
	"encoding/json"
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"
)

// SaveCertResultHandler 处理保存证书结果请求
func SaveCertResultHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SaveCertReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewCertLogic(r.Context(), svcCtx)
		err := l.SaveCerts(&req)
		if err != nil {
			response.Error(w, err)
			return
		}

		response.Success(w, map[string]interface{}{})
	}
}
