package container

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// ContainerListHandler 列出 cscan 相关 Docker 容器
// POST /api/v1/container/list
func ContainerListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewContainerListLogic(r.Context(), svcCtx)
		resp, err := l.ContainerList()
		if err != nil {
			logx.Errorf("[ContainerList] err=%v", err)
			response.Error(w, err)
			return
		}
		// resp 自身已含 code/msg/list 字段,直接 OkJson 避免外层再套 data
		httpx.OkJson(w, resp)
	}
}

// ContainerLogsFetchHandler 一次性拉取最近 N 行日志(降级/导出用)
// POST /api/v1/container/logs/fetch
func ContainerLogsFetchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ContainerLogsFetchReq
		if err := parseBody(r, &req); err != nil {
			response.ParamError(w, "invalid request body")
			return
		}
		if req.Name == "" {
			response.ParamError(w, "name is required")
			return
		}
		l := logic.NewContainerLogStreamLogic(r.Context(), svcCtx)
		resp, err := l.FetchLogs(&req)
		if err != nil {
			logx.Errorf("[ContainerLogsFetch] name=%s err=%v", req.Name, err)
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
