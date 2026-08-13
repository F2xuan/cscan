package ai

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func GeneratePocHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GeneratePocReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.WriteJson(w, http.StatusOK, &types.GeneratePocResp{Code: 400, Msg: err.Error()})
			return
		}

		l := logic.NewGeneratePocLogic(r.Context(), svcCtx)
		resp, err := l.GeneratePoc(&req)
		if err != nil {
			httpx.WriteJson(w, http.StatusOK, &types.GeneratePocResp{Code: 500, Msg: err.Error()})
			return
		}

		httpx.WriteJson(w, http.StatusOK, resp)
	}
}

// AIConfigGetHandler 获取AI配置
func AIConfigGetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewAIConfigLogic(r.Context(), svcCtx)
		resp, err := l.GetConfig()
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.WriteJson(w, http.StatusOK, resp)
	}
}

// AIConfigSaveHandler 保存AI配置
func AIConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AIConfigSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewAIConfigLogic(r.Context(), svcCtx)
		resp, err := l.SaveConfig(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.WriteJson(w, http.StatusOK, resp)
	}
}
