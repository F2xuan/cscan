package logic

import (
	"context"

	"cscan/model"
	"cscan/pkg/notify"
	"cscan/rpc/task/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// collectHighRiskInfoShared 收集任务的高危信息（T1.4）。
// 抽公共函数，消除 incrsubtaskdonelogic.go 与 updatetasklogic.go 两处重复实现（B5）。
// 在旧逻辑（高危指纹/端口/严重级别/新资产数）基础上，新增：
//   - 新资产明细 NewAssetList（由 NewAssetNotify 开关门控）
//   - 新风险明细 NewRisks（由 NewRiskNotify 开关门控，默认开）
//   - 已修复漏洞数 FixedVulCount（由 FixedNotify 开关门控，默认开）
//
// 开关语义：指针类型 nil 视为默认开；旧配置缺字段时行为不变（默认展示明细）。
// 排序与上限截断在 buildHighRiskDetails 内完成，此处只负责取数与门控。
func collectHighRiskInfoShared(ctx context.Context, svcCtx *svc.ServiceContext, workspaceId, mainTaskId string, configs []notify.ConfigItem) *notify.HighRiskInfo {
	hasHighRiskFilter := false
	hasNewAssetNotify := false
	newRiskEnabled := false
	fixedEnabled := false
	var allFingerprints []string
	var allPorts []int
	var allSeverities []string

	for _, cfg := range configs {
		f := cfg.HighRiskFilter
		if f == nil || !f.Enabled {
			continue
		}
		hasHighRiskFilter = true
		allFingerprints = append(allFingerprints, f.HighRiskFingerprints...)
		allPorts = append(allPorts, f.HighRiskPorts...)
		allSeverities = append(allSeverities, f.HighRiskPocSeverities...)
		if f.NewAssetNotify {
			hasNewAssetNotify = true
		}
		if f.NewRiskNotifyEnabled() {
			newRiskEnabled = true
		}
		if f.FixedNotifyEnabled() {
			fixedEnabled = true
		}
	}

	if !hasHighRiskFilter {
		return nil
	}

	info := &notify.HighRiskInfo{
		HighRiskFingerprints:  []string{},
		HighRiskPorts:         []int{},
		HighRiskVulSeverities: make(map[string]int),
	}

	// 收集高危指纹（从资产的指纹中匹配）
	if len(allFingerprints) > 0 {
		assetModel := svcCtx.GetAssetModel(workspaceId)
		assets, err := assetModel.FindByTaskId(ctx, mainTaskId)
		if err == nil {
			fingerprintSet := make(map[string]bool)
			for _, fp := range allFingerprints {
				fingerprintSet[fp] = true
			}
			foundFpSet := make(map[string]bool)
			for _, asset := range assets {
				for _, fp := range asset.Fingerprints {
					if fingerprintSet[fp] && !foundFpSet[fp] {
						info.HighRiskFingerprints = append(info.HighRiskFingerprints, fp)
						foundFpSet[fp] = true
					}
				}
			}
		}
	}

	// 收集高危端口（从资产的端口中匹配）
	if len(allPorts) > 0 {
		assetModel := svcCtx.GetAssetModel(workspaceId)
		assets, err := assetModel.FindByTaskId(ctx, mainTaskId)
		if err == nil {
			portSet := make(map[int]bool)
			for _, port := range allPorts {
				portSet[port] = true
			}
			foundPortSet := make(map[int]bool)
			for _, asset := range assets {
				if portSet[asset.Port] && !foundPortSet[asset.Port] {
					info.HighRiskPorts = append(info.HighRiskPorts, asset.Port)
					foundPortSet[asset.Port] = true
				}
			}
		}
	}

	// 收集高危漏洞统计
	if len(allSeverities) > 0 {
		vulModel := svcCtx.GetVulModel(workspaceId)
		vuls, err := vulModel.Find(ctx, bson.M{"task_id": mainTaskId}, 0, 0)
		if err == nil {
			severitySet := make(map[string]bool)
			for _, s := range allSeverities {
				severitySet[s] = true
			}
			for _, vul := range vuls {
				if severitySet[vul.Severity] {
					info.HighRiskVulSeverities[vul.Severity]++
					info.HighRiskVulCount++
				}
			}
		}
	}

	// 收集新资产数量 + 明细（T1.2 口径：scan_diff 的 added 记录；T1.4 新增明细）
	if hasNewAssetNotify {
		diffModel := model.NewScanDiffModel(svcCtx.MongoDB, workspaceId)
		if added, err := diffModel.CountByTaskIdAndType(ctx, workspaceId, mainTaskId, model.ScanDiffTypeAsset, model.ScanDiffChangeAdded); err == nil {
			info.NewAssetCount = int(added)
		}
		docs, total, err := diffModel.FindByTaskId(ctx, workspaceId, mainTaskId, model.ScanDiffTypeAsset, model.ScanDiffChangeAdded, 1, 0)
		if err == nil {
			list := make([]notify.AssetSummary, 0, len(docs))
			for _, d := range docs {
				list = append(list, notify.AssetSummary{
					Authority:     d.TargetKey,
					FirstSeenTime: d.CreateTime.Format("2006-01-02 15:04:05"),
				})
			}
			if total > int64(len(list)) || int64(len(list)) > notify.MaxDetailItems {
				info.NewAssetTruncated = true
			}
			if len(list) > notify.MaxDetailItems {
				list = list[:notify.MaxDetailItems]
			}
			info.NewAssetList = list
		}
	}

	// 收集新风险明细（T1.4：vul 类 added 记录；weakpass/cert 由 T3.3/T3.4 扩展）
	if newRiskEnabled {
		diffModel := model.NewScanDiffModel(svcCtx.MongoDB, workspaceId)
		docs, _, err := diffModel.FindByTaskId(ctx, workspaceId, mainTaskId, model.ScanDiffTypeVul, model.ScanDiffChangeAdded, 1, 0)
		if err == nil {
			list := make([]notify.RiskSummary, 0, len(docs))
			for _, d := range docs {
				list = append(list, notify.RiskSummary{
					Kind:          "vuln",
					Severity:      d.Severity,
					Name:          d.Summary,
					Target:        d.TargetKey,
					FirstSeenTime: d.CreateTime.Format("2006-01-02 15:04:05"),
				})
			}
			info.NewRisks = list
		}
	}

	// 收集已修复漏洞数（T1.4：本任务内 status=fixed 的漏洞；T3.3/T3.4 复验将驱动该值）
	if fixedEnabled {
		vulModel := svcCtx.GetVulModel(workspaceId)
		if n, err := vulModel.Count(ctx, bson.M{"task_id": mainTaskId, "status": "fixed"}); err == nil {
			info.FixedVulCount = int(n)
		}
	}

	logx.Infof("[HIGH-RISK COLLECT] workspaceId=%s, mainTaskId=%s, newAsset=%d, newAssetList=%d, newRisks=%d, fixedVul=%d",
		workspaceId, mainTaskId, info.NewAssetCount, len(info.NewAssetList), len(info.NewRisks), info.FixedVulCount)

	return info
}
