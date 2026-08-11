package logic

import (
	"context"
	"fmt"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserApproveLogic 用户审核逻辑（仅允许 pending → enable）
type UserApproveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserApproveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserApproveLogic {
	return &UserApproveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Approve 审核用户（通过或拒绝）
func (l *UserApproveLogic) Approve(req *types.UserApproveReq) (*types.BaseResp, error) {
	oid, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, xerr.NewParamError("无效的用户ID")
	}

	// 获取用户详情以检查状态
	user, err := l.svcCtx.UserModel.FindByObjectId(l.ctx, oid)
	if err != nil {
		logx.Errorf("[UserApprove] FindByObjectId failed: id=%s err=%v", req.Id, err)
		return nil, xerr.NewServerError("用户不存在")
	}
	if user == nil {
		return nil, xerr.NewNotFoundError("用户不存在")
	}

	// 仅 pending 状态可审核
	if user.Status != "pending" {
		return nil, xerr.NewParamError(fmt.Sprintf("用户当前状态为 %s，仅待审核用户可审核", user.Status))
	}

	// 审核通过：enable；拒绝：disable
	newStatus := req.Status
	if newStatus != model.StatusEnable && newStatus != model.StatusDisable {
		newStatus = model.StatusEnable
	}

	update := bson.M{"status": newStatus}
	if err := l.svcCtx.UserModel.Update(l.ctx, req.Id, update); err != nil {
		logx.Errorf("[UserApprove] Update user status failed: id=%s err=%v", req.Id, err)
		return nil, xerr.NewServerError("审核失败，请稍后重试")
	}

	action := "已启用"
	if newStatus == model.StatusDisable {
		action = "已拒绝"
	}
	return &types.BaseResp{
		Code: 0,
		Msg:  fmt.Sprintf("用户 %s %s", user.Username, action),
	}, nil
}
