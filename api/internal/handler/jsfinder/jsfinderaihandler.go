package jsfinder

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// JSFinderAIAnalyzeHandler 单条AI研判
func JSFinderAIAnalyzeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderAIAnalyzeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		resp, err := l.AnalyzeSingle(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// JSFinderAIBatchAnalyzeHandler 批量研判（异步启动）
func JSFinderAIBatchAnalyzeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderAIBatchAnalyzeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}
		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		resp, err := l.BatchAnalyzeAsync(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// JSFinderAIBatchProgressHandler 批量研判进度查询
func JSFinderAIBatchProgressHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderAIBatchProgressReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		resp, err := l.GetBatchProgress(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}

// JSFinderAIStopBatchHandler 停止批量研判
func JSFinderAIStopBatchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.JSFinderAIStopBatchReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewJSFinderLogic(r.Context(), svcCtx)
		err := l.StopBatchTask(req.TaskId)
		if err != nil {
			httpx.OkJson(w, &types.JSFinderAIStopBatchResp{Code: 500, Msg: err.Error()})
		} else {
			httpx.OkJson(w, &types.JSFinderAIStopBatchResp{Code: 0, Msg: "停止指令已发送"})
		}
	}
}
