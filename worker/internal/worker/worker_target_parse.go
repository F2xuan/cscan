package worker

import (
	"net"
	"strings"

	"cscan/internal/scanner"
)

func filterSkippedHostsAssets(assets []*scanner.Asset, skippedHosts []string) []*scanner.Asset {
	if len(skippedHosts) == 0 || len(assets) == 0 {
		return assets
	}
	skippedSet := make(map[string]bool, len(skippedHosts))
	for _, h := range skippedHosts {
		skippedSet[h] = true
	}
	var result []*scanner.Asset
	for _, a := range assets {
		if !skippedSet[a.Host] {
			result = append(result, a)
		}
	}
	return result
}

// commonSecondLevelSuffixes 常见的二级域名后缀（公共后缀）。
// 在其之下的“一级域名”为 eTLD+1，共三段（如 example.com.cn）。
var commonSecondLevelSuffixes = map[string]bool{
	"com.cn": true, "net.cn": true, "org.cn": true, "gov.cn": true, "edu.cn": true, "ac.cn": true,
	"co.uk": true, "org.uk": true, "me.uk": true,
	"co.jp": true, "ne.jp": true, "or.jp": true, "ac.jp": true,
	"com.hk": true, "org.hk": true, "edu.hk": true,
	"com.au": true, "net.au": true, "org.au": true, "co.nz": true, "net.nz": true,
	"co.za": true, "org.za": true, "co.in": true, "net.in": true, "org.in": true,
	"com.br": true, "com.mx": true, "com.tw": true, "com.sg": true,
}

// registrableDomain 返回主机名的注册域名（eTLD+1）。
// 对于多段公共后缀（如 com.cn），注册域名为倒数第三段；否则为倒数第二段。
// 若 host 不是合法域名（如 IP、CIDR、端口范围，或无法解析为主机名），返回空字符串。
func registrableDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	// IP / CIDR 不是域名（端口范围已被解析器的 TargetTypeRange 排除，不会到达此处）
	if net.ParseIP(host) != nil {
		return ""
	}
	if _, _, err := net.ParseCIDR(host); err == nil {
		return ""
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "" // 单标签不是完整域名
	}
	n := len(labels)
	// 多段公共后缀：注册域名为倒数第三段
	if n >= 3 && commonSecondLevelSuffixes[labels[n-2]+"."+labels[n-1]] {
		return strings.Join(labels[n-3:], ".")
	}
	return strings.Join(labels[n-2:], ".")
}

// isEligibleForSubdomainScan 判断目标是否需要进行子域名扫描。
// 仅“纯一级域名”（注册域名 eTLD+1，含 com.cn 等多段后缀）参与子域名枚举；
// IP、CIDR、端口范围、以及其子域名（如 www.example.com、a.b.com.cn）直接跳过。
// 复用 scanner.NewTargetParser 做目标类型判定（不重写解析器）。
func isEligibleForSubdomainScan(raw string) bool {
	t := scanner.NewTargetParser().Parse(raw)
	if t == nil {
		return false
	}
	// IP / IPv6 / CIDR / 端口范围 直接跳过
	switch t.Type {
	case scanner.TargetTypeIPv4, scanner.TargetTypeIPv6, scanner.TargetTypeCIDR, scanner.TargetTypeRange:
		return false
	}
	host := strings.ToLower(t.Host)
	if host == "" {
		return false
	}
	// URL 取其 Host；域名/子域名取 Host。仅当 Host 恰好等于注册域名时才枚举其下子域名。
	return host == registrableDomain(host)
}

// filterEligibleSubdomainTargets 从目标列表中筛选可进行子域名扫描的“纯一级域名”。
func filterEligibleSubdomainTargets(targets []string) []string {
	var eligible []string
	for _, t := range targets {
		if isEligibleForSubdomainScan(t) {
			eligible = append(eligible, t)
		}
	}
	return eligible
}

// generateBruteAssetsFromTargets 从目标生成弱口令扫描资产
// 用于强制扫描模式：
// - 如果目标携带端口（如 192.168.1.215:63791），保留该端口进行扫描
// - 如果目标没有端口，为指定服务生成默认端口资产
func (w *Worker) generateBruteAssetsFromTargets(target string, services []string) []*scanner.Asset {
	var assets []*scanner.Asset

	// 服务默认端口映射
	servicePorts := map[string]int{
		"ssh":        22,
		"mysql":      3306,
		"redis":      6379,
		"mongodb":    27017,
		"postgresql": 5432,
		"mssql":      1433,
		"ftp":        21,
		"snmp":       161,
		"oracle":     1521,
		"smb":        445,
		"mqtt":       1883,
	}

	// 端口到服务的反向映射
	portToService := map[int]string{
		22:    "ssh",
		3306:  "mysql",
		6379:  "redis",
		27017: "mongodb",
		5432:  "postgresql",
		1433:  "mssql",
		21:    "ftp",
		161:   "snmp",
		1521:  "oracle",
		445:   "smb",
		1883:  "mqtt",
	}

	// 解析目标，生成临时资产
	tempAssets := scanner.GenerateAssetsFromTargetsWithoutDNS(target)
	if len(tempAssets) == 0 {
		return assets
	}

	// 确定目标服务列表
	var targetServices []string
	if len(services) > 0 {
		targetServices = services
	} else {
		// 如果没有指定服务，使用所有已知服务
		for svc := range servicePorts {
			targetServices = append(targetServices, svc)
		}
	}

	// 转换为集合便于查找
	targetServiceSet := make(map[string]bool)
	for _, svc := range targetServices {
		targetServiceSet[svc] = true
	}

	// 按主机分组处理，避免重复
	hostAssets := make(map[string][]*scanner.Asset)

	for _, tempAsset := range tempAssets {
		host := tempAsset.Host

		// 检查是否已处理过该主机
		if _, exists := hostAssets[host]; exists {
			continue
		}

		// 如果目标有明确端口（用户指定了端口）
		if tempAsset.Port > 0 {
			// 根据端口识别服务
			if svc, ok := portToService[tempAsset.Port]; ok {
				// 检查该服务是否在目标列表中
				if targetServiceSet[svc] {
					asset := &scanner.Asset{
						Host:    host,
						Port:    tempAsset.Port, // 使用用户指定的端口
						Service: svc,
						IsHTTP:  false,
					}
					hostAssets[host] = append(hostAssets[host], asset)
				}
			} else {
				// 端口没有匹配到已知服务，但用户明确指定了端口
				// 为所有目标服务生成资产，但保留用户指定的端口
				for _, svc := range targetServices {
					asset := &scanner.Asset{
						Host:    host,
						Port:    tempAsset.Port, // 保留用户指定的端口
						Service: svc,
						IsHTTP:  false,
					}
					hostAssets[host] = append(hostAssets[host], asset)
				}
			}
		} else {
			// 没有指定端口，为所有目标服务生成默认端口资产
			for _, svc := range targetServices {
				port, ok := servicePorts[svc]
				if !ok {
					continue
				}
				asset := &scanner.Asset{
					Host:    host,
					Port:    port,
					Service: svc,
					IsHTTP:  false,
				}
				hostAssets[host] = append(hostAssets[host], asset)
			}
		}
	}

	// 合并所有资产
	for _, hostAssetList := range hostAssets {
		assets = append(assets, hostAssetList...)
	}

	return assets
}

// executePocValidateTask 执行POC验证任务
