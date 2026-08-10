package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	cfg := ToolConfigs["fingerprintx"]
	return &FingerprintxScanner{
		BaseScanner: BaseScanner{name: "fingerprintx"},
		executor:    NewCmdExecutor(cfg.BinaryName, cfg.MemoryLimitMB, cfg.DefaultTimeout),
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
	Host       string                 `json:"host"`
	IP         string                 `json:"ip"`
	Port       int                    `json:"port"`
	Protocol   string                 `json:"protocol"`
	Version    string                 `json:"version,omitempty"`
	Banner     string                 `json:"banner,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
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
	const batchSize = 20
	var identifiedAssets []*Asset
	total := len(assets)
	completed := 0

	for batchStart := 0; batchStart < total; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > total {
			batchEnd = total
		}
		batch := assets[batchStart:batchEnd]

		if ctx.Err() != nil {
			return identifiedAssets
		}

		batchAssets := s.scanBatch(ctx, batch, opts, taskLog)
		identifiedAssets = append(identifiedAssets, batchAssets...)
		completed = batchEnd

		if onProgress != nil {
			progress := completed * 100 / total
			onProgress(progress, fmt.Sprintf("Scanned %d/%d assets", completed, total))
		}
	}

	logx.Infof("Fingerprintx(CLI): completed scanning %d assets", len(identifiedAssets))
	return identifiedAssets
}

func (s *FingerprintxScanner) scanBatch(ctx context.Context, assets []*Asset, opts *FingerprintxOptions,
	taskLog func(level, format string, args ...interface{}),
) []*Asset {
	var targets []string
	for _, asset := range assets {
		targets = append(targets, fmt.Sprintf("%s:%d", asset.Host, asset.Port))
	}

	args := []string{"-json", "-timeout", fmt.Sprintf("%d", opts.Timeout)}
	if opts.FastMode {
		args = append(args, "-fast")
	}
	args = append(args, targets...)

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout+10) * time.Second,
	})
	if err != nil {
		logx.Debugf("Fingerprintx(CLI): scan error: %v", err)
		for _, asset := range assets {
			asset.IsHTTP = IsHTTPService(asset.Service, asset.Port)
		}
		return assets
	}

	resultMap := make(map[string]*FxResult)
	scanner := bufio.NewScanner(strings.NewReader(res.Stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
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

	for _, asset := range assets {
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

	return assets
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
