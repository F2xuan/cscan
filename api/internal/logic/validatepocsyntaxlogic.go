package logic

import (
	"context"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/scanner"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidatePocSyntaxLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewValidatePocSyntaxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidatePocSyntaxLogic {
	return &ValidatePocSyntaxLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ValidatePocSyntax 验证POC语法
func (l *ValidatePocSyntaxLogic) ValidatePocSyntax(req *types.ValidatePocSyntaxReq) (*types.ValidatePocSyntaxResp, error) {
	if req.Content == "" {
		return &types.ValidatePocSyntaxResp{
			Code:  400,
			Msg:   "POC内容不能为空",
			Valid: false,
			Error: "POC内容不能为空",
		}, nil
	}

	// 使用 Nuclei SDK 验证模板
	err := scanner.ValidatePocTemplate(req.Content)
	if err != nil {
		return &types.ValidatePocSyntaxResp{
			Code:  0,
			Msg:   "验证完成",
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	return &types.ValidatePocSyntaxResp{
		Code:  0,
		Msg:   "验证通过",
		Valid: true,
		Error: "",
	}, nil
}
