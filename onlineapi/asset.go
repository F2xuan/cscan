package onlineapi

import (
	"fmt"
	"net"
	"strings"
	"time"

	"cscan/model"
)

// ParseApps 解析指纹字符串，支持逗号分隔，过滤空值（手动导入与定时拉取共用，T3.1）。
func ParseApps(product string) []string {
	if product == "" {
		return nil
	}

	var apps []string
	// 支持中英文逗号分隔
	parts := strings.FieldsFunc(product, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；'
	})

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			apps = append(apps, p)
		}
	}

	return apps
}

// CleanHost 去除 host 字符串中的协议前缀、路径与端口（仅保留主机名）。
func CleanHost(host string) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	} else if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}
	// Remove path
	if idx := strings.Index(host, "/"); idx > 0 {
		host = host[:idx]
	}
	// Remove port if present (e.g. example.com:8080 -> example.com)
	// Ignore IPv6 brackets for now, assuming standard output from APIs
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		// Verify if it's likely a port (not part of IPv6 address without brackets)
		if strings.Count(host, ":") == 1 || strings.Contains(host, "]:") {
			host = host[:idx]
		}
	}
	return host
}

// ExtractDomain 从 host 字段中提取域名部分（去除协议前缀、端口、路径）。
// 如果提取结果是 IP 地址则返回空字符串。
func ExtractDomain(host string) string {
	cleaned := CleanHost(host)
	if cleaned == "" {
		return ""
	}
	// 如果是 IP 地址，不作为域名返回
	if net.ParseIP(cleaned) != nil {
		return ""
	}
	return cleaned
}

// ResolveHostAndDomain 从在线搜索结果中解析 host 和 domain。
// 优先从 rawHost 提取域名作为 host（资产标识），IP 仅存到 Ip 字段。
// 返回值: host（用于 Authority/Host 字段）, domain（用于 Domain 字段）。
func ResolveHostAndDomain(rawHost, rawIP, rawDomain string) (host, domain string) {
	hostDomain := ExtractDomain(rawHost)

	if hostDomain != "" {
		host = hostDomain
		domain = hostDomain
	} else {
		host = CleanHost(rawHost)
		if host == "" {
			host = rawIP
		}
		domain = rawDomain
	}
	return
}

// BuildAsset 由在线 API 搜索结果构造资产对象（手动导入与定时拉取共用，T3.1 抽公共函数）。
// 内部完成 host/domain 解析、标签生成、IP 归集与默认值初始化。
// 注意：Source 字段由各调用方按自身语义设置（手动导入用 "onlineapi"，批量/拉取用 "onlineapi-<platform>"），
// 以保持既有行为不变。
func BuildAsset(rawHost, rawIP, rawDomain, protocol, title, server, country, city, banner, product string, port int, platform string) *model.Asset {
	host, domain := ResolveHostAndDomain(rawHost, rawIP, rawDomain)

	// 自动添加标签
	platformTag := platform
	if len(platformTag) > 0 {
		platformTag = strings.ToUpper(platformTag[:1]) + platformTag[1:]
	}
	if platformTag == "" {
		platformTag = "OnlineAPI"
	}

	labels := []string{"OnlineAPI", platformTag}

	asset := &model.Asset{
		Authority: fmt.Sprintf("%s:%d", host, port),
		Host:      host,
		Port:      port,
		Service:   protocol,
		Title:     title,
		App:       ParseApps(product),
		Labels:    labels,
		IsHTTP:    protocol == "http" || protocol == "https",
		Domain:    domain,
		Server:    server,
		Banner:    banner,
		// Initialize default fields to ensure compatibility
		IsNewAsset: true,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	// Populate IP info if available
	if rawIP != "" {
		asset.Ip = model.IP{
			IpV4: []model.IPV4{{IPName: rawIP, Location: country + " " + city}},
		}
	}

	return asset
}
