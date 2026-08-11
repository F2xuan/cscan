package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// FingerprintxScanner fingerprintx 扫描器 (CLI 模式)
type FingerprintxScanner struct {
	BaseScanner
	executor *CmdExecutor
}

// NewFingerprintxScanner 创建 fingerprintx 扫描器
func NewFingerprintxScanner() *FingerprintxScanner {
	return &FingerprintxScanner{
		BaseScanner: BaseScanner{name: "fingerprintx"},
		executor:    NewExecutorForTool("fingerprintx"),
	}
}

// FingerprintxOptions fingerprintx 扫描选项
type FingerprintxOptions struct {
	Timeout     int  `json:"timeout"`
	Concurrency int  `json:"concurrency"`
	UDP         bool `json:"udp"`
	FastMode    bool `json:"fastMode"`
}

// Validate 验证配置
func (o *FingerprintxOptions) Validate() error {
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	if o.Concurrency < 0 {
		return fmt.Errorf("concurrency must be non-negative, got %d", o.Concurrency)
	}
	return nil
}

// FxResult fingerprintx CLI JSON 输出结构
type FxResult struct {
	Host     string                 `json:"host"`
	IP       string                 `json:"ip"`
	Port     int                    `json:"port"`
	Protocol string                 `json:"protocol"`
	Version  string                 `json:"version,omitempty"`
	Banner   string                 `json:"banner,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Scan 执行 fingerprintx 扫描
func (s *FingerprintxScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	opts := &FingerprintxOptions{
		Timeout:     10,
		Concurrency: 1,
		UDP:         false,
		FastMode:    false,
	}
	if config.Options != nil {
		switch v := config.Options.(type) {
		case *FingerprintxOptions:
			opts = v
		default:
			if data, err := json.Marshal(config.Options); err == nil {
				json.Unmarshal(data, opts)
			}
		}
	}

	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}
	if opts.Concurrency > 5 {
		logx.Infof("Fingerprintx concurrency %d exceeds maximum 5, limiting to 5", opts.Concurrency)
		opts.Concurrency = 5
	}

	if len(config.Assets) == 0 {
		return &ScanResult{WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId, Assets: []*Asset{}}, nil
	}

	logx.Infof("Fingerprintx(CLI): scanning %d assets", len(config.Assets))

	// 确保并发数继承 worker 自适应值
	if opts.Concurrency <= 0 {
		opts.Concurrency = config.WorkerConcurrency
	}
	identifiedAssets := s.runFingerprintxCLI(ctx, config.Assets, opts, config.TaskLogger, config.OnProgress)

	return &ScanResult{
		WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId,
		Assets: identifiedAssets,
	}, nil
}

func (s *FingerprintxScanner) runFingerprintxCLI(ctx context.Context, assets []*Asset, opts *FingerprintxOptions,
	taskLog func(level, format string, args ...interface{}),
	onProgress func(int, string),
) []*Asset {
	// 并发 Worker Pool：每个目标一个 fingerprintx 进程，完成一个补一个
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}
	if concurrency > len(assets) {
		concurrency = len(assets)
	}
	total := len(assets)
	logx.Infof("Fingerprintx(CLI): scanning %d assets with %d workers", total, concurrency)

	targetChan := make(chan *Asset, total)
	resultChan := make(chan *Asset, total)
	var scanWg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			for asset := range targetChan {
				select {
				case <-ctx.Done():
					resultChan <- asset
					return
				default:
				}
				s.scanSingleTarget(ctx, asset, opts, taskLog)
				resultChan <- asset
			}
		}()
	}

dispatch:
	for _, asset := range assets {
		select {
		case <-ctx.Done():
			break dispatch
		case targetChan <- asset:
		}
	}
	close(targetChan)

	go func() {
		scanWg.Wait()
		close(resultChan)
	}()

	var identifiedAssets []*Asset
	completed := 0
	for res := range resultChan {
		completed++
		if res != nil {
			identifiedAssets = append(identifiedAssets, res)
		}
		if onProgress != nil {
			progress := completed * 100 / total
			onProgress(progress, fmt.Sprintf("Scanned %d/%d assets", completed, total))
		}
	}

	logx.Infof("Fingerprintx(CLI): completed scanning %d assets", len(identifiedAssets))
	return identifiedAssets
}

func (s *FingerprintxScanner) scanSingleTarget(
	ctx context.Context,
	asset *Asset,
	opts *FingerprintxOptions,
	taskLog func(level, format string, args ...interface{}),
) {
	target := fmt.Sprintf("%s:%d", asset.Host, asset.Port)
	args := []string{"-json", "-timeout", fmt.Sprintf("%d", opts.Timeout)}
	if opts.FastMode {
		args = append(args, "-fast")
	}
	args = append(args, target)

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout+10) * time.Second,
	})
	if err != nil {
		logx.Debugf("Fingerprintx(CLI): scan error for %s: %v", target, err)
		asset.IsHTTP = IsHTTPService(asset.Service, asset.Port)
		return
	}

	resultMap := make(map[string]*FxResult)
	scanner := newLineScanner(res.Stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var fxr FxResult
		if err := json.Unmarshal([]byte(line), &fxr); err != nil {
			continue
		}
		key := fmt.Sprintf("%s:%d", fxr.Host, fxr.Port)
		resultMap[key] = &fxr
	}

	key := fmt.Sprintf("%s:%d", asset.Host, asset.Port)
	if fxr, ok := resultMap[key]; ok {
		if fxr.Protocol != "" {
			asset.Service = fxr.Protocol
		}
		if fxr.Version != "" {
			productInfo := fxr.Protocol
			if fxr.Version != "" {
				productInfo += ":" + fxr.Version
			}
			found := false
			for _, app := range asset.App {
				if app == productInfo {
					found = true
					break
				}
			}
			if !found {
				asset.App = append(asset.App, productInfo)
			}
		}
		if len(fxr.Banner) > 0 {
			maxBannerLen := 1024
			if len(fxr.Banner) > maxBannerLen {
				asset.Banner = fxr.Banner[:maxBannerLen] + "...[truncated]"
			} else {
				asset.Banner = fxr.Banner
			}
		}
		if fxr.Metadata != nil {
			metadataStr := formatMetadataMap(fxr.Metadata)
			if metadataStr != "" {
				if asset.Banner != "" {
					asset.Banner += "\n" + metadataStr
				} else {
					asset.Banner = metadataStr
				}
			}
		}
	}
	asset.IsHTTP = IsHTTPService(asset.Service, asset.Port)
}

// formatMetadataMap 格式化 metadata map 为字符串
func formatMetadataMap(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	if string(data) == "{}" || string(data) == "null" {
		return ""
	}
	return string(data)
}

// CheckFingerprintxAvailable 检查 fingerprintx 是否可用
func CheckFingerprintxAvailable() bool {
	return true
}
