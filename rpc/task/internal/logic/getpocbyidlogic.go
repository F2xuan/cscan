package logic

import (
	"context"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPocByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPocByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPocByIdLogic {
	return &GetPocByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据ID获取POC内容
func (l *GetPocByIdLogic) GetPocById(in *pb.GetPocByIdReq) (*pb.GetPocByIdResp, error) {
	pocId := in.PocId
	pocType := in.PocType

	if pocId == "" {
		return &pb.GetPocByIdResp{
			Success: false,
			Message: "POC ID不能为空",
		}, nil
	}

	// 根据类型查询不同的集合
	if pocType == "custom" {
		// 查询自定义POC
		poc, err := l.svcCtx.CustomPocModel.FindById(l.ctx, pocId)
		if err != nil {
			l.Logger.Errorf("GetPocById: failed to get custom poc: %v", err)
			return &pb.GetPocByIdResp{
				Success: false,
				Message: "获取自定义POC失败: " + err.Error(),
			}, nil
		}
		if poc == nil {
			return &pb.GetPocByIdResp{
				Success: false,
				Message: "自定义POC不存在",
			}, nil
		}

		return &pb.GetPocByIdResp{
			Success:    true,
			Message:    "success",
			PocId:      poc.Id.Hex(),
			Name:       poc.Name,
			TemplateId: poc.TemplateId,
			Severity:   poc.Severity,
			Tags:       poc.Tags,
			Content:    poc.Content,
			PocType:    "custom",
		}, nil
	}

	// 默认查询Nuclei模板
	// 先尝试按ObjectID查询
	template, err := l.svcCtx.NucleiTemplateModel.FindByIds(l.ctx, []string{pocId})
	if err == nil && len(template) > 0 {
		t := template[0]
		return &pb.GetPocByIdResp{
			Success:    true,
			Message:    "success",
			PocId:      t.Id.Hex(),
			Name:       t.Name,
			TemplateId: t.TemplateId,
			Severity:   t.Severity,
			Tags:       t.Tags,
			Content:    t.Content,
			PocType:    "nuclei",
		}, nil
	}

	// 尝试按TemplateId查询
	template2, err := l.svcCtx.NucleiTemplateModel.FindByTemplateId(l.ctx, pocId)
	if err != nil {
		l.Logger.Errorf("GetPocById: failed to get nuclei template: %v", err)
		return &pb.GetPocByIdResp{
			Success: false,
			Message: "获取Nuclei模板失败: " + err.Error(),
		}, nil
	}
	if template2 == nil {
		return &pb.GetPocByIdResp{
			Success: false,
			Message: "Nuclei模板不存在",
		}, nil
	}

	return &pb.GetPocByIdResp{
		Success:    true,
		Message:    "success",
		PocId:      template2.Id.Hex(),
		Name:       template2.Name,
		TemplateId: template2.TemplateId,
		Severity:   template2.Severity,
		Tags:       template2.Tags,
		Content:    template2.Content,
		PocType:    "nuclei",
	}, nil
}
