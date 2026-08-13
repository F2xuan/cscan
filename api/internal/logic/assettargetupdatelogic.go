package logic

import (
	"context"
	"fmt"
	"strings"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/xerr"

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
func (l *AssetTargetUpdateLogic) AssetTargetUpdate(req *types.AssetTargetUpdateReq) error {
	targetId := strings.TrimSpace(req.TargetId)
	if targetId == "" {
		return fmt.Errorf("targetId is empty")
	}
	if _, _, err := model.DecodeTargetID(targetId); err != nil {
		return err
	}

	if !targetMetaExists(l.ctx, l.svcCtx, targetId) {
		return xerr.NewNotFoundError(fmt.Sprintf("target %s not found", targetId))
	}

	metaModel := l.svcCtx.GetAssetTargetMetaModel()

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
