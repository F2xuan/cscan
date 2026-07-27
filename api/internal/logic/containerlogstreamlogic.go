package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ContainerLogStreamLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewContainerLogStreamLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContainerLogStreamLogic {
	return &ContainerLogStreamLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// FetchLogs 一次性拉取最近 N 行日志,用于导出或 SSE 降级 fallback
func (l *ContainerLogStreamLogic) FetchLogs(req *types.ContainerLogsFetchReq) (*types.ContainerLogsFetchResp, error) {
	if l.svcCtx.DockerService == nil {
		return &types.ContainerLogsFetchResp{Code: 503, Msg: "docker service unavailable"}, nil
	}
	lines, err := l.svcCtx.DockerService.FetchLogs(l.ctx, req.Name, req.Tail, req.Since)
	if err != nil {
		l.Errorf("[ContainerLogsFetch] name=%s err=%v", req.Name, err)
		return &types.ContainerLogsFetchResp{Code: 500, Msg: err.Error()}, nil
	}
	out := make([]types.ContainerLogLine, 0, len(lines))
	for _, ln := range lines {
		out = append(out, types.ContainerLogLine{Stream: ln.Stream, TS: ln.TS, Line: ln.Line})
	}
	return &types.ContainerLogsFetchResp{Code: 0, Msg: "success", List: out}, nil
}
