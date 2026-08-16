package utils

import "testing"

// TestParseScanTargetRootDomain 校验任务目标 token → 顶层目标的解析口径：
// 域名必须归一到根域（与资产写入侧 ResolveAssetTarget 一致），
// 避免同一目标在资产空间搜索出现 www 根域两个条目。
func TestParseScanTargetRootDomain(t *testing.T) {
	testCases := []struct {
		name       string
		input      string
		targetType string
		value      string
		ok         bool
	}{
		{"裸根域", "example.com", "domain", "example.com", true},
		{"www 子域归一到根域", "www.example.com", "domain", "example.com", true},
		{"多级子域归一到根域", "a.b.example.com", "domain", "example.com", true},
		{"URL 取 hostname 后归一", "https://www.example.com/path", "domain", "example.com", true},
		{"大写域名归一", "WWW.Example.COM", "domain", "example.com", true},
		{"IP 不归一", "8.8.8.8", "ip", "8.8.8.8", true},
		{"CIDR 跳过", "192.168.1.0/24", "", "", false},
		{"IP 段跳过", "192.168.1.1-192.168.1.10", "", "", false},
		{"空串跳过", "", "", "", false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotValue, gotOk := ParseScanTarget(tc.input)
			if gotOk != tc.ok || gotType != tc.targetType || gotValue != tc.value {
				t.Fatalf("ParseScanTarget(%q) = (%q, %q, %v), expected (%q, %q, %v)",
					tc.input, gotType, gotValue, gotOk, tc.targetType, tc.value, tc.ok)
			}
		})
	}
}
