package logic

import (
	"context"
	"fmt"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/xerr"
)

type AssetHistoryV2Logic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetHistoryV2Logic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetHistoryV2Logic {
	return &AssetHistoryV2Logic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetHistoryV2 retrieves historical scan versions for a specific asset
func (l *AssetHistoryV2Logic) AssetHistoryV2(req *types.AssetScanHistoryReq, workspaceId string) (*types.AssetScanHistoryResp, error) {
	// Validate asset ID
	if req.AssetId == "" {
		return nil, xerr.NewParamError("asset_id is required")
	}

	// Fetch asset - 当 workspaceId 为 "all" 时，需要遍历所有工作空间查找资产
	var asset *model.Asset
	var actualWorkspaceId string

	workspaceIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	for _, wsId := range workspaceIds {
		assetModel := model.NewAssetModel(l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName), wsId)
		found, err := assetModel.FindById(l.ctx, req.AssetId)
		if err == nil && found != nil {
			asset = found
			actualWorkspaceId = wsId
			break
		}
	}

	if asset == nil {
		return nil, xerr.NewNotFoundError(fmt.Sprintf("asset not found: %s", req.AssetId))
	}

	// Parse time range if provided
	var startTime, endTime time.Time
	var err error
	if req.StartTime != "" {
		startTime, err = time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time format: %w", err)
		}
	}
	if req.EndTime != "" {
		endTime, err = time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return nil, xerr.NewParamError(fmt.Sprintf("invalid end_time format: %v", err))
		}
	}

	// Create history service and fetch historical versions
	historyService := svc.NewHistoryService(l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName))

	historyReq := &svc.GetResultHistoryReq{
		WorkspaceId: actualWorkspaceId,
		Authority:   asset.Authority,
		Host:        asset.Host,
		Port:        asset.Port,
		StartTime:   startTime,
		EndTime:     endTime,
	}

	historyResp, err := historyService.GetResultHistory(l.ctx, historyReq)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	versions := make([]types.HistoricalVersion, len(historyResp.Versions))
	for i, version := range historyResp.Versions {
		versions[i] = types.HistoricalVersion{
			VersionId:      version.VersionId,
			ScanTimestamp:  version.ScanTimestamp.Format(time.RFC3339),
			DirScanCount:   version.DirScanCount,
			VulnScanCount:  version.VulnScanCount,
			ChangesSummary: version.ChangesSummary,
		}
	}

	// 同时查询字段级变更历史（原 V1 功能），供时间线组件使用
	historyModel := l.svcCtx.GetAssetHistoryModel(actualWorkspaceId)
	histories, _ := historyModel.FindByAssetId(l.ctx, req.AssetId, 20)

	historyList := make([]types.AssetHistoryItem, 0, len(histories))
	for _, h := range histories {
		var changes []types.FieldChange
		for _, c := range h.Changes {
			changes = append(changes, types.FieldChange{
				Field:    c.Field,
				OldValue: c.OldValue,
				NewValue: c.NewValue,
			})
		}
		historyList = append(historyList, types.AssetHistoryItem{
			Id:         h.Id.Hex(),
			Authority:  h.Authority,
			Host:       h.Host,
			Port:       h.Port,
			Service:    h.Service,
			Title:      h.Title,
			App:        h.App,
			HttpStatus: h.HttpStatus,
			TaskId:     h.TaskId,
			CreateTime: h.CreateTime.Local().Format("2006-01-02 15:04:05"),
			Changes:    changes,
		})
	}

	return &types.AssetScanHistoryResp{
		Code:     0,
		Msg:      "success",
		Versions: versions,
		List:     historyList,
	}, nil
}
