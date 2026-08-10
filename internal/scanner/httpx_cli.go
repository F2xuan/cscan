package scanner

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"cscan/pkg/geolocation"

	"github.com/zeromicro/go-zero/core/logx"
)

// HttpxScanner httpx 扫描器 (CLI 模式)
type HttpxScanner struct {
	BaseScanner
	executor *CmdExecutor
}

// NewHttpxScanner 创建 httpx 扫描器
func NewHttpxScanner() *HttpxScanner {
	cfg := ToolConfigs["httpx"]
	return &HttpxScanner{
		BaseScanner: BaseScanner{name: "httpx"},
		executor:    NewCmdExecutor(cfg.BinaryName, cfg.MemoryLimitMB, cfg.DefaultTimeout),
	}
}

// HttpxOptions httpx 扫描选项
type HttpxOptions struct {
	Concurrency      int      `json:"concurrency"`
	Timeout          int      `json:"timeout"`
	FollowRedirects  bool     `json:"followRedirects"`
	MaxRedirects     int      `json:"maxRedirects"`
	TechDetect       bool     `json:"techDetect"`
	Favicon          bool     `json:"favicon"`
	ServerHeader     bool     `json:"serverHeader"`
	ContentType      bool     `json:"contentType"`
	Body             bool     `json:"body"`
	StatusCode       bool     `json:"statusCode"`
	Title            bool     `json:"title"`
	Screenshot       bool     `json:"screenshot"`
	OutputIP         bool     `json:"outputIP"`
	CustomHeaders    []string `json:"customHeaders"`
}

// Validate 验证配置
func (o *HttpxOptions) Validate() error {
	if o.Concurrency < 0 {
		return fmt.Errorf("concurrency must be non-negative, got %d", o.Concurrency)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	return nil
}

// HttpxCLIResult httpx CLI JSON 输出结构
type HttpxCLIResult struct {
	Input       string   `json:"input"`
	URL         string   `json:"url"`
	Scheme      string   `json:"scheme"`
	Host        string   `json:"host"`
	Port        string   `json:"port"`
	StatusCode  int      `json:"status-code"`
	Title       string   `json:"title"`
	Technologies []string `json:"tech,omitempty"`
	WebServer   string   `json:"webserver,omitempty"`
	ContentType string   `json:"content-type,omitempty"`
	ContentLength int64  `json:"content-length,omitempty"`
	ResponseBody string `json:"body,omitempty"`
	Headers     string   `json:"headers,omitempty"`
	FaviconMMH3 string   `json:"favicon-mmh3,omitempty"`
	FaviconData string   `json:"favicon,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Chain       []string `json:"chain,omitempty"`
}

// Scan 执行 httpx 扫描
func (s *HttpxScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	result := &ScanResult{
		WorkspaceId: config.WorkspaceId,
		MainTaskId:  config.MainTaskId,
	}

	opts := &HttpxOptions{
		Concurrency:     150,
		Timeout:         10,
		FollowRedirects: true,
		MaxRedirects:    5,
		TechDetect:      true,
		Favicon:         true,
		ServerHeader:    true,
		ContentType:     true,
		StatusCode:      true,
		Title:           true,
		OutputIP:        true,
	}
	if config.Options != nil {
		switch v := config.Options.(type) {
		case *HttpxOptions:
			opts = v
		default:
			if data, err := json.Marshal(config.Options); err == nil {
				json.Unmarshal(data, opts)
			}
		}
	}

	if len(config.Assets) == 0 && len(config.Targets) == 0 && config.Target == "" {
		return result, nil
	}

	logx.Infof("Httpx(CLI): scanning assets")

	// 构建目标列表
	targets, targetMap := s.buildTargets(config.Assets)

	// 写入目标文件
	tmpFile, err := os.CreateTemp("", "httpx-targets-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create targets file: %w", err)
	}
	for _, t := range targets {
		tmpFile.WriteString(t + "\n")
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{
		"-l", tmpPath,
		"-json",
		"-silent",
		"-threads", fmt.Sprintf("%d", opts.Concurrency),
		"-timeout", fmt.Sprintf("%d", opts.Timeout),
		"-follow-redirects",
		"-max-redirects", fmt.Sprintf("%d", opts.MaxRedirects),
		"-disable-update-check",
	}
	if opts.TechDetect {
		args = append(args, "-tech-detect")
	}
	if opts.Favicon {
		args = append(args, "-favicon")
	}
	if opts.ServerHeader {
		args = append(args, "-web-server")
	}
	if opts.ContentType {
		args = append(args, "-content-type")
	}
	if opts.Body {
		args = append(args, "-body")
	}
	if opts.StatusCode {
		args = append(args, "-status-code")
	}
	if opts.Title {
		args = append(args, "-title")
	}
	if opts.Screenshot {
		args = append(args, "-screenshot")
	}
	if opts.OutputIP {
		args = append(args, "-ip")
	}
	for _, h := range opts.CustomHeaders {
		args = append(args, "-header", h)
	}
	if opts.FollowRedirects {
		args = append(args, "-follow-redirects")
		if opts.MaxRedirects > 0 {
			args = append(args, "-max-redirects", fmt.Sprintf("%d", opts.MaxRedirects))
		}
	}

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout*2) * time.Second,
	})
	if err != nil {
		logx.Errorf("Httpx(CLI): execution error: %v", err)
		return result, nil
	}

	// 解析结果并原地更新 Asset
	processed := make(map[*Asset]bool)
	scanner := bufio.NewScanner(strings.NewReader(res.Stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var hr HttpxCLIResult
		if err := json.Unmarshal([]byte(line), &hr); err != nil {
			continue
		}

		// 匹配到 Asset
		asset := s.matchAsset(hr, targetMap)
		if asset == nil {
			continue
		}

		if processed[asset] {
			continue
		}
		processed[asset] = true

		// 原地更新
		if hr.Scheme != "" {
			asset.Service = hr.Scheme
		}
		asset.HttpStatus = fmt.Sprintf("%d", hr.StatusCode)
		asset.Title = hr.Title
		if len(hr.Technologies) > 0 {
			for _, tech := range hr.Technologies {
				asset.App = append(asset.App, tech+"[httpx]")
			}
		}
		if hr.FaviconMMH3 != "" {
			asset.IconHash = hr.FaviconMMH3
		}
		if hr.FaviconData != "" {
			if data, err := base64.StdEncoding.DecodeString(hr.FaviconData); err == nil && len(data) > 0 {
				asset.IconData = data
			}
		}
		if hr.WebServer != "" {
			asset.Server = hr.WebServer
		}
		if hr.Headers != "" {
			asset.HttpHeader = hr.Headers
		}
		if hr.ResponseBody != "" {
			body := hr.ResponseBody
			if len(body) > 50*1024 {
				body = body[:50*1024] + "\n...[truncated]"
			}
			asset.HttpBody = body
		}
		asset.IsHTTP = true

		if len(hr.IP) > 0 {
			ipLocator := geolocation.NewIPLocator()
			for _, ipStr := range hr.IP {
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
		}
	}

	logx.Infof("Httpx(CLI): completed, updated %d assets", len(processed))
	return result, nil
}

// buildTargets 构建目标列表和映射
func (s *HttpxScanner) buildTargets(assets []*Asset) ([]string, map[string]*Asset) {
	var targets []string
	targetMap := make(map[string]*Asset)

	for _, asset := range assets {
		ports := []int{asset.Port}
		if asset.Port == 0 {
			ports = []int{80, 443}
		}
		for _, p := range ports {
			target := fmt.Sprintf("%s:%d", asset.Host, p)
			targets = append(targets, target)
			targetMap[target] = asset
			targetMap[fmt.Sprintf("http://%s:%d", asset.Host, p)] = asset
			targetMap[fmt.Sprintf("https://%s:%d", asset.Host, p)] = asset
		}
	}
	return targets, targetMap
}

// matchAsset 将 httpx 结果匹配到 Asset
func (s *HttpxScanner) matchAsset(hr HttpxCLIResult, targetMap map[string]*Asset) *Asset {
	if hr.Input != "" {
		if asset, ok := targetMap[hr.Input]; ok {
			return asset
		}
	}
	if hr.URL != "" {
		if asset, ok := targetMap[hr.URL]; ok {
			return asset
		}
		if u, err := url.Parse(hr.URL); err == nil {
			host := u.Hostname()
			port := u.Port()
			if port == "" {
				if u.Scheme == "https" {
					port = "443"
				} else {
					port = "80"
				}
			}
			key := fmt.Sprintf("%s:%s", host, port)
			if asset, ok := targetMap[key]; ok {
				return asset
			}
		}
	}
	return nil
}

// RunHttpxLib 使用 CLI 方式执行 httpx 扫描（兼容旧接口）
func RunHttpxLib(ctx context.Context, assets []*Asset, opts *FingerprintOptions, taskLog func(level, format string, args ...interface{})) error {
	if len(assets) == 0 {
		return nil
	}

	scanner := NewHttpxScanner()
	httpxOpts := &HttpxOptions{
		Concurrency:     150,
		Timeout:         opts.TargetTimeout,
		FollowRedirects: true,
		MaxRedirects:    5,
		TechDetect:      true,
		Favicon:         opts.Screenshot,
		ServerHeader:    true,
		ContentType:     true,
		StatusCode:      true,
		Title:           true,
		OutputIP:        true,
	}

	scanConfig := &ScanConfig{
		Assets:      assets,
		Options:     httpxOpts,
		WorkspaceId: "",
		MainTaskId:  "",
		TaskLogger:  taskLog,
	}

	_, err := scanner.Scan(ctx, scanConfig)
	return err
}
