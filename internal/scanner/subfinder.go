package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"cscan/pkg/geolocation"

	"github.com/zeromicro/go-zero/core/logx"
)

// SubfinderScanner Subfinder子域名扫描器 (CLI 模式)
type SubfinderScanner struct {
	BaseScanner
	executor *CmdExecutor
}

// NewSubfinderScanner 创建 Subfinder 扫描器
func NewSubfinderScanner() *SubfinderScanner {
	return &SubfinderScanner{
		BaseScanner: BaseScanner{name: "subfinder"},
		executor:    NewExecutorForTool("subfinder"),
	}
}

// SubfinderOptions Subfinder 扫描选项
type SubfinderOptions struct {
	Timeout            int                 `json:"timeout"`
	MaxEnumerationTime int                 `json:"maxEnumerationTime"`
	Threads            int                 `json:"threads"`
	RateLimit          int                 `json:"rateLimit"`
	Sources            []string            `json:"sources"`
	ExcludeSources     []string            `json:"excludeSources"`
	All                bool                `json:"all"`
	Recursive          bool                `json:"recursive"`
	RemoveWildcard     bool                `json:"removeWildcard"`
	ProviderConfig     map[string][]string `json:"providerConfig"`
	ProviderConfigFile string              `json:"providerConfigFile"` // 临时 provider config 文件路径
	ResolveDNS         bool                `json:"resolveDNS"`
	Concurrent         int                 `json:"concurrent"`
}

// Validate 验证配置
func (o *SubfinderOptions) Validate() error {
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	if o.MaxEnumerationTime < 0 {
		return fmt.Errorf("maxEnumerationTime must be non-negative, got %d", o.MaxEnumerationTime)
	}
	if o.Threads < 0 {
		return fmt.Errorf("threads must be non-negative, got %d", o.Threads)
	}
	if o.RateLimit < 0 {
		return fmt.Errorf("rateLimit must be non-negative, got %d", o.RateLimit)
	}
	if o.Concurrent < 0 {
		return fmt.Errorf("concurrent must be non-negative, got %d", o.Concurrent)
	}
	return nil
}

// SubfinderResult subfinder CLI JSON 输出
type SubfinderResult struct {
	Host   string `json:"host"`
	Input  string `json:"input,omitempty"`  // 原始输入域名
	Source string `json:"source,omitempty"` // 数据源名称（string，非数组）
}

// Scan 执行子域名扫描
func (s *SubfinderScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	result := &ScanResult{
		MainTaskId: config.MainTaskId,
		Assets:      make([]*Asset, 0),
	}

	opts := &SubfinderOptions{
		Timeout:            30,
		MaxEnumerationTime: 10,
		Threads:            10,
		RateLimit:          0,
		RemoveWildcard:     true,
		ResolveDNS:         true,
		Concurrent: 1,
	}
	if config.Options != nil {
		if o, ok := config.Options.(*SubfinderOptions); ok {
			opts = o
		}
	}

	domains := s.parseDomains(config.Target)
	if len(config.Targets) > 0 {
		domains = append(domains, config.Targets...)
	}
	if len(domains) == 0 {
		logx.Info("No domains for subfinder scan")
		return result, nil
	}

	if len(config.Assets) > 0 {
		for _, asset := range config.Assets {
			if asset.Category == "domain" {
				domains = append(domains, asset.Host)
			}
		}
	}

	logx.Infof("Subfinder(CLI): scanning %d domains", len(domains))

	// 并发 Worker Pool：每个域名一个 subfinder 进程，完成一个补一个
	concurrency := opts.Concurrent
	if concurrency <= 0 {
		concurrency = config.WorkerConcurrency
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > 5 {
		concurrency = 5
	}
	if concurrency > len(domains) {
		concurrency = len(domains)
	}
	logx.Infof("Subfinder(CLI): scanning %d domains with %d workers", len(domains), concurrency)

	type domainResult struct {
		assets []*Asset
		domain string
		err    error
	}
	targetChan := make(chan string, len(domains))
	resultChan := make(chan domainResult, len(domains))
	var scanWg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			for domain := range targetChan {
				select {
				case <-ctx.Done():
					resultChan <- domainResult{err: ctx.Err()}
					return
				default:
				}
				assets, err := s.scanDomain(ctx, domain, opts, config.TaskLogger)
			resultChan <- domainResult{assets: assets, domain: domain, err: err}
			}
		}()
	}

dispatch:
	for _, domain := range domains {
		select {
		case <-ctx.Done():
			break dispatch
		case targetChan <- domain:
		}
	}
	close(targetChan)

	go func() {
		scanWg.Wait()
		close(resultChan)
	}()

	var allAssets []*Asset
	for res := range resultChan {
		if res.err != nil {
			logx.Errorf("Subfinder: error for domain: %v", res.err)
			continue
		}
		allAssets = append(allAssets, res.assets...)
		// 流式入库：单域名完成后立即回调
		if config.OnTargetDone != nil && len(res.assets) > 0 {
			config.OnTargetDone(res.domain, res.assets)
		}
	}

	// 全局去重
	seen := make(map[string]*Asset)
	for _, asset := range allAssets {
		key := asset.Host
		if existing, ok := seen[key]; ok {
			existing.IPV4 = append(existing.IPV4, asset.IPV4...)
			existing.IPV6 = append(existing.IPV6, asset.IPV6...)
		} else {
			seen[key] = asset
		}
	}
	result.Assets = make([]*Asset, 0, len(seen))
	for _, asset := range seen {
		asset.Source = "subfinder"
		result.Assets = append(result.Assets, asset)
	}

	logx.Infof("Subfinder(CLI): completed, found %d subdomains", len(result.Assets))
	return result, nil
}

func (s *SubfinderScanner) scanDomain(
	ctx context.Context, domain string, opts *SubfinderOptions,
	taskLogger func(level, format string, args ...interface{}),
) ([]*Asset, error) {
	args := []string{
		"-d", domain,
		"-json",
		"-silent",
		"-timeout", fmt.Sprintf("%d", opts.Timeout),
		"-max-time", fmt.Sprintf("%d", opts.MaxEnumerationTime),
		"-t", fmt.Sprintf("%d", opts.Threads),
		"-disable-update-check",
	}
	if opts.RateLimit > 0 {
		args = append(args, "-rate-limit", fmt.Sprintf("%d", opts.RateLimit))
	}
	if len(opts.Sources) > 0 {
		args = append(args, "-sources", strings.Join(opts.Sources, ","))
	}
	if opts.All {
		args = append(args, "-all")
	}
	if opts.Recursive {
		args = append(args, "-recursive")
	}
	if opts.RemoveWildcard {
		args = append(args, "-active")
	}
	if opts.ProviderConfigFile != "" {
		args = append(args, "-provider-config", opts.ProviderConfigFile)
	}

	// 输出到临时文件
	tmpFile, err := os.CreateTemp("", "subfinder-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)
	args = append(args, "-o", tmpPath)

	providers := make([]string, 0, len(opts.ProviderConfig))
	for p := range opts.ProviderConfig {
		providers = append(providers, p)
	}
	if len(providers) > 0 {
		taskLogger("INFO", "Subfinder CLI: providers=%s, config=%s", strings.Join(providers, ","), opts.ProviderConfigFile)
	} else {
		taskLogger("INFO", "Subfinder CLI: no providers configured, using default passive sources")
	}
	taskLogger("INFO", "Subfinder CLI: %s %s", ToolConfigs["subfinder"].BinaryName, strings.Join(args, " "))

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout+10) * time.Second,
	})
	if err != nil {
		logx.Debugf("[Subfinder] execution error domain=%s err=%v", domain, err)
		s.executor.LogResult("Subfinder: "+domain, res, err)
		return nil, fmt.Errorf("subfinder execution: %w", err)
	}
	if res.ExitCode != 0 {
		logx.Debugf("[Subfinder] non-zero exit domain=%s exit=%d stderr=%s", domain, res.ExitCode, strings.TrimSpace(res.Stderr))
		return nil, fmt.Errorf("subfinder exit code %d: %s", res.ExitCode, res.Stderr)
	}

	// 读取输出文件，若为空则回退到 stdout
	content, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		logx.Infof("[WARN] Subfinder(CLI): failed to read output file: %v", readErr)
		return nil, fmt.Errorf("read output: %w", readErr)
	}

	var parseSource io.Reader
	if len(content) > 0 {
		parseSource = strings.NewReader(string(content))
	} else if len(res.Stdout) > 0 {
		logx.Infof("Subfinder(CLI): output file is empty, falling back to stdout (%d bytes)", len(res.Stdout))
		parseSource = strings.NewReader(res.Stdout)
	} else {
		logx.Infof("Subfinder(CLI): no output from file or stdout, returning empty result")
		return []*Asset{}, nil
	}

	var assets []*Asset
	scanner := bufio.NewScanner(parseSource)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineCount := 0
	parseFailCount := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineCount++
		var sr SubfinderResult
		if err := json.Unmarshal([]byte(line), &sr); err != nil {
			parseFailCount++
			logx.Debugf("[Subfinder] JSON parse failed line=%d: %v, line=%s", lineCount, err, line)
			continue
		}
		if sr.Host == "" {
			continue
		}

		asset := &Asset{
			Authority: sr.Host,
			Host:      sr.Host,
			Category:  "domain",
		}

		// DNS 解析（如果启用）
		if opts.ResolveDNS {
			resolvedIPs, cname := s.resolveDNS(ctx, []string{sr.Host})
			for _, ip := range resolvedIPs {
				parsedIP := parseIP(ip)
				if parsedIP == nil {
					continue
				}
				if ip4 := parsedIP.To4(); ip4 != nil {
					locStr, _ := ipLocator.Locate(ip4.String())
					asset.IPV4 = append(asset.IPV4, IPInfo{IP: ip4.String(), Location: geolocation.NormalizeLocation(locStr)})
				} else {
					locStr, _ := ipLocator.Locate(parsedIP.String())
					asset.IPV6 = append(asset.IPV6, IPInfo{IP: parsedIP.String(), Location: geolocation.NormalizeLocation(locStr)})
				}
			}
			if cname != "" {
				asset.CName = cname
			}
		}

		assets = append(assets, asset)
	}

	logx.Debugf("[Subfinder] domain=%s: lines=%d parseFail=%d assets=%d", domain, lineCount, parseFailCount, len(assets))
	return assets, nil
}

func (s *SubfinderScanner) parseDomains(target string) []string {
	var domains []string
	seen := make(map[string]bool)
	lines := strings.Split(target, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if net.ParseIP(line) != nil {
			continue
		}
		if strings.Contains(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			continue
		}
		if !seen[line] {
			seen[line] = true
			domains = append(domains, line)
		}
	}
	return domains
}

func (s *SubfinderScanner) resolveDNS(ctx context.Context, domains []string) ([]string, string) {
	// 使用 dnsx CLI 进行批量 DNS 解析（含 CNAME）
	tmpFile, err := os.CreateTemp("", "dnsx-targets-*.txt")
	if err != nil {
		return nil, ""
	}
	for _, d := range domains {
		tmpFile.WriteString(d + "\n")
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	executor := NewCmdExecutor("dnsx", 128, 3*time.Minute)
	args := []string{
		"-l", tmpPath,
		"-json",
		"-silent",
		"-a", "-aaaa", "-cname",
		"-timeout", "5",
		"-retry", "1",
		"-disable-update-check",
	}

	res, err := executor.Execute(ctx, args, ExecuteOpts{
		Timeout: 5 * time.Minute,
	})
	if err != nil {
		return nil, ""
	}

	var ips []string
	var cname string
	scanner := newLineScanner(res.Stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var dnsResult map[string]interface{}
		if err := json.Unmarshal([]byte(line), &dnsResult); err != nil {
			continue
		}
		// A 记录
		if a, ok := dnsResult["a"].([]interface{}); ok {
			for _, ip := range a {
				if ipStr, ok := ip.(string); ok && ipStr != "" {
					ips = append(ips, ipStr)
				}
			}
		}
		// AAAA 记录
		if aaaa, ok := dnsResult["aaaa"].([]interface{}); ok {
			for _, ip := range aaaa {
				if ipStr, ok := ip.(string); ok && ipStr != "" {
					ips = append(ips, ipStr)
				}
			}
		}
		// CNAME 记录（取第一个）
		if cnames, ok := dnsResult["cname"].([]interface{}); ok && len(cnames) > 0 && cname == "" {
			if cnameStr, ok := cnames[0].(string); ok {
				cname = strings.TrimSuffix(cnameStr, ".")
			}
		}
	}

	return ips, cname
}

func parseIP(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	return ip
}
