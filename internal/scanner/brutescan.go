package scanner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cscan/internal/scanner/brute"
)

// ServiceDictEntry 服务特定的用户名:密码条目
type ServiceDictEntry struct {
	Username string
	Password string
}

// BruteScanConfig 弱口令扫描配置
type BruteScanConfig struct {
	// 服务列表：ssh,mysql,redis,mongodb,postgresql,mssql,ftp,oracle,smb,mqtt
	Services        []string `json:"services"`
	Threads         int      `json:"threads"`         // 并发线程数
	Timeout         int      `json:"timeout"`         // 单次连接超时时间(秒)
	DelayMs         int      `json:"delayMs"`         // 每次尝试间隔(毫秒)
	WeakpassDictIds []string `json:"weakpassDictIds"` // 字典ID列表
	UseDefaultDict  bool     `json:"useDefaultDict"`  // 是否使用默认字典
	StopOnFirst     bool     `json:"stopOnFirst"`     // 发现一个弱口令即停止
	ForceScan       bool     `json:"forceScan"`       // 强制扫描（不检测端口开放状态）
	// 字典内容（由 worker 注入）
	UsernameDict string `json:"usernameDict"`
	PasswordDict string `json:"passwordDict"`
	// 服务特定字典：service -> []ServiceDictEntry
	// 优先级高于通用 UsernameDict/PasswordDict
	ServiceDicts map[string][]ServiceDictEntry `json:"-"`
	// 内部使用，不序列化
	stopChan chan struct{} `json:"-"` // 用于 StopOnFirst 跨服务停止信号
}

// Validate 验证 BruteScanConfig 配置
// 使用指针接收者，确保对字段的修改能反映到原始配置（修复 P0-29）
func (c *BruteScanConfig) Validate() error {
	if c.Threads < 1 {
		c.Threads = 10
	}
	if c.Threads > 100 {
		c.Threads = 100
	}
	if c.Timeout < 1 {
		c.Timeout = 5
	}
	if c.Timeout > 60 {
		c.Timeout = 60
	}
	if c.DelayMs < 0 {
		c.DelayMs = 0
	}
	return nil
}

// BruteScanScanner 弱口令扫描器
type BruteScanScanner struct {
	BaseScanner
}

// NewBruteScanScanner 创建弱口令扫描器
func NewBruteScanScanner() *BruteScanScanner {
	return &BruteScanScanner{
		BaseScanner: BaseScanner{name: "brutescan"},
	}
}

// logHelper 日志辅助函数
func logHelper(taskLogger func(string, string, ...interface{}), level, format string, args ...interface{}) {
	if taskLogger != nil {
		taskLogger(level, format, args...)
	}
}

// Scan 执行弱口令扫描
func (s *BruteScanScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	// 获取 TaskLogger
	taskLogger := config.TaskLogger

	// 获取配置 - 使用直接类型断言
	var bruteConfig BruteScanConfig
	if config.Options != nil {
		if bc, ok := config.Options.(BruteScanConfig); ok {
			bruteConfig = bc
		} else if bcPtr, ok := config.Options.(*BruteScanConfig); ok {
			bruteConfig = *bcPtr
		}
	}

	logHelper(taskLogger, "INFO", "[BruteScan] Direct type assertion: Services=%v, UseDefaultDict=%v, ServiceDicts count=%d",
		bruteConfig.Services, bruteConfig.UseDefaultDict, len(bruteConfig.ServiceDicts))

	// 如果没有有效配置，使用默认配置
	if bruteConfig.Threads == 0 {
		logHelper(taskLogger, "INFO", "[BruteScan] Using default config (no service dicts will be available)")
		bruteConfig = BruteScanConfig{
			Threads:        10,
			Timeout:        5,
			DelayMs:        100,
			UseDefaultDict: true,
			StopOnFirst:    false,
			ForceScan:      false,
		}
	}

	logHelper(taskLogger, "INFO", "[BruteScan] After config check: ServiceDicts count=%d", len(bruteConfig.ServiceDicts))
	for svc, entries := range bruteConfig.ServiceDicts {
		logHelper(taskLogger, "INFO", "[BruteScan]   ServiceDicts[%s] has %d entries", svc, len(entries))
	}

	// P0-29: 调用指针接收者 Validate，确保对 Threads/Timeout/DelayMs 的默认值修正生效
	if err := bruteConfig.Validate(); err != nil {
		logHelper(taskLogger, "ERROR", "[BruteScan] Invalid brute config: %v", err)
		return nil, err
	}

	// 解析字典
	usernames, passwords := s.parseDicts(bruteConfig)

	if len(usernames) == 0 && len(passwords) == 0 && !bruteConfig.UseDefaultDict {
		logHelper(taskLogger, "INFO", "[BruteScan] No dictionaries provided")
		return &ScanResult{}, nil
	}

	// 获取目标资产
	assets := config.Assets
	if len(assets) == 0 {
		logHelper(taskLogger, "INFO", "[BruteScan] No assets to scan")
		return &ScanResult{}, nil
	}

	// 按服务分组资产
	serviceAssets := s.groupAssetsByService(assets, bruteConfig.Services)

	// 打印所有按服务分组的资产
	for service, assets := range serviceAssets {
		logHelper(taskLogger, "INFO", "[BruteScan] Service %s has %d assets", service, len(assets))
		for _, asset := range assets {
			logHelper(taskLogger, "INFO", "[BruteScan]   -> %s:%d (%s)", asset.Host, asset.Port, asset.Service)
		}
	}

	// 执行扫描
	var wg sync.WaitGroup
	var mu sync.Mutex
	vulns := make([]*Vulnerability, 0)

	// StopOnFirst: 记录每个目标是否已找到弱口令（目标级别，只用host，同一IP的所有服务都停止）
	type targetKey struct {
		host string
	}
	hostFound := make(map[targetKey]bool)
	var hostFoundMu sync.RWMutex

	// StopOnFirst: 创建跨服务的停止 channel
	if bruteConfig.StopOnFirst {
		bruteConfig.stopChan = make(chan struct{})
	}

	// 进度回调
	totalAssets := 0
	for _, assets := range serviceAssets {
		totalAssets += len(assets)
	}
	processedAssets := int32(0)

	for service, assets := range serviceAssets {
		plugin := brute.GetPlugin(service)
		if plugin == nil {
			logHelper(taskLogger, "ERROR", "[BruteScan] No plugin for service: %s", service)
			continue
		}

		logHelper(taskLogger, "INFO", "[BruteScan] Processing service: %s, assets count: %d, usernames: %d, passwords: %d", service, len(assets), len(usernames), len(passwords))

		for _, asset := range assets {
			wg.Add(1)
			go func(asset *Asset, service string, plugin brute.BrutePlugin) {
				defer wg.Done()

				// StopOnFirst: 检查该目标是否已找到弱口令（只检查 host，同一 IP 的所有服务都停止）
				if bruteConfig.StopOnFirst {
					hostFoundMu.RLock()
					found := hostFound[targetKey{host: asset.Host}]
					hostFoundMu.RUnlock()
					if found {
						logHelper(taskLogger, "INFO", "[BruteScan] [%s] Skipping %s:%d (already found weak password for this host, stopping all services)", service, asset.Host, asset.Port)
						atomic.AddInt32(&processedAssets, 1)
						s.reportProgress(config, int(atomic.LoadInt32(&processedAssets)), totalAssets, service, asset.Host)
						return
					}
				}

				// StopOnFirst: 检查跨服务停止信号
				if bruteConfig.stopChan != nil {
					select {
					case <-bruteConfig.stopChan:
						logHelper(taskLogger, "INFO", "[BruteScan] [%s] Stopped %s:%d (received stop signal)", service, asset.Host, asset.Port)
						atomic.AddInt32(&processedAssets, 1)
						s.reportProgress(config, int(atomic.LoadInt32(&processedAssets)), totalAssets, service, asset.Host)
						return
					default:
					}
				}

				// 检查端口是否开放（除非强制扫描）
				if !bruteConfig.ForceScan {
					if !plugin.Probe(ctx, asset.Host, asset.Port) {
						atomic.AddInt32(&processedAssets, 1)
						s.reportProgress(config, int(atomic.LoadInt32(&processedAssets)), totalAssets, service, asset.Host)
						return
					}
				}

				// 执行爆破
				bruteCtx, cancel := context.WithTimeout(ctx, time.Duration(bruteConfig.Timeout*30)*time.Second)
				defer cancel()

				result := s.bruteService(bruteCtx, plugin, asset.Host, asset.Port, usernames, passwords, &bruteConfig, service, taskLogger)

				atomic.AddInt32(&processedAssets, 1)
				s.reportProgress(config, int(atomic.LoadInt32(&processedAssets)), totalAssets, service, asset.Host)

				if result.Success {
					// StopOnFirst: 标记该目标已找到弱口令（只用 host，同一 IP 的所有服务都停止）
					if bruteConfig.StopOnFirst {
						hostFoundMu.Lock()
						hostFound[targetKey{host: asset.Host}] = true
						hostFoundMu.Unlock()
						logHelper(taskLogger, "INFO", "[BruteScan] [%s] Found weak password on %s:%d - %s/%s (stopping all services for this host)", service, asset.Host, asset.Port, result.Username, result.Password)
					}

					// 构建漏洞URL: service://username:password@host:port
					// Oracle 特殊处理：附加匹配到的服务名，如 oracle://user:pass@host:port(xe)
					vulnUrl := fmt.Sprintf("%s://%s:%s@%s:%d", service, result.Username, result.Password, asset.Host, asset.Port)
					if service == "oracle" && result.ExtraInfo != "" {
						vulnUrl = fmt.Sprintf("%s://%s:%s@%s:%d(%s)", service, result.Username, result.Password, asset.Host, asset.Port, result.ExtraInfo)
					}

					vuln := &Vulnerability{
						Authority: fmt.Sprintf("%s:%d", asset.Host, asset.Port),
						Host:      asset.Host,
						Port:      asset.Port,
						Url:       vulnUrl,
						Source:    "brutescan",
						Severity:  "high",
						Result:    fmt.Sprintf("Weak password found: %s/%s", result.Username, result.Password),
						Extra:     result.Message,
						VulName:   fmt.Sprintf("%s Weak Password", strings.ToUpper(service)),
						Tags:      []string{"bruteforce", "weak-password", service},
					}

					mu.Lock()
					vulns = append(vulns, vuln)
					mu.Unlock()
				}
			}(asset, service, plugin)
		}
	}

	wg.Wait()

	return &ScanResult{
		WorkspaceId:     config.WorkspaceId,
		MainTaskId:      config.MainTaskId,
		Vulnerabilities: vulns,
	}, nil
}

// bruteService 对单个服务执行爆破
func (s *BruteScanScanner) bruteService(ctx context.Context, plugin brute.BrutePlugin, host string, port int, usernames, passwords []string, config *BruteScanConfig, service string, taskLogger func(string, string, ...interface{})) *brute.BruteResult {
	logHelper(taskLogger, "INFO", "[BruteScan] bruteService called: host=%s, port=%d, service=%s, usernames=%d, passwords=%d", host, port, service, len(usernames), len(passwords))

	// 根据服务类型决定使用哪些字典
	var usernamesToUse, passwordsToUse []string

	// 检查是否有服务特定的字典
	normalizedService := brute.NormalizeServiceName(service)

	// 调试日志：打印 ServiceDicts 的键名
	if config.ServiceDicts != nil {
		keys := make([]string, 0, len(config.ServiceDicts))
		for k := range config.ServiceDicts {
			keys = append(keys, k)
		}
		logHelper(taskLogger, "DEBUG", "[BruteScan] bruteService: normalizedService=%s, ServiceDicts keys=%v", normalizedService, keys)
	}

	// 严格配对列表：用于保持字典中的原始 user:pass 组合顺序
	// 当有 ServiceDicts 时，优先使用配对列表以保证组合不变
	var pairList []ServiceDictEntry

	if config.ServiceDicts != nil {
		// 首先查找服务特定的字典
		entries, hasServiceDict := config.ServiceDicts[normalizedService]

		// 如果没有服务特定的字典，尝试 "common" 通用字典
		if !hasServiceDict || len(entries) == 0 {
			if commonEntries, hasCommon := config.ServiceDicts["common"]; hasCommon && len(commonEntries) > 0 {
				entries = commonEntries
				hasServiceDict = true
				logHelper(taskLogger, "DEBUG", "[BruteScan] Using 'common' dict for service: %s", normalizedService)
			}
		}

		if hasServiceDict && len(entries) > 0 {
			// 去重
			seen := make(map[string]bool)
			for _, entry := range entries {
				key := entry.Username + ":" + entry.Password
				if !seen[key] {
					seen[key] = true
					pairList = append(pairList, entry)
				}
			}
			logHelper(taskLogger, "INFO", "[BruteScan] Service %s using service dict: %d unique pairs", service, len(pairList))
		}
	}

	// 如果有配对列表，使用配对模式爆破（保持原始组合）
	if len(pairList) > 0 {
		return s.bruteWithPairs(ctx, plugin, host, port, pairList, config, service, taskLogger)
	}

	// 没有服务特定字典，使用通用字典
	switch normalizedService {
	case "redis", "mongodb":
		// 这些服务只需要密码字典
		usernamesToUse = []string{""}
		if len(passwords) > 0 {
			passwordsToUse = passwords
		}
	case "oracle":
		// Oracle 需要特殊的用户名和密码组合
		usernamesToUse = usernames
		passwordsToUse = passwords
	default:
		// 其他服务使用通用字典
		if len(usernames) > 0 {
			usernamesToUse = usernames
		} else {
			usernamesToUse = []string{"admin", "root"}
		}
		if len(passwords) > 0 {
			passwordsToUse = passwords
		} else {
			passwordsToUse = []string{"password", "123456", "admin"}
		}
	}

	if len(usernamesToUse) == 0 && len(passwordsToUse) == 0 {
		return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "No credentials to try"}
	}

	logHelper(taskLogger, "INFO", "[BruteScan] Service %s will use %d usernames, %d passwords", service, len(usernamesToUse), len(passwordsToUse))
	if len(usernamesToUse) > 0 && len(usernamesToUse) <= 10 {
		logHelper(taskLogger, "INFO", "[BruteScan] Usernames: %v", usernamesToUse)
	}
	if len(passwordsToUse) > 0 && len(passwordsToUse) <= 20 {
		logHelper(taskLogger, "INFO", "[BruteScan] Passwords: %v", passwordsToUse)
	}

	// 并发控制
	sem := make(chan struct{}, config.Threads)
	var wg sync.WaitGroup
	var resultMu sync.Mutex
	var finalResult *brute.BruteResult

	// StopOnFirst: 创建可取消的 context，找到弱口令后立即取消所有 goroutine
	var cancel context.CancelFunc
	bruteCtx := ctx
	if config.StopOnFirst {
		bruteCtx, cancel = context.WithCancel(ctx)
		defer cancel() // 确保函数退出时取消
	}

nextUsername:
	for _, username := range usernamesToUse {
		for _, password := range passwordsToUse {
			// StopOnFirst: 启动前检查是否已找到
			if config.StopOnFirst {
				resultMu.Lock()
				if finalResult != nil && finalResult.Success {
					resultMu.Unlock()
					if cancel != nil {
						cancel() // 取消所有 goroutine
					}
					goto nextUsername // 已找到，跳到下一个用户名
				}
				resultMu.Unlock()
			}

			select {
			case <-ctx.Done():
				return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "canceled"}
			case <-bruteCtx.Done():
				// StopOnFirst: 已被取消（找到弱口令）
				return finalResult
			case sem <- struct{}{}:
				// 获取到信号量，继续执行
			}

			wg.Add(1)
			go func(user, pass string) {
				defer wg.Done()
				defer func() { <-sem }()

				// 检查是否已找到或已取消
				select {
				case <-bruteCtx.Done():
					// 已取消，直接返回
					return
				default:
				}

				// 检查是否已找到
				resultMu.Lock()
				if finalResult != nil && finalResult.Success {
					resultMu.Unlock()
					return
				}
				resultMu.Unlock()

				// 执行爆破
				logHelper(taskLogger, "DEBUG", "[BruteScan] [%s] Trying %s:%s@%s:%d", service, user, pass, host, port)
				result := plugin.Brute(bruteCtx, host, port, []string{user}, []string{pass}, config.Timeout)

				if result.Success {
					resultMu.Lock()
					if finalResult == nil { // 确保只设置一次
						finalResult = result
						logHelper(taskLogger, "INFO", "[BruteScan] [%s] Found weak password: %s:%s@%s:%d", service, user, result.Password, host, port)
					}
					resultMu.Unlock()
					if cancel != nil {
						cancel() // 取消所有其他 goroutine
					}
					return // 成功后直接返回，不再延迟
				}

				// 延迟（仅失败时）
				if config.DelayMs > 0 {
					time.Sleep(time.Duration(config.DelayMs) * time.Millisecond)
				}
			}(username, password)
		}
	}

	wg.Wait()

	if finalResult == nil {
		return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "No valid credentials found"}
	}
	return finalResult
}

// bruteWithPairs 使用配对列表执行爆破（保持字典中原始 user:pass 组合顺序）
// 支持并发控制、延迟、StopOnFirst，并且每次尝试都有日志输出
func (s *BruteScanScanner) bruteWithPairs(ctx context.Context, plugin brute.BrutePlugin, host string, port int, pairs []ServiceDictEntry, config *BruteScanConfig, service string, taskLogger func(string, string, ...interface{})) *brute.BruteResult {
	logHelper(taskLogger, "INFO", "[BruteScan] bruteWithPairs: host=%s, port=%d, service=%s, pairs=%d", host, port, service, len(pairs))

	normalizedService := brute.NormalizeServiceName(service)

	// 并发控制
	sem := make(chan struct{}, config.Threads)
	var wg sync.WaitGroup
	var resultMu sync.Mutex
	var finalResult *brute.BruteResult

	// StopOnFirst: 创建可取消的 context
	var cancel context.CancelFunc
	bruteCtx := ctx
	if config.StopOnFirst {
		bruteCtx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	// StopOnFirst: 跨资产停止信号
	if config.stopChan != nil {
		select {
		case <-config.stopChan:
			return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "stopped by signal"}
		default:
		}
	}

	for i, pair := range pairs {
		// StopOnFirst: 启动前检查是否已找到
		if config.StopOnFirst {
			resultMu.Lock()
			if finalResult != nil && finalResult.Success {
				resultMu.Unlock()
				if cancel != nil {
					cancel()
				}
				goto done
			}
			resultMu.Unlock()
		}

		// 检查停止信号
		if config.stopChan != nil {
			select {
			case <-config.stopChan:
				goto done
			default:
			}
		}

		select {
		case <-ctx.Done():
			return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "canceled"}
		case <-bruteCtx.Done():
			// 已被取消（找到弱口令）
			if finalResult != nil {
				return finalResult
			}
			return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "canceled"}
		case sem <- struct{}{}:
			// 获取到信号量，继续执行
		}

		wg.Add(1)
		go func(idx int, user, pass string) {
			defer wg.Done()
			defer func() { <-sem }()

			// 检查是否已找到或已取消
			select {
			case <-bruteCtx.Done():
				return
			default:
			}

			// 检查是否已找到
			resultMu.Lock()
			if finalResult != nil && finalResult.Success {
				resultMu.Unlock()
				return
			}
			resultMu.Unlock()

			// 执行爆破
			logHelper(taskLogger, "DEBUG", "[BruteScan] [%s] Trying %s:%s@%s:%d (%d/%d)", service, user, pass, host, port, idx+1, len(pairs))
			result := plugin.Brute(bruteCtx, host, port, []string{user}, []string{pass}, config.Timeout)

			if result.Success {
				resultMu.Lock()
				if finalResult == nil {
					finalResult = result
					logHelper(taskLogger, "WARN", "[BruteScan] [%s] Found weak password: %s:%s@%s:%d", service, user, pass, host, port)
				}
				resultMu.Unlock()
				if cancel != nil {
					cancel()
				}
				// 通知其他资产停止
				if config.stopChan != nil {
					select {
					case config.stopChan <- struct{}{}:
					default:
					}
				}
				return
			}

			// 延迟（仅失败时）
			if config.DelayMs > 0 {
				time.Sleep(time.Duration(config.DelayMs) * time.Millisecond)
			}
		}(i, pair.Username, pair.Password)
	}

done:
	wg.Wait()

	// 输出汇总日志
	if finalResult != nil && finalResult.Success {
		logHelper(taskLogger, "INFO", "[BruteScan] [%s] %s:%d - weak password found: %s/%s", service, host, port, finalResult.Username, finalResult.Password)
	} else {
		logHelper(taskLogger, "INFO", "[BruteScan] [%s] %s:%d - no weak password found (tried %d pairs)", normalizedService, host, port, len(pairs))
	}

	if finalResult == nil {
		return &brute.BruteResult{Host: host, Port: port, Service: service, Success: false, Message: "No valid credentials found"}
	}
	return finalResult
}

// parseDicts 解析字典内容
func (s *BruteScanScanner) parseDicts(config BruteScanConfig) (usernames, passwords []string) {
	if config.UsernameDict != "" {
		usernames = s.splitLines(config.UsernameDict)
	}
	if config.PasswordDict != "" {
		passwords = s.splitLines(config.PasswordDict)
	}
	return
}

// splitLines 分割字典内容为行
// 注意：Go 的 strings.Split 按 \n 分割，不会按 \r\n 分割
// 所以 Windows 格式的 \r 会保留在每行的末尾，需要单独处理
func (s *BruteScanScanner) splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		// 清理行尾的 \r（处理 Windows CRLF 格式：\r\n）
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			result = append(result, line)
		}
	}
	return result
}

// groupAssetsByService 按服务分组资产
func (s *BruteScanScanner) groupAssetsByService(assets []*Asset, services []string) map[string][]*Asset {
	serviceSet := make(map[string]bool)
	for _, svc := range services {
		serviceSet[brute.NormalizeServiceName(svc)] = true
	}

	result := make(map[string][]*Asset)
	for _, asset := range assets {
		if asset.Service == "" {
			continue
		}
		normalizedService := brute.NormalizeServiceName(asset.Service)

		// 检查是否在目标服务列表中
		if len(services) > 0 && !serviceSet[normalizedService] {
			continue
		}

		// 检查是否有对应的插件
		if brute.GetPlugin(normalizedService) == nil {
			continue
		}

		result[normalizedService] = append(result[normalizedService], asset)
	}
	return result
}

// reportProgress 报告进度
func (s *BruteScanScanner) reportProgress(config *ScanConfig, processed, total int, service, host string) {
	if config.OnProgress != nil {
		percent := 0
		if total > 0 {
			percent = processed * 100 / total
		}
		config.OnProgress(percent, fmt.Sprintf("Scanning %s:%s (%d/%d)", service, host, processed, total))
	}
}
