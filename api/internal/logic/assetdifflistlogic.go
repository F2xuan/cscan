package logic

import (
	"context"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// AssetDiffListLogic 资产变化快照列表
type AssetDiffListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetDiffListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetDiffListLogic {
	return &AssetDiffListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetDiffList 按任务/时间范围/类型查询变化明细
func (l *AssetDiffListLogic) AssetDiffList(req *types.AssetDiffListReq) (*types.AssetDiffListResp, error) {
	diffModel := model.NewScanDiffModel(l.svcCtx.MongoDB)

	page := int64(req.Page)
	if page <= 0 {
		page = 1
	}
	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	pageSize := int64(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}

	docs, total, err := diffModel.FindByTaskId(l.ctx, req.TaskId, req.DiffType, req.ChangeType, page, pageSize)
	if err != nil {
		l.Errorf("[AssetDiff] list query failed: %v", err)
		return &types.AssetDiffListResp{Code: -1, Msg: "查询变化列表失败"}, nil
	}

	list := make([]types.AssetDiffItem, 0, len(docs))
	for _, d := range docs {
		item := types.AssetDiffItem{
			Id:         d.Id.Hex(),
			TaskId:     d.TaskId,
			DiffType:   d.DiffType,
			ChangeType: d.ChangeType,
			TargetKey:  d.TargetKey,
			Summary:    d.Summary,
			Changes:    toTypesFieldChanges(d.Changes),
		}
		if !d.CreateTime.IsZero() {
			item.CreateTime = d.CreateTime.Format(time.RFC3339)
		}
		list = append(list, item)
	}

	return &types.AssetDiffListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  list,
	}, nil
}

func toTypesFieldChanges(changes []model.FieldChange) []types.FieldChange {
	if len(changes) == 0 {
		return nil
	}
	out := make([]types.FieldChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, types.FieldChange{
			Field:    c.Field,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		})
	}
	return out
}
