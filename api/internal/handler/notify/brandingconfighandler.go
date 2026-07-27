package notify

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// BrandingConfigGetHandler 获取品牌配置（Logo / 标题）
func BrandingConfigGetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewBrandingConfigGetLogic(r.Context(), svcCtx)
		resp, err := l.BrandingConfigGet()
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// BrandingConfigSaveHandler 保存品牌配置（Logo / 标题）
func BrandingConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BrandingConfigSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewBrandingConfigSaveLogic(r.Context(), svcCtx)
		resp, err := l.BrandingConfigSave(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
