package jsfinder

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// JSFinderListHandler 获取 JSFinder 结果列表
func JSFinderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		resp, err := l.GetJSFinderList(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			// 直接返回，不使用 response.Success 包装，以匹配其他列表接口的返回格式
			httpx.OkJson(w, resp)
		}
	}
}

// JSFinderClearHandler 清空 JSFinder 结果
func JSFinderClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		err := l.ClearJSFinderResults()
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, map[string]any{
				"code": 0,
				"msg":  "清空成功",
			})
		}
	}
}
