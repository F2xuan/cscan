package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// StringUtils 字符串工具集

// ContainsAny 检查字符串是否包含任意一个子串
func ContainsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// MD5Hash 计算MD5哈希
func MD5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}

// SHA256Hash 计算SHA256哈希
func SHA256Hash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}
