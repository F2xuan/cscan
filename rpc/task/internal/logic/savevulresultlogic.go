package logic

import (
	"context"

	"cscan/internal/model"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveVulResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveVulResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveVulResultLogic {
	return &SaveVulResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SaveVulResult 保存漏洞结果（薄壳：调用 VulWriteService）
func (l *SaveVulResultLogic) SaveVulResult(in *pb.SaveVulResultReq) (*pb.SaveVulResultResp, error) {
	if len(in.Vuls) == 0 {
		return &pb.SaveVulResultResp{
			Success: true,
			Message: "No vulnerabilities to save",
			Total:   0,
		}, nil
	}

	workspaceId := in.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	// 将 pb.VulDocument 转换为 model.ScannerVulnerability
	scannerVuls := make([]*model.ScannerVulnerability, 0, len(in.Vuls))
	for _, pbVul := range in.Vuls {
		sv := &model.ScannerVulnerability{
			Authority: pbVul.Authority,
			Host:      pbVul.Host,
			Port:      int(pbVul.Port),
			Url:       pbVul.Url,
			PocFile:   pbVul.PocFile,
			Source:    pbVul.Source,
			Severity:  pbVul.Severity,
			Extra:     pbVul.Extra,
			Result:    pbVul.Result,
		}

		if pbVul.CvssScore != nil {
			sv.CvssScore = *pbVul.CvssScore
		}
		if pbVul.CveId != nil {
			sv.CveId = *pbVul.CveId
		}
		if pbVul.CweId != nil {
			sv.CweId = *pbVul.CweId
		}
		if pbVul.Remediation != nil {
			sv.Remediation = *pbVul.Remediation
		}
		if len(pbVul.References) > 0 {
			sv.References = pbVul.References
		}
		if pbVul.MatcherName != nil {
			sv.MatcherName = *pbVul.MatcherName
		}
		if len(pbVul.ExtractedResults) > 0 {
			sv.ExtractedResults = pbVul.ExtractedResults
		}
		if pbVul.CurlCommand != nil {
			sv.CurlCommand = *pbVul.CurlCommand
		}
		if pbVul.Request != nil {
			sv.Request = *pbVul.Request
		}
		if pbVul.Response != nil {
			sv.Response = *pbVul.Response
		}
		if pbVul.ResponseTruncated != nil {
			sv.ResponseTruncated = *pbVul.ResponseTruncated
		}
		if pbVul.VulName != nil {
			sv.VulName = *pbVul.VulName
		}
		if len(pbVul.Tags) > 0 {
			sv.Tags = pbVul.Tags
		}

		scannerVuls = append(scannerVuls, sv)
	}

	// 调用 VulWriteService
	writeService := model.NewVulWriteService(l.svcCtx.MongoDB, workspaceId)
	result, err := writeService.SaveVuls(l.ctx, in.MainTaskId, scannerVuls)
	if err != nil {
		return &pb.SaveVulResultResp{
			Success: false,
			Message: err.Error(),
			Total:   result.SavedCount,
		}, err
	}

	return &pb.SaveVulResultResp{
		Success: true,
		Message: "Vulnerabilities saved successfully",
		Total:   result.SavedCount,
	}, nil
}
