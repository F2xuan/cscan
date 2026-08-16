package svc

import "testing"

func TestNormalizeTechName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Kibana[httpx]", "kibana"},
		{"Nginx:1.18.0[httpx]", "nginx"},
		{"Kibana[httpx+wappalyzer]", "kibana"},
		{"Kibana[httpx+custom(1,2)]", "kibana"},
		{"  React ", "react"},
		{"Nginx", "nginx"},
		{"", ""},
		{"[httpx]", ""},
	}
	for _, c := range cases {
		if got := NormalizeTechName(c.in); got != c.want {
			t.Errorf("NormalizeTechName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTechIconMetaLoadAndResolve(t *testing.T) {
	m := NewTechIconMeta()
	m.Load()

	for _, name := range []string{"Nginx:1.18.0[httpx]", "Kibana[httpx+wappalyzer]", "React"} {
		if icon := m.ResolveIconFile(name); icon == "" {
			t.Errorf("ResolveIconFile(%q) 为空，期望能从内嵌指纹解析出图标文件名", name)
		}
	}
	if icon := m.ResolveIconFile("___不存在的技术___"); icon != "" {
		t.Errorf("未知技术应返回空串，实际 %q", icon)
	}
}

func TestTokenizeTechName(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"tengine httpd", []string{"tengine", "httpd"}},
		{"spring-boot", []string{"spring", "boot"}},
		{"mini_httpd", []string{"mini", "httpd"}},
		{"  nginx  ", []string{"nginx"}},
		{"", nil},
	}
	for _, c := range cases {
		got := tokenizeTechName(c.in)
		if len(got) != len(c.want) {
			t.Errorf("tokenizeTechName(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("tokenizeTechName(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

// TestResolveIconFileFuzzy 自定义指纹名（ARL finger.json 风格）分词模糊兜底。
// 期望值依赖内嵌指纹库的固定命名，若升级 wappalyzergo 后失败需同步调整。
func TestResolveIconFileFuzzy(t *testing.T) {
	m := NewTechIconMeta()
	m.Load()

	cases := []struct {
		name string
		want string
	}{
		// 用户报障场景：整串精确匹配 404，分词后命中内置名
		{"Tengine httpd", "Tengine.png"},
		{"Microsoft IIS httpd", "Microsoft.svg"}, // 命中内置名 IIS，其图标即 Microsoft.svg
		{"Apache Tomcat", "Apache Tomcat.svg"},   // 整名即内置名（多词窗口直接命中）
		// 精确命中的场景不回归
		{"Nginx:1.18.0[httpx]", "Nginx.svg"},
		// 来源后缀 + 版本号剥离后再模糊
		{"Tengine httpd[custom]", "Tengine.png"},
		// 无命中：不存在的词 + 全噪声词窗口
		{"Ruoyi", ""},
		{"httpd web server", ""},
		// 超短名不因最小长度缺失而误命中（"C"/"Go" 为合法内置名）
		{"Go Web Framework", ""},
		// 连字符分隔等价空格
		{"Apache-Tomcat", "Apache Tomcat.svg"},
	}
	for _, c := range cases {
		if got := m.ResolveIconFileFuzzy(c.name); got != c.want {
			t.Errorf("ResolveIconFileFuzzy(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestResolveIconFileFuzzyLongestWindowWins 多个窗口可命中时，最长（最具体）窗口优先
func TestResolveIconFileFuzzyLongestWindowWins(t *testing.T) {
	m := &TechIconMeta{iconByName: map[string]string{
		"tomcat":         "Tomcat.svg",
		"apache tomcat":  "Apache Tomcat.svg",
		"microsoft":      "Microsoft.svg",
		"iis":            "Microsoft.svg",
		"c":              "C.png",
	}}
	cases := []struct {
		name string
		want string
	}{
		{"Microsoft Apache Tomcat httpd", "Apache Tomcat.svg"}, // "apache tomcat" 而非更靠左的单词 "microsoft"
		{"Apache Tomcat", "Apache Tomcat.svg"},
		{"IIS", "Microsoft.svg"},
		{"Cde Server", ""}, // "cde" 非内置名；"c" 因长度<3 不参与匹配
	}
	for _, c := range cases {
		if got := m.ResolveIconFileFuzzy(c.name); got != c.want {
			t.Errorf("ResolveIconFileFuzzy(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
