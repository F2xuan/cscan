package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssetDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetDetailLogic {
	return &AssetDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetDetail 按需返回单个资产的完整信息（含 body/header/banner 等大字段）。
// 清单列表投影已排除这些大字段以减小 payload，详情抽屉打开时再单独拉取。
func (l *AssetDetailLogic) AssetDetail(req *types.AssetDetailReq) (resp *types.AssetDetailResp, err error) {
	if req.Id == "" {
		return &types.AssetDetailResp{Code: 400, Msg: "资产ID不能为空"}, nil
	}

	assetModel := l.svcCtx.GetAssetModel()
	asset, err := assetModel.FindById(l.ctx, req.Id)
	if err != nil {
		l.Logger.Errorf("[AssetDetail] FindById 失败: %v", err)
		return &types.AssetDetailResp{Code: 500, Msg: "查询失败"}, nil
	}
	if asset == nil {
		return &types.AssetDetailResp{Code: 404, Msg: "资产不存在"}, nil
	}

	return &types.AssetDetailResp{
		Code: 0,
		Msg:  "success",
		Data: convertAssetToInventoryItem(*asset),
	}, nil
}
