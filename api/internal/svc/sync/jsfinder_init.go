package sync

import (
	"context"
	"encoding/json"

	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// InitJSFinderConfig 初始化 JSFinder 全局配置（不存在则写入内置默认值）
func InitJSFinderConfig(m *model.JSFinderConfigModel) {
	if m == nil {
		return
	}
	ctx := context.Background()
	if err := m.EnsureDefault(ctx); err != nil {
		logx.Errorf("[JSFinderInit] EnsureDefault error: %v", err)
		return
	}
	// 合并补充黑名单条目（幂等：仅追加新增条目，不覆盖用户自定义）
	if err := m.MergeMissingDomainBlacklist(ctx); err != nil {
		logx.Errorf("[JSFinderInit] MergeMissingDomainBlacklist error: %v", err)
	} else {
		logx.Info("[JSFinderInit] JSFinder config ensured (domain blacklist merged)")
	}
}

// MigrateBuiltinTemplatesAddJSFinder 为已存在的内置模板补全 jsfinder 字段（幂等）
// 标准扫描默认开启，其他模板默认关闭，保留原有其他配置
func MigrateBuiltinTemplatesAddJSFinder(templateModel *model.ScanTemplateModel) {
	if templateModel == nil {
		return
	}
	ctx := context.Background()

	builtins, err := templateModel.FindBuiltinTemplates(ctx)
	if err != nil {
		logx.Errorf("[JSFinderMigrate] FindBuiltinTemplates error: %v", err)
		return
	}

	for i := range builtins {
		t := &builtins[i]
		if t.Config == "" {
			continue
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(t.Config), &cfg); err != nil {
			logx.Errorf("[JSFinderMigrate] Unmarshal config %s failed: %v", t.Name, err)
			continue
		}

		if _, exists := cfg["jsfinder"]; exists {
			continue
		}

		enable := t.Category == "standard"
		cfg["jsfinder"] = map[string]interface{}{
			"enable":            enable,
			"threads":           10,
			"timeout":           10,
			"enableSourcemap":   enable,
			"enableUnauthCheck": enable,
		}

		newConfig, err := json.Marshal(cfg)
		if err != nil {
			logx.Errorf("[JSFinderMigrate] Marshal config %s failed: %v", t.Name, err)
			continue
		}

		if err := templateModel.UpdateOne(ctx, bson.M{"_id": t.Id}, bson.M{"config": string(newConfig)}); err != nil {
			logx.Errorf("[JSFinderMigrate] Update template %s failed: %v", t.Name, err)
			continue
		}
		logx.Infof("[JSFinderMigrate] Patched template %s with jsfinder.enable=%v", t.Name, enable)
	}
}
