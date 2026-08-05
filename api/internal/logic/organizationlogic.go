package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// OrganizationListLogic 组织列表
type OrganizationListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrganizationListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrganizationListLogic {
	return &OrganizationListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OrganizationListLogic) OrganizationList(req *types.PageReq) (resp *types.OrganizationListResp, err error) {
	filter := bson.M{}

	// 空 filter 场景使用 EstimatedDocumentCount（O(1) 元数据查询）
	total, err := l.svcCtx.OrganizationModel.EstimatedCount(l.ctx)
	if err != nil {
		return &types.OrganizationListResp{Code: 500, Msg: "查询失败"}, nil
	}

	req.Page, req.PageSize = model.NormalizePage(req.Page, req.PageSize)
	orgs, err := l.svcCtx.OrganizationModel.Find(l.ctx, filter, req.Page, req.PageSize)
	if err != nil {
		return &types.OrganizationListResp{Code: 500, Msg: "查询失败"}, nil
	}

	list := make([]types.Organization, 0, len(orgs))
	for _, o := range orgs {
		list = append(list, types.Organization{
			Id:          o.Id.Hex(),
			Name:        o.Name,
			Description: o.Description,
			Status:      o.Status,
			CreateTime:  o.CreateTime.Local().Format("2006-01-02 15:04:05"),
		})
	}

	return &types.OrganizationListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  list,
	}, nil
}

// OrganizationSaveLogic 保存组织
type OrganizationSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrganizationSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrganizationSaveLogic {
	return &OrganizationSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// validateOrgName 组织名称白名单校验（修复存储型 XSS / 空名 / 超长名）
// 仅允许：Unicode 字母、数字、空格，以及有限标点 _ - . / @ ( ) （ ） 。
// 其余字符（含 < > " ' ; \ 等）一律拒绝，从根本上杜绝脚本注入。
func validateOrgName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("组织名称不能为空")
	}
	if len([]rune(trimmed)) > 64 {
		return "", errors.New("组织名称长度不能超过 64 个字符")
	}
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case ' ', '_', '-', '.', '/', '@', '(', ')', '（', '）':
			continue
		}
		return "", xerr.NewParamError(fmt.Sprintf("组织名称含有非法字符: %q", r))
	}
	return trimmed, nil
}

func (l *OrganizationSaveLogic) OrganizationSave(req *types.OrganizationSaveReq) (resp *types.BaseResp, err error) {
	validName, verr := validateOrgName(req.Name)
	if verr != nil {
		return &types.BaseResp{Code: 400, Msg: verr.Error()}, nil
	}

	if req.Id != "" {
		// 更新
		update := bson.M{
			"name":        validName,
			"description": req.Description,
		}
		if req.Status != "" {
			update["status"] = req.Status
		}
		err = l.svcCtx.OrganizationModel.Update(l.ctx, req.Id, update)
		if err != nil {
			return &types.BaseResp{Code: 500, Msg: "更新失败"}, nil
		}
		common.InvalidateOrgMap(l.svcCtx)
		return &types.BaseResp{Code: 0, Msg: "更新成功"}, nil
	}

	// 新增
	org := &model.Organization{
		Name:        validName,
		Description: req.Description,
	}
	if err = l.svcCtx.OrganizationModel.Insert(l.ctx, org); err != nil {
		return &types.BaseResp{Code: 500, Msg: "创建失败"}, nil
	}
	common.InvalidateOrgMap(l.svcCtx)

	return &types.BaseResp{Code: 0, Msg: "创建成功"}, nil
}

// OrganizationDeleteLogic 删除组织
type OrganizationDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrganizationDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrganizationDeleteLogic {
	return &OrganizationDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OrganizationDeleteLogic) OrganizationDelete(req *types.OrganizationDeleteReq) (resp *types.BaseResp, err error) {
	if req.Id == "" {
		return &types.BaseResp{Code: 400, Msg: "ID不能为空"}, nil
	}

	if err = l.svcCtx.OrganizationModel.Delete(l.ctx, req.Id); err != nil {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	common.InvalidateOrgMap(l.svcCtx)

	return &types.BaseResp{Code: 0, Msg: "删除成功"}, nil
}

// OrganizationUpdateStatusLogic 更新组织状态
type OrganizationUpdateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrganizationUpdateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrganizationUpdateStatusLogic {
	return &OrganizationUpdateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OrganizationUpdateStatusLogic) OrganizationUpdateStatus(req *types.OrganizationUpdateStatusReq) (resp *types.BaseResp, err error) {
	if req.Id == "" {
		return &types.BaseResp{Code: 400, Msg: "ID不能为空"}, nil
	}
	if req.Status == "" {
		return &types.BaseResp{Code: 400, Msg: "状态不能为空"}, nil
	}

	err = l.svcCtx.OrganizationModel.Update(l.ctx, req.Id, bson.M{"status": req.Status})
	if err != nil {
		return &types.BaseResp{Code: 500, Msg: "更新状态失败"}, nil
	}
	common.InvalidateOrgMap(l.svcCtx)

	return &types.BaseResp{Code: 0, Msg: "状态更新成功"}, nil
}
