package logic

import (
	"context"

	"cscan/internal/model"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveTaskResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveTaskResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveTaskResultLogic {
	return &SaveTaskResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SaveTaskResultLogic) SaveTaskResult(in *pb.SaveTaskResultReq) (*pb.SaveTaskResultResp, error) {
	if len(in.Assets) == 0 {
		return &pb.SaveTaskResultResp{
			Success: true,
			Message: "No assets to save",
		}, nil
	}

	workspaceId := in.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	scannerAssets := make([]*model.ScannerAsset, 0, len(in.Assets))
	for _, pbAsset := range in.Assets {
		sa := &model.ScannerAsset{
			Authority:  pbAsset.Authority,
			Host:       pbAsset.Host,
			Port:       int(pbAsset.Port),
			Category:   pbAsset.Category,
			Service:    pbAsset.Service,
			Title:      pbAsset.Title,
			App:        pbAsset.App,
			HttpStatus: pbAsset.HttpStatus,
			HttpHeader: pbAsset.HttpHeader,
			HttpBody:   pbAsset.HttpBody,
			IconHash:   pbAsset.IconHash,
			IconData:   pbAsset.IconData,
			Screenshot: pbAsset.Screenshot,
			Server:     pbAsset.Server,
			Banner:     pbAsset.Banner,
			IsHTTP:     pbAsset.IsHttp,
			Source:     pbAsset.Source,
			CName:      pbAsset.Cname,
		}

		for _, ip := range pbAsset.Ipv4 {
			sa.IPV4 = append(sa.IPV4, model.ScannerIPInfo{
				IP:       ip.Ip,
				Location: ip.Location,
			})
		}
		for _, ip := range pbAsset.Ipv6 {
			sa.IPV6 = append(sa.IPV6, model.ScannerIPInfo{
				IP:       ip.Ip,
				Location: ip.Location,
			})
		}

		scannerAssets = append(scannerAssets, sa)
	}

	writeService := model.NewAssetWriteService(l.svcCtx.MongoDB, workspaceId)
	result, err := writeService.SaveAssets(l.ctx, in.MainTaskId, in.OrgId, scannerAssets)
	if err != nil {
		return nil, err
	}

	return &pb.SaveTaskResultResp{
		Success:     result.FailedWrites == 0,
		Message:     "Assets saved successfully",
		TotalAsset:  result.TotalAsset,
		NewAsset:    result.NewAsset,
		UpdateAsset: result.UpdateAsset,
	}, nil
}
