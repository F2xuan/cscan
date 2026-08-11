package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// UserRegisterLogic 公开注册逻辑
type UserRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserRegisterLogic {
	return &UserRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Register 处理公开注册请求
func (l *UserRegisterLogic) Register(req *types.RegisterReq) (*types.BaseResp, error) {
	username := req.Username
	password := req.Password

	// 1. 密码强度校验
	if err := model.ValidatePasswordStrength(password); err != nil {
		return nil, xerr.NewParamError(err.Error())
	}

	// 2. 用户名唯一性检查
	existing, err := l.svcCtx.UserModel.FindByUsername(l.ctx, username)
	if err != nil {
		logx.Errorf("[Register] FindByUsername failed: %v", err)
		return nil, xerr.NewServerError("注册失败，请稍后重试")
	}
	if existing != nil {
		return nil, xerr.NewParamError("用户名已存在")
	}

	// 3. 查询当前用户总数
	total, err := l.svcCtx.UserModel.Count(l.ctx, bson.M{})
	if err != nil {
		logx.Errorf("[Register] Count users failed: %v", err)
		return nil, xerr.NewServerError("注册失败，请稍后重试")
	}

	var role, status string

	// 4. 首装用户：无条件成为 superadmin + enable
	if total == 0 {
		role = "superadmin"
		status = model.StatusEnable
	} else {
		// 5. 读取注册配置
		collection := l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName).Collection("system_config")
		var result struct {
			Config types.RegistrationConfig `bson:"config"`
		}
		err := collection.FindOne(l.ctx, bson.M{"key": registrationConfigKey}).Decode(&result)
		if err != nil {
			// 未配置默认关闭注册
			return nil, xerr.NewCodeErrorMsg(403, "注册功能未开放，请联系管理员")
		}

		cfg := result.Config
		// 注册功能关闭
		if !cfg.Enabled {
			return nil, xerr.NewCodeErrorMsg(403, "注册功能未开放，请联系管理员")
		}

		role = "user"
		if cfg.RequireApproval {
			status = "pending"
		} else {
			status = model.StatusEnable
		}
	}

	// 6. 插入用户（密码传明文，UserModel.Insert 内部统一 bcrypt 加密一次）
	userDoc := &model.User{
		Username: username,
		Password: password,
		Role:     role,
		Status:   status,
	}
	if err := l.svcCtx.UserModel.Insert(l.ctx, userDoc); err != nil {
		logx.Errorf("[Register] Insert user failed: %v", err)
		return nil, xerr.NewServerError("注册失败，请稍后重试")
	}

	if status == "pending" {
		return &types.BaseResp{
			Code: 0,
			Msg:  "注册成功，请等待管理员审核",
		}, nil
	}

	return &types.BaseResp{
		Code: 0,
		Msg:  "注册成功",
	}, nil
}
