package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ContainerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewContainerListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContainerListLogic {
	return &ContainerListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ContainerListLogic) ContainerList() (*types.ContainerListResp, error) {
	if l.svcCtx.DockerService == nil {
		return &types.ContainerListResp{Code: 503, Msg: "docker service unavailable"}, nil
	}
	list, err := l.svcCtx.DockerService.ListCscanContainers(l.ctx)
	if err != nil {
		l.Errorf("[ContainerList] err=%v", err)
		return &types.ContainerListResp{Code: 500, Msg: err.Error()}, nil
	}
	out := make([]types.ContainerInfo, 0, len(list))
	for _, c := range list {
		ports := make([]types.ContainerPort, 0, len(c.Ports))
		for _, p := range c.Ports {
			ports = append(ports, types.ContainerPort{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}
		out = append(out, types.ContainerInfo{
			ID:     c.ID,
			Name:   c.Name,
			Image:  c.Image,
			State:  c.State,
			Status: c.Status,
			Ports:  ports,
			Labels: c.Labels,
		})
	}
	return &types.ContainerListResp{Code: 0, Msg: "success", List: out}, nil
}
