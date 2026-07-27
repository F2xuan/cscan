package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// 容器日志分组：容器列表 + 日志批量拉取 + 实时日志流。
func init() {
	tag := "容器日志"
	tagDesc := "Worker 容器列表与日志查看"

	register(http.MethodPost, "/api/v1/container/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "容器列表",
		Description: "返回当前所有 Worker 的容器列表（含端口映射）。",
		RespType:    "ContainerListResp",
		Security:    TierAuth,
	})
	register(http.MethodPost, "/api/v1/container/logs/fetch", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "批量拉取容器日志",
		Description: "按 container / since / until / tail 过滤，分页拉取容器日志。",
		ReqType:     "ContainerLogsFetchReq",
		RespType:    "ContainerLogsFetchResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})
	// /api/v1/container/logs/stream 是 GET 长连接流式接口，不走默认 POST body fallback
	register(http.MethodGet, "/api/v1/container/logs/stream", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "容器日志流",
		Description: "以 SSE 流式输出容器实时日志。Query：container / since / follow。",
		Security:    TierContainer,
	})

	RegisterTypes(
		types.ContainerInfo{},
		types.ContainerPort{},
		types.ContainerListResp{},
		types.ContainerLogsFetchReq{},
		types.ContainerLogLine{},
		types.ContainerLogsFetchResp{},
	)
}
