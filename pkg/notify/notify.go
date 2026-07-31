package notify

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NotifyResult 通知结果
type NotifyResult struct {
	TaskId      string    `json:"taskId"`
	TaskName    string    `json:"taskName"`
	Status      string    `json:"status"` // SUCCESS, FAILURE
	AssetCount  int       `json:"assetCount"`
	VulCount    int       `json:"vulCount"`
	Duration    string    `json:"duration"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	WorkspaceId string    `json:"workspaceId"`
	ReportURL   string    `json:"reportUrl"` // 报告URL地址
	// 高危检测结果
	HighRiskInfo *HighRiskInfo `json:"highRiskInfo,omitempty"`
}

// HighRiskInfo 高危检测信息
type HighRiskInfo struct {
	HighRiskFingerprints  []string       `json:"highRiskFingerprints"`  // 发现的高危指纹
	HighRiskPorts         []int          `json:"highRiskPorts"`         // 发现的高危端口
	HighRiskVulCount      int            `json:"highRiskVulCount"`      // 高危漏洞数量
	HighRiskVulSeverities map[string]int `json:"highRiskVulSeverities"` // 按严重级别统计: critical->5, high->10
	NewAssetCount         int            `json:"newAssetCount"`         // 新发现资产数量
	// T1.4: 新增/新风险明细（仅追加，不改已有字段名，避免破坏外部 Webhook 消费方）
	NewAssetList      []AssetSummary `json:"newAssetList,omitempty"`      // 新增资产明细（上限 20 条）
	NewAssetTruncated bool           `json:"newAssetTruncated,omitempty"` // 新增资产是否超出上限被截断
	NewRisks          []RiskSummary  `json:"newRisks,omitempty"`          // 新风险明细（vuln/weakpass/cert）
	FixedVulCount     int            `json:"fixedVulCount"`               // 已修复漏洞数量
}

// AssetSummary 新增资产摘要（T1.4）
type AssetSummary struct {
	Authority     string `json:"authority"`              // 资产 authority（域名或 IP）
	Category      string `json:"category,omitempty"`     // subdomain / ip / port / site ...
	Source        string `json:"source,omitempty"`       // 发现来源
	FirstSeenTime string `json:"firstSeenTime,omitempty"` // 首次发现时间（已格式化）
}

// RiskSummary 新风险摘要（T1.4）
type RiskSummary struct {
	Kind          string `json:"kind"`                    // vuln / weakpass / cert
	Severity      string `json:"severity,omitempty"`      // critical / high / medium / low / info
	Name          string `json:"name,omitempty"`          // 风险名称（如漏洞名）
	Target        string `json:"target,omitempty"`        // 目标（host:port:pocfile 等）
	FirstSeenTime string `json:"firstSeenTime,omitempty"` // 首次发现时间（已格式化）
}

// Provider 通知提供者接口
type Provider interface {
	// Name 返回提供者名称
	Name() string
	// Send 发送通知
	Send(ctx context.Context, result *NotifyResult) error
}

// Notifier 通知服务
type Notifier struct {
	providers []Provider
	mu        sync.RWMutex
}

// NewNotifier 创建通知服务
func NewNotifier() *Notifier {
	return &Notifier{
		providers: make([]Provider, 0),
	}
}

// AddProvider 添加通知提供者
func (n *Notifier) AddProvider(p Provider) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.providers = append(n.providers, p)
}

// ClearProviders 清空所有提供者
func (n *Notifier) ClearProviders() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.providers = make([]Provider, 0)
}

// ProviderCount 返回已加载的提供者数量
func (n *Notifier) ProviderCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.providers)
}

// Send 发送通知到所有提供者
func (n *Notifier) Send(ctx context.Context, result *NotifyResult) error {
	n.mu.RLock()
	providers := make([]Provider, len(n.providers))
	copy(providers, n.providers)
	n.mu.RUnlock()

	if len(providers) == 0 {
		return nil
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    []string
		failed  int
	)

	for _, p := range providers {
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()
			// 每个 provider 独立超时，避免相互影响
			pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if err := provider.Send(pctx, result); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", provider.Name(), err))
				failed++
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()

	// 全部失败才返回 error，部分失败只记日志（通过 errs 体现）
	if failed == len(providers) {
		return fmt.Errorf("notify errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// FormatMessage 格式化通知消息
func FormatMessage(result *NotifyResult, template string) string {
	if template == "" {
		template = DefaultTemplate
	}

	statusEmoji := "✅"
	if result.Status == "FAILURE" {
		statusEmoji = "❌"
	}

	// 构建高危详情字符串
	highRiskDetails := buildHighRiskDetails(result.HighRiskInfo)

	replacer := strings.NewReplacer(
		"{{taskName}}", result.TaskName,
		"{{taskId}}", result.TaskId,
		"{{status}}", result.Status,
		"{{statusEmoji}}", statusEmoji,
		"{{assetCount}}", fmt.Sprintf("%d", result.AssetCount),
		"{{vulCount}}", fmt.Sprintf("%d", result.VulCount),
		"{{duration}}", result.Duration,
		"{{startTime}}", result.StartTime.Format("2006-01-02 15:04:05"),
		"{{endTime}}", result.EndTime.Format("2006-01-02 15:04:05"),
		"{{workspaceId}}", result.WorkspaceId,
		"{{reportUrl}}", result.ReportURL,
		"{{highRiskDetails}}", highRiskDetails,
	)

	return replacer.Replace(template)
}

// MaxDetailItems 明细列表单条通知展示上限（超过则截断并显式标注，禁止静默丢弃）
const MaxDetailItems = 20

// severityRank 严重级别排序权重：critical > high > medium > low > info > unknown
func severityRank(sev string) int {
	switch strings.ToLower(sev) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// buildHighRiskDetails 构建高危详情字符串
// T1.4：新增资产/新风险明细按严重度排序、上限截断并显式标注"仅显示前 N 条"；空值省略区块。
func buildHighRiskDetails(info *HighRiskInfo) string {
	if info == nil {
		return ""
	}

	parts := make([]string, 0, 8)
	hasContent := false

	// 高危指纹
	if len(info.HighRiskFingerprints) > 0 {
		parts = append(parts, fmt.Sprintf("\n🚨 高危指纹: %s", strings.Join(info.HighRiskFingerprints, ", ")))
		hasContent = true
	}

	// 高危端口
	if len(info.HighRiskPorts) > 0 {
		portStrs := make([]string, len(info.HighRiskPorts))
		for i, port := range info.HighRiskPorts {
			portStrs[i] = strconv.Itoa(port)
		}
		parts = append(parts, fmt.Sprintf("\n🚨 高危端口: %s", strings.Join(portStrs, ", ")))
		hasContent = true
	}

	// 高危漏洞：安全处理nil map
	if info.HighRiskVulCount > 0 && len(info.HighRiskVulSeverities) > 0 {
		vulParts := make([]string, 0, len(info.HighRiskVulSeverities))
		for severity, count := range info.HighRiskVulSeverities {
			vulParts = append(vulParts, fmt.Sprintf("%s: %d", severity, count))
		}
		parts = append(parts, fmt.Sprintf("\n🚨 高危漏洞: %s (共 %d 个)", strings.Join(vulParts, ", "), info.HighRiskVulCount))
		hasContent = true
	}

	// 新发现资产（总数）
	if info.NewAssetCount > 0 {
		parts = append(parts, fmt.Sprintf("\n🆕 新发现资产: %d 个", info.NewAssetCount))
		hasContent = true
	}

	// 新增资产明细（T1.4）
	if len(info.NewAssetList) > 0 {
		truncated := info.NewAssetTruncated
		list := info.NewAssetList
		if len(list) > MaxDetailItems {
			list = list[:MaxDetailItems]
			truncated = true
		}
		var b strings.Builder
		b.WriteString("\n🆕 新增资产明细:")
		for i, a := range list {
			cat := a.Category
			if cat == "" {
				cat = "资产"
			}
			line := fmt.Sprintf("\n  %d. %s", i+1, a.Authority)
			if a.Category != "" && a.Category != "资产" {
				line += fmt.Sprintf(" [%s]", a.Category)
			}
			if a.FirstSeenTime != "" {
				line += fmt.Sprintf(" (首次: %s)", a.FirstSeenTime)
			}
			b.WriteString(line)
		}
		if truncated {
			b.WriteString(fmt.Sprintf("\n  ...仅显示前 %d 条，更多见报告", MaxDetailItems))
		}
		parts = append(parts, b.String())
		hasContent = true
	}

	// 新风险明细（T1.4）：按严重度降序排序，超限截断并标注
	if len(info.NewRisks) > 0 {
		sorted := make([]RiskSummary, len(info.NewRisks))
		copy(sorted, info.NewRisks)
		sort.SliceStable(sorted, func(i, j int) bool {
			return severityRank(sorted[i].Severity) > severityRank(sorted[j].Severity)
		})
		show := sorted
		riskTruncated := false
		if len(show) > MaxDetailItems {
			show = show[:MaxDetailItems]
			riskTruncated = true
		}
		var b strings.Builder
		b.WriteString("\n⚠️ 新风险明细:")
		for i, r := range show {
			line := fmt.Sprintf("\n  %d. [%s] %s", i+1, r.Kind, r.Name)
			if r.Target != "" {
				line += fmt.Sprintf(" @ %s", r.Target)
			}
			if r.Severity != "" {
				line += fmt.Sprintf(" (%s)", r.Severity)
			}
			b.WriteString(line)
		}
		if riskTruncated {
			b.WriteString(fmt.Sprintf("\n  ...仅显示前 %d 条，更多见报告", MaxDetailItems))
		}
		parts = append(parts, b.String())
		hasContent = true
	}

	// 已修复漏洞（T1.4）
	if info.FixedVulCount > 0 {
		parts = append(parts, fmt.Sprintf("\n🟢 已修复漏洞: %d 个", info.FixedVulCount))
		hasContent = true
	}

	if !hasContent {
		return ""
	}

	return strings.Join(parts, "")
}

// DefaultTemplate 默认消息模板
const DefaultTemplate = `{{statusEmoji}} 扫描任务完成

任务名称: {{taskName}}
任务状态: {{status}}
发现资产: {{assetCount}}
发现漏洞: {{vulCount}}
执行时长: {{duration}}
开始时间: {{startTime}}
结束时间: {{endTime}}
报告地址: {{reportUrl}}{{highRiskDetails}}`

// MarkdownTemplate Markdown格式模板
const MarkdownTemplate = `## {{statusEmoji}} 扫描任务完成

| 项目 | 内容 |
|------|------|
| 任务名称 | {{taskName}} |
| 任务状态 | {{status}} |
| 发现资产 | {{assetCount}} |
| 发现漏洞 | {{vulCount}} |
| 执行时长 | {{duration}} |
| 开始时间 | {{startTime}} |
| 结束时间 | {{endTime}} |
| 报告地址 | {{reportUrl}} |`
