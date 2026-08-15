package worker

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"cscan/internal/scanner"
	"cscan/internal/scheduler"
)

// buildAuthority 构建主机权威表示（host:port），80/443 端口省略端口号
func buildAuthority(host string, port int) string {
	if port == 80 || port == 443 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// generateAssetsFromTarget 从目标生成初始资产列表（用于端口扫描禁用时）
// 支持的目标格式：
// - 单个IP: 192.168.1.1
// - IP范围: 192.168.1.1-192.168.1.10
// - CIDR: 192.168.1.0/24
// - 域名: example.com
// - 带端口: 192.168.1.1:8080 或 example.com:443
// - URL: http://example.com:8080
// 域名无法解析到有效IPv4/IPv6地址时，该目标会被跳过
func (w *Worker) generateAssetsFromTarget(target string, portConfig *scheduler.PortScanConfig) []*scanner.Asset {
	var assets []*scanner.Asset

	// 默认端口列表（当不进行端口扫描时，默认只探测常见的 Web 端口）
	defaultPorts := []int{80, 443}

	// 只有在启用了端口扫描的情况下，才使用配置中的自定义端口列表进行资产拆分
	if portConfig != nil && portConfig.Enable && portConfig.Ports != "" {
		defaultPorts = parsePortList(portConfig.Ports)
	}

	// 解析目标
	targets := strings.Split(target, "\n")
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		// 处理URL格式
		if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
			asset := w.parseURLToAsset(t)
			if asset != nil && w.isDomainResolvable(asset.Host) {
				assets = append(assets, asset)
			} else if asset != nil {
				w.logger.Info("Skipping target %s: domain cannot be resolved to valid IP", asset.Host)
			}
			continue
		}

		// 处理带端口的格式 host:port
		if strings.Contains(t, ":") && !strings.Contains(t, "/") {
			parts := strings.Split(t, ":")
			if len(parts) == 2 {
				host := parts[0]
				port := 80
				if p, err := strconv.Atoi(parts[1]); err == nil {
					port = p
				}
				if !w.isDomainResolvable(host) {
					w.logger.Info("Skipping target %s: domain cannot be resolved to valid IP", host)
					continue
				}
				asset := &scanner.Asset{
					Host:      host,
					Port:      port,
					Authority: fmt.Sprintf("%s:%d", host, port),
					IsHTTP:    scanner.IsHTTPService("", port),
				}
				assets = append(assets, asset)
				continue
			}
		}

		// 处理CIDR格式 - 跳过，因为没有端口扫描无法确定开放端口
		if strings.Contains(t, "/") {
			w.logger.Warn("CIDR target %s skipped: port scan disabled, cannot determine open ports", t)
			continue
		}

		// 处理IP范围格式 - 跳过
		if strings.Contains(t, "-") && !strings.Contains(t, ".") {
			w.logger.Warn("IP range target %s skipped: port scan disabled, cannot determine open ports", t)
			continue
		}

		// 单个主机（IP或域名），使用默认端口
		// 域名目标需先验证DNS解析
		if !w.isDomainResolvable(t) {
			w.logger.Info("Skipping target %s: domain cannot be resolved to valid IP", t)
			continue
		}
		for _, port := range defaultPorts {
			asset := &scanner.Asset{
				Host:      t,
				Port:      port,
				Authority: buildAuthority(t, port),
				IsHTTP:    scanner.IsHTTPService("", port),
			}
			assets = append(assets, asset)
		}
	}

	return assets
}

// isDomainResolvable 检查域名目标是否可以解析到有效的IPv4/IPv6地址
// IP地址目标直接返回true
func (w *Worker) isDomainResolvable(host string) bool {
	if host == "" {
		return false
	}
	// IP地址直接可用，无需DNS解析
	if net.ParseIP(host) != nil {
		return true
	}
	// 域名：尝试DNS解析
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	// 至少有一个非回环地址
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return true
		}
	}
	return false
}

// parseURLToAsset 解析URL为资产
func (w *Worker) parseURLToAsset(urlStr string) *scanner.Asset {
	// 简单解析URL
	scheme := "http"
	host := ""
	port := 80

	if strings.HasPrefix(urlStr, "https://") {
		scheme = "https"
		port = 443
		urlStr = strings.TrimPrefix(urlStr, "https://")
	} else if strings.HasPrefix(urlStr, "http://") {
		urlStr = strings.TrimPrefix(urlStr, "http://")
	}

	// 移除路径部分
	if idx := strings.Index(urlStr, "/"); idx > 0 {
		urlStr = urlStr[:idx]
	}

	// 解析host:port
	if strings.Contains(urlStr, ":") {
		parts := strings.Split(urlStr, ":")
		host = parts[0]
		if p, err := strconv.Atoi(parts[1]); err == nil {
			port = p
		}
	} else {
		host = urlStr
	}

	if host == "" {
		return nil
	}

	return &scanner.Asset{
		Host:      host,
		Port:      port,
		Authority: buildAuthority(host, port),
		Service:   scheme,
		IsHTTP:    true,
	}
}

// parsePortList 解析端口列表字符串
func parsePortList(portsStr string) []int {
	var ports []int
	seen := make(map[int]bool)

	parts := strings.Split(portsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 处理端口范围 (如 80-90)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 == nil && err2 == nil && start <= end {
					for p := start; p <= end && p <= 65535; p++ {
						if !seen[p] {
							seen[p] = true
							ports = append(ports, p)
						}
					}
				}
			}
		} else {
			// 单个端口
			if p, err := strconv.Atoi(part); err == nil && p > 0 && p <= 65535 {
				if !seen[p] {
					seen[p] = true
					ports = append(ports, p)
				}
			}
		}
	}

	return ports
}

// initGeolocation 初始化 IP 地理位置服务
