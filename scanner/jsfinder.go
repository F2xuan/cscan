package scanner

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/zeromicro/go-zero/core/logx"
)

// JSFinderOptions JSFinder 扫描选项
type JSFinderOptions struct {
	HighRiskRoutes       []string // 高危路由关键词，命中则跳过未授权检测
	AuthRequiredKeywords []string // 鉴权关键词，响应包含视为已正确鉴权（不报未授权）
	SensitiveKeywords    []string // 敏感数据关键词，响应包含视为信息泄漏
	DomainBlacklist      []string // 域名黑名单，命中则跳过抓取/未授权检测
	Threads              int      // 单目标并发下载/探测数
	Timeout              int      // 单个 HTTP 请求超时(秒)
	EnableSourcemap      bool     // 是否抓取 .js.map 增强提取覆盖率
	EnableUnauthCheck    bool     // 是否进行未授权探测
	UserAgent            string   // 自定义 UA
	MaxJSSize            int64    // 单个 JS/Map 文件最大读取字节数
	MaxJSPerTarget       int      // 单目标最多下载 JS 数量
	MaxAPIPerTarget      int      // 单目标最多探测 API 数量
}

// Validate 校验选项
func (o *JSFinderOptions) Validate() error {
	if o.Threads < 0 {
		return fmt.Errorf("threads must be non-negative, got %d", o.Threads)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	return nil
}

// JSFinderScanner JS 静态分析与未授权探测扫描器
type JSFinderScanner struct{}

// NewJSFinderScanner 创建 JSFinder 扫描器
func NewJSFinderScanner() *JSFinderScanner { return &JSFinderScanner{} }

// Name 返回扫描器名称
func (s *JSFinderScanner) Name() string { return "jsfinder" }

// 默认 UA
const jsFinderDefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// 各类正则（包级初始化，复用编译结果）
var (
	jsFinderReScriptSrc = regexp.MustCompile(`(?i)<script[^>]+src\s*=\s*["']([^"'<>]+)["']`)
	// API/路径：以 / 开头的相对路径，限制字符避免抓到 SVG path/CSS 之类
	jsFinderReURL = regexp.MustCompile(`["'` + "`" + `](\/[a-zA-Z0-9_\-/.?=&%~+#@:]{1,256})["'` + "`" + `]`)
	// 绝对 http(s) 链接
	jsFinderReAbsURL = regexp.MustCompile(`https?://[a-zA-Z0-9._\-]+(?::\d+)?(?:/[a-zA-Z0-9_\-/.?=&%~+#@:]*)?`)
	jsFinderReIPv4   = regexp.MustCompile(`\b(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}\b`)
	jsFinderReEmail  = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	jsFinderRePhone  = regexp.MustCompile(`\b1[3-9][0-9]{9}\b`)
	jsFinderReIDCard = regexp.MustCompile(`\b[1-9][0-9]{5}(?:19|20)[0-9]{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]\b`)
	jsFinderReJWT    = regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`)
	// 凭据键名 = 值
	jsFinderReSecret = regexp.MustCompile(`(?i)(access[_\-]?key|api[_\-]?key|secret[_\-]?key|secret[_\-]?token|app[_\-]?key|app[_\-]?secret|auth[_\-]?token|access[_\-]?token|client[_\-]?secret|private[_\-]?key|aws[_\-]?secret)["'\s:=]+["']?([A-Za-z0-9_\-]{16,256})["']?`)
	// 路由 token 拆分（路径分隔符与常见标点），用于高危路由精确匹配
	jsFinderReRouteSplit = regexp.MustCompile(`[/?#&=._\-]+`)
	// camelCase 边界：小写或数字 → 大写
	jsFinderReCamelBound = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// jsFinderFinding 单条提取结果
type jsFinderFinding struct {
	Type     string // url, absurl, ip, email, phone, idcard, jwt, secret
	Value    string
	Source   string // 来源 JS 文件 URL
	Severity string
	VulName  string
}

// jsFinderUnauthResult 未授权探测结果
type jsFinderUnauthResult struct {
	URL            string
	Status         int
	Reason         string // sensitive_leak / unauth
	Snippet        string // 匹配片段
	MatchedKeyword string // 命中的敏感关键词（用于显示匹配规则）
	VulName        string
	Severity       string
	RequestRaw     string
	ResponseRaw    string
}

// jsFinderTargetCtx 单个目标扫描上下文
type jsFinderTargetCtx struct {
	BaseURL   string
	Authority string
	Host      string
	Port      int
	Scheme    string
	Asset     *Asset
}

// resolveJSFinderOptions 合并默认值和外部配置
func resolveJSFinderOptions(in *JSFinderOptions) *JSFinderOptions {
	opts := &JSFinderOptions{
		Threads:           10,
		Timeout:           10,
		EnableSourcemap:   true,
		EnableUnauthCheck: true,
		UserAgent:         jsFinderDefaultUA,
		MaxJSSize:         5 * 1024 * 1024,
		MaxJSPerTarget:    200,
		MaxAPIPerTarget:   500,
	}
	if in == nil {
		return opts
	}
	if in.Threads > 0 {
		opts.Threads = in.Threads
	}
	if in.Timeout > 0 {
		opts.Timeout = in.Timeout
	}
	opts.EnableSourcemap = in.EnableSourcemap
	opts.EnableUnauthCheck = in.EnableUnauthCheck
	if in.UserAgent != "" {
		opts.UserAgent = in.UserAgent
	}
	if in.MaxJSSize > 0 {
		opts.MaxJSSize = in.MaxJSSize
	}
	if in.MaxJSPerTarget > 0 {
		opts.MaxJSPerTarget = in.MaxJSPerTarget
	}
	if in.MaxAPIPerTarget > 0 {
		opts.MaxAPIPerTarget = in.MaxAPIPerTarget
	}
	opts.HighRiskRoutes = lowerSlice(in.HighRiskRoutes)
	opts.AuthRequiredKeywords = lowerSlice(in.AuthRequiredKeywords)
	opts.SensitiveKeywords = lowerSlice(in.SensitiveKeywords)
	opts.DomainBlacklist = lowerSlice(in.DomainBlacklist)
	return opts
}

func lowerSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Scan 执行 JSFinder 扫描
func (s *JSFinderScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	var inOpts *JSFinderOptions
	if config.Options != nil {
		if v, ok := config.Options.(*JSFinderOptions); ok {
			inOpts = v
		}
	}
	opts := resolveJSFinderOptions(inOpts)

	logInfo := func(format string, args ...interface{}) {
		if config.TaskLogger != nil {
			config.TaskLogger("INFO", format, args...)
			return // 已由 TaskLogger 统一输出，避免双写
		}
		logx.Infof(format, args...)
	}
	logWarn := func(format string, args ...interface{}) {
		if config.TaskLogger != nil {
			config.TaskLogger("WARN", format, args...)
			return
		}
		logx.Errorf(format, args...)
	}

	targets := s.collectTargets(config.Assets)
	if len(targets) == 0 {
		logWarn("[JSFinder] 无可用 HTTP 目标")
		return &ScanResult{WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId}, nil
	}

	logInfo("[JSFinder] 开始扫描，目标数: %d，sourcemap=%v，未授权检测=%v", len(targets), opts.EnableSourcemap, opts.EnableUnauthCheck)

	client := newJSFinderHTTPClient(opts.Timeout)
	defer client.CloseIdleConnections()

	result := &ScanResult{WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId}
	var mu sync.Mutex

	totalTargets := len(targets)
	var done int32
	var doneMu sync.Mutex

	concurrency := opts.Threads
	if concurrency <= 0 {
		concurrency = 10
	}
	if concurrency > totalTargets {
		concurrency = totalTargets
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, tgt := range targets {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(t *jsFinderTargetCtx) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					logWarn("[JSFinder] 目标 %s 处理 panic: %v\n%s", t.BaseURL, r, debug.Stack())
				}
				doneMu.Lock()
				done++
				cur := done
				doneMu.Unlock()
				if config.OnProgress != nil {
					config.OnProgress(int(cur*100/int32(totalTargets)), fmt.Sprintf("JSFinder %d/%d", cur, totalTargets))
				}
			}()
			vuls := s.scanTarget(ctx, t, opts, client, logInfo, logWarn)
			if len(vuls) > 0 {
				mu.Lock()
				result.JSFinderResults = append(result.JSFinderResults, vuls...)
				mu.Unlock()
			}
		}(tgt)
	}
	wg.Wait()

	logInfo("[JSFinder] 扫描完成，发现结果 %d 条", len(result.JSFinderResults))
	return result, nil
}

// collectTargets 从资产列表抽取 HTTP 目标
func (s *JSFinderScanner) collectTargets(assets []*Asset) []*jsFinderTargetCtx {
	seen := make(map[string]bool)
	out := make([]*jsFinderTargetCtx, 0, len(assets))
	for _, a := range assets {
		if a == nil {
			continue
		}
		if !a.IsHTTP && !IsHTTPService(a.Service, a.Port) {
			continue
		}
		scheme := "http"
		if a.Service == "https" || a.Port == 443 || a.Port == 8443 || strings.HasPrefix(a.Service, "https") {
			scheme = "https"
		}
		var base string
		if (scheme == "http" && a.Port == 80) || (scheme == "https" && a.Port == 443) {
			base = fmt.Sprintf("%s://%s", scheme, a.Host)
		} else {
			base = fmt.Sprintf("%s://%s:%d", scheme, a.Host, a.Port)
		}
		if a.Path != "" && a.Path != "/" {
			base = base + strings.TrimSuffix(a.Path, "/")
		}
		if seen[base] {
			continue
		}
		seen[base] = true
		out = append(out, &jsFinderTargetCtx{
			BaseURL:   base,
			Authority: a.Authority,
			Host:      a.Host,
			Port:      a.Port,
			Scheme:    scheme,
			Asset:     a,
		})
	}
	return out
}

// newJSFinderHTTPClient 构造 JSFinder 专用 HTTP 客户端
func newJSFinderHTTPClient(timeoutSec int) *http.Client {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   time.Duration(timeoutSec) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// scanTarget 单目标完整流水线
func (s *JSFinderScanner) scanTarget(ctx context.Context, t *jsFinderTargetCtx, opts *JSFinderOptions, client *http.Client, logInfo, logWarn func(string, ...interface{})) []*JSFinderResult {
	logInfo("[JSFinder] [*] 正在提取 %s 的 JS 链接", t.BaseURL)

	htmlBody, _, err := jsFinderHTTPGet(ctx, client, t.BaseURL, opts.UserAgent, opts.MaxJSSize, "")
	if err != nil {
		logWarn("[JSFinder] 抓取首页失败: %s -> %v", t.BaseURL, err)
		return nil
	}

	jsURLs := extractScriptURLs(string(htmlBody), t.BaseURL)
	jsURLs = filterJSURLs(jsURLs, opts.DomainBlacklist)
	if len(jsURLs) > opts.MaxJSPerTarget {
		jsURLs = jsURLs[:opts.MaxJSPerTarget]
	}
	logInfo("[JSFinder] [+] 共提取到 JS 链接: %d 个", len(jsURLs))
	for _, u := range jsURLs {
		logInfo("[JSFinder] %s", u)
	}

	allFindings := analyzeContent(string(htmlBody), t.BaseURL)

	type jsJob struct {
		url string
	}
	jobCh := make(chan jsJob, len(jsURLs))
	for _, u := range jsURLs {
		jobCh <- jsJob{url: u}
	}
	close(jobCh)

	var fmu sync.Mutex
	var wg sync.WaitGroup
	workers := opts.Threads
	if workers <= 0 {
		workers = 10
	}
	if workers > len(jsURLs) {
		workers = len(jsURLs)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logWarn("[JSFinder] JS 分析 worker panic: %v\n%s", r, debug.Stack())
				}
			}()
			for job := range jobCh {
				if ctx.Err() != nil {
					return
				}
				body, _, err := jsFinderHTTPGet(ctx, client, job.url, opts.UserAgent, opts.MaxJSSize, t.BaseURL)
				if err != nil {
					continue
				}
				findings := analyzeContent(string(body), job.url)
				if opts.EnableSourcemap {
					mapFindings := tryFetchSourcemap(ctx, client, job.url, body, opts)
					findings = append(findings, mapFindings...)
				}
				if len(findings) > 0 {
					fmu.Lock()
					allFindings = append(allFindings, findings...)
					fmu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	allFindings = dedupFindings(allFindings)

	apiURLs := collectAPIURLs(allFindings, t.BaseURL, opts.MaxAPIPerTarget)
	logInfo("[JSFinder] [+] 共提取到 API: %d 个", len(apiURLs))

	var unauths []*jsFinderUnauthResult
	if opts.EnableUnauthCheck && len(apiURLs) > 0 {
		unauths = runUnauthChecks(ctx, client, apiURLs, opts, logInfo)
	}

	logInfo("[JSFinder] [*] 任务运行结束: %s", t.BaseURL)
	return buildJSFinderResults(t, jsURLs, allFindings, unauths)
}

// jsFinderHTTPGet 带 UA / Referer / 大小限制 的 HTTP GET
func jsFinderHTTPGet(ctx context.Context, client *http.Client, target, ua string, maxSize int64, referer string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, 0, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if maxSize <= 0 {
		maxSize = 5 * 1024 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// extractScriptURLs 从 HTML 中提取 <script src> 链接，并解析为绝对 URL
func extractScriptURLs(html, base string) []string {
	matches := jsFinderReScriptSrc.FindAllStringSubmatch(html, -1)
	baseU, _ := url.Parse(base)
	seen := make(map[string]bool)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			continue
		}
		abs := resolveJSURL(baseU, raw)
		if abs == "" {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

// resolveJSURL 把相对路径解析为绝对 URL
func resolveJSURL(base *url.URL, ref string) string {
	if base == nil {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

// analyzeContent 对单段内容运行所有正则，返回去重前的 findings
func analyzeContent(content, sourceURL string) []*jsFinderFinding {
	if content == "" {
		return nil
	}
	out := make([]*jsFinderFinding, 0, 16)
	for _, m := range jsFinderReURL.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		out = append(out, &jsFinderFinding{Type: "url", Value: m[1], Source: sourceURL})
	}
	for _, v := range jsFinderReAbsURL.FindAllString(content, -1) {
		out = append(out, &jsFinderFinding{Type: "absurl", Value: v, Source: sourceURL})
	}
	for _, v := range jsFinderReIPv4.FindAllString(content, -1) {
		out = append(out, &jsFinderFinding{Type: "ip", Value: v, Source: sourceURL, Severity: "low", VulName: "JS 内嵌 IP 地址"})
	}
	for _, v := range jsFinderReEmail.FindAllString(content, -1) {
		out = append(out, &jsFinderFinding{Type: "email", Value: v, Source: sourceURL, Severity: "low", VulName: "JS 内嵌邮箱"})
	}
	for _, v := range jsFinderRePhone.FindAllString(content, -1) {
		out = append(out, &jsFinderFinding{Type: "phone", Value: v, Source: sourceURL, Severity: "medium", VulName: "JS 内嵌手机号"})
	}
	for _, v := range jsFinderReIDCard.FindAllString(content, -1) {
		out = append(out, &jsFinderFinding{Type: "idcard", Value: v, Source: sourceURL, Severity: "high", VulName: "JS 内嵌身份证号"})
	}
	for _, v := range jsFinderReJWT.FindAllString(content, -1) {
		out = append(out, &jsFinderFinding{Type: "jwt", Value: v, Source: sourceURL, Severity: "high", VulName: "JS 内嵌 JWT Token"})
	}
	for _, m := range jsFinderReSecret.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		out = append(out, &jsFinderFinding{Type: "secret", Value: m[1] + "=" + m[2], Source: sourceURL, Severity: "critical", VulName: "JS 硬编码密钥"})
	}
	return out
}

// sourcemapDoc 仅解析需要的字段
type sourcemapDoc struct {
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent"`
}

// tryFetchSourcemap 尝试拉取 .js.map 并对原始源码再次运行正则
func tryFetchSourcemap(ctx context.Context, client *http.Client, jsURL string, jsBody []byte, opts *JSFinderOptions) []*jsFinderFinding {
	mapURL := deriveSourcemapURL(jsURL, jsBody)
	if mapURL == "" {
		return nil
	}
	body, status, err := jsFinderHTTPGet(ctx, client, mapURL, opts.UserAgent, opts.MaxJSSize, jsURL)
	if err != nil || status >= 400 || len(body) == 0 {
		return nil
	}
	var doc sourcemapDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}
	if len(doc.SourcesContent) == 0 {
		return nil
	}
	out := make([]*jsFinderFinding, 0, 32)
	for i, content := range doc.SourcesContent {
		if content == "" {
			continue
		}
		src := mapURL
		if i < len(doc.Sources) && doc.Sources[i] != "" {
			src = mapURL + "#" + doc.Sources[i]
		}
		out = append(out, analyzeContent(content, src)...)
	}
	return out
}

// deriveSourcemapURL 从 //# sourceMappingURL= 注释或默认 .map 后缀派生
// 兼容三种注释结尾：行尾换行、块注释 */、文件 EOF
func deriveSourcemapURL(jsURL string, jsBody []byte) string {
	if len(jsBody) > 0 {
		tail := jsBody
		if len(tail) > 4096 {
			tail = tail[len(tail)-4096:]
		}
		idx := strings.LastIndex(string(tail), "sourceMappingURL=")
		if idx >= 0 {
			rest := string(tail)[idx+len("sourceMappingURL="):]
			// 仅按换行截断，避免误伤 URL 中的 / 与 .
			if end := strings.IndexAny(rest, "\r\n"); end >= 0 {
				rest = rest[:end]
			}
			rest = strings.TrimSpace(rest)
			// 块注释结尾 */ 单独剥离
			rest = strings.TrimSuffix(rest, "*/")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				if strings.HasPrefix(rest, "data:") {
					return ""
				}
				baseU, err := url.Parse(jsURL)
				if err == nil {
					return resolveJSURL(baseU, rest)
				}
			}
		}
	}
	return jsURL + ".map"
}

// filterJSURLs 过滤掉黑名单域
func filterJSURLs(urls []string, blacklist []string) []string {
	if len(blacklist) == 0 {
		return urls
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if !isBlacklistedHost(u, blacklist) {
			out = append(out, u)
		}
	}
	return out
}

// isBlacklistedHost 检查 URL 主机是否命中黑名单
// 匹配规则：
//   - "*.example.com" 通配：host 等于 example.com 或以 .example.com 结尾
//   - "example.com" 精确：host 等于 example.com 或以 .example.com 结尾（自然子域）
func isBlacklistedHost(rawURL string, blacklist []string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, b := range blacklist {
		if b == "" {
			continue
		}
		base := strings.TrimPrefix(b, "*.")
		base = strings.TrimPrefix(base, ".")
		if base == "" {
			continue
		}
		if host == base || strings.HasSuffix(host, "."+base) {
			return true
		}
	}
	return false
}

// splitRouteTokens 把 URL path 拆分为可比对的 token 列表
// 同时按标点（/?#&=._-）和 camelCase 边界拆分，统一小写
func splitRouteTokens(path string) []string {
	if path == "" {
		return nil
	}
	// 先在 camelCase 边界插入分隔符
	expanded := jsFinderReCamelBound.ReplaceAllString(path, "$1/$2")
	raw := jsFinderReRouteSplit.Split(expanded, -1)
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// matchesHighRiskRoute 判断 URL 是否命中高危路由关键词
// 规则：URL path 拆 token 后，token 必须完全等于关键词才算命中（不再用子串）
// 避免 /admin/listdeleted 误匹配 delete；同时通过 camelCase 拆分支持 deleteUser
func matchesHighRiskRoute(rawURL string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return false
	}
	tokens := splitRouteTokens(u.Path)
	if len(tokens) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(keywords))
	for _, k := range keywords {
		if k != "" {
			set[k] = struct{}{}
		}
	}
	for _, tk := range tokens {
		if _, ok := set[tk]; ok {
			return true
		}
	}
	return false
}

// dedupFindings 按 Type+Value+Source 去重
func dedupFindings(in []*jsFinderFinding) []*jsFinderFinding {
	seen := make(map[string]bool, len(in))
	out := make([]*jsFinderFinding, 0, len(in))
	for _, f := range in {
		if f == nil || f.Value == "" {
			continue
		}
		k := f.Type + "|" + f.Value + "|" + f.Source
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}

// collectAPIURLs 把相对/绝对 URL findings 解析为绝对地址，限制数量
func collectAPIURLs(findings []*jsFinderFinding, baseURL string, max int) []string {
	baseU, _ := url.Parse(baseURL)
	seen := make(map[string]bool)
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		if f == nil {
			continue
		}
		var abs string
		switch f.Type {
		case "url":
			abs = resolveJSURL(baseU, f.Value)
		case "absurl":
			abs = f.Value
		default:
			continue
		}
		if abs == "" || seen[abs] {
			continue
		}
		if !strings.HasPrefix(abs, "http://") && !strings.HasPrefix(abs, "https://") {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

// runUnauthChecks 对发现的 URL 列表执行未授权探测
func runUnauthChecks(ctx context.Context, client *http.Client, urls []string, opts *JSFinderOptions, logInfo func(string, ...interface{})) []*jsFinderUnauthResult {
	results := make([]*jsFinderUnauthResult, 0)
	var mu sync.Mutex

	// 负向结果日志限流：避免单任务输出上千条"不存在未授权访问"
	// 前 negativeLogHead 条正常输出，之后每 negativeLogSample 条采样输出1条
	// 修复 m2：原 int32 在百万级 URL 场景下会溢出，改为 int64
	var negativeCount int64
	const negativeLogHead = 20
	const negativeLogSample = 200

	workers := opts.Threads
	if workers <= 0 {
		workers = 10
	}
	if workers > len(urls) {
		workers = len(urls)
	}

	jobs := make(chan string, len(urls))
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logInfo("[JSFinder] 未授权检测 worker panic: %v\n%s", r, debug.Stack())
				}
			}()
			for u := range jobs {
				if ctx.Err() != nil {
					return
				}
				if isBlacklistedHost(u, opts.DomainBlacklist) {
					continue
				}
				if matchesHighRiskRoute(u, opts.HighRiskRoutes) {
					logInfo("[JSFinder] [!] 跳过高危路由: %s", u)
					continue
				}
				body, status, err := jsFinderHTTPGet(ctx, client, u, opts.UserAgent, 256*1024, "")
				if err != nil {
					continue
				}
				low := strings.ToLower(string(body))
				if status == 401 || status == 403 || containsKeyword(low, opts.AuthRequiredKeywords) {
					// 限流输出：前N条正常输出，之后采样输出
					n := atomic.AddInt64(&negativeCount, 1)
					if n <= negativeLogHead || n%negativeLogSample == 0 {
						if n > negativeLogHead {
							logInfo("[JSFinder] [-] 已跳过 %d 条负向日志，最新: %s 不存在未授权访问", n-negativeLogHead, u)
						} else {
							logInfo("[JSFinder] [-] %s 不存在未授权访问", u)
						}
					}
					continue
				}
				if status >= 200 && status < 300 {
					reqRaw := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: */*\r\n",
						u, extractHostFromURL(u), opts.UserAgent)
					respBody := string(body)
					if len(respBody) > 4096 {
						respBody = respBody[:4096]
					}
					respRaw := fmt.Sprintf("HTTP/1.1 %d\r\nContent-Length: %d\r\n\r\n%s", status, len(body), respBody)

					if matched := firstKeywordMatch(low, opts.SensitiveKeywords); matched != "" {
						snippet := extractSnippet(string(body), matched, 80)
						mu.Lock()
						results = append(results, &jsFinderUnauthResult{
							URL: u, Status: status, Reason: "sensitive_leak",
							Snippet: snippet, MatchedKeyword: matched, VulName: "JS API 未授权敏感信息泄漏", Severity: "high",
							RequestRaw: reqRaw, ResponseRaw: respRaw,
						})
						mu.Unlock()
						logInfo("[JSFinder] [+] %s 命中敏感关键词 [%s]", u, matched)
						continue
					}
					mu.Lock()
					results = append(results, &jsFinderUnauthResult{
						URL: u, Status: status, Reason: "unauth",
						VulName: "JS API 未授权访问", Severity: "medium",
						RequestRaw: reqRaw, ResponseRaw: respRaw,
					})
					mu.Unlock()
					logInfo("[JSFinder] [+] %s 疑似未授权访问 (HTTP %d)", u, status)
				}
			}
		}()
	}
	wg.Wait()
	return results
}

// containsKeyword 判断字符串是否包含任一关键词（关键词应已小写）
func containsKeyword(s string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	low := strings.ToLower(s)
	for _, k := range keywords {
		if k != "" && strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// firstKeywordMatch 返回首个命中的关键词，无命中返回空串
func firstKeywordMatch(low string, keywords []string) string {
	for _, k := range keywords {
		if k != "" && strings.Contains(low, k) {
			return k
		}
	}
	return ""
}

func extractHostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

// extractSnippet 截取关键词周围片段，保留原始大小写
// 使用 lowOffsetToBodyOffset 将 lowercase 中的偏移映射回原始 body，避免 Unicode case-folding 导致越界
func extractSnippet(body, keyword string, around int) string {
	lowBody := strings.ToLower(body)
	lowKeyword := strings.ToLower(keyword)
	idx := strings.Index(lowBody, lowKeyword)
	if idx < 0 {
		return ""
	}
	bodyStart := lowOffsetToBodyOffset(body, idx)
	bodyEnd := lowOffsetToBodyOffset(body, idx+len(lowKeyword))

	start := bodyStart - around
	if start < 0 {
		start = 0
	}
	end := bodyEnd + around
	if end > len(body) {
		end = len(body)
	}
	return body[start:end]
}

// lowOffsetToBodyOffset 将 strings.ToLower(body) 中的字节偏移映射回原始 body 的字节偏移
// 解决 Unicode case-folding 可能改变字符串长度的问题（如 İ U+0130 → i̇ 长度 2→3）
func lowOffsetToBodyOffset(body string, lowOff int) int {
	if lowOff <= 0 {
		return 0
	}
	bi := 0
	li := 0
	for bi < len(body) {
		_, bSize := utf8.DecodeRuneInString(body[bi:])
		low := strings.ToLower(body[bi : bi+bSize])
		li += len(low)
		bi += bSize
		if li >= lowOff {
			return bi
		}
	}
	return bi
}

// jsFinderTypeToRuleName 将发现类型映射为可读的匹配规则名称
func jsFinderTypeToRuleName(t string) string {
	switch t {
	case "url":
		return "JS Relative Path Regex"
	case "absurl":
		return "JS Absolute URL Regex"
	case "ip":
		return "JS IPv4 Regex"
	case "email":
		return "JS Email Regex"
	case "phone":
		return "JS Phone Number Regex"
	case "idcard":
		return "JS ID Card Regex"
	case "jwt":
		return "JS JWT Token Regex"
	case "secret":
		return "JS Hardcoded Secret Regex"
	default:
		return "JS " + t + " Regex"
	}
}

// jsFinderTypeToRulePattern 将发现类型映射为对应的正则表达式
func jsFinderTypeToRulePattern(t string) string {
	switch t {
	case "url":
		return "[\"'`](\\/[a-zA-Z0-9_\\-/.?=&%~+#@:]{1,256})[\"'`]"
	case "absurl":
		return `https?://[a-zA-Z0-9._\\-]+(?::\\d+)?(?:/[a-zA-Z0-9_\\-/.?=&%~+#@:]*)?`
	case "ip":
		return `\b(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?:\\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}\b`
	case "email":
		return `\b[A-Za-z0-9._%+\\-]+@[A-Za-z0-9.\\-]+\\.[A-Za-z]{2,}\\b`
	case "phone":
		return `\b1[3-9][0-9]{9}\b`
	case "idcard":
		return `\b[1-9][0-9]{5}(?:19|20)[0-9]{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]\b`
	case "jwt":
		return `\beyJ[A-Za-z0-9_\\-]+\\.eyJ[A-Za-z0-9_\\-]+\\.[A-Za-z0-9_\\-]+\\b`
	case "secret":
		return `(?i)(access[_\\-]?key|api[_\\-]?key|secret[_\\-]?key|...)[\"'\\s:=]+[\"']?([A-Za-z0-9_\\-]{16,256})[\"']?`
	default:
		return t
	}
}

// jsFinderRiskTag 根据发现类型和严重级别生成风险标签
func jsFinderRiskTag(f *jsFinderFinding) string {
	switch f.Severity {
	case "critical":
		return "high-risk"
	case "high":
		return "risk"
	case "medium":
		return "sensitive"
	case "low":
		return "info-leak"
	default:
		return ""
	}
}

// buildVulnerabilities 把 findings + unauths 转为标准 Vulnerability 列表
// 所有 JS 文件均触发报告机制；对有安全风险的 JS 自动添加风险标签
// MatcherName 记录匹配规则名称，ExtractedResults 记录匹配到的风险内容
func buildJSFinderResults(t *jsFinderTargetCtx, jsURLs []string, findings []*jsFinderFinding, unauths []*jsFinderUnauthResult) []*JSFinderResult {
	out := make([]*JSFinderResult, 0, len(jsURLs)+len(findings)+len(unauths)+2)

	// 收集有安全风险的 JS 文件源
	riskJSSources := make(map[string][]string) // sourceJS -> finding types

	for _, f := range findings {
		if f == nil || f.Value == "" {
			continue
		}

		// 收集风险 JS 文件来源
		if f.Severity != "" && f.Severity != "info" && f.Source != "" {
			riskJSSources[f.Source] = append(riskJSSources[f.Source], f.Type)
		}

		switch f.Type {
		case "url":
			f.Severity = "info"
			f.VulName = "JS 提取 API 路径"
		case "absurl":
			f.Severity = "info"
			f.VulName = "JS 提取绝对 URL"
		}
		if f.Severity == "" || f.VulName == "" {
			continue
		}

		// 构建标签：基础标签 + 风险标签
		tags := []string{"jsfinder", f.Type}
		if riskTag := jsFinderRiskTag(f); riskTag != "" {
			tags = append(tags, riskTag)
		}

		ruleName := jsFinderTypeToRuleName(f.Type)

		// url/absurl 类型：URL 字段显示提取到的路径/URL，来源信息放入 Result
		// 其他类型：URL 字段显示来源 JS 文件，Result 显示提取到的值
		urlField := f.Source
		resultField := f.Value
		curlTarget := f.Source
		if f.Type == "url" || f.Type == "absurl" {
			urlField = f.Value
			resultField = fmt.Sprintf("Source: %s", f.Source)
			curlTarget = f.Value
		}

		out = append(out, &JSFinderResult{
			Authority:        t.Authority,
			Host:             t.Host,
			Port:             t.Port,
			URL:              urlField,
			Severity:         f.Severity,
			VulName:          f.VulName,
			Result:           resultField,
			Tags:             tags,
			MatcherName:      ruleName,
			ExtractedResults: []string{f.Value},
			CurlCommand:      fmt.Sprintf("curl -sS '%s'", curlTarget),
			Response:         fmt.Sprintf("[%s] %s\nSource: %s", f.Type, f.Value, f.Source),
		})
	}

	// 为所有发现的 JS 文件生成记录（不论是否有敏感信息）
	// 有风险的 JS 文件已有 riskJSSources 记录，用于关联 findings
	riskJSSet := make(map[string]bool, len(riskJSSources))
	for jsURL := range riskJSSources {
		riskJSSet[jsURL] = true
	}
	for _, jsURL := range jsURLs {
		findingTypes, hasRisk := riskJSSources[jsURL]
		if hasRisk {
			// 有敏感发现的 JS 文件 - 生成 JS 敏感信息发现记录
			uniqueTypes := make(map[string]struct{})
			for _, ft := range findingTypes {
				uniqueTypes[ft] = struct{}{}
			}
			typeList := make([]string, 0, len(uniqueTypes))
			for ft := range uniqueTypes {
				typeList = append(typeList, ft)
			}

			// 确定该 JS 文件的整体风险等级
			severity := "low"
			for _, ft := range typeList {
				switch ft {
				case "secret":
					severity = "critical"
				case "idcard", "jwt":
					if severity != "critical" {
						severity = "high"
					}
				case "phone":
					if severity != "critical" && severity != "high" {
						severity = "medium"
					}
				case "email", "ip":
					if severity == "low" {
						severity = "low"
					}
				}
			}

			ruleNames := make([]string, 0, len(typeList))
			for _, ft := range typeList {
				ruleNames = append(ruleNames, jsFinderTypeToRuleName(ft))
			}

			// 收集该 JS 文件的所有风险内容
			extractedResults := make([]string, 0, len(typeList))
			for _, f := range findings {
				if f != nil && f.Source == jsURL && f.Type != "url" && f.Type != "absurl" && f.Value != "" {
					extractedResults = append(extractedResults, f.Value)
				}
			}

			tags := []string{"jsfinder", "js-file"}
			if severity == "critical" || severity == "high" {
				tags = append(tags, "high-risk")
			} else if severity == "medium" {
				tags = append(tags, "sensitive")
			} else {
				tags = append(tags, "info-leak")
			}

			out = append(out, &JSFinderResult{
				Authority:        t.Authority,
				Host:             t.Host,
				Port:             t.Port,
				URL:              jsURL,
				Severity:         severity,
				VulName:          "JS 敏感信息发现",
				Result:           fmt.Sprintf("发现 %d 类敏感信息: %s", len(typeList), strings.Join(typeList, ", ")),
				Tags:             tags,
				MatcherName:      strings.Join(ruleNames, " | "),
				ExtractedResults: extractedResults,
				CurlCommand:      fmt.Sprintf("curl -sS '%s'", jsURL),
				Response:         fmt.Sprintf("JS File: %s\nRisk Types: %s\nFindings:\n%s", jsURL, strings.Join(typeList, ", "), strings.Join(extractedResults, "\n")),
			})
		} else {
			// 无敏感发现的 JS 文件 - 生成 JS 文件发现记录
			out = append(out, &JSFinderResult{
				Authority:        t.Authority,
				Host:             t.Host,
				Port:             t.Port,
				URL:              jsURL,
				Severity:         "info",
				VulName:          "JS 文件发现",
				Result:           jsURL,
				Tags:             []string{"jsfinder", "js-file"},
				MatcherName:      "JS Script Src Extractor",
				ExtractedResults: []string{jsURL},
				CurlCommand:      fmt.Sprintf("curl -sS '%s'", jsURL),
				Response:         fmt.Sprintf("JS File: %s\nStatus: clean (no sensitive findings)", jsURL),
			})
		}
	}

	for _, u := range unauths {
		if u == nil {
			continue
		}
		extra := fmt.Sprintf("status=%d; reason=%s", u.Status, u.Reason)
		if u.Snippet != "" {
			extra += "; snippet=" + u.Snippet
		}

		// 未授权探测结果的风险标签
		tags := []string{"jsfinder", "unauth"}
		if u.Severity == "high" || u.Reason == "sensitive_leak" {
			tags = append(tags, "high-risk")
		} else {
			tags = append(tags, "risk")
		}

		// 根据是否命中敏感关键词设置匹配规则名称
		matcherName := "JS API Unauth Check"
		extractedResults := []string{u.URL}
		// 如果命中了敏感关键词，使用 "关键词:xxx" 格式的匹配规则名称
		if u.MatchedKeyword != "" {
			matcherName = "keyword:" + u.MatchedKeyword
			// 保留 URL 和上下文片段，但高亮时只匹配关键词
			extractedResults = []string{u.MatchedKeyword, u.Snippet}
		} else if u.Snippet != "" {
			extractedResults = append(extractedResults, u.Snippet)
		}

		out = append(out, &JSFinderResult{
			Authority:        t.Authority,
			Host:             t.Host,
			Port:             t.Port,
			URL:              u.URL,
			Severity:         u.Severity,
			VulName:          u.VulName,
			Result:           u.URL,
			Tags:             tags,
			MatcherName:      matcherName,
			ExtractedResults: extractedResults,
			CurlCommand:      fmt.Sprintf("curl -sS -o /dev/null -w '%%{http_code}' '%s'", u.URL),
			Request:          u.RequestRaw,
			Response:         u.ResponseRaw,
		})
	}
	return out
}
