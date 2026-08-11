package scanner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
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
	return &HttpxScanner{
		BaseScanner: BaseScanner{name: "httpx"},
		executor:    NewExecutorForTool("httpx"),
	}
}

// HttpxOptions httpx 扫描选项
type HttpxOptions struct {
	Concurrency     int      `json:"concurrency"`
	Timeout         int      `json:"timeout"`
	FollowRedirects bool     `json:"followRedirects"`
	MaxRedirects    int      `json:"maxRedirects"`
	TechDetect      bool     `json:"techDetect"`
	Favicon         bool     `json:"favicon"`
	ServerHeader    bool     `json:"serverHeader"`
	ContentType     bool     `json:"contentType"`
	Body            bool     `json:"body"`
	StatusCode      bool     `json:"statusCode"`
	Title           bool     `json:"title"`
	Screenshot      bool     `json:"screenshot"`
	OutputIP        bool     `json:"outputIP"`
	CustomHeaders   []string `json:"customHeaders"`
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
	Input         string   `json:"input"`
	URL           string   `json:"url"`
	Scheme        string   `json:"scheme"`
	Host          string   `json:"host"`
	Port          string   `json:"port"`
	StatusCode    int      `json:"status-code"`
	Title         string   `json:"title"`
	Technologies  []string `json:"tech,omitempty"`
	WebServer     string   `json:"webserver,omitempty"`
	ContentType   string   `json:"content-type,omitempty"`
	ContentLength int64    `json:"content-length,omitempty"`
	ResponseBody  string   `json:"body,omitempty"`
	Headers       string   `json:"headers,omitempty"`
	FaviconMMH3   string   `json:"favicon-mmh3,omitempty"`
	FaviconData   string   `json:"favicon,omitempty"`
	IP            []string `json:"ip,omitempty"`
	Chain         []string `json:"chain,omitempty"`
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

	// 构建目标列表
	targets, targetMap := s.buildTargets(config.Assets)
	if len(targets) == 0 {
		return result, nil
	}

	logx.Infof("Httpx(CLI): scanning %d targets", len(targets))

	// 并发 Worker Pool：每个目标一个 httpx 进程，完成一个补一个
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = config.WorkerConcurrency
	}
	if concurrency <= 0 {
		concurrency = 5
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}
	logx.Infof("Httpx(CLI): using %d workers for %d targets", concurrency, len(targets))

	type scanResult struct {
		processedAsset *Asset
		err            error
	}
	targetChan := make(chan string, len(targets))
	resultChan := make(chan scanResult, len(targets))
	var scanWg sync.WaitGroup
	var resultMu sync.Mutex
	processed := make(map[*Asset]bool)

	for i := 0; i < concurrency; i++ {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			for target := range targetChan {
				select {
				case <-ctx.Done():
					resultChan <- scanResult{err: ctx.Err()}
					return
				default:
				}
				asset := s.scanSingleTargetCLI(ctx, target, opts, targetMap, &resultMu, processed)
				if asset != nil {
					resultChan <- scanResult{processedAsset: asset}
				} else {
					resultChan <- scanResult{}
				}
				if config.OnTargetDone != nil {
					if asset != nil {
						config.OnTargetDone(target, []*Asset{asset})
					} else {
						config.OnTargetDone(target, nil)
					}
				}
			}
		}()
	}

dispatch:
	for _, target := range targets {
		select {
		case <-ctx.Done():
			break dispatch
		case targetChan <- target:
		}
	}
	close(targetChan)

	go func() {
		scanWg.Wait()
		close(resultChan)
	}()

	for res := range resultChan {
		if res.err != nil {
			logx.Errorf("Httpx(CLI): error: %v", res.err)
			continue
		}
	}

	logx.Infof("Httpx(CLI): completed, updated %d assets", len(processed))
	return result, nil
}

// scanSingleTargetCLI 对单个目标执行 httpx CLI 并更新 Asset
func (s *HttpxScanner) scanSingleTargetCLI(
	ctx context.Context, target string, opts *HttpxOptions,
	targetMap map[string]*Asset, resultMu *sync.Mutex, processed map[*Asset]bool,
) *Asset {
	args := []string{
		"-u", target,
		"-json",
		"-silent",
		"-timeout", fmt.Sprintf("%d", opts.Timeout),
		"-disable-update-check",
	}
	if opts.FollowRedirects {
		args = append(args, "-follow-redirects")
		if opts.MaxRedirects > 0 {
			args = append(args, "-max-redirects", fmt.Sprintf("%d", opts.MaxRedirects))
		}
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

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout+10) * time.Second,
	})
	if err != nil {
		logx.Errorf("Httpx(CLI): %s execution error: %v", target, err)
		return nil
	}

	scanner := newLineScanner(res.Stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var hr HttpxCLIResult
		if err := json.Unmarshal([]byte(line), &hr); err != nil {
			continue
		}

		asset := s.matchAsset(hr, targetMap)
		if asset == nil {
			continue
		}

		resultMu.Lock()
		if processed[asset] {
			resultMu.Unlock()
			continue
		}
		processed[asset] = true
		resultMu.Unlock()

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
				appendIPInfo(asset, ipStr, ipLocator)
			}
		}
		return asset
	}
	return nil
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
