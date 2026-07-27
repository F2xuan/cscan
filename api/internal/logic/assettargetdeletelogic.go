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
	"go.mongodb.org/mongo-driver/bson"
)

type AssetTargetDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetTargetDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetTargetDeleteLogic {
	return &AssetTargetDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetTargetDelete 删除顶层资产元信息。
// delete_assets=true 时连带删除 {wsId}_asset 中匹配该目标的所有资产 + 关联 {wsId}_vul，
// 以及该目标的目录扫描结果（dirscan_result）与 JSFinder 结果（{wsId}_jsfinder）。
// delete_assets=false 仅删 meta 记录，业务数据保持原样。
func (l *AssetTargetDeleteLogic) AssetTargetDelete(req *types.AssetTargetDeleteReq, workspaceId string) (*types.AssetTargetDeleteResp, error) {
	targetId := strings.TrimSpace(req.TargetId)
	if targetId == "" {
		return nil, fmt.Errorf("targetId is empty")
	}
	tType, tValue, err := model.DecodeTargetID(targetId)
	if err != nil {
		return nil, err
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	owningWs := locateOwningWsMeta(l.ctx, l.svcCtx, wsIds, targetId)
	if owningWs == "" {
		return nil, fmt.Errorf("target %s not found", targetId)
	}

	var deletedAssets int64
	if req.DeleteAssets {
		hostFilter := hostFilterForTarget(tType, tValue)
		assetModel := l.svcCtx.GetAssetModel(owningWs)
		n, err := assetModel.DeleteByFilter(l.ctx, bson.M{"host": hostFilter})
		if err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete assets ws=%s fail: %v", owningWs, err)
		} else {
			deletedAssets = n
		}
		vulModel := l.svcCtx.GetVulModel(owningWs)
		if _, err := vulModel.DeleteByFilter(l.ctx, bson.M{"host": hostFilter}); err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete vul ws=%s fail: %v", owningWs, err)
		}

		// 级联清理目录扫描结果（全局 dirscan_result 集合，按 workspace_id + host 限定）
		dirModel := l.svcCtx.GetDirScanResultModel()
		if dirModel != nil {
			dirFilter := bson.M{"workspace_id": owningWs, "host": hostFilter}
			if _, err := dirModel.DeleteByFilter(l.ctx, dirFilter); err != nil {
				l.Logger.Errorf("[AssetTargetDelete] delete dirscan ws=%s fail: %v", owningWs, err)
			}
		}

		// 级联清理 JSFinder 结果（per-ws 集合 {wsId}_jsfinder，按 host 限定）
		jsModel := l.svcCtx.GetJSFinderResultModel(owningWs)
		if _, err := jsModel.DeleteMany(l.ctx, bson.M{"host": hostFilter}); err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete jsfinder ws=%s fail: %v", owningWs, err)
		}
	}

	if err := l.svcCtx.GetAssetTargetMetaModel(owningWs).Delete(l.ctx, targetId); err != nil {
		return nil, fmt.Errorf("delete meta fail: %w", err)
	}
	invalidateAssetTargetCaches(l.svcCtx, targetId)
	return &types.AssetTargetDeleteResp{
		Code:         0,
		Msg:          "success",
		DeletedCount: deletedAssets,
	}, nil
}
