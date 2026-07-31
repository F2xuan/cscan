package cert

import (
	"encoding/json"
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"
)

// CertListHandler 证书列表
func CertListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CertListReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		l := logic.NewCertLogic(r.Context(), svcCtx)
		resp, err := l.GetCertList(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// CertDetailHandler 证书详情
func CertDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CertDetailReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		l := logic.NewCertLogic(r.Context(), svcCtx)
		resp, err := l.GetCertDetail(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
