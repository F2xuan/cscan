package notify

import (
	"strings"
	"testing"
)

func TestBuildHighRiskDetails_Empty(t *testing.T) {
	if got := buildHighRiskDetails(nil); got != "" {
		t.Fatalf("nil info expected empty, got %q", got)
	}
	if got := buildHighRiskDetails(&HighRiskInfo{}); got != "" {
		t.Fatalf("empty info expected empty, got %q", got)
	}
}

// 新风险明细需按 critical > high > medium > low 排序（T1.4 验收）
func TestBuildHighRiskDetails_NewRiskSortedBySeverity(t *testing.T) {
	info := &HighRiskInfo{
		NewRisks: []RiskSummary{
			{Kind: "vuln", Severity: "low", Name: "svc-low"},
			{Kind: "vuln", Severity: "critical", Name: "svc-crit"},
			{Kind: "vuln", Severity: "high", Name: "svc-high"},
			{Kind: "vuln", Severity: "medium", Name: "svc-med"},
		},
	}
	out := buildHighRiskDetails(info)
	ci := strings.Index(out, "svc-crit")
	hi := strings.Index(out, "svc-high")
	mi := strings.Index(out, "svc-med")
	li := strings.Index(out, "svc-low")
	if ci < 0 || hi < 0 || mi < 0 || li < 0 {
		t.Fatalf("missing risk name in output: %s", out)
	}
	if !(ci < hi && hi < mi && mi < li) {
		t.Fatalf("risk order wrong: crit=%d high=%d med=%d low=%d\n%s", ci, hi, mi, li, out)
	}
}

// 超上限必须截断并显式标注"仅显示前 20 条"，禁止静默丢弃（T1.4 验收）
func TestBuildHighRiskDetails_NewRiskTruncation(t *testing.T) {
	risks := make([]RiskSummary, 0, 25)
	for i := 0; i < 25; i++ {
		risks = append(risks, RiskSummary{Kind: "vuln", Severity: "high", Name: "R"})
	}
	info := &HighRiskInfo{NewRisks: risks}
	out := buildHighRiskDetails(info)
	if !strings.Contains(out, "仅显示前 20 条") {
		t.Fatalf("expected truncation note, got %q", out)
	}
	if strings.Contains(out, "\n  21.") {
		t.Fatalf("item 21 should be truncated: %s", out)
	}
	if !strings.Contains(out, "\n  20.") {
		t.Fatalf("item 20 should be present: %s", out)
	}
}

// 新增资产明细 + 已修复漏洞数渲染（T1.4 验收）
func TestBuildHighRiskDetails_NewAssetListAndFixed(t *testing.T) {
	info := &HighRiskInfo{
		NewAssetList: []AssetSummary{
			{Authority: "a.example.com", FirstSeenTime: "2026-07-28 10:00:00"},
			{Authority: "b.example.com"},
		},
		FixedVulCount: 3,
	}
	out := buildHighRiskDetails(info)
	if !strings.Contains(out, "a.example.com") {
		t.Fatalf("expected authority in output, got %q", out)
	}
	if !strings.Contains(out, "已修复漏洞: 3 个") {
		t.Fatalf("expected fixed vul count, got %q", out)
	}
}

// 空值省略：无明细/无新风险/无修复时不应出现对应区块（T1.4 验收）
func TestBuildHighRiskDetails_EmptyValueOmission(t *testing.T) {
	info := &HighRiskInfo{
		HighRiskVulCount: 0,
	}
	out := buildHighRiskDetails(info)
	if strings.Contains(out, "新风险明细") || strings.Contains(out, "已修复漏洞") || strings.Contains(out, "新增资产明细") {
		t.Fatalf("unexpected blocks in output: %q", out)
	}
}

// 新增资产明细同样需截断并标注（T1.4 验收）
func TestBuildHighRiskDetails_NewAssetListTruncation(t *testing.T) {
	list := make([]AssetSummary, 0, 25)
	for i := 0; i < 25; i++ {
		list = append(list, AssetSummary{Authority: "host"})
	}
	info := &HighRiskInfo{NewAssetList: list}
	out := buildHighRiskDetails(info)
	if !strings.Contains(out, "仅显示前 20 条") {
		t.Fatalf("expected truncation note for asset list, got %q", out)
	}
}
