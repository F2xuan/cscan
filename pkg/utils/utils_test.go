package utils

import (
	"testing"
)

func TestIsIPAddress(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"有效IPv4", "192.168.1.1", true},
		{"有效IPv6", "::1", true},
		{"无效IP", "not-an-ip", false},
		{"域名", "example.com", false},
		{"空字符串", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsIPAddress(tc.input)
			if result != tc.expected {
				t.Errorf("IsIPAddress(%q) = %v, 期望 %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetRootDomain(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"子域名", "api.example.com", "example.com"},
		{"根域名", "example.com", "example.com"},
		{"多级子域名", "test.api.example.com", "example.com"},
		{"中国域名", "api.test.com.cn", "test.com.cn"},
		{"英国域名", "api.test.co.uk", "test.co.uk"},
		{"IP地址", "192.168.1.1", "192.168.1.1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetRootDomain(tc.input)
			if result != tc.expected {
				t.Errorf("GetRootDomain(%q) = %q, 期望 %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsValidDomain(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"有效域名", "example.com", true},
		{"子域名", "api.example.com", true},
		{"泛域名", "*.example.com", true},
		{"带连字符", "my-site.com", true},
		{"无效域名-无后缀", "example", false},
		{"IP地址", "192.168.1.1", false},
		{"空字符串", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidDomain(tc.input)
			if result != tc.expected {
				t.Errorf("IsValidDomain(%q) = %v, 期望 %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected int // 期望的唯一元素数量
	}{
		{"有重复", []string{"a", "b", "a", "c", "b"}, 3},
		{"无重复", []string{"a", "b", "c"}, 3},
		{"全部重复", []string{"a", "a", "a"}, 1},
		{"空切片", []string{}, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := UniqueStrings(tc.input)
			if len(result) != tc.expected {
				t.Errorf("UniqueStrings(%v) 长度 = %d, 期望 %d", tc.input, len(result), tc.expected)
			}
		})
	}
}

func TestRandomInt(t *testing.T) {
	t.Run("正常范围", func(t *testing.T) {
		result := RandomInt(1, 10)
		if result < 1 || result > 10 {
			t.Errorf("RandomInt(1, 10) = %d, 期望在 [1, 10] 范围内", result)
		}
	})

	t.Run("min大于max应交换", func(t *testing.T) {
		result := RandomInt(10, 1)
		if result < 1 || result > 10 {
			t.Errorf("RandomInt(10, 1) = %d, 期望在 [1, 10] 范围内", result)
		}
	})

	t.Run("相等范围", func(t *testing.T) {
		result := RandomInt(5, 5)
		if result != 5 {
			t.Errorf("RandomInt(5, 5) = %d, 期望 5", result)
		}
	})
}

func TestIsSubdomain(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"子域名", "api.example.com", true},
		{"根域名", "example.com", false},
		{"多级子域名", "test.api.example.com", true},
		{"IP地址", "192.168.1.1", false},
		{"带协议", "https://api.example.com", true},
		{"带端口", "api.example.com:8080", true},
		{"带路径", "api.example.com/path", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsSubdomain(tc.input)
			if result != tc.expected {
				t.Errorf("IsSubdomain(%q) = %v, 期望 %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParseTarget(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		expectedHost string
		expectedPort int
		expectedPath string
		isIP         bool
		isDomain     bool
		isSubdomain  bool
		hasPort      bool
		protocol     string
	}{
		{
			"完整URL",
			"https://api.example.com:8080/admin/login",
			"api.example.com", 8080, "/admin/login",
			false, true, true, true, "https",
		},
		{
			"域名带端口",
			"example.com:443",
			"example.com", 443, "",
			false, true, false, true, "",
		},
		{
			"纯域名",
			"example.com",
			"example.com", 0, "",
			false, true, false, false, "",
		},
		{
			"IP带端口",
			"192.168.1.1:8080",
			"192.168.1.1", 8080, "",
			true, false, false, true, "",
		},
		{
			"HTTP协议",
			"http://test.com/path",
			"test.com", 0, "/path",
			false, true, false, false, "http",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := ParseTarget(tc.input)

			if info.Host != tc.expectedHost {
				t.Errorf("Host = %q, 期望 %q", info.Host, tc.expectedHost)
			}
			if info.Port != tc.expectedPort {
				t.Errorf("Port = %d, 期望 %d", info.Port, tc.expectedPort)
			}
			if info.Path != tc.expectedPath {
				t.Errorf("Path = %q, 期望 %q", info.Path, tc.expectedPath)
			}
			if info.IsIP != tc.isIP {
				t.Errorf("IsIP = %v, 期望 %v", info.IsIP, tc.isIP)
			}
			if info.IsDomain != tc.isDomain {
				t.Errorf("IsDomain = %v, 期望 %v", info.IsDomain, tc.isDomain)
			}
			if info.IsSubdomain != tc.isSubdomain {
				t.Errorf("IsSubdomain = %v, 期望 %v", info.IsSubdomain, tc.isSubdomain)
			}
			if info.HasPort != tc.hasPort {
				t.Errorf("HasPort = %v, 期望 %v", info.HasPort, tc.hasPort)
			}
			if info.Protocol != tc.protocol {
				t.Errorf("Protocol = %q, 期望 %q", info.Protocol, tc.protocol)
			}
		})
	}
}

func TestBuildTargetWithPort(t *testing.T) {
	testCases := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{"带端口", "example.com", 8080, "example.com:8080"},
		{"无端口", "example.com", 0, "example.com"},
		{"负端口", "example.com", -1, "example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildTargetWithPort(tc.host, tc.port)
			if result != tc.expected {
				t.Errorf("BuildTargetWithPort(%q, %d) = %q, 期望 %q", tc.host, tc.port, result, tc.expected)
			}
		})
	}
}
