package logic

import (
	"context"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const themeConfigKey = "theme_config"

// ThemeConfigGetLogic 获取主题配置
type ThemeConfigGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewThemeConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ThemeConfigGetLogic {
	return &ThemeConfigGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ThemeConfigGetLogic) ThemeConfigGet() (*types.ThemeConfigResp, error) {
	collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")

	var result struct {
		Key    string            `bson:"key"`
		Config types.ThemeConfig `bson:"config"`
	}

	err := collection.FindOne(l.ctx, bson.M{"key": themeConfigKey}).Decode(&result)
	if err != nil {
		// 如果没有配置，返回默认配置
		return &types.ThemeConfigResp{
			Code: 0,
			Msg:  "success",
			Config: &types.ThemeConfig{
				Theme: "system",
			},
		}, nil
	}

	return &types.ThemeConfigResp{
		Code:   0,
		Msg:    "success",
		Config: &result.Config,
	}, nil
}

// ThemeConfigSaveLogic 保存主题配置
type ThemeConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewThemeConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ThemeConfigSaveLogic {
	return &ThemeConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ThemeConfigSaveLogic) ThemeConfigSave(req *types.ThemeConfigSaveReq) (*types.ThemeConfigResp, error) {
	collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")

	config := types.ThemeConfig{
		Theme:      req.Theme,
		ThemeStyle: req.ThemeStyle,
		UpdateTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 使用upsert更新或插入配置
	filter := bson.M{"key": themeConfigKey}
	update := bson.M{
		"$set": bson.M{
			"key":    themeConfigKey,
			"config": config,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(l.ctx, filter, update, opts)
	if err != nil {
		return &types.ThemeConfigResp{
			Code: 500,
			Msg:  "保存失败: " + err.Error(),
		}, nil
	}

	return &types.ThemeConfigResp{
		Code:   0,
		Msg:    "保存成功",
		Config: &config,
	}, nil
}
