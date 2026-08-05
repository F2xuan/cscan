package common

import (
	"cscan/pkg/utils"
)

// IsIPAddress 判断是否为IP地址
func IsIPAddress(s string) bool {
	return utils.IsIPAddress(s)
}

// GetRootDomain 获取根域名
func GetRootDomain(domain string) string {
	return utils.GetRootDomain(domain)
}
