package logic

import (
	"context"
	"fmt"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/xerr"
)

type CompareVersionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompareVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompareVersionsLogic {
	return &CompareVersionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CompareVersions compares two historical scan versions
func (l *CompareVersionsLogic) CompareVersions(req *types.CompareVersionsReq) (*types.CompareVersionsResp, error) {
	// Validate version IDs
	if req.VersionId1 == "" || req.VersionId2 == "" {
		return nil, xerr.NewParamError("both version IDs are required")
	}

	// Call HistoryService to compare versions
	historyService := svc.NewHistoryService(l.svcCtx.MongoDB)
	compareReq := &svc.CompareVersionsReq{
		VersionId1: req.VersionId1,
		VersionId2: req.VersionId2,
	}

	compareResp, err := historyService.CompareVersions(l.ctx, compareReq)
	if err != nil {
		return nil, fmt.Errorf("failed to compare versions: %w", err)
	}

	// Convert to response format
	return &types.CompareVersionsResp{
		Code: 0,
		Msg:  "success",
		Version1: types.HistoricalVersion{
			VersionId:      compareResp.Version1.VersionId,
			ScanTimestamp:  compareResp.Version1.ScanTimestamp.Format("2006-01-02T15:04:05Z07:00"),
			DirScanCount:   compareResp.Version1.DirScanCount,
			VulnScanCount:  compareResp.Version1.VulnScanCount,
			ChangesSummary: compareResp.Version1.ChangesSummary,
		},
		Version2: types.HistoricalVersion{
			VersionId:      compareResp.Version2.VersionId,
			ScanTimestamp:  compareResp.Version2.ScanTimestamp.Format("2006-01-02T15:04:05Z07:00"),
			DirScanCount:   compareResp.Version2.DirScanCount,
			VulnScanCount:  compareResp.Version2.VulnScanCount,
			ChangesSummary: compareResp.Version2.ChangesSummary,
		},
		DirScansAdded:    compareResp.DirScansAdded,
		DirScansRemoved:  compareResp.DirScansRemoved,
		VulnsAdded:       compareResp.VulnsAdded,
		VulnsRemoved:     compareResp.VulnsRemoved,
		ComparisonDetail: compareResp.ComparisonDetail,
	}, nil
}
