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
// delete_assets=true 时连带删除匹配该目标的所有资产（含 host 和 domain/ip 维度）、
// 关联漏洞、目录扫描结果、JSFinder 结果及资产历史记录。
// delete_assets=false 仅删 meta 记录，业务数据保持原样。
func (l *AssetTargetDeleteLogic) AssetTargetDelete(req *types.AssetTargetDeleteReq) (*types.AssetTargetDeleteResp, error) {
	targetId := strings.TrimSpace(req.TargetId)
	if targetId == "" {
		return nil, fmt.Errorf("targetId is empty")
	}
	tType, tValue, err := model.DecodeTargetID(targetId)
	if err != nil {
		return nil, err
	}

	if !targetMetaExists(l.ctx, l.svcCtx, targetId) {
		return nil, xerr.NewNotFoundError(fmt.Sprintf("target %s not found", targetId))
	}

	var deletedAssets int64
	if req.DeleteAssets {
		// 构建综合过滤条件，同时匹配 host 和 domain/ip 字段，
		// 确保 host 为 IP 但 domain 匹配的关联资产也能被级联删除。
		hostFilter := hostFilterForTarget(tType, tValue)
		var cascadeFilter bson.M
		if tType == model.AssetTargetTypeIP {
			cascadeFilter = bson.M{"$or": []bson.M{
				{"host": tValue},
				{"ip.ipv4.ip": tValue},
			}}
		} else {
			cascadeFilter = bson.M{"$or": []bson.M{
				{"host": hostFilter},
				{"domain": hostFilter},
			}}
		}

		assetModel := l.svcCtx.GetAssetModel()
		n, err := assetModel.DeleteByFilter(l.ctx, cascadeFilter)
		if err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete assets fail: %v", err)
		} else {
			deletedAssets = n
		}
		vulModel := l.svcCtx.GetVulModel()
		if _, err := vulModel.DeleteByFilter(l.ctx, cascadeFilter); err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete vul fail: %v", err)
		}

		// 级联清理目录扫描结果（按 host/domain 限定）
		dirModel := l.svcCtx.GetDirScanResultModel()
		if dirModel != nil {
			dirFilter := bson.M{"host": hostFilter}
			if _, err := dirModel.DeleteByFilter(l.ctx, dirFilter); err != nil {
				l.Logger.Errorf("[AssetTargetDelete] delete dirscan fail: %v", err)
			}
		}

		// 级联清理 JSFinder 结果（按 host 限定）
		jsModel := l.svcCtx.GetJSFinderResultModel()
		if _, err := jsModel.DeleteMany(l.ctx, bson.M{"host": hostFilter}); err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete jsfinder fail: %v", err)
		}

		// 级联清理资产历史记录
		histModel := l.svcCtx.GetAssetHistoryModel()
		if _, err := histModel.DeleteByFilter(l.ctx, cascadeFilter); err != nil {
			l.Logger.Errorf("[AssetTargetDelete] delete asset history fail: %v", err)
		}
	}

	if err := l.svcCtx.GetAssetTargetMetaModel().Delete(l.ctx, targetId); err != nil {
		return nil, fmt.Errorf("delete meta fail: %w", err)
	}
	invalidateAssetTargetCaches(l.svcCtx, targetId)
	return &types.AssetTargetDeleteResp{
		Code:         0,
		Msg:          "success",
		DeletedCount: deletedAssets,
	}, nil
}
