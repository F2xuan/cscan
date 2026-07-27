package logic

import (
	"context"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const brandingConfigKey = "branding_config"

// 单张 Logo 上限：约 1MB base64（≈ 750KB 原图），避免异常大对象写入
const brandingLogoMaxBytes = 1 << 20

// BrandingConfigGetLogic 获取品牌配置
type BrandingConfigGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBrandingConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BrandingConfigGetLogic {
	return &BrandingConfigGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BrandingConfigGetLogic) BrandingConfigGet() (*types.BrandingConfigResp, error) {
	collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")

	var result struct {
		Key    string               `bson:"key"`
		Config types.BrandingConfig `bson:"config"`
	}

	err := collection.FindOne(l.ctx, bson.M{"key": brandingConfigKey}).Decode(&result)
	if err != nil {
		// 未配置时返回默认值（空 LogoData 表示前端使用默认 /logo.png）
		return &types.BrandingConfigResp{
			Code: 0,
			Msg:  "success",
			Config: &types.BrandingConfig{
				LogoData: "",
				Title:    "CSCAN",
			},
		}, nil
	}

	return &types.BrandingConfigResp{
		Code:   0,
		Msg:    "success",
		Config: &result.Config,
	}, nil
}

// BrandingConfigSaveLogic 保存品牌配置
type BrandingConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBrandingConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BrandingConfigSaveLogic {
	return &BrandingConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BrandingConfigSaveLogic) BrandingConfigSave(req *types.BrandingConfigSaveReq) (*types.BrandingConfigResp, error) {
	logoData := strings.TrimSpace(req.LogoData)
	title := strings.TrimSpace(req.Title)

	if logoData != "" {
		if len(logoData) > brandingLogoMaxBytes {
			return nil, xerr.NewParamError("Logo 大小超过 1MB 限制")
		}
		// 仅接受 http(s) URL 或 data:image/* base64
		if !strings.HasPrefix(logoData, "http://") &&
			!strings.HasPrefix(logoData, "https://") &&
			!strings.HasPrefix(logoData, "data:image/") {
			return nil, xerr.NewParamError("Logo 仅支持 http(s) URL 或 data:image/* 图片")
		}
	}

	if len([]rune(title)) > 32 {
		return nil, xerr.NewParamError("标题最多 32 个字符")
	}

	config := types.BrandingConfig{
		LogoData:   logoData,
		Title:      title,
		UpdateTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")
	filter := bson.M{"key": brandingConfigKey}
	update := bson.M{
		"$set": bson.M{
			"key":    brandingConfigKey,
			"config": config,
		},
	}
	if _, err := collection.UpdateOne(l.ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
		return nil, xerr.NewServerError("保存品牌配置失败: " + err.Error())
	}

	return &types.BrandingConfigResp{
		Code:   0,
		Msg:    "保存成功",
		Config: &config,
	}, nil
}
