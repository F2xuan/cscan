package dirscan

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// DirScanDictListHandler 目录扫描字典列表
func DirScanDictListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanDictListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewDirScanDictListLogic(r.Context(), svcCtx)
		resp, err := l.DirScanDictList(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DirScanDictSaveHandler 保存目录扫描字典
func DirScanDictSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanDictSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewDirScanDictSaveLogic(r.Context(), svcCtx)
		resp, err := l.DirScanDictSave(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DirScanDictDeleteHandler 删除目录扫描字典
func DirScanDictDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirScanDictDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		l := logic.NewDirScanDictDeleteLogic(r.Context(), svcCtx)
		resp, err := l.DirScanDictDelete(&req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DirScanDictClearHandler 清空目录扫描字典
func DirScanDictClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewDirScanDictClearLogic(r.Context(), svcCtx)
		resp, err := l.DirScanDictClear()
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}

// DirScanDictEnabledListHandler 获取启用的目录扫描字典列表（用于任务创建时选择）
func DirScanDictEnabledListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewDirScanDictEnabledListLogic(r.Context(), svcCtx)
		resp, err := l.DirScanDictEnabledList()
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
