package scanner

import (
	"bufio"
	"net"
	"strings"
)

// dnsResolver DNS 解析器接口 - 消除重复代码
type dnsResolver interface {
	resolve(domain string) ([]net.IP, error)
}

// stdlibResolver 使用标准库的解析器
type stdlibResolver struct{}

func (r *stdlibResolver) resolve(domain string) ([]net.IP, error) {
	return net.LookupIP(domain)
}

// resolveSingleDomainAsset 解析单个域名，返回带 IP 信息的资产
// 所有 IP 都是回环地址时返回 nil（防止扫描本地服务）
func resolveSingleDomainAsset(domain string, resolver dnsResolver) *Asset {
	ips, err := resolver.resolve(domain)
	if err != nil || len(ips) == 0 {
		return nil
	}

	// 过滤回环地址：如果所有 IP 都是 127.0.0.1 等回环地址，跳过该域名
	allLoopback := true
	for _, ip := range ips {
		if !ip.IsLoopback() {
			allLoopback = false
			break
		}
	}
	if allLoopback {
		return nil
	}

	asset := &Asset{
		Authority: domain,
		Host:      domain,
		Category:  "domain",
	}

	for _, ip := range ips {
		// 跳过回环地址
		if ip.IsLoopback() {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			asset.IPV4 = append(asset.IPV4, IPInfo{IP: ip4.String()})
		} else {
			asset.IPV6 = append(asset.IPV6, IPInfo{IP: ip.String()})
		}
	}

	if cname, err := net.LookupCNAME(domain); err == nil && cname != domain+"." {
		asset.CName = strings.TrimSuffix(cname, ".")
	}

	return asset
}

// newLineScanner 创建一个带扩容缓冲的行扫描器，用于解析 CLI 的 JSON 行输出
func newLineScanner(s string) *bufio.Scanner {
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return sc
}

// isImageData 通过文件头魔数判断数据是否为常见图片格式
// 过滤 HTML 错误页等非图片响应
func isImageData(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return true
	}
	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return true
	}
	// ICO: 00 00 01 00 or 00 00 02 00
	if data[0] == 0x00 && data[1] == 0x00 && (data[2] == 0x01 || data[2] == 0x02) && data[3] == 0x00 {
		return true
	}
	// BMP: 42 4D
	if data[0] == 0x42 && data[1] == 0x4D {
		return true
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return true
	}
	// SVG: 以 '<svg' 或 '<?xml' 开头（文本格式）
	if data[0] == '<' {
		header := strings.ToLower(string(data[:min(len(data), 100)]))
		if strings.HasPrefix(header, "<svg") || (strings.HasPrefix(header, "<?xml") && strings.Contains(header, "<svg")) {
			return true
		}
	}
	return false
}
