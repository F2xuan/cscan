package scheduler

// 历史上 OnlineAPIPuller 相关测试（isRetryableError/nextRunTime/safeErrMsg）已随自动拉取功能废弃而移除。
// 保留 BuildAsset / ParseApps 的共享逻辑测试（这两个函数位于 onlineapi 包，属于公共资产构造，未废弃）。

import (
	"testing"

	"cscan/onlineapi"
)

func TestBuildAssetShared(t *testing.T) {
	// 域名 URL → 解析为域名主机 + 正确 authority
	a := onlineapi.BuildAsset("https://example.com", "", "", "https", "title", "nginx", "CN", "BJ", "banner", "nginx", 443, "fofa")
	if a.Host != "example.com" {
		t.Errorf("expected host example.com, got %q", a.Host)
	}
	if a.Authority != "example.com:443" {
		t.Errorf("expected authority example.com:443, got %q", a.Authority)
	}
	if a.Port != 443 {
		t.Errorf("expected port 443, got %d", a.Port)
	}
	// Source 由调用方设置，BuildAsset 不设置
	if a.Source != "" {
		t.Errorf("expected empty Source (caller sets), got %q", a.Source)
	}
	hasOnline := false
	hasFofa := false
	for _, l := range a.Labels {
		if l == "OnlineAPI" {
			hasOnline = true
		}
		if l == "Fofa" {
			hasFofa = true
		}
	}
	if !hasOnline || !hasFofa {
		t.Errorf("expected labels OnlineAPI+Fofa, got %v", a.Labels)
	}

	// IP 主机 → 保留 IP
	b := onlineapi.BuildAsset("1.2.3.4", "1.2.3.4", "", "http", "", "", "", "", "", "", 80, "hunter")
	if b.Host != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %q", b.Host)
	}
	if b.Authority != "1.2.3.4:80" {
		t.Errorf("expected authority 1.2.3.4:80, got %q", b.Authority)
	}
	if len(b.Ip.IpV4) != 1 || b.Ip.IpV4[0].IPName != "1.2.3.4" {
		t.Errorf("expected IP populated, got %+v", b.Ip)
	}
}

func TestParseAppsShared(t *testing.T) {
	apps := onlineapi.ParseApps("nginx, redis，mysql;tomcat")
	if len(apps) != 4 {
		t.Errorf("expected 4 apps, got %v", apps)
	}
	if onlineapi.ParseApps("") != nil {
		t.Errorf("expected nil for empty")
	}
}
