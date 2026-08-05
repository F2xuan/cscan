package scanner

import (
	"bytes"
	"io"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ToUTF8 将原始响应字节按字符集探测解码为 UTF-8 字符串，消除
// “GBK 页面被当作 UTF-8 记录/打印”导致的乱码（mojibake）。
//
// 设计要点：
//   - 仅用于生成“给人看 / 落库 / 日志”的字符串；调用方仍应保留原始字节
//     （如 FingerprintData.BodyBytes）用于 GBK 关键词匹配，不要把 BodyBytes 也转换。
//   - 探测优先级：Content-Type / BOM / <meta charset> → 若探测到 GBK 等编码则解码；
//     若探测为 Nop（即假定 UTF-8），再校验字节是否真的合法 UTF-8，不合法则退回 GBK 解码。
func ToUTF8(raw []byte, contentType string) string {
	if len(raw) == 0 {
		return ""
	}

	// 1) 用 Content-Type / BOM / <meta> 探测真实编码
	if e, _, _ := charset.DetermineEncoding(raw, contentType); e != nil && e != encoding.Nop {
		if decoded, err := decodeWith(raw, e); err == nil && decoded != string(raw) {
			return decoded
		}
	}

	// 2) 探测为 Nop（多为 UTF-8/ASCII）：若字节本身合法 UTF-8 直接返回，避免任何转换
	if utf8.Valid(raw) {
		return string(raw)
	}

	// 3) 含非法 UTF-8 序列（典型为 GBK 页面未声明 charset）：尝试按 GBK 解码
	if decoded, err := decodeGBK(raw); err == nil && decoded != string(raw) {
		return decoded
	}

	// 4) 兜底：原样返回（可能含少量乱码，但保证不崩溃）
	return string(raw)
}

// decodeWith 使用给定编码将原始字节解码为 UTF-8 字符串。
func decodeWith(raw []byte, e encoding.Encoding) (string, error) {
	r := transform.NewReader(bytes.NewReader(raw), e.NewDecoder())
	out, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// decodeGBK 将 GBK 编码字节解码为 UTF-8 字符串。
func decodeGBK(raw []byte) (string, error) {
	return decodeWith(raw, simplifiedchinese.GBK)
}
