package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
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
	cfg := ToolConfigs["subfinder"]
	return &SubfinderScanner{
		BaseScanner: BaseScanner{name: "subfinder"},
		executor:    NewCmdExecutor(cfg.BinaryName, cfg.MemoryLimitMB, cfg.DefaultTimeout),
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
	Host string `json:"host"`
	Source []string `json:"source,omitempty"`
}

// Scan 执行子域名扫描
func (s *SubfinderScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	result := &ScanResult{
		WorkspaceId: config.WorkspaceId,
		MainTaskId:  config.MainTaskId,
		Assets:      make([]*Asset, 0),
	}

	opts := &SubfinderOptions{
		Timeout:            30,
		MaxEnumerationTime: 10,
		Threads:            10,
		RateLimit:          0,
		RemoveWildcard:     true,
		ResolveDNS:         false,
		Concurrent:         50,
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

	var allAssets []*Asset
	for _, domain := range domains {
		assets, err := s.scanDomain(ctx, domain, opts, config.TaskLogger)
		if err != nil {
			logx.Errorf("Subfinder: error for %s: %v", domain, err)
			continue
		}
		allAssets = append(allAssets, assets...)
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

func (s *SubfinderScanner) scanDomain(ctx context.Context, domain string, opts *SubfinderOptions, taskLogger func(level, format string, args ...interface{})) ([]*Asset, error) {
	args := []string{
		"-d", domain,
		"-json",
		"-silent",
		"-timeout", fmt.Sprintf("%d", opts.Timeout),
		"-max-time", fmt.Sprintf("%d", opts.MaxEnumerationTime),
		"-threads", fmt.Sprintf("%d", opts.Threads),
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
		args = append(args, "-rm-wildcards")
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

	taskLogger("INFO", "Subfinder CLI: scanning %s", domain)

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout+10) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("subfinder execution: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("subfinder exit code %d: %s", res.ExitCode, res.Stderr)
	}

	// 读取输出文件
	content, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		return nil, fmt.Errorf("read output: %w", readErr)
	}

	var assets []*Asset
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var sr SubfinderResult
		if err := json.Unmarshal([]byte(line), &sr); err != nil {
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
			resolvedIPs := s.resolveDNS([]string{sr.Host})
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
		}

		assets = append(assets, asset)
	}

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

func (s *SubfinderScanner) resolveDNS(domains []string) []string {
	// 使用 dnsx CLI 进行批量 DNS 解析
	tmpFile, err := os.CreateTemp("", "dnsx-targets-*.txt")
	if err != nil {
		return nil
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
		"-a", "-aaaa",
		"-timeout", "5",
		"-retries", "1",
		"-disable-update-check",
	}

	res, err := executor.Execute(context.Background(), args, ExecuteOpts{
		Timeout: 5 * time.Minute,
	})
	if err != nil {
		return nil
	}

	var ips []string
	scanner := bufio.NewScanner(strings.NewReader(res.Stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var dnsResult map[string]interface{}
		if err := json.Unmarshal([]byte(line), &dnsResult); err != nil {
			continue
		}
		if host, ok := dnsResult["host"].(string); ok && host != "" {
			ips = append(ips, host)
		}
	}

	return ips
}

func parseIP(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	return ip
}
