package logic

import (
	"context"
	"fmt"
	"strings"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssetTargetUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetTargetUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetTargetUpdateLogic {
	return &AssetTargetUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetTargetUpdate 更新顶层资产的用户字段（labels/memo/color_tag）。
// labels 为全量覆盖；memo/color_tag 仅在 req 字段非空时更新。
// 跨 workspace 解析 owning ws，非 admin 操作非本 ws 返回 Forbidden。
func (l *AssetTargetUpdateLogic) AssetTargetUpdate(req *types.AssetTargetUpdateReq, workspaceId string) error {
	targetId := strings.TrimSpace(req.TargetId)
	if targetId == "" {
		return fmt.Errorf("targetId is empty")
	}
	if _, _, err := model.DecodeTargetID(targetId); err != nil {
		return err
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	owningWs := locateOwningWsMeta(l.ctx, l.svcCtx, wsIds, targetId)
	if owningWs == "" {
		return fmt.Errorf("target %s not found", targetId)
	}

	metaModel := l.svcCtx.GetAssetTargetMetaModel(owningWs)

	if req.Labels != nil {
		if err := metaModel.UpdateLabels(l.ctx, targetId, req.Labels); err != nil {
			return fmt.Errorf("update labels fail: %w", err)
		}
	}

	if strings.TrimSpace(req.Memo) != "" {
		if err := metaModel.UpdateMemo(l.ctx, targetId, req.Memo); err != nil {
			return fmt.Errorf("update memo fail: %w", err)
		}
	}

	if strings.TrimSpace(req.ColorTag) != "" {
		if err := metaModel.UpdateColorTag(l.ctx, targetId, req.ColorTag); err != nil {
			return fmt.Errorf("update color_tag fail: %w", err)
		}
	}

	invalidateAssetTargetCaches(l.svcCtx, targetId)
	return nil
}
