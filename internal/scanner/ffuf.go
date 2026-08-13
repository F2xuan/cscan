package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// FFufScanner 基于 CLI 的目录扫描器
type FFufScanner struct {
	BaseScanner
	executor *CmdExecutor
}

// NewFFufScanner 创建 ffuf 目录扫描器
func NewFFufScanner() *FFufScanner {
	return &FFufScanner{
		BaseScanner: BaseScanner{name: "ffuf"},
		executor:    NewExecutorForTool("ffuf"),
	}
}

// FFufOptions ffuf 扫描选项
type FFufOptions struct {
	Paths           []string `json:"paths"`
	Threads         int      `json:"threads"`
	Timeout         int      `json:"timeout"`
	Extensions      []string `json:"extensions"`
	FollowRedirect  bool     `json:"followRedirect"`
	AutoCalibration bool     `json:"autoCalibration"`
	StatusCodes     []int    `json:"statusCodes"`
	FilterSize      string   `json:"filterSize"`
	FilterWords     string   `json:"filterWords"`
	FilterLines     string   `json:"filterLines"`
	FilterRegex     string   `json:"filterRegex"`
	MatcherMode     string   `json:"matcherMode"`
	FilterMode      string   `json:"filterMode"`
	Rate            int      `json:"rate"`
	Recursion       bool     `json:"recursion"`
	RecursionDepth  int      `json:"recursionDepth"`
}

// Validate 验证配置
func (o *FFufOptions) Validate() error {
	if o.Threads < 0 {
		return fmt.Errorf("threads must be non-negative, got %d", o.Threads)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	return nil
}

// FFufCLIResult ffuf 2.x JSON 输出中 results[] 的单条记录
type FFufCLIResult struct {
	Url              string `json:"url"`
	StatusCode       int    `json:"status"`
	ContentLength    int64  `json:"length"`
	ContentWords     int64  `json:"words"`
	ContentLines     int64  `json:"lines"`
	ContentType      string `json:"content-type"`
	RedirectLocation string `json:"redirectlocation"`
	DurationNs       int64  `json:"duration"`
}

// ffufCLIOutput ffuf 2.x JSON 输出顶层包裹结构（-of json）
type ffufCLIOutput struct {
	Results []FFufCLIResult `json:"results"`
}

// Scan 执行目录扫描
func (s *FFufScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	result := &ScanResult{
		MainTaskId: config.MainTaskId,
	}

	opts := &FFufOptions{
		Threads:         50,
		Timeout:         10,
		FollowRedirect:  false,
		AutoCalibration: true,
	}
	if config.Options != nil {
		if v, ok := config.Options.(*FFufOptions); ok {
			opts = v
		}
	}

	logInfo := func(format string, args ...interface{}) {
		if config.TaskLogger != nil {
			config.TaskLogger("INFO", format, args...)
			return
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

	if len(opts.Paths) == 0 {
		logWarn("[FFuf] 未提供扫描路径")
		return result, nil
	}

	targets := s.collectTargets(config, logInfo, func(string, ...interface{}) {})
	if len(targets) == 0 {
		logWarn("[FFuf] 无有效目标")
		return result, nil
	}

	wordlistFile, err := s.writeWordlistFile(opts.Paths)
	if err != nil {
		return nil, fmt.Errorf("创建字典临时文件失败: %w", err)
	}
	defer os.Remove(wordlistFile)

	logInfo("[FFuf] 开始目录扫描，目标数: %d，路径数: %d", len(targets), len(opts.Paths))

	var allAssets []*Asset

	// 并发 Worker Pool：每个目标一个 ffuf 进程，完成一个补一个
	concurrency := opts.Threads
	if concurrency <= 0 {
		concurrency = config.WorkerConcurrency
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > 5 {
		concurrency = 5
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}
	logInfo("[FFuf] 开始目录扫描，目标数: %d，并发: %d", len(targets), concurrency)

	type scanResult struct {
		assets []*Asset
		target string
		err    error
	}
	targetChan := make(chan string, len(targets))
	resultChan := make(chan scanResult, len(targets))
	var scanWg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			for target := range targetChan {
				select {
				case <-ctx.Done():
					resultChan <- scanResult{target: target, err: ctx.Err()}
					return
				default:
				}
				logInfo("[FFuf] 扫描目标: %s", target)
				assets, err := s.scanTarget(ctx, target, wordlistFile, opts, logInfo, func(string, ...interface{}) {})
				resultChan <- scanResult{assets: assets, target: target, err: err}
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

	completed := 0
	for res := range resultChan {
		completed++
		if res.err != nil {
			logWarn("[FFuf] 扫描目标 %s 失败: %v", res.target, res.err)
		} else {
			allAssets = append(allAssets, res.assets...)
			logInfo("[FFuf] 目标 %s 发现 %d 个有效路径", res.target, len(res.assets))
			// 流式入库：每完成一个目标立即回调
			if config.OnTargetDone != nil && len(res.assets) > 0 {
				config.OnTargetDone(res.target, res.assets)
			}
		}
		if config.OnProgress != nil {
			progress := completed * 100 / len(targets)
			config.OnProgress(progress, fmt.Sprintf("已完成 %d/%d 个目标", completed, len(targets)))
		}
	}

	logInfo("[FFuf] 目录扫描完成，共发现 %d 个有效路径", len(allAssets))
	result.Assets = allAssets
	return result, nil
}

func (s *FFufScanner) scanTarget(ctx context.Context, target, wordlistFile string, opts *FFufOptions, logInfo, logDebug func(string, ...interface{})) ([]*Asset, error) {
	scanCtx, scanCancel := context.WithCancel(ctx)
	defer scanCancel()

	baseURL := strings.TrimSuffix(target, "/") + "/FUZZ"

	args := []string{
		"-u", baseURL,
		"-w", wordlistFile,
		"-of", "json",
		"-t", fmt.Sprintf("%d", opts.Threads),
		"-timeout", fmt.Sprintf("%d", opts.Timeout),
		"-mc", "200,204,301,302,307,401,403,405,500",
	}

	if opts.FollowRedirect {
		args = append(args, "-r")
	}
	if opts.Recursion {
		args = append(args, "-recursion", "-recursion-depth", fmt.Sprintf("%d", opts.RecursionDepth))
	}
	if opts.Rate > 0 {
		args = append(args, "-rate", fmt.Sprintf("%d", opts.Rate))
	}
	for _, ext := range opts.Extensions {
		args = append(args, "-e", strings.TrimPrefix(ext, "."))
	}

	// 输出到临时文件
	tmpFile, err := os.CreateTemp("", "ffuf-*.json")
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args = append(args, "-o", tmpPath)

	logx.Infof("[FFuf] CLI: target=%s wordlist=%s args=%s", target, wordlistFile, strings.Join(args, " "))

	logInfo("[FFuf] executing ffuf for %s", target)

	res, err := s.executor.Execute(scanCtx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout*2) * time.Second,
	})
	if err != nil {
		logx.Debugf("[FFuf] execution error target=%s err=%v", target, err)
		s.executor.LogResult("FFuf: "+target, res, err)
		return nil, fmt.Errorf("ffuf execution: %w", err)
	}

	// 读取 JSON 输出，若为空则回退到 stdout
	content, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		return nil, fmt.Errorf("read ffuf output: %w", readErr)
	}

	var parseData []byte
	if len(content) > 0 {
		parseData = content
	} else if len(res.Stdout) > 0 {
		logx.Debugf("[FFuf] output file empty, falling back to stdout (%d bytes)", len(res.Stdout))
		parseData = []byte(res.Stdout)
	} else {
		logx.Debugf("[FFuf] no output from file or stdout")
		return nil, nil
	}

	var wrapper ffufCLIOutput
	if err := json.Unmarshal(parseData, &wrapper); err != nil {
		logx.Debugf("[FFuf] JSON parse error target=%s err=%v raw=%s", target, err, string(parseData))
		return nil, fmt.Errorf("parse ffuf results: %w", err)
	}
	ffufResults := wrapper.Results

	logx.Debugf("[FFuf] %s: parsed %d results", target, len(ffufResults))

	return s.convertResults(target, ffufResults), nil
}

// convertResults 将 ffuf 结果转换为 Asset 列表
func (s *FFufScanner) convertResults(target string, results []FFufCLIResult) []*Asset {
	assets := make([]*Asset, 0, len(results))

	parsedTarget, err := url.Parse(target)
	if err != nil {
		return assets
	}

	for _, r := range results {
		if r.Url == "" {
			continue
		}

		parsedURL, err := url.Parse(r.Url)
		if err != nil {
			continue
		}

		port := 80
		if parsedURL.Scheme == "https" {
			port = 443
		}
		if parsedURL.Port() != "" {
			fmt.Sscanf(parsedURL.Port(), "%d", &port)
		}

		authority := parsedURL.Host
		if authority == "" {
			authority = parsedTarget.Host
		}
		hostname := parsedURL.Hostname()
		if hostname == "" {
			hostname = parsedTarget.Hostname()
		}

		asset := &Asset{
			Authority:     authority,
			Host:          hostname,
			Port:          port,
			Category:      "url",
			Service:       parsedURL.Scheme,
			HttpStatus:    fmt.Sprintf("%d", r.StatusCode),
			IsHTTP:        true,
			Source:        "ffuf",
			Path:          parsedURL.Path,
			ContentLength: r.ContentLength,
			ContentType:   r.ContentType,
			ContentWords:  r.ContentWords,
			ContentLines:  r.ContentLines,
			Duration:      r.DurationNs / int64(time.Millisecond),
		}

		if r.RedirectLocation != "" {
			asset.Title = r.RedirectLocation
		}

		assets = append(assets, asset)
	}

	return assets
}

// collectTargets 从 ScanConfig 中提取目标列表
func (s *FFufScanner) collectTargets(config *ScanConfig, logInfo, logDebug func(string, ...interface{})) []string {
	var targets []string

	if len(config.Assets) > 0 {
		for _, asset := range config.Assets {
			if asset.IsHTTP && IsHTTPService(asset.Service, asset.Port) {
				scheme := "http"
				if asset.Port == 443 || strings.HasPrefix(asset.Service, "https") {
					scheme = "https"
				}
				var baseURL string
				if (scheme == "http" && asset.Port == 80) || (scheme == "https" && asset.Port == 443) {
					baseURL = fmt.Sprintf("%s://%s", scheme, asset.Host)
				} else {
					baseURL = fmt.Sprintf("%s://%s:%d", scheme, asset.Host, asset.Port)
				}
				if asset.Path != "" && asset.Path != "/" {
					path := strings.TrimSuffix(asset.Path, "/")
					baseURL = baseURL + path
					logInfo("[FFuf] 使用带路径的目标: %s (基础路径: %s)", baseURL, asset.Path)
				}
				targets = append(targets, baseURL)
			} else {
				logDebug("[FFuf] 跳过非HTTP资产: %s:%d", asset.Host, asset.Port)
			}
		}
	} else if len(config.Targets) > 0 {
		targets = config.Targets
	} else if config.Target != "" {
		targets = strings.Split(config.Target, "\n")
	}

	for i, t := range targets {
		targets[i] = normalizeURL(t)
	}

	return targets
}

// writeWordlistFile 将路径列表写入临时文件
func (s *FFufScanner) writeWordlistFile(paths []string) (string, error) {
	tmpFile, err := os.CreateTemp("", "ffuf-wordlist-*.txt")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	for _, p := range paths {
		line := strings.TrimSpace(p)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "/")
		if _, err := fmt.Fprintln(tmpFile, line); err != nil {
			os.Remove(tmpFile.Name())
			return "", err
		}
	}

	return tmpFile.Name(), nil
}
