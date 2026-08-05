package utils

import (
	"net"
	"strings"
)

// IPUtils IP工具集
// 提供IP地址相关的通用操作

// IsIPv4 判断是否为IPv4地址
func IsIPv4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() != nil
}

// IsIPv6 判断是否为IPv6地址
func IsIPv6(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() == nil
}

// GetLocalIP 获取本机IP地址
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// SplitHostPort 分离主机和端口（支持IPv6）
func SplitHostPort(hostport string) (host string, port string) {
	// IPv6格式: [::1]:8080
	if strings.HasPrefix(hostport, "[") {
		if idx := strings.LastIndex(hostport, "]:"); idx > 0 {
			return hostport[1:idx], hostport[idx+2:]
		}
		// 没有端口的IPv6: [::1]
		if strings.HasSuffix(hostport, "]") {
			return hostport[1 : len(hostport)-1], ""
		}
		return hostport, ""
	}

	// IPv4/域名格式
	if idx := strings.LastIndex(hostport, ":"); idx > 0 {
		return hostport[:idx], hostport[idx+1:]
	}

	return hostport, ""
}
