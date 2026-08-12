package scanner

import (
	"time"
)

// ToolConfig 各 CLI 工具的固定配置
// 包含版本、路径、超时、内存限制、输出格式等元信息
type ToolConfig struct {
	Name           string
	BinaryName     string
	InstallCmd     string
	FixedVersion   string
	DefaultTimeout time.Duration
	MemoryLimitMB  int64
	JSONOutput     bool
	SilentOutput   bool
	// DisableUpdateCheck 工具支持 -duc/-disable-update-check 时置为 true，
	// 执行器会自动注入该参数，避免每次扫描访问 GitHub 检查新版本
	DisableUpdateCheck bool
}

// ToolConfigs 预置所有 CLI 工具的配置
var ToolConfigs = map[string]ToolConfig{
	"nuclei": {
		Name:               "nuclei",
		BinaryName:         "nuclei",
		InstallCmd:         "go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@v3.11.1",
		FixedVersion:       "v3.11.1",
		DefaultTimeout:     10 * time.Minute,
		MemoryLimitMB:      512,
		JSONOutput:         true,
		SilentOutput:       true,
		DisableUpdateCheck: true,
	},
	"httpx": {
		Name:               "httpx",
		BinaryName:         "httpx",
		InstallCmd:         "go install github.com/projectdiscovery/httpx/cmd/httpx@v1.10.0",
		FixedVersion:       "v1.10.0",
		DefaultTimeout:     5 * time.Minute,
		MemoryLimitMB:      256,
		JSONOutput:         true,
		SilentOutput:       true,
		DisableUpdateCheck: true,
	},
	"naabu": {
		Name:               "naabu",
		BinaryName:         "naabu",
		InstallCmd:         "go install github.com/projectdiscovery/naabu/v2/cmd/naabu@v2.6.1",
		FixedVersion:       "v2.6.1",
		DefaultTimeout:     15 * time.Minute,
		MemoryLimitMB:      512,
		JSONOutput:         true,
		SilentOutput:       true,
		DisableUpdateCheck: true,
	},
	"subfinder": {
		Name:               "subfinder",
		BinaryName:         "subfinder",
		InstallCmd:         "go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@v2.15.0",
		FixedVersion:       "v2.15.0",
		DefaultTimeout:     10 * time.Minute,
		MemoryLimitMB:      384,
		JSONOutput:         true,
		SilentOutput:       true,
		DisableUpdateCheck: true,
	},
	"ffuf": {
		Name:           "ffuf",
		BinaryName:     "ffuf",
		InstallCmd:     "go install github.com/ffuf/ffuf/v2@v2.2.1",
		FixedVersion:   "v2.2.1",
		DefaultTimeout: 10 * time.Minute,
		MemoryLimitMB:  384,
		JSONOutput:     true,
		SilentOutput:   false,
	},
	"fingerprintx": {
		Name:           "fingerprintx",
		BinaryName:     "fingerprintx",
		InstallCmd:     "go install github.com/praetorian-inc/fingerprintx/cmd/fingerprintx@v1.1.19",
		FixedVersion:   "v1.1.19",
		DefaultTimeout: 3 * time.Minute,
		MemoryLimitMB:  256,
		JSONOutput:     true,
		SilentOutput:   false,
	},
	"dnsx": {
		Name:               "dnsx",
		BinaryName:         "dnsx",
		InstallCmd:         "go install github.com/projectdiscovery/dnsx/cmd/dnsx@v1.3.0",
		FixedVersion:       "v1.3.0",
		DefaultTimeout:     3 * time.Minute,
		MemoryLimitMB:      128,
		JSONOutput:         true,
		SilentOutput:       true,
		DisableUpdateCheck: true,
	},
}

// GetToolConfig 获取工具配置
func GetToolConfig(name string) (ToolConfig, bool) {
	cfg, ok := ToolConfigs[name]
	return cfg, ok
}

// NewExecutorForTool 按工具名从 ToolConfigs 构造 CmdExecutor
func NewExecutorForTool(name string) *CmdExecutor {
	cfg := ToolConfigs[name]
	return NewCmdExecutor(cfg.BinaryName, cfg.MemoryLimitMB, cfg.DefaultTimeout)
}

// presetArgsForBinary 返回按二进制名匹配的固定注入参数
// 当前仅用于对支持 -duc 的工具注入禁用自动更新检查参数
func presetArgsForBinary(binaryName string) []string {
	for _, cfg := range ToolConfigs {
		if cfg.BinaryName == binaryName && cfg.DisableUpdateCheck {
			return []string{"-duc"}
		}
	}
	return nil
}
