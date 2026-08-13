package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssetUpdateLabelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetUpdateLabelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetUpdateLabelsLogic {
	return &AssetUpdateLabelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// recordLabelHistory 记录标签变更历史到时间线
func recordLabelHistory(ctx context.Context, historyModel *model.AssetHistoryModel, asset *model.Asset, oldLabels, newLabels []string) {
	if historyModel == nil || asset == nil {
		return
	}
	oldStr := sortedJoinLabels(oldLabels)
	newStr := sortedJoinLabels(newLabels)
	if oldStr == newStr {
		return
	}
	history := model.SnapshotFromAsset(asset, "", time.Now(), []model.FieldChange{
		{Field: "labels", OldValue: oldStr, NewValue: newStr},
	})
	if err := historyModel.Insert(ctx, history); err != nil {
		logx.Errorf("[AssetLabels] insert label change history failed: %v", err)
	}
}

func sortedJoinLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	cp := make([]string, len(labels))
	copy(cp, labels)
	sort.Strings(cp)
	return strings.Join(cp, ", ")
}

// AssetUpdateLabels 更新资产标签
func (l *AssetUpdateLabelsLogic) AssetUpdateLabels(req *types.AssetUpdateLabelsReq) (resp *types.BaseResp, err error) {
	assetModel := l.svcCtx.GetAssetModel()
	historyModel := l.svcCtx.GetAssetHistoryModel()

	// 先获取旧标签用于历史记录
	existing, _ := assetModel.FindById(l.ctx, req.Id)

	err = assetModel.UpdateLabels(l.ctx, req.Id, req.Labels)
	if err != nil {
		l.Logger.Errorf("更新资产标签失败: %v", err)
		return &types.BaseResp{
			Code: 1,
			Msg:  "更新失败",
		}, nil
	}

	if existing != nil {
		recordLabelHistory(l.ctx, historyModel, existing, existing.Labels, req.Labels)
	}

	return &types.BaseResp{
		Code: 0,
		Msg:  "success",
	}, nil
}

type AssetAddLabelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetAddLabelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetAddLabelLogic {
	return &AssetAddLabelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetAddLabel 添加资产标签
func (l *AssetAddLabelLogic) AssetAddLabel(req *types.AssetAddLabelReq) (resp *types.BaseResp, err error) {
	assetModel := l.svcCtx.GetAssetModel()
	historyModel := l.svcCtx.GetAssetHistoryModel()

	existing, _ := assetModel.FindById(l.ctx, req.Id)

	err = assetModel.AddLabel(l.ctx, req.Id, req.Label)
	if err != nil {
		l.Logger.Errorf("添加资产标签失败: %v", err)
		return &types.BaseResp{
			Code: 1,
			Msg:  "添加失败",
		}, nil
	}

	if existing != nil {
		newLabels := append(append([]string{}, existing.Labels...), req.Label)
		recordLabelHistory(l.ctx, historyModel, existing, existing.Labels, newLabels)
	}

	return &types.BaseResp{
		Code: 0,
		Msg:  "success",
	}, nil
}

type AssetRemoveLabelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetRemoveLabelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetRemoveLabelLogic {
	return &AssetRemoveLabelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetRemoveLabel 删除资产标签
func (l *AssetRemoveLabelLogic) AssetRemoveLabel(req *types.AssetRemoveLabelReq) (resp *types.BaseResp, err error) {
	assetModel := l.svcCtx.GetAssetModel()
	historyModel := l.svcCtx.GetAssetHistoryModel()

	existing, _ := assetModel.FindById(l.ctx, req.Id)

	err = assetModel.RemoveLabel(l.ctx, req.Id, req.Label)
	if err != nil {
		l.Logger.Errorf("删除资产标签失败: %v", err)
		return &types.BaseResp{
			Code: 1,
			Msg:  "删除失败",
		}, nil
	}

	if existing != nil {
		var newLabels []string
		for _, lb := range existing.Labels {
			if lb != req.Label {
				newLabels = append(newLabels, lb)
			}
		}
		recordLabelHistory(l.ctx, historyModel, existing, existing.Labels, newLabels)
	}

	return &types.BaseResp{
		Code: 0,
		Msg:  "success",
	}, nil
}
