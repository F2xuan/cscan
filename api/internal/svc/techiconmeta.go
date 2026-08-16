package svc

import (
	"encoding/json"
	"strings"
	"sync"

	wappalyzer "github.com/projectdiscovery/wappalyzergo"
	"github.com/zeromicro/go-zero/core/logx"
)

// NormalizeTechName 归一化技术名：去掉检测来源后缀（如 [httpx+wappalyzer]）与 :版本号，
// 统一小写。asset.app 条目形如 "Kibana[httpx]"、"Nginx:1.18.0[httpx]"。
func NormalizeTechName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.IndexByte(name, '['); i >= 0 {
		name = name[:i]
	}
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

const (
	// techIconFuzzyMinLen 分词模糊匹配候选窗口的最小字符数，
	// 指纹库存在 "C"、"Go"、"D3" 等超短名，不设下限会导致任意名字误命中
	techIconFuzzyMinLen = 3
)

// techIconNoiseTokens 噪声词：常出现在自定义指纹名（如 "Tengine httpd"）中，
// 但单独作为窗口匹配到的图标没有辨识意义，整窗全为噪声词时跳过
var techIconNoiseTokens = map[string]bool{
	"httpd": true, "server": true, "web": true, "http": true,
	"framework": true, "cms": true, "cdn": true, "os": true,
	"software": true, "system": true, "platform": true,
}

// techIconNameSeparatorReplacer 分隔符归一化：连字符/下划线在指纹命名中与空格等价
var techIconNameSeparatorReplacer = strings.NewReplacer("-", " ", "_", " ")

// tokenizeTechName 把归一化后的技术名拆成小写词元（"Spring-Boot" → ["spring","boot"]）
func tokenizeTechName(normalized string) []string {
	return strings.Fields(techIconNameSeparatorReplacer.Replace(normalized))
}

// TechIconMeta 技术名 → 图标文件名 的内存映射。
// 数据来自 wappalyzergo 内嵌指纹库（Wappalyzer 社区延续版 webappanalyzer），
// 构建映射无需外部请求；图标文件名与上游 images/icons 目录一一对应。
type TechIconMeta struct {
	mu         sync.RWMutex
	iconByName map[string]string // 归一化名 → 图标文件名
}

func NewTechIconMeta() *TechIconMeta {
	return &TechIconMeta{iconByName: make(map[string]string)}
}

// Load 解析 wappalyzergo 内嵌指纹数据，构建名称→图标映射。
// 幂等；失败仅记录日志并保持空映射（图标功能降级，不影响其他服务）。
func (m *TechIconMeta) Load() {
	raw := wappalyzer.GetFingerprints()
	if raw == "" {
		logx.Error("[TechIcon] embedded fingerprints data unavailable")
		return
	}

	// 只提取 icon 字段，避免为几万条指纹解析完整匹配规则
	var payload struct {
		Apps map[string]struct {
			Icon string `json:"icon"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		logx.Errorf("[TechIcon] failed to parse fingerprints: %v", err)
		return
	}

	iconByName := make(map[string]string, len(payload.Apps))
	for name, fp := range payload.Apps {
		if fp.Icon == "" {
			continue
		}
		normalized := NormalizeTechName(name)
		iconByName[normalized] = fp.Icon
		// 追加分隔符归一化别名（"React-Bootstrap" → "react bootstrap"），
		// 供分词窗口模糊匹配使用；与原键指向同一图标，冲突时后写覆盖无实际影响
		if tokenized := strings.Join(tokenizeTechName(normalized), " "); tokenized != normalized {
			iconByName[tokenized] = fp.Icon
		}
	}

	m.mu.Lock()
	m.iconByName = iconByName
	m.mu.Unlock()
	logx.Infof("[TechIcon] loaded icon metadata for %d technologies", len(iconByName))
}

// ResolveIconFile 返回技术名对应的图标文件名，无映射时返回空串
func (m *TechIconMeta) ResolveIconFile(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.iconByName[NormalizeTechName(name)]
}

// ResolveIconFileFuzzy 先精确匹配；未命中时按分词窗口模糊匹配内置指纹库，
// 供自定义指纹名兜底——这类名字（ARL finger.json 风格，如 "Tengine httpd"、
// "Microsoft IIS httpd"）往往是内置库已知名加上 httpd/server 等后缀词。
// 窗口按长度降序枚举（最长=最具体优先），同长取最左；全噪声词窗口跳过。
func (m *TechIconMeta) ResolveIconFileFuzzy(name string) string {
	if icon := m.ResolveIconFile(name); icon != "" {
		return icon
	}
	normalized := NormalizeTechName(name)
	if normalized == "" {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.iconByName) == 0 {
		return ""
	}

	tokens := tokenizeTechName(normalized)
	for size := len(tokens); size >= 1; size-- {
		for start := 0; start+size <= len(tokens); start++ {
			candidate := strings.Join(tokens[start:start+size], " ")
			if len([]rune(candidate)) < techIconFuzzyMinLen || allNoiseTokens(tokens[start:start+size]) {
				continue
			}
			if icon, ok := m.iconByName[candidate]; ok {
				return icon
			}
		}
	}
	return ""
}

// allNoiseTokens 判断窗口内词元是否全为噪声词（httpd/server 等）
func allNoiseTokens(tokens []string) bool {
	for _, tok := range tokens {
		if !techIconNoiseTokens[tok] {
			return false
		}
	}
	return true
}
