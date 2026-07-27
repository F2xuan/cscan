package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"
	"cscan/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// assetTargetDetailDenormMaxAge 与 list 保持一致：>此阈值的 meta 视为需要回填。
const assetTargetDetailDenormMaxAge = 30 * time.Minute

type AssetTargetDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetTargetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetTargetDetailLogic {
	return &AssetTargetDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetTargetDetail 获取顶层资产详情（meta + 暴露面统计 + 风险统计）。
// 跨 workspace 解析 owning ws；meta 命中失败时回退到 {wsId}_asset 重建 meta（不写回 DB）。
func (l *AssetTargetDetailLogic) AssetTargetDetail(req *types.AssetTargetDetailReq, workspaceId string) (*types.AssetTargetDetailResp, error) {
	targetId := strings.TrimSpace(req.TargetId)
	if targetId == "" {
		return nil, fmt.Errorf("targetId is empty")
	}
	tType, tValue, err := model.DecodeTargetID(targetId)
	if err != nil {
		return nil, err
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	var meta *model.AssetTargetMeta
	var owningWs string
	for _, wsId := range wsIds {
		m, err := l.svcCtx.GetAssetTargetMetaModel(wsId).FindByID(l.ctx, targetId)
		if err != nil {
			l.Logger.Errorf("[AssetTargetDetail] FindByID ws=%s fail: %v", wsId, err)
			continue
		}
		if m != nil {
			meta = m
			owningWs = wsId
			break
		}
	}

	metaPersisted := meta != nil
	if meta == nil {
		// 回退：在第一个 ws 上重建 meta，仅本次请求用，不落库
		if len(wsIds) == 0 {
			return nil, fmt.Errorf("workspace not found")
		}
		owningWs = wsIds[0]
		meta = rebuildMetaFromAssets(l, owningWs, targetId, tType, tValue)
		if meta == nil {
			return nil, fmt.Errorf("target %s not found", targetId)
		}
	}

	exposure := l.computeExposure(owningWs, tType, tValue)
	risk := l.computeRisk(owningWs, tType, tValue)

	// 顺手回填：只有 meta 是从 DB 命中时才写回，避免把临时重建结果落库。
	// 写回失败仅日志告警，不影响本次响应。
	if metaPersisted && model.NeedsRefresh(meta, assetTargetDetailDenormMaxAge) {
		l.writebackDenormalized(owningWs, meta, exposure, risk)
	}

	return &types.AssetTargetDetailResp{
		Code: 0,
		Msg:  "success",
		Data: types.AssetTargetDetailData{
			Meta:     metaToItem(*meta),
			Exposure: exposure,
			Risk:     risk,
		},
	}, nil
}

// writebackDenormalized 把 detail 已算好的 exposure/risk 快照写回 meta 集合，
// 同时覆盖入参 meta 的内联字段，让本次响应 Meta 与 Exposure/Risk 数据一致。
func (l *AssetTargetDetailLogic) writebackDenormalized(wsId string, meta *model.AssetTargetMeta, exp types.AssetTargetExposureStats, risk types.AssetTargetRiskStats) {
	expSnap := model.ExposureSnapshot{
		Subdomains:  exp.Subdomains,
		Ips:         exp.Ips,
		Ports:       exp.Ports,
		Sites:       exp.Sites,
		Icons:       exp.Icons,
		Apps:        exp.Apps,
		Dirs:        exp.Dirs,
		Js:          exp.Js,
		Screenshots: exp.Screenshots,
	}
	riskSnap := model.RiskSnapshot{
		SensitiveInfo: risk.SensitiveInfo,
		SensitiveDir:  risk.SensitiveDir,
		VulnHigh:      risk.VulnHigh,
		VulnTotal:     risk.VulnTotal,
	}
	if err := l.svcCtx.GetAssetTargetMetaModel(wsId).UpdateDenormalized(l.ctx, meta.Id, expSnap, riskSnap); err != nil {
		l.Logger.Errorf("[AssetTargetDetail] UpdateDenormalized ws=%s id=%s fail: %v", wsId, meta.Id, err)
	}
	meta.ExposureSubdomains = expSnap.Subdomains
	meta.ExposureIps = expSnap.Ips
	meta.ExposurePorts = expSnap.Ports
	meta.ExposureSites = expSnap.Sites
	meta.ExposureIcons = expSnap.Icons
	meta.ExposureApps = expSnap.Apps
	meta.ExposureDirs = expSnap.Dirs
	meta.ExposureJs = expSnap.Js
	meta.ExposureScreenshots = expSnap.Screenshots
	meta.RiskSensitiveInfo = riskSnap.SensitiveInfo
	meta.RiskSensitiveDir = riskSnap.SensitiveDir
	meta.RiskVulnHigh = riskSnap.VulnHigh
	meta.RiskVulnTotal = riskSnap.VulnTotal
	meta.RiskUpdatedAt = time.Now()
}

// computeExposure 通过 AggregateGroupByDomain 一次扫描 owning ws 的 asset 集合，
// 按根域名/IP 归并到该目标，再累加 asset 的字段维度。
// 子域/IP/站点按 host 去重（同一 host 多端口只计一次），避免计数膨胀。
func (l *AssetTargetDetailLogic) computeExposure(wsId string, tType model.AssetTargetType, tValue string) types.AssetTargetExposureStats {
	var stats types.AssetTargetExposureStats
	assetModel := l.svcCtx.GetAssetModel(wsId)
	rows, err := assetModel.AggregateGroupByDomain(l.ctx)
	if err != nil {
		l.Logger.Errorf("[AssetTargetDetail] AggregateGroupByDomain ws=%s fail: %v", wsId, err)
		return stats
	}

	seenHosts := make(map[string]struct{})
	for _, row := range rows {
		if !rowMatchesTarget(row.Host, row.Domain, tType, tValue) {
			continue
		}
		stats.Sites++ // 每条 asset (host:port) 视为一个站点
		if _, dup := seenHosts[row.Host]; dup {
			continue
		}
		seenHosts[row.Host] = struct{}{}
		if utils.IsIPAddress(row.Host) {
			stats.Ips++
		} else if row.Host != "" {
			stats.Subdomains++
		}
	}

	hostFilter := hostFilterForTarget(tType, tValue)

	portCount, _ := assetModel.Count(l.ctx, bson.M{
		"port": bson.M{"$gt": 0},
		"host": hostFilter,
	})
	stats.Ports = int(portCount)

	// Icon/App 用 Distinct 按值去重，与 IconList/AppList 页面聚合逻辑一致
	iconVals, _ := assetModel.Distinct(l.ctx, "icon_hash", bson.M{
		"host":      hostFilter,
		"icon_hash": bson.M{"$ne": ""},
	})
	stats.Icons = countNonEmpty(iconVals)

	appVals, _ := assetModel.Distinct(l.ctx, "app", bson.M{
		"host": hostFilter,
		"app":  bson.M{"$ne": nil},
	})
	stats.Apps = countNonEmpty(appVals)

	screenshotCount, _ := assetModel.Count(l.ctx, bson.M{
		"$and": bson.A{
			bson.M{"screenshot": bson.M{"$exists": true}},
			bson.M{"screenshot": bson.M{"$ne": ""}},
			bson.M{"screenshot": bson.M{"$ne": nil}},
		},
		"host": hostFilter,
	})
	stats.Screenshots = int(screenshotCount)

	return stats
}

// computeRisk 统计 {wsId}_vul 中命中该目标的漏洞计数。
// 高危=severity in {critical,high} 或 cvss>=7；is_risk=true 计入风险层。
// SensitiveInfo/SensitiveDir 来自 risk_source="auto:info-leak" 分桶 + DirScanResult 旁路补 SensitiveDir。
// SensitiveInfoItems/SensitiveDirItems/SensitivePathItems 各返回 top-N 命中条目供前端展开。
func (l *AssetTargetDetailLogic) computeRisk(wsId string, tType model.AssetTargetType, tValue string) types.AssetTargetRiskStats {
	var stats types.AssetTargetRiskStats
	vulModel := l.svcCtx.GetVulModel(wsId)

	hostFilter := hostFilterForTarget(tType, tValue)
	total, err := vulModel.Count(l.ctx, bson.M{"host": hostFilter})
	if err != nil {
		l.Logger.Errorf("[AssetTargetDetail] vul Count ws=%s fail: %v", wsId, err)
		return stats
	}
	stats.VulnTotal = int(total)

	// SensitiveInfo / SensitiveDir：基于 risk_source=auto:info-leak + 关键字分桶
	stats.SensitiveInfo = l.countRiskByKeyword(vulModel, hostFilter, sensitiveInfoKeywords)
	stats.SensitiveDir = l.countRiskByKeyword(vulModel, hostFilter, sensitiveDirKeywords)
	// 旁路补充：dirscan_result 集合中按 host 后缀命中且 path 含敏感特征的条目
	stats.SensitiveDir += l.countSensitiveDirFromScanResult(wsId, hostFilter)

	// top-N 命中条目（默认 10），前端可点击展开
	stats.SensitiveInfoItems = l.listRiskByKeyword(vulModel, hostFilter, sensitiveInfoKeywords, sensitiveTopN)
	stats.SensitiveDirItems = l.listRiskByKeyword(vulModel, hostFilter, sensitiveDirKeywords, sensitiveTopN)
	stats.SensitivePathItems = l.listSensitivePathFromScanResult(wsId, hostFilter, sensitiveTopN)

	highCount, err := vulModel.Count(l.ctx, bson.M{
		"host": hostFilter,
		"$or": bson.A{
			bson.M{"severity": bson.M{"$in": bson.A{"critical", "high"}}},
			bson.M{"cvss_score": bson.M{"$gte": 7.0}},
		},
	})
	if err == nil {
		stats.VulnHigh = int(highCount)
	}
	return stats
}

// countRiskByKeyword 在 {wsId}_vul 上按 host 后缀 + is_risk=true + risk_source=auto:info-leak + 关键字分桶计数。
func (l *AssetTargetDetailLogic) countRiskByKeyword(vulModel *model.VulModel, hostFilter interface{}, keywords []string) int {
	if len(keywords) == 0 {
		return 0
	}
	filter := bson.M{
		"host":        hostFilter,
		"is_risk":     true,
		"risk_source": "auto:info-leak",
		"$or":         keywordOrClause(keywords),
	}
	n, err := vulModel.Count(l.ctx, filter)
	if err != nil {
		l.Logger.Errorf("[AssetTargetDetail] countRiskByKeyword fail: %v", err)
		return 0
	}
	return int(n)
}

// countSensitiveDirFromScanResult 在全局 dirscan_result 集合按 workspace_id + host 后缀 + path 关键字计数。
func (l *AssetTargetDetailLogic) countSensitiveDirFromScanResult(wsId string, hostFilter interface{}) int {
	dirModel := l.svcCtx.GetDirScanResultModel()
	if dirModel == nil {
		return 0
	}
	filter := bson.M{
		"workspace_id": wsId,
		"host":         hostFilter,
		"$or":          pathKeywordOrClause(sensitivePathKeywords),
	}
	n, err := dirModel.CountByFilter(l.ctx, filter)
	if err != nil {
		l.Logger.Errorf("[AssetTargetDetail] countSensitiveDirFromScanResult ws=%s fail: %v", wsId, err)
		return 0
	}
	return int(n)
}

// listRiskByKeyword 在 {wsId}_vul 上按 host 后缀 + is_risk=true + risk_source=auto:info-leak + 关键字分桶取 top-N 条目。
// 复用 VulModel.Find（已投影排除 request/response/curl_command，自动按 create_time desc 排序）。
func (l *AssetTargetDetailLogic) listRiskByKeyword(vulModel *model.VulModel, hostFilter interface{}, keywords []string, limit int) []types.AssetTargetSensitiveVulItem {
	if len(keywords) == 0 || limit <= 0 {
		return nil
	}
	filter := bson.M{
		"host":        hostFilter,
		"is_risk":     true,
		"risk_source": "auto:info-leak",
		"$or":         keywordOrClause(keywords),
	}
	docs, err := vulModel.Find(l.ctx, filter, 1, limit)
	if err != nil {
		l.Logger.Errorf("[AssetTargetDetail] listRiskByKeyword fail: %v", err)
		return nil
	}
	items := make([]types.AssetTargetSensitiveVulItem, 0, len(docs))
	for _, v := range docs {
		tags := v.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, types.AssetTargetSensitiveVulItem{
			Id:         v.Id.Hex(),
			VulName:    v.VulName,
			Severity:   v.Severity,
			Host:       v.Host,
			Port:       v.Port,
			Url:        v.Url,
			Source:     v.Source,
			Tags:       tags,
			CreateTime: tsMilli(v.CreateTime),
		})
	}
	return items
}

// listSensitivePathFromScanResult 在全局 dirscan_result 集合按 workspace_id + host 后缀 + path 关键字取 top-N 条目。
func (l *AssetTargetDetailLogic) listSensitivePathFromScanResult(wsId string, hostFilter interface{}, limit int) []types.AssetTargetSensitiveDirItem {
	dirModel := l.svcCtx.GetDirScanResultModel()
	if dirModel == nil || limit <= 0 {
		return nil
	}
	filter := bson.M{
		"workspace_id": wsId,
		"host":         hostFilter,
		"$or":          pathKeywordOrClause(sensitivePathKeywords),
	}
	docs, err := dirModel.FindByFilterWithSort(l.ctx, filter, 1, limit, "", "")
	if err != nil {
		l.Logger.Errorf("[AssetTargetDetail] listSensitivePathFromScanResult ws=%s fail: %v", wsId, err)
		return nil
	}
	items := make([]types.AssetTargetSensitiveDirItem, 0, len(docs))
	for _, d := range docs {
		items = append(items, types.AssetTargetSensitiveDirItem{
			Id:         d.Id.Hex(),
			Host:       d.Host,
			Port:       d.Port,
			Path:       d.Path,
			Url:        d.URL,
			StatusCode: d.StatusCode,
			Title:      d.Title,
			CreateTime: tsMilli(d.CreateTime),
		})
	}
	return items
}

// countNonEmpty 统计 Distinct 返回值中非空元素的数量。
// Distinct 返回 []interface{}，可能包含 "" 或 nil（MongoDB 对缺失字段返回 null）。
func countNonEmpty(vals []interface{}) int {
	n := 0
	for _, v := range vals {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		n++
	}
	return n
}

// rowMatchesTarget 判断 AggregateGroupByDomain 的一行是否归属该目标。
// IP 目标：row.Host 是该 IP。
// 域名目标：resolveRootDomain(row.Host, row.Domain) == tValue。
func rowMatchesTarget(host, domain string, tType model.AssetTargetType, tValue string) bool {
	if tType == model.AssetTargetTypeIP {
		return host == tValue
	}
	return resolveRootDomain(host, domain) == tValue
}

// hostFilterForTarget 返回按 host 过滤的 bson 值。
// IP 目标直接精确匹配；域名目标用后缀正则匹配根域及其所有子域。
func hostFilterForTarget(tType model.AssetTargetType, tValue string) interface{} {
	if tType == model.AssetTargetTypeIP {
		return tValue
	}
	// 匹配根域或任意子域：example.com / *.example.com
	pattern := "^(" + regexpEscape(tValue) + `|.*\.` + regexpEscape(tValue) + ")$"
	return bson.M{"$regex": pattern, "$options": "i"}
}

// regexpEscape 简单转义正则元字符，仅用于 host 这种受控输入。
func regexpEscape(s string) string {
	const meta = `\.+*?()[]{}|^$`
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(meta, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// rebuildMetaFromAssets 当 meta 未命中时，从 {wsId}_asset 重建临时 meta（不写库）。
func rebuildMetaFromAssets(l *AssetTargetDetailLogic, wsId, targetId string, tType model.AssetTargetType, tValue string) *model.AssetTargetMeta {
	rows, err := l.svcCtx.GetAssetModel(wsId).AggregateGroupByDomain(l.ctx)
	if err != nil {
		return nil
	}
	for _, row := range rows {
		if rowMatchesTarget(row.Host, row.Domain, tType, tValue) {
			return &model.AssetTargetMeta{
				Id:           targetId,
				WorkspaceId:  wsId,
				TargetType:   string(tType),
				TargetValue:  tValue,
				CreateTime:   row.CreateTime,
				UpdateTime:   row.UpdateTime,
				FirstSeenTime: row.CreateTime,
				LastScanTime:  row.UpdateTime,
			}
		}
	}
	return nil
}
