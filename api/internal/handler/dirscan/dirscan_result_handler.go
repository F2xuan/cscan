package dirscan

import (
	"net/http"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/model"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ==================== 目录扫描结果 API ====================

// DirScanResultStatReq 统计请求（不需要参数，从 context 获取 workspaceId）
type DirScanResultStatReq struct{}

// DirScanResultStatResp 统计响应
type DirScanResultStatResp struct {
	Code int              `json:"code"`
	Msg  string           `json:"msg"`
	Stat map[string]int64 `json:"stat"`
}

// DirScanResultStatHandler 目录扫描结果统计
func DirScanResultStatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceId := middleware.GetWorkspaceId(ctx)
		resultModel := model.NewDirScanResultModel(svcCtx.MongoDB)

		stat, err := resultModel.Stat(ctx, workspaceId)
		if err != nil {
			httpx.OkJson(w, &DirScanResultStatResp{Code: 500, Msg: "统计失败: " + err.Error()})
			return
		}

		httpx.OkJson(w, &DirScanResultStatResp{
			Code: 0,
			Msg:  "success",
			Stat: stat,
		})
	}
}

// DirScanResultDeleteReq 删除请求
type DirScanResultDeleteReq struct {
	Id string `json:"id"`
}

// DirScanResultDeleteResp 删除响应
type DirScanResultDeleteResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// DirScanResultDeleteHandler 删除目录扫描结果
func DirScanResultDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DirScanResultDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &DirScanResultDeleteResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.Id == "" {
			httpx.OkJson(w, &DirScanResultDeleteResp{Code: 400, Msg: "ID不能为空"})
			return
		}

		ctx := r.Context()
		resultModel := model.NewDirScanResultModel(svcCtx.MongoDB)

		if err := resultModel.Delete(ctx, req.Id); err != nil {
			httpx.OkJson(w, &DirScanResultDeleteResp{Code: 500, Msg: "删除失败: " + err.Error()})
			return
		}

		httpx.OkJson(w, &DirScanResultDeleteResp{Code: 0, Msg: "删除成功"})
	}
}

// DirScanResultBatchDeleteReq 批量删除请求
type DirScanResultBatchDeleteReq struct {
	Ids []string `json:"ids"`
}

// DirScanResultBatchDeleteResp 批量删除响应
type DirScanResultBatchDeleteResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Deleted int64  `json:"deleted"`
}

// DirScanResultBatchDeleteHandler 批量删除目录扫描结果
func DirScanResultBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DirScanResultBatchDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.OkJson(w, &DirScanResultBatchDeleteResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if len(req.Ids) == 0 {
			httpx.OkJson(w, &DirScanResultBatchDeleteResp{Code: 400, Msg: "ID列表不能为空"})
			return
		}

		ctx := r.Context()
		resultModel := model.NewDirScanResultModel(svcCtx.MongoDB)

		deleted, err := resultModel.DeleteByIds(ctx, req.Ids)
		if err != nil {
			httpx.OkJson(w, &DirScanResultBatchDeleteResp{Code: 500, Msg: "删除失败: " + err.Error()})
			return
		}

		httpx.OkJson(w, &DirScanResultBatchDeleteResp{
			Code:    0,
			Msg:     "删除成功",
			Deleted: deleted,
		})
	}
}

// DirScanResultClearReq 清空请求（不需要参数，从 context 获取 workspaceId）
type DirScanResultClearReq struct{}

// DirScanResultClearResp 清空响应
type DirScanResultClearResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Deleted int64  `json:"deleted"`
}

// DirScanResultClearHandler 清空目录扫描结果
func DirScanResultClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceId := middleware.GetWorkspaceId(ctx)
		resultModel := model.NewDirScanResultModel(svcCtx.MongoDB)

		deleted, err := resultModel.DeleteByWorkspace(ctx, workspaceId)
		if err != nil {
			httpx.OkJson(w, &DirScanResultClearResp{Code: 500, Msg: "清空失败: " + err.Error()})
			return
		}

		httpx.OkJson(w, &DirScanResultClearResp{
			Code:    0,
			Msg:     "清空成功",
			Deleted: deleted,
		})
	}
}
