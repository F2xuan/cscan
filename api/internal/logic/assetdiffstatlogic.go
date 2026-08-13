package logic

import (
	"context"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// AssetDiffStatLogic 资产变化快照聚合统计
type AssetDiffStatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetDiffStatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetDiffStatLogic {
	return &AssetDiffStatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetDiffStat 按 diff_type + change_type 聚合计数
func (l *AssetDiffStatLogic) AssetDiffStat(req *types.AssetDiffStatReq) (*types.AssetDiffStatResp, error) {
	diffModel := model.NewScanDiffModel(l.svcCtx.MongoDB)

	var start, end time.Time
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			start = t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			end = t
		}
	}

	items, err := diffModel.Stat(l.ctx, start, end)
	if err != nil {
		l.Errorf("[AssetDiff] stat failed: %v", err)
		return &types.AssetDiffStatResp{Code: -1, Msg: "统计变化失败"}, nil
	}

	var total int64
	list := make([]types.AssetDiffStatItem, 0, len(items))
	for _, it := range items {
		total += it.Count
		list = append(list, types.AssetDiffStatItem{
			DiffType:   it.DiffType,
			ChangeType: it.ChangeType,
			Count:      it.Count,
		})
	}

	return &types.AssetDiffStatResp{
		Code:  0,
		Msg:   "success",
		Total: total,
		List:  list,
	}, nil
}
