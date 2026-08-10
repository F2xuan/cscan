package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"cscan/pkg/geolocation"
	"cscan/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

// ErrPortThresholdExceeded 端口阈值超过错误
var ErrPortThresholdExceeded = fmt.Errorf("port threshold exceeded")

var ipLocator = geolocation.NewIPLocator()

// NaabuScanner Naabu端口扫描器 (CLI 模式)
type NaabuScanner struct {
	BaseScanner
	skippedHosts   []string
	dnsFailedHosts []string
	mu             sync.Mutex
	executor       *CmdExecutor
}

// NewNaabuScanner 创建Naabu扫描器
func NewNaabuScanner() *NaabuScanner {
	cfg := ToolConfigs["naabu"]
	return &NaabuScanner{
		BaseScanner: BaseScanner{name: "naabu"},
		skippedHosts: make([]string, 0),
		executor:    NewCmdExecutor(cfg.BinaryName, cfg.MemoryLimitMB, cfg.DefaultTimeout),
	}
}

// NaabuOptions Naabu扫描选项
type NaabuOptions struct {
	Ports             string `json:"ports"`
	Rate              int    `json:"rate"`
	Timeout           int    `json:"timeout"`
	ScanType          string `json:"scanType"`
	PortThreshold     int    `json:"portThreshold"`
	SkipHostDiscovery bool   `json:"skipHostDiscovery"`
	ExcludeCDN        bool   `json:"excludeCDN"`
	ExcludeHosts      string `json:"excludeHosts"`
	Retries           int    `json:"retries"`
	WarmUpTime        int    `json:"warmUpTime"`
	Workers           int    `json:"workers"`
	Verify            bool   `json:"verify"`
}

// Validate 验证配置
func (o *NaabuOptions) Validate() error {
	if o.Rate < 0 {
		return fmt.Errorf("rate must be non-negative, got %d", o.Rate)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	if o.PortThreshold < 0 {
		return fmt.Errorf("portThreshold must be non-negative, got %d", o.PortThreshold)
	}
	if o.ScanType != "" && o.ScanType != "s" && o.ScanType != "c" {
		return fmt.Errorf("scanType must be 's' or 'c', got %s", o.ScanType)
	}
	return nil
}

// NaabuHostResult Naabu JSON 输出结构
type NaabuHostResult struct {
	Host string `json:"host"`
	IP   string `json:"ip"`
	Ports []struct {
		Port int    `json:"port"`
		Proto string `json:"proto"`
		Status string `json:"status"`
	} `json:"ports"`
}

// Scan 执行Naabu扫描
func (s *NaabuScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	s.mu.Lock()
	s.skippedHosts = s.skippedHosts[:0]
	s.dnsFailedHosts = s.dnsFailedHosts[:0]
	s.mu.Unlock()

	adaptive := GetGlobalAdaptiveConfig()
	opts := &NaabuOptions{
		Ports:         "80,443,8080",
		Rate:          adaptive.NaabuRate,
		Timeout:       60,
		ScanType:      "c",
		PortThreshold: 0,
		Retries:       adaptive.NaabuRetries,
		WarmUpTime:    1,
		Workers:       adaptive.NaabuWorkers,
		Verify:        false,
	}
	if config.Options != nil {
		switch v := config.Options.(type) {
		case *NaabuOptions:
			opts = v
		case *PortScanOptions:
			if v.Ports != "" { opts.Ports = v.Ports }
			if v.Rate > 0 { opts.Rate = v.Rate }
			if v.Timeout > 0 { opts.Timeout = v.Timeout }
			if v.PortThreshold > 0 { opts.PortThreshold = v.PortThreshold }
		default:
			if data, err := json.Marshal(config.Options); err == nil {
				var portConfig struct {
					Ports string `json:"ports"`
					Rate int `json:"rate"`
					Timeout int `json:"timeout"`
					PortThreshold int `json:"portThreshold"`
					ScanType string `json:"scanType"`
					SkipHostDiscovery bool `json:"skipHostDiscovery"`
					ExcludeCDN bool `json:"excludeCDN"`
					ExcludeHosts string `json:"excludeHosts"`
					Retries int `json:"retries"`
					WarmUpTime int `json:"warmUpTime"`
					Workers int `json:"workers"`
					Verify bool `json:"verify"`
				}
				if err := json.Unmarshal(data, &portConfig); err == nil {
					if portConfig.Ports != "" { opts.Ports = portConfig.Ports }
					if portConfig.Rate > 0 { opts.Rate = portConfig.Rate }
					if portConfig.Timeout > 0 { opts.Timeout = portConfig.Timeout }
					if portConfig.PortThreshold > 0 { opts.PortThreshold = portConfig.PortThreshold }
					if portConfig.ScanType != "" { opts.ScanType = portConfig.ScanType }
					if portConfig.Retries > 0 { opts.Retries = portConfig.Retries }
					if portConfig.WarmUpTime >= 0 { opts.WarmUpTime = portConfig.WarmUpTime }
					if portConfig.Workers > 0 { opts.Workers = portConfig.Workers }
					opts.SkipHostDiscovery = portConfig.SkipHostDiscovery
					opts.ExcludeCDN = portConfig.ExcludeCDN
					opts.ExcludeHosts = portConfig.ExcludeHosts
					opts.Verify = portConfig.Verify
				}
			}
		}
	}

	targetParseResult := ParseTargetsForPortScan(config.Target)
	for _, t := range config.Targets {
		res := ParseTargetsForPortScan(t)
		targetParseResult.WithPort = append(targetParseResult.WithPort, res.WithPort...)
		targetParseResult.WithoutPort = append(targetParseResult.WithoutPort, res.WithoutPort...)
	}

	var cleanTargets []string
	seenHost := make(map[string]bool)
	for _, host := range targetParseResult.WithoutPort {
		if !seenHost[host] {
			seenHost[host] = true
			cleanTargets = append(cleanTargets, host)
		}
	}
	originalPorts := opts.Ports
	ports := parsePorts(opts.Ports)
	portSet := make(map[int]bool)
	for _, p := range ports { portSet[p] = true }
	for _, taskWithPort := range targetParseResult.WithPort {
		if !seenHost[taskWithPort.Host] {
			seenHost[taskWithPort.Host] = true
			cleanTargets = append(cleanTargets, taskWithPort.Host)
		}
		if !portSet[taskWithPort.Port] {
			portSet[taskWithPort.Port] = true
			ports = append(ports, taskWithPort.Port)
		}
	}
	if len(targetParseResult.WithPort) > 0 {
		opts.Ports = portsToString(ports)
	} else {
		opts.Ports = originalPorts
	}

	if len(cleanTargets) == 0 {
		return &ScanResult{WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId, Assets: []*Asset{}}, nil
	}

	assets, thresholdExceeded := s.runNaabuCLI(ctx, cleanTargets, opts)
	if thresholdExceeded {
		return &ScanResult{
			WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId,
			Assets: assets, SkippedHosts: s.collectSkippedHosts(), DNSFailedHosts: s.collectDNSFailedHosts(),
		}, ErrPortThresholdExceeded
	}
	return &ScanResult{
		WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId,
		Assets: assets, SkippedHosts: s.collectSkippedHosts(), DNSFailedHosts: s.collectDNSFailedHosts(),
	}, nil
}

func (s *NaabuScanner) runNaabuCLI(ctx context.Context, targets []string, opts *NaabuOptions) ([]*Asset, bool) {
	var allAssets []*Asset
	anyThresholdExceeded := false

	var portsStr, topPorts string
	switch opts.Ports {
	case "top100":
		topPorts = "100"
	case "top1000":
		topPorts = "1000"
	default:
		portsStr = optimizePortsForNaabu(opts.Ports)
	}

	totalTargets := len(targets)
	logx.Infof("Naabu(CLI): scanning %d targets, ports=%s, rate=%d", totalTargets, opts.Ports, opts.Rate)

	for i, target := range targets {
		select {
		case <-ctx.Done():
			return allAssets, anyThresholdExceeded
		default:
		}

		assets, thresholdExceeded := s.scanTargetCLI(ctx, target, portsStr, topPorts, opts)
		if thresholdExceeded {
			anyThresholdExceeded = true
			s.mu.Lock()
			s.skippedHosts = append(s.skippedHosts, target)
			s.mu.Unlock()
			continue
		}
		allAssets = append(allAssets, assets...)
		_ = i
	}

	logx.Infof("Naabu(CLI): completed, found %d open ports", len(allAssets))
	return allAssets, anyThresholdExceeded
}

func (s *NaabuScanner) scanTargetCLI(ctx context.Context, target, portsStr, topPorts string, opts *NaabuOptions) ([]*Asset, bool) {
	args := []string{
		"-host", target,
		"-json",
		"-silent",
		"-rate", strconv.Itoa(opts.Rate),
		"-timeout", fmt.Sprintf("%ds", opts.Timeout),
		"-retries", strconv.Itoa(opts.Retries),
		"-warm-up-time", strconv.Itoa(opts.WarmUpTime),
		"-threads", strconv.Itoa(opts.Workers),
	}
	if portsStr != "" {
		args = append(args, "-p", portsStr)
	}
	if topPorts != "" {
		args = append(args, "-top-ports", topPorts)
	}
	if opts.ScanType == "s" {
		args = append(args, "-s")
	} else {
		args = append(args, "-c")
	}
	if opts.SkipHostDiscovery {
		args = append(args, "-Pn")
	}
	if opts.ExcludeCDN {
		args = append(args, "-ec")
	}
	if opts.ExcludeHosts != "" {
		args = append(args, "-eh", opts.ExcludeHosts)
	}
	if opts.Verify {
		args = append(args, "-verify")
	}
	if opts.PortThreshold > 0 {
		args = append(args, "-port-threshold", strconv.Itoa(opts.PortThreshold))
	}

	// 输出到临时文件
	tmpFile, err := os.CreateTemp("", "naabu-*.json")
	if err != nil {
		return nil, false
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args = append(args, "-o", tmpPath)

	logx.Infof("Naabu(CLI): %s %s", ToolConfigs["naabu"].BinaryName, strings.Join(args, " "))

	res, err := s.executor.Execute(ctx, args, ExecuteOpts{
		Timeout: time.Duration(opts.Timeout + 30) * time.Second,
	})
	_ = res

	// 读取 JSON 输出文件
	content, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		logx.Infof("[WARN] Naabu(CLI): failed to read output file: %v", readErr)
		return nil, false
	}

	var assets []*Asset
	var foundPorts []string
	hostPortCount := 0

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var hostResult NaabuHostResult
		if err := json.Unmarshal([]byte(line), &hostResult); err != nil {
			continue
		}
		for _, p := range hostResult.Ports {
			if p.Status == "open" {
				hostPortCount++
				if opts.PortThreshold > 0 && hostPortCount > opts.PortThreshold {
					return nil, true
				}
				locStr, _ := ipLocator.Locate(hostResult.IP)
				location := geolocation.NormalizeLocation(locStr)
				asset := &Asset{
					Authority: utils.BuildTargetWithPort(hostResult.Host, p.Port),
					Host:      hostResult.Host,
					Port:      p.Port,
					Category:  getCategory(hostResult.Host),
				}
				if hostResult.IP != "" {
					if strings.Contains(hostResult.IP, ":") {
						asset.IPV6 = []IPInfo{{IP: hostResult.IP, Location: location}}
					} else {
						asset.IPV4 = []IPInfo{{IP: hostResult.IP, Location: location}}
					}
				}
				assets = append(assets, asset)
				foundPorts = append(foundPorts, strconv.Itoa(p.Port))
			}
		}
	}

	if len(foundPorts) > 0 {
		logx.Infof("Naabu(CLI): %s -> %s", target, strings.Join(foundPorts, ","))
	} else {
		logx.Infof("Naabu(CLI): %s -> no open ports found", target)
	}

	return assets, false
}

func (s *NaabuScanner) collectSkippedHosts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.skippedHosts))
	copy(result, s.skippedHosts)
	return result
}

func (s *NaabuScanner) collectDNSFailedHosts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.dnsFailedHosts))
	copy(result, s.dnsFailedHosts)
	return result
}

// optimizePortsForNaabu 优化端口参数
func optimizePortsForNaabu(portStr string) string {
	portStr = strings.TrimSpace(portStr)
	if portStr == "top100" {
		return portsToString(GetTop100Ports())
	}
	if portStr == "top1000" {
		return portsToString(GetTop1000Ports())
	}
	parts := strings.Split(portStr, ",")
	if len(parts) == 1 && strings.Contains(parts[0], "-") {
		return portStr
	}
	hasLargeRange := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rParts := strings.Split(part, "-")
			if len(rParts) == 2 {
				start, _ := strconv.Atoi(strings.TrimSpace(rParts[0]))
				end, _ := strconv.Atoi(strings.TrimSpace(rParts[1]))
				if end-start > 1000 {
					hasLargeRange = true
					break
				}
			}
		}
	}
	if hasLargeRange {
		return portStr
	}
	return portsToString(parsePorts(portStr))
}
