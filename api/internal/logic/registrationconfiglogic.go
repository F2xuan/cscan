package logic

import (
	"context"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const registrationConfigKey = "registration_config"

// RegistrationConfigGetLogic 获取注册配置
type RegistrationConfigGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegistrationConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegistrationConfigGetLogic {
	return &RegistrationConfigGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegistrationConfigGetLogic) Get() (*types.RegistrationConfigResp, error) {
	collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")

	var result struct {
		Key    string                   `bson:"key"`
		Config types.RegistrationConfig `bson:"config"`
	}

	err := collection.FindOne(l.ctx, bson.M{"key": registrationConfigKey}).Decode(&result)
	if err != nil {
		// 未配置时返回默认值：关闭注册 + 需要审核
		return &types.RegistrationConfigResp{
			Code: 0,
			Msg:  "success",
			Config: &types.RegistrationConfig{
				Enabled:         false,
				RequireApproval: true,
				UpdateTime:      time.Now().Format("2006-01-02 15:04:05"),
			},
		}, nil
	}

	return &types.RegistrationConfigResp{
		Code:   0,
		Msg:    "success",
		Config: &result.Config,
	}, nil
}

// RegistrationConfigSaveLogic 保存注册配置
type RegistrationConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegistrationConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegistrationConfigSaveLogic {
	return &RegistrationConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegistrationConfigSaveLogic) Save(req *types.RegistrationConfigSaveReq) (*types.RegistrationConfigResp, error) {
	config := types.RegistrationConfig{
		Enabled:         req.Enabled,
		RequireApproval: req.RequireApproval,
		UpdateTime:      time.Now().Format("2006-01-02 15:04:05"),
	}

	collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")
	filter := bson.M{"key": registrationConfigKey}
	update := bson.M{
		"$set": bson.M{
			"key":    registrationConfigKey,
			"config": config,
		},
	}
	if _, err := collection.UpdateOne(l.ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
		return nil, xerr.NewServerError("保存注册配置失败: " + err.Error())
	}

	return &types.RegistrationConfigResp{
		Code:   0,
		Msg:    "保存成功",
		Config: &config,
	}, nil
}
