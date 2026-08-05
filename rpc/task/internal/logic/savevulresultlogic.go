package logic

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cscan/internal/model"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// AssetCache 资产缓存，用于批量处理时减少数据库查询
type AssetCache struct {
	assets map[string]*model.Asset
}

func NewAssetCache() *AssetCache {
	return &AssetCache{
		assets: make(map[string]*model.Asset),
	}
}

func (c *AssetCache) getKey(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func (c *AssetCache) getOrCreate(ctx context.Context, assetModel *model.AssetModel, host string, port int) *model.Asset {
	key := c.getKey(host, port)
	if asset, ok := c.assets[key]; ok {
		return asset
	}

	// 从数据库查询
	asset, _ := assetModel.FindByHostPort(ctx, host, port)
	if asset == nil {
		// 资产不存在，创建一个新的
		asset = &model.Asset{
			Host:       host,
			Port:       port,
			Authority:  fmt.Sprintf("%s:%d", host, port),
			Service:    "http",
			IsHTTP:     true,
			Source:     "poc_scan",
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		// 设置 HTTPS 端口
		if port == 443 || port == 8443 {
			asset.Service = "https"
		}
		// 插入数据库
		if err := assetModel.Insert(ctx, asset); err != nil {
			logx.Errorf("Failed to create asset for vul: %v", err)
			return nil
		}
		// 查询获取完整对象
		asset, _ = assetModel.FindByHostPort(ctx, host, port)
	}
	if asset != nil {
		c.assets[key] = asset
	}
	return asset
}

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

// 保存漏洞结果
// Note: This currently saves to the {workspace}_vul collection using the legacy Vul model.
// The vulnerability scanner produces Vul objects, not ScanResult objects.
// For now, we keep this behavior and add scan_timestamp tracking for history purposes.
func (l *SaveVulResultLogic) SaveVulResult(in *pb.SaveVulResultReq) (*pb.SaveVulResultResp, error) {
	// C-5 修复：原 5s 超时在批量漏洞保存时导致静默丢数据（context.DeadlineExceeded 后 continue，
	// 但函数末尾仍返回 Success:true）。提高到 60s，并在首次 DB 错误时立即返回失败让 worker 重试。
	// C-4 修复：使用局部 ctx，不回写 l.ctx，避免 defer cancel 后逃逸使用拿到已取消的 ctx。
	ctx, cancel := context.WithTimeout(l.ctx, 60*time.Second)
	defer cancel()

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

	vulModel := l.svcCtx.GetVulModel(workspaceId)
	assetModel := l.svcCtx.GetAssetModel(workspaceId)

	// T1.1: 漏洞新增变化快照模型与批量缓冲
	diffModel := model.NewScanDiffModel(l.svcCtx.MongoDB, workspaceId)
	var vulDiffs []model.ScanDiff

	// 创建资产缓存，减少重复查询
	assetCache := NewAssetCache()

	// 按资产分组计算风险评分
	assetRiskMap := make(map[string]float64) // Key: "host:port", Value: maxScore
	assetVulCount := make(map[string]int)    // Key: "host:port", Value: vul count

	var savedCount int32
	var newVulCount int32 // 本次新增的漏洞数（UpsertedCount>0），用于幂等累加 vul_count

	for _, pbVul := range in.Vuls {
		// 解析 URL 获取 host 和 port（如果 Authority 为空）
		host := pbVul.Host
		port := int(pbVul.Port)

		if host == "" && pbVul.Url != "" {
			// 从 URL 解析 host 和 port
			parsedHost, parsedPort := parseHostFromUrl(pbVul.Url)
			if parsedHost != "" {
				host = parsedHost
				if port == 0 {
					port = parsedPort
				}
			}
		}

		// 确保资产存在（自动创建），放入缓存供后续批量更新使用
		assetCache.getOrCreate(ctx, assetModel, host, port)

		// 构建漏洞对象
		vul := &model.Vul{
			Authority: pbVul.Authority,
			Host:      host,
			Port:      port,
			Url:       pbVul.Url,
			PocFile:   pbVul.PocFile,
			Source:    pbVul.Source,
			Severity:  pbVul.Severity,
			Extra:     pbVul.Extra,
			Result:    pbVul.Result,
			TaskId:    in.MainTaskId,
		}

		// 漏洞知识库关联字段
		if pbVul.CvssScore != nil {
			vul.CvssScore = *pbVul.CvssScore
		}
		if pbVul.CveId != nil {
			vul.CveId = *pbVul.CveId
		}
		if pbVul.CweId != nil {
			vul.CweId = *pbVul.CweId
		}
		if pbVul.Remediation != nil {
			vul.Remediation = *pbVul.Remediation
		}
		if len(pbVul.References) > 0 {
			vul.References = pbVul.References
		}

		// 证据链字段
		if pbVul.MatcherName != nil {
			vul.MatcherName = *pbVul.MatcherName
		}
		if len(pbVul.ExtractedResults) > 0 {
			vul.ExtractedResults = pbVul.ExtractedResults
		}
		if pbVul.CurlCommand != nil {
			vul.CurlCommand = *pbVul.CurlCommand
		}
		if pbVul.Request != nil {
			vul.Request = *pbVul.Request
		}
		if pbVul.Response != nil {
			vul.Response = *pbVul.Response
		}
		if pbVul.ResponseTruncated != nil {
			vul.ResponseTruncated = *pbVul.ResponseTruncated
		}

		// 漏洞名称和标签
		if pbVul.VulName != nil {
			vul.VulName = *pbVul.VulName
		}
		if len(pbVul.Tags) > 0 {
			vul.Tags = pbVul.Tags
		}

		// T3.3: 打通 risk_source 落库传输链。scanner 层赋值的 RiskSource 在 gRPC/pb 边界被丢弃
		// （pb.VulDocument 无该字段），故在此按 Source 归一化写入，使复验（FindOpenByRiskSource）
		// 与风险视图可按来源查询。不强改 is_risk，避免影响既有风险分桶。
		if rs := deriveRiskSource(pbVul); rs != "" {
			vul.RiskSource = rs
		}

		pbVulName := ""
		if pbVul.VulName != nil {
			pbVulName = *pbVul.VulName
		}
		l.Logger.Infof("[SaveVulResult] poc=%s pbVulName.nil=%v pbVulName=%q pbTags=%v modelVulName=%q modelTags=%v", pbVul.PocFile, pbVul.VulName == nil, pbVulName, pbVul.Tags, vul.VulName, vul.Tags)

		// 使用Upsert避免重复
		// Note: The Upsert method in VulModel already handles scan_count and timestamps
		// which provides basic history tracking through first_seen_time and last_seen_time
		// 修复 H-5：Upsert 失败立即返回错误给 Worker，Worker 因 gRPC 错误会重试整批，
		// 避免部分漏洞静默丢失（原实现 continue 后仍返回 Success:true，Worker 停止重试）。
		res, err := vulModel.Upsert(ctx, vul)
		if err != nil {
			l.Logger.Errorf("SaveVulResult: failed to upsert vul (host=%s port=%d poc=%s): %v", host, port, pbVul.PocFile, err)
			return &pb.SaveVulResultResp{
				Success: false,
				Message: fmt.Sprintf("failed to save vul at host=%s port=%d poc=%s: %v", host, port, pbVul.PocFile, err),
				Total:   savedCount,
			}, err
		}
		savedCount++

		// 记录风险评分用于后续更新资产
		key := fmt.Sprintf("%s:%d", host, port)
		score := vul.CvssScore * 10 // CVSS 10分制转换为 100分制
		if val, ok := assetRiskMap[key]; !ok || score > val {
			assetRiskMap[key] = score
		}

		// 修复 H-6：基于本次 UpsertedCount（新增漏洞数）幂等累加 vul_count，
		// 不再依赖 isMainTaskTerminal 判断。Worker 扫描过程中持续保存漏洞，
		// 只有真正新增的才累加，避免重复上报导致计数虚高。
		isNewVul := res != nil && res.UpsertedCount > 0
		if isNewVul {
			newVulCount++
			assetVulCount[key]++
			// T1.1: 仅当本次为新增漏洞（UpsertedCount>0）才记录 added 快照
			vulDiffs = append(vulDiffs, model.ScanDiff{
				TaskId:      in.MainTaskId,
				WorkspaceId: workspaceId,
				DiffType:    model.ScanDiffTypeVul,
				ChangeType:  model.ScanDiffChangeAdded,
				Severity:    pbVul.Severity,
				TargetKey:   fmt.Sprintf("%s:%d:%s", host, port, pbVul.PocFile),
				Summary:     vul.VulName,
			})
		}
	}

	// 批量更新资产风险评分
	// 修复 H-6：vul_count 使用 $inc 原子累加"本次新增漏洞数"（基于 UpsertedCount 精确统计），
	// 不再依赖任务终态判断，扫描过程中即可实时更新，且幂等（重复上报相同漏洞不会重复计数）。
	for key, maxScore := range assetRiskMap {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}
		host := parts[0]
		port, _ := strconv.Atoi(parts[1])

		asset := assetCache.getOrCreate(ctx, assetModel, host, port)
		if asset == nil {
			continue
		}

		// Determine Risk Level based on CVSS score (already converted to 100 scale)
		riskLevel := "info"
		if maxScore >= 90 {
			riskLevel = "critical"
		} else if maxScore >= 70 {
			riskLevel = "high"
		} else if maxScore >= 40 {
			riskLevel = "medium"
		} else if maxScore > 0 {
			riskLevel = "low"
		}

		// 构建更新文档：risk_score / risk_level / last_scan_time 用 $set，vul_count 用 $inc 原子累加
		setFields := bson.M{
			"last_scan_time": time.Now(),
		}
		needUpdate := false
		// Update if new score is higher
		if maxScore > asset.RiskScore {
			setFields["risk_score"] = maxScore
			setFields["risk_level"] = riskLevel
			needUpdate = true
		}

		rawUpdate := bson.M{
			"$set": setFields,
		}

		// 始终累加本次新增漏洞数（幂等：仅基于 UpsertedCount，重复漏洞不累加）
		newCount := assetVulCount[key]
		if newCount > 0 {
			rawUpdate["$inc"] = bson.M{"vul_count": int(newCount)}
			needUpdate = true
		}

		if needUpdate {
			if err := assetModel.UpdateWithRaw(ctx, asset.Id.Hex(), rawUpdate); err != nil {
				l.Logger.Errorf("Failed to update asset risk/vul_count: %v", err)
			}
		}
	}

	l.Logger.Infof("SaveVulResult: saved %d vulnerabilities (new=%d), updated %d assets", savedCount, newVulCount, len(assetRiskMap))

	// T1.1: 批量写入本次新增漏洞的变化快照。失败仅记录日志，不阻断主流程。
	if len(vulDiffs) > 0 {
		if err := diffModel.BatchInsert(ctx, vulDiffs); err != nil {
			l.Logger.Errorf("[ScanDiff] vul batch insert failed (task=%s): %v", in.MainTaskId, err)
		} else {
			l.Logger.Infof("[ScanDiff] wrote %d vul diff records for task=%s", len(vulDiffs), in.MainTaskId)
		}
	}

	// 打印保存成功的漏洞详情
	for _, pbVul := range in.Vuls {
		vulName := ""
		if pbVul.VulName != nil {
			vulName = *pbVul.VulName
		}
		l.Logger.Infof("[SaveVulResult] Saved vul: host=%s, port=%d, url=%s, pocFile=%s, severity=%s, vulName=%s",
			pbVul.Host, pbVul.Port, pbVul.Url, pbVul.PocFile, pbVul.Severity, vulName)
	}

	return &pb.SaveVulResultResp{
		Success: true,
		Message: "Vulnerabilities saved successfully",
		Total:   savedCount,
	}, nil
}

// deriveRiskSource 依据漏洞来源（Source）归一化 risk_source，打通 risk 视图落库传输链（T3.3）。
// 集中在此单点处理，避免改造 gRPC pb / worker mapper / API handler 多环传输。
func deriveRiskSource(pbVul *pb.VulDocument) string {
	switch pbVul.Source {
	case "brutescan":
		return model.VulRiskSourceWeakPass
	case "certcheck":
		return model.VulRiskSourceCertExpiry
	}
	return ""
}

// isMainTaskTerminal 检查主任务是否处于终态（SUCCESS/FAILURE/COMPLETED）
// 用于 vul_count 累加时机判断，避免扫描过程中反复覆盖统计虚高
// 查询失败时保守返回 false（不累加 vul_count），避免误统计
func (l *SaveVulResultLogic) isMainTaskTerminal(mainTaskId, workspaceId string) bool {
	if mainTaskId == "" || workspaceId == "" {
		return false
	}
	taskModel := l.svcCtx.GetMainTaskModel(workspaceId)
	task, err := taskModel.FindById(l.ctx, mainTaskId)
	if err != nil || task == nil {
		return false
	}
	switch task.Status {
	case "SUCCESS", "FAILURE", "COMPLETED":
		return true
	}
	return false
}

// parseHostFromUrl 从 URL 解析 host 和 port
func parseHostFromUrl(rawUrl string) (string, int) {
	if rawUrl == "" {
		return "", 0
	}

	// 确保 URL 有协议前缀
	if !strings.Contains(rawUrl, "://") {
		rawUrl = "http://" + rawUrl
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return "", 0
	}

	host := u.Hostname()
	port := 80 // 默认 HTTP 端口

	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	} else if u.Scheme == "https" {
		port = 443
	}

	return host, port
}
