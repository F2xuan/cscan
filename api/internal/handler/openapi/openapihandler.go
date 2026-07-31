package openapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// OpenApiHandler 开放 API 统一响应辅助。

func writeOpenJSON(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOpenErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code": status,
		"msg":  msg,
	})
}

// writeOpenLogicErr 将 logic 错误映射为 HTTP 状态：参数/资源类错误 400，其余 500。
func writeOpenLogicErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	status := http.StatusInternalServerError
	if strings.Contains(msg, "缺少") || strings.Contains(msg, "不存在") || strings.Contains(msg, "工作空间") {
		status = http.StatusBadRequest
	}
	writeOpenErr(w, status, msg)
}

func ensureGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		writeOpenErr(w, http.StatusMethodNotAllowed, "开放 API 仅支持 GET（只读）")
		return false
	}
	return true
}

// ---- 资产 ----

func OpenAssetsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGet(w, r) {
			return
		}
		var req types.OpenAssetsReq
		if err := httpx.Parse(r, &req); err != nil {
			writeOpenErr(w, http.StatusBadRequest, "参数解析失败: "+err.Error())
			return
		}
		resp, err := logic.NewOpenAssetsLogic(r.Context(), svcCtx).Assets(&req)
		if err != nil {
			writeOpenLogicErr(w, err)
			return
		}
		writeOpenJSON(w, resp)
	}
}

func OpenAssetDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGet(w, r) {
			return
		}
		var req types.OpenAssetDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			writeOpenErr(w, http.StatusBadRequest, "参数解析失败: "+err.Error())
			return
		}
		resp, err := logic.NewOpenAssetDetailLogic(r.Context(), svcCtx).AssetDetail(&req)
		if err != nil {
			writeOpenLogicErr(w, err)
			return
		}
		writeOpenJSON(w, resp)
	}
}

// ---- 漏洞 ----

func OpenVulnsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGet(w, r) {
			return
		}
		var req types.OpenVulnsReq
		if err := httpx.Parse(r, &req); err != nil {
			writeOpenErr(w, http.StatusBadRequest, "参数解析失败: "+err.Error())
			return
		}
		resp, err := logic.NewOpenVulnsLogic(r.Context(), svcCtx).Vulns(&req)
		if err != nil {
			writeOpenLogicErr(w, err)
			return
		}
		writeOpenJSON(w, resp)
	}
}

func OpenVulnDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGet(w, r) {
			return
		}
		var req types.OpenVulnDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			writeOpenErr(w, http.StatusBadRequest, "参数解析失败: "+err.Error())
			return
		}
		resp, err := logic.NewOpenVulnDetailLogic(r.Context(), svcCtx).VulnDetail(&req)
		if err != nil {
			writeOpenLogicErr(w, err)
			return
		}
		writeOpenJSON(w, resp)
	}
}

// ---- 证书 ----

func OpenCertsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGet(w, r) {
			return
		}
		var req types.OpenCertsReq
		if err := httpx.Parse(r, &req); err != nil {
			writeOpenErr(w, http.StatusBadRequest, "参数解析失败: "+err.Error())
			return
		}
		resp, err := logic.NewOpenCertsLogic(r.Context(), svcCtx).Certs(&req)
		if err != nil {
			writeOpenLogicErr(w, err)
			return
		}
		writeOpenJSON(w, resp)
	}
}

func OpenCertDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGet(w, r) {
			return
		}
		var req types.OpenCertDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			writeOpenErr(w, http.StatusBadRequest, "参数解析失败: "+err.Error())
			return
		}
		resp, err := logic.NewOpenCertDetailLogic(r.Context(), svcCtx).CertDetail(&req)
		if err != nil {
			writeOpenLogicErr(w, err)
			return
		}
		writeOpenJSON(w, resp)
	}
}


