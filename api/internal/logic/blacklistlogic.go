package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// BlacklistLogic 黑名单逻辑
type BlacklistLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewBlacklistLogic 创建黑名单逻辑
func NewBlacklistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BlacklistLogic {
	return &BlacklistLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetBlacklistConfig 获取黑名单配置
func (l *BlacklistLogic) GetBlacklistConfig() (*types.BlacklistConfigResp, error) {
	blacklistModel := model.NewBlacklistConfigModel(l.svcCtx.MongoDB)
	doc, err := blacklistModel.Get(l.ctx)
	if err != nil {
		l.Errorf("GetBlacklistConfig error: %v", err)
		return &types.BlacklistConfigResp{
			Code: -1,
			Msg:  "获取黑名单配置失败",
		}, nil
	}

	return &types.BlacklistConfigResp{
		Code: 0,
		Msg:  "success",
		Data: &types.BlacklistConfig{
			Rules:      doc.Rules,
			Status:     doc.Status,
			UpdateTime: doc.UpdateTime.Format("2006-01-02 15:04:05"),
		},
	}, nil
}

// SaveBlacklistConfig 保存黑名单配置
func (l *BlacklistLogic) SaveBlacklistConfig(req *types.BlacklistConfigSaveReq) (*types.BaseResp, error) {
	blacklistModel := model.NewBlacklistConfigModel(l.svcCtx.MongoDB)

	doc := &model.BlacklistConfig{
		Rules:  req.Rules,
		Status: req.Status,
	}

	if doc.Status == "" {
		doc.Status = "enable"
	}

	err := blacklistModel.Upsert(l.ctx, doc)
	if err != nil {
		l.Errorf("SaveBlacklistConfig error: %v", err)
		return &types.BaseResp{
			Code: -1,
			Msg:  "保存黑名单配置失败",
		}, nil
	}

	// 更新缓存到Redis（供Worker使用）
	l.updateBlacklistCache(req.Rules, req.Status)

	return &types.BaseResp{
		Code: 0,
		Msg:  "保存成功",
	}, nil
}

// updateBlacklistCache 更新黑名单缓存到Redis
func (l *BlacklistLogic) updateBlacklistCache(rules, status string) {
	if l.svcCtx.RedisClient == nil {
		return
	}

	ctx := context.Background()
	key := "cscan:blacklist:rules"

	if status != "enable" {
		// 禁用时删除缓存
		l.svcCtx.RedisClient.Del(ctx, key)
		return
	}

	// 启用时更新缓存
	l.svcCtx.RedisClient.Set(ctx, key, rules, 0)
}

// GetBlacklistRules 获取黑名单规则列表（供Worker调用）
func (l *BlacklistLogic) GetBlacklistRules() (*types.BlacklistRulesResp, error) {
	blacklistModel := model.NewBlacklistConfigModel(l.svcCtx.MongoDB)
	rules, err := blacklistModel.GetRules(l.ctx)
	if err != nil {
		l.Errorf("GetBlacklistRules error: %v", err)
		return &types.BlacklistRulesResp{
			Code: -1,
			Msg:  "获取黑名单规则失败",
		}, nil
	}

	return &types.BlacklistRulesResp{
		Code:  0,
		Msg:   "success",
		Rules: rules,
	}, nil
}
