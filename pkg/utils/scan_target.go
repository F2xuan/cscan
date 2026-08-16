package utils

import (
	"net/url"
	"strings"
)

// SplitTargetTokens 把任务 Target 字段按换行/逗号/分号/空白切成 token 列表。
func SplitTargetTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t'
	})
}

// ParseScanTarget 把扫描任务里的单个目标 token 解析为顶层资产目标。
// 支持 IP / 域名 / URL（取 hostname）；CIDR、IP 段等暂不支持的类型返回 ok=false。
// 域名统一归一到根域名，与资产写入侧 ResolveAssetTarget 口径一致，
// 避免任务目标 www.example.com 与资产归并出的 example.com 出现两个顶层目标。
func ParseScanTarget(raw string) (targetType, value string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}

	// URL：http(s)://host[:port]/path → 取 hostname
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			raw = u.Hostname()
		}
	}

	// CIDR / IP 段：当前 meta 只支持 ip/domain 两类，先跳过
	if strings.Contains(raw, "/") || strings.Contains(raw, "-") {
		return "", "", false
	}

	if IsIPAddress(raw) {
		return "ip", raw, true
	}
	if raw != "" && strings.Contains(raw, ".") {
		if root := GetRootDomain(strings.ToLower(raw)); root != "" {
			return "domain", root, true
		}
	}
	return "", "", false
}
