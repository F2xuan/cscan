package notification

import (
	"context"

	"cscan/internal/model"
	"cscan/pkg/notify"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// collectHighRiskInfo 收集任务的高危信息。
// 高危结果收集器，独立于 svc.ServiceContext。
func collectHighRiskInfo(ctx context.Context, db *mongo.Database, workspaceId, mainTaskId string, configs []notify.ConfigItem) *notify.HighRiskInfo {
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

	assetModel := model.NewAssetModel(db, workspaceId)

	// 高危指纹
	if len(allFingerprints) > 0 {
		assets, err := assetModel.FindByTaskId(ctx, mainTaskId)
		if err == nil {
			fpSet := make(map[string]bool, len(allFingerprints))
			for _, fp := range allFingerprints {
				fpSet[fp] = true
			}
			found := make(map[string]bool)
			for _, a := range assets {
				for _, fp := range a.Fingerprints {
					if fpSet[fp] && !found[fp] {
						info.HighRiskFingerprints = append(info.HighRiskFingerprints, fp)
						found[fp] = true
					}
				}
			}
		}
	}

	// 高危端口
	if len(allPorts) > 0 {
		assets, err := assetModel.FindByTaskId(ctx, mainTaskId)
		if err == nil {
			portSet := make(map[int]bool, len(allPorts))
			for _, p := range allPorts {
				portSet[p] = true
			}
			found := make(map[int]bool)
			for _, a := range assets {
				if portSet[a.Port] && !found[a.Port] {
					info.HighRiskPorts = append(info.HighRiskPorts, a.Port)
					found[a.Port] = true
				}
			}
		}
	}

	// 高危漏洞统计
	if len(allSeverities) > 0 {
		vulModel := model.NewVulModel(db, workspaceId)
		vuls, err := vulModel.Find(ctx, bson.M{"task_id": mainTaskId}, 0, 0)
		if err == nil {
			sevSet := make(map[string]bool, len(allSeverities))
			for _, s := range allSeverities {
				sevSet[s] = true
			}
			for _, v := range vuls {
				if sevSet[v.Severity] {
					info.HighRiskVulSeverities[v.Severity]++
					info.HighRiskVulCount++
				}
			}
		}
	}

	// 新资产数量 + 明细
	if hasNewAssetNotify {
		diffModel := model.NewScanDiffModel(db, workspaceId)
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

	// 新风险明细
	if newRiskEnabled {
		diffModel := model.NewScanDiffModel(db, workspaceId)
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

	// 已修复漏洞数
	if fixedEnabled {
		vulModel := model.NewVulModel(db, workspaceId)
		if n, err := vulModel.Count(ctx, bson.M{"task_id": mainTaskId, "status": "fixed"}); err == nil {
			info.FixedVulCount = int(n)
		}
	}

	logx.Infof("[Notification] HighRiskInfo collected: workspaceId=%s, mainTaskId=%s, newAsset=%d, newAssetList=%d, newRisks=%d, fixedVul=%d",
		workspaceId, mainTaskId, info.NewAssetCount, len(info.NewAssetList), len(info.NewRisks), info.FixedVulCount)

	return info
}
