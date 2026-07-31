package notify

import (
	"fmt"
	"strings"
	"testing"
)

// sampleResult 构造一条含混合严重度新风险的通知结果（T4.5 测试用）
func sampleResult() *NotifyResult {
	return &NotifyResult{
		TaskId:    "task-1",
		TaskName:  "每日扫描",
		Status:    "SUCCESS",
		AssetCount: 10,
		VulCount:  3,
		ReportURL: "https://example.com/report?taskId=task-1",
		HighRiskInfo: &HighRiskInfo{
			NewRisks: []RiskSummary{
				{Kind: "vuln", Severity: "high", Name: "Apache Log4j", Target: "10.0.0.1:8080"},
				{Kind: "cert", Severity: "critical", Name: "证书即将过期", Target: "example.com"},
				{Kind: "weakpass", Severity: "medium", Name: "弱口令 admin", Target: "10.0.0.2:22"},
			},
			NewAssetCount: 2,
			FixedVulCount: 1,
		},
	}
}

// countTopRiskLines 统计卡片正文中"最紧急项"的条数（形如 "1. [漏洞] ..."）
func countTopRiskLines(body string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 0 && line[0] >= '1' && line[0] <= '9' && strings.Contains(line, "[") {
			n++
		}
	}
	return n
}

// 飞书交互式卡片：结构、标题色（随最高严重度 critical→red）、跳转按钮指向证书页
func TestFeishuCard_Structure(t *testing.T) {
	p := &FeishuProvider{}
	payload := p.buildCard(sampleResult())
	if payload["msg_type"] != "interactive" {
		t.Fatalf("feishu msg_type expected interactive, got %v", payload["msg_type"])
	}
	card, ok := payload["card"].(map[string]interface{})
	if !ok {
		t.Fatalf("card missing")
	}
	header := card["header"].(map[string]interface{})
	if header["template"] != "red" {
		t.Fatalf("header template expected red (critical), got %v", header["template"])
	}
	elements := card["elements"].([]map[string]interface{})
	var jumpURL string
	for _, el := range elements {
		if el["tag"] == "action" {
			actions := el["actions"].([]map[string]interface{})
			jumpURL = actions[0]["url"].(string)
		}
	}
	if jumpURL == "" {
		t.Fatalf("feishu card missing jump button")
	}
	if !strings.Contains(jumpURL, "/asset-management/risk/cert") {
		t.Fatalf("feishu jump should point to cert page, got %s", jumpURL)
	}
	// 正文概览 + 最紧急项
	for _, el := range elements {
		if el["tag"] == "div" {
			content := el["text"].(map[string]interface{})["content"].(string)
			if !strings.Contains(content, "概览") || !strings.Contains(content, "最紧急项") {
				t.Fatalf("feishu card body missing 概览/最紧急项: %s", content)
			}
		}
	}
}

// 钉钉 ActionCard：结构 + 跳转按钮（singleTitle/singleURL）指向证书页
func TestDingTalkActionCard_Structure(t *testing.T) {
	p := &DingTalkProvider{}
	payload := p.buildActionCard(sampleResult())
	if payload["msgtype"] != "actionCard" {
		t.Fatalf("dingtalk msgtype expected actionCard, got %v", payload["msgtype"])
	}
	ac := payload["actionCard"].(map[string]interface{})
	if ac["singleTitle"] != "查看详情" {
		t.Fatalf("expected singleTitle 查看详情, got %v", ac["singleTitle"])
	}
	jump, ok := ac["singleURL"].(string)
	if !ok || jump == "" {
		t.Fatalf("dingtalk singleURL missing")
	}
	if !strings.Contains(jump, "/asset-management/risk/cert") {
		t.Fatalf("dingtalk jump should point to cert page, got %s", jump)
	}
}

// 企微 markdown：结构 + 含跳转链接 + 概览/最紧急项
func TestWeComMarkdown_Structure(t *testing.T) {
	p := &WeComProvider{}
	payload := p.buildMarkdown(sampleResult())
	if payload["msgtype"] != "markdown" {
		t.Fatalf("wecom msgtype expected markdown, got %v", payload["msgtype"])
	}
	md := payload["markdown"].(map[string]interface{})
	content, _ := md["content"].(string)
	if !strings.Contains(content, "[查看详情](") {
		t.Fatalf("wecom markdown should contain jump link, got %s", content)
	}
	if !strings.Contains(content, "/asset-management/risk/cert") {
		t.Fatalf("wecom jump should point to cert page, got %s", content)
	}
	if !strings.Contains(content, "概览") || !strings.Contains(content, "最紧急项") {
		t.Fatalf("wecom card should contain 概览 and 最紧急项, got %s", content)
	}
}

// 标题色随严重度：high → orange
func TestCardHeaderColor_High(t *testing.T) {
	r := sampleResult()
	r.HighRiskInfo.NewRisks = []RiskSummary{
		{Kind: "vuln", Severity: "high", Name: "x"},
	}
	if got := cardHeaderColor(r); got != "orange" {
		t.Fatalf("expected orange for high, got %s", got)
	}
}

// 普通完成（无高风险明细）→ 蓝色；跳转回退报告页
func TestCardHeaderColor_NormalCompletion(t *testing.T) {
	r := &NotifyResult{TaskName: "t", Status: "SUCCESS", ReportURL: "https://example.com/report?taskId=t"}
	if got := cardHeaderColor(r); got != "blue" {
		t.Fatalf("expected blue for normal completion, got %s", got)
	}
	if got := primaryJumpURL(r); got != r.ReportURL {
		t.Fatalf("expected fallback to report url, got %s", got)
	}
}

// 任务失败 → 红色
func TestCardHeaderColor_Failure(t *testing.T) {
	r := &NotifyResult{TaskName: "t", Status: "FAILURE"}
	if got := cardHeaderColor(r); got != "red" {
		t.Fatalf("expected red for failure, got %s", got)
	}
}

// 最紧急项上限 3 条（超出截断，不静默丢弃也不超 3）
func TestCard_TopRisksLimitedToThree(t *testing.T) {
	r := sampleResult()
	risks := make([]RiskSummary, 0, 6)
	for i := 0; i < 6; i++ {
		risks = append(risks, RiskSummary{Kind: "vuln", Severity: "low", Name: fmt.Sprintf("R%d", i)})
	}
	r.HighRiskInfo.NewRisks = risks
	body := buildCardBody(r)
	if n := countTopRiskLines(body); n != 3 {
		t.Fatalf("expected exactly 3 top-risk lines, got %d\n%s", n, body)
	}
}

// 用户自定义模板时，卡片正文应沿用模板（其余渠道格式不变的保障）
func TestCard_CustomTemplatePreserved(t *testing.T) {
	r := sampleResult()
	p := &FeishuProvider{config: FeishuConfig{MessageTemplate: "自定义: {{taskName}}"}}
	payload := p.buildCard(r)
	card := payload["card"].(map[string]interface{})
	for _, el := range card["elements"].([]map[string]interface{}) {
		if el["tag"] == "div" {
			content := el["text"].(map[string]interface{})["content"].(string)
			if !strings.Contains(content, "自定义: 每日扫描") {
				t.Fatalf("custom template not used, got %s", content)
			}
		}
	}
}
