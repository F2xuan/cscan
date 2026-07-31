package logic

import (
	"context"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type UserOnboardingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserOnboardingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserOnboardingLogic {
	return &UserOnboardingLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// UserOnboardingStatus 返回当前用户首次引导是否已完成。
// 完成条件：用户已显式完成/跳过引导（OnboardingDone），或当前工作空间已存在扫描任务（老用户不重复弹窗）。
func (l *UserOnboardingLogic) UserOnboardingStatus() (*types.UserOnboardingStatusResp, error) {
	uid := middleware.GetUserId(l.ctx)
	if uid == "" {
		return &types.UserOnboardingStatusResp{Code: 401, Msg: "未认证"}, nil
	}
	user, err := l.svcCtx.UserModel.FindById(l.ctx, uid)
	if err != nil {
		logx.Errorf("[Onboarding] query user failed: %v", err)
		return &types.UserOnboardingStatusResp{Code: 500, Msg: "系统错误"}, nil
	}
	if user == nil {
		return &types.UserOnboardingStatusResp{Code: 404, Msg: "用户不存在"}, nil
	}

	done := user.OnboardingDone
	if !done {
		// 老用户（当前工作空间已有扫描任务）视为已完成引导，避免重复弹出
		if wsId := middleware.GetWorkspaceId(l.ctx); wsId != "" {
			n, cerr := l.svcCtx.GetMainTaskModel(wsId).Count(l.ctx, bson.M{})
			if cerr == nil && n > 0 {
				done = true
			}
		}
	}
	return &types.UserOnboardingStatusResp{Code: 0, Msg: "success", Done: done}, nil
}

// UserOnboardingComplete 标记当前用户首次引导已完成（完成或跳过均调用）
func (l *UserOnboardingLogic) UserOnboardingComplete() (*types.BaseResp, error) {
	uid := middleware.GetUserId(l.ctx)
	if uid == "" {
		return &types.BaseResp{Code: 401, Msg: "未认证"}, nil
	}
	if err := l.svcCtx.UserModel.SetOnboardingDone(l.ctx, uid); err != nil {
		logx.Errorf("[Onboarding] set done failed: %v", err)
		return &types.BaseResp{Code: 500, Msg: "更新失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "success"}, nil
}
