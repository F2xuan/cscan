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
	"cscan/pkg/utils"
)

// DnsxScanner DNS 查询扫描器 (CLI 模式)
type DnsxScanner struct {
	BaseScanner
	executor *CmdExecutor
}

// NewDnsxScanner 创建 Dnsx 扫描器
func NewDnsxScanner() *DnsxScanner {
	cfg := ToolConfigs["dnsx"]
	return &DnsxScanner{
		BaseScanner: BaseScanner{name: "dnsx"},
		executor:    NewCmdExecutor(cfg.BinaryName, cfg.MemoryLimitMB, cfg.DefaultTimeout),
	}
}

// DnsxOptions DNS 扫描选项
type DnsxOptions struct {
	Timeout    int      `json:"timeout"`
	Retries    int      `json:"retries"`
	Resolvers  []string `json:"resolvers"`
	WildcardIPs map[string]bool `json:"wildcardIPs"`
	WildcardFilter bool `json:"wildcardFilter"`
}

// Validate 验证配置
func (o *DnsxOptions) Validate() error {
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	if o.Retries < 0 {
		return fmt.Errorf("retries must be non-negative, got %d", o.Retries)
	}
	return nil
}

// DnsxResult dnsx CLI JSON 输出
type DnsxResult struct {
	Host     string   `json:"host"`
	IP       []string `json:"ip,omitempty"`
	CNAME    []string `json:"cname,omitempty"`
	Resolver string   `json:"resolver,omitempty"`
}

// Scan 执行 DNS 查询
func (s *DnsxScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	result := &ScanResult{
		WorkspaceId: config.WorkspaceId,
		MainTaskId:  config.MainTaskId,
	}

	opts := &DnsxOptions{
		Timeout:    5,
		Retries:    1,
	}
	if config.Options != nil {
		if o, ok := config.Options.(*DnsxOptions); ok {
			opts = o
		}
	}

	var domains []string
	if len(config.Targets) > 0 {
		domains = config.Targets
	} else if config.Target != "" {
		domains = strings.Split(config.Target, "\n")
	}
	for i := range domains {
		domains[i] = strings.TrimSpace(domains[i])
		if domains[i] == "" {
			domains = append(domains[:i], domains[i+1:]...)
		}
	}

	if len(domains) == 0 {
		return result, nil
	}

	assets, err := s.queryDomains(ctx, domains, opts)
	if err != nil {
		return nil, fmt.Errorf("dnsx query: %w", err)
	}
	result.Assets = assets
	return result, nil
}

func (s *DnsxScanner) queryDomains(ctx context.Context, domains []string, opts *DnsxOptions) ([]*Asset, error) {
	// 写入临时文件
	tmpFile, err := os.CreateTemp("", "dnsx-targets-*.txt")
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		tmpFile.WriteString(d + "\n")
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{
		"-l", tmpPath,
		"-json",
		"-silent",
		"-a", "-aaaa",
		"-cname",
		"-timeout", fmt.Sprintf("%d", opts.Timeout),
		"-retries", fmt.Sprintf("%d", opts.Retries),
		"-disable-update-check",
	}
	if len(opts.Resolvers) > 0 {
		args = append(args, "-r", strings.Join(opts.Resolvers, ","))
	}

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout*len(domains)+30) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("dnsx execution: %w", err)
	}

	var assets []*Asset
	ipLocator := geolocation.NewIPLocator()

	scanner := bufio.NewScanner(strings.NewReader(res.Stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var dr DnsxResult
		if err := json.Unmarshal([]byte(line), &dr); err != nil {
			continue
		}
		if dr.Host == "" {
			continue
		}

		asset := &Asset{
			Authority: dr.Host,
			Host:      dr.Host,
			Category:  "domain",
		}

		for _, ipStr := range dr.IP {
			parsedIP := net.ParseIP(ipStr)
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

		if len(dr.CNAME) > 0 {
			asset.CName = strings.TrimSuffix(dr.CNAME[0], ".")
		}

		assets = append(assets, asset)
	}

	return assets, nil
}

// DetectWildcard 检测泛解析（使用 dnsx CLI）
func (s *DnsxScanner) DetectWildcard(domain string) map[string]bool {
	wildcardIPs := make(map[string]bool)

	testSubdomains := []string{
		fmt.Sprintf("wildcard-test-%d.%s", utils.RandomInt(100000, 999999), domain),
		fmt.Sprintf("random-%d.%s", utils.RandomInt(100000, 999999), domain),
		fmt.Sprintf("nonexistent-%d.%s", utils.RandomInt(100000, 999999), domain),
	}

	tmpFile, err := os.CreateTemp("", "dnsx-wildcard-*.txt")
	if err != nil {
		return wildcardIPs
	}
	for _, d := range testSubdomains {
		tmpFile.WriteString(d + "\n")
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{
		"-l", tmpPath,
		"-json",
		"-silent",
		"-a",
		"-timeout", "5",
		"-retries", "1",
		"-disable-update-check",
	}

	res, err := s.executor.Execute(context.Background(), args, ExecuteOpts{Timeout: 30 * time.Second})
	if err != nil {
		return wildcardIPs
	}

	scanner := bufio.NewScanner(strings.NewReader(res.Stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var dr DnsxResult
		if err := json.Unmarshal([]byte(line), &dr); err != nil {
			continue
		}
		for _, ip := range dr.IP {
			wildcardIPs[ip] = true
		}
	}

	return wildcardIPs
}

// Lookup DNS 查询单个域名
func (s *DnsxScanner) Lookup(domain string) ([]string, error) {
	tmpFile, err := os.CreateTemp("", "dnsx-lookup-*.txt")
	if err != nil {
		return nil, err
	}
	tmpFile.WriteString(domain + "\n")
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{
		"-l", tmpPath,
		"-json",
		"-silent",
		"-a",
		"-timeout", "5",
		"-retries", "1",
		"-disable-update-check",
	}

	res, err := s.executor.Execute(context.Background(), args, ExecuteOpts{Timeout: 15 * time.Second})
	if err != nil {
		return nil, err
	}

	var ips []string
	scanner := bufio.NewScanner(strings.NewReader(res.Stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var dr DnsxResult
		if err := json.Unmarshal([]byte(line), &dr); err != nil {
			continue
		}
		ips = append(ips, dr.IP...)
	}

	return ips, nil
}
