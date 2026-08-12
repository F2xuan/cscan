package worker

import (
	"context"
	"sort"
	"strings"

	"cscan/internal/model"
	"cscan/internal/scanner"
	"cscan/internal/scheduler"
	"cscan/pkg/mapping"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TagMatchInfo struct {
	Fingerprint string   // 匹配的指纹名称
	Tags        []string // 匹配到的标签
	Source      string   // 来源: "custom"(自定义标签映射) 或 "builtin"(内置映射)
}

// generateAutoTags 根据资产的应用信息生成Nuclei标签
// 返回: 标签列表, 匹配信息列表
func (w *Worker) generateAutoTags(assets []*scanner.Asset, pocConfig *scheduler.PocScanConfig) ([]string, []TagMatchInfo) {
	// 添加 panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("Generate auto tags panic recovered: %v, stack: %s", r, string(getStackTrace()))
		}
	}()

	tagSet := make(map[string]bool)
	matchInfoMap := make(map[string]*TagMatchInfo) // 用于去重，key为 fingerprint+source

	for _, asset := range assets {
		for _, app := range asset.App {
			appName := parseAppName(app)
			appNameLower := strings.ToLower(appName)

			// 模式1: 基于自定义标签映射
			if pocConfig.AutoScan && pocConfig.TagMappings != nil {
				for mappedApp, tags := range pocConfig.TagMappings {
					if strings.ToLower(mappedApp) == appNameLower {
						key := appName + "_custom"
						if _, exists := matchInfoMap[key]; !exists {
							matchInfoMap[key] = &TagMatchInfo{
								Fingerprint: appName,
								Tags:        tags,
								Source:      "custom",
							}
						}
						for _, tag := range tags {
							tagSet[tag] = true
						}
						break
					}
				}
			}

			// 模式2: 基于Wappalyzer内置映射
			if pocConfig.AutomaticScan {
				if tags, ok := mapping.WappalyzerNucleiMapping[appNameLower]; ok {
					key := appName + "_builtin"
					if _, exists := matchInfoMap[key]; !exists {
						matchInfoMap[key] = &TagMatchInfo{
							Fingerprint: appName,
							Tags:        tags,
							Source:      "builtin",
						}
					}
					for _, tag := range tags {
						tagSet[tag] = true
					}
				} else if strings.Contains(appNameLower, " ") {
					// 拆分多词 Nmap 产品名，逐词尝试匹配映射
					// 例如 "Elasticsearch Kibana" -> "elasticsearch" + "kibana"
					for _, part := range strings.Fields(appNameLower) {
						if partTags, ok := mapping.WappalyzerNucleiMapping[part]; ok {
							compositeKey := appName + "_" + part + "_builtin"
							if _, exists := matchInfoMap[compositeKey]; !exists {
								matchInfoMap[compositeKey] = &TagMatchInfo{
									Fingerprint: appName,
									Tags:        partTags,
									Source:      "builtin",
								}
							}
							for _, tag := range partTags {
								tagSet[tag] = true
							}
						}
					}
				}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	matchInfos := make([]TagMatchInfo, 0, len(matchInfoMap))
	for _, info := range matchInfoMap {
		matchInfos = append(matchInfos, *info)
	}

	return tags, matchInfos
}

// AssetGroup 资产组，具有相同标签集的资产归为一组
type AssetGroup struct {
	Assets []*scanner.Asset
	Tags   []string
}

// generateAssetTags 为单个资产生成标签（基于自定义标签映射和Wappalyzer映射）
func (w *Worker) generateAssetTags(asset *scanner.Asset, pocConfig *scheduler.PocScanConfig) []string {
	tagSet := make(map[string]bool)

	for _, app := range asset.App {
		appName := parseAppName(app)
		appNameLower := strings.ToLower(appName)

		// 模式1: 基于自定义标签映射
		if pocConfig.AutoScan && pocConfig.TagMappings != nil {
			for mappedApp, tags := range pocConfig.TagMappings {
				if strings.ToLower(mappedApp) == appNameLower {
					for _, tag := range tags {
						tagSet[tag] = true
					}
					break
				}
			}
		}

		// 模式2: 基于Wappalyzer内置映射
		if pocConfig.AutomaticScan {
			if tags, ok := mapping.WappalyzerNucleiMapping[appNameLower]; ok {
				for _, tag := range tags {
					tagSet[tag] = true
				}
			} else if strings.Contains(appNameLower, " ") {
				// 拆分多词 Nmap 产品名，逐词尝试匹配映射
				for _, part := range strings.Fields(appNameLower) {
					if partTags, ok := mapping.WappalyzerNucleiMapping[part]; ok {
						for _, tag := range partTags {
							tagSet[tag] = true
						}
					}
				}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

// groupAssetsByTags 按标签集对资产进行分组
// 具有相同标签集的资产归为同一组，可以共享同一套POC模板
func (w *Worker) groupAssetsByTags(assets []*scanner.Asset, pocConfig *scheduler.PocScanConfig) []*AssetGroup {
	// 使用标签集的签名作为key进行分组
	groups := make(map[string]*AssetGroup)

	for _, asset := range assets {
		tags := w.generateAssetTags(asset, pocConfig)
		if len(tags) == 0 {
			continue
		}

		// 创建标签集的签名（排序后拼接）
		sortedTags := make([]string, len(tags))
		copy(sortedTags, tags)
		sort.Strings(sortedTags)
		sig := strings.Join(sortedTags, ",")

		if _, ok := groups[sig]; !ok {
			groups[sig] = &AssetGroup{
				Assets: make([]*scanner.Asset, 0),
				Tags:   tags,
			}
		}
		groups[sig].Assets = append(groups[sig].Assets, asset)
	}

	result := make([]*AssetGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, group)
	}
	return result
}

// getTemplatesByTags 通过 HTTP 接口从数据库获取符合标签的模板
func (w *Worker) getTemplatesByTags(ctx context.Context, tags []string, severities []string) []string {
	if len(tags) == 0 {
		return nil
	}

	// 通过 HTTP 接口获取模板
	resp, err := w.httpClient.GetTemplates(ctx, &TemplatesReq{
		Tags:       tags,
		Severities: severities,
	})
	if err != nil {
		w.logger.Error("GetTemplates HTTP failed: %v", err)
		return nil
	}

	if !resp.Success {
		w.logger.Error("GetTemplates failed: %s", resp.Msg)
		return nil
	}

	w.logger.Info("GetTemplatesByTags: fetched %d templates for tags %v", resp.Count, tags)
	return resp.Templates
}

// getTemplatesByIds 通过 HTTP 接口根据ID列表获取模板内容
func (w *Worker) getTemplatesByIds(ctx context.Context, nucleiTemplateIds, customPocIds []string) []string {
	if len(nucleiTemplateIds) == 0 && len(customPocIds) == 0 {
		return nil
	}

	// 通过 HTTP 接口获取模板
	resp, err := w.httpClient.GetTemplates(ctx, &TemplatesReq{
		NucleiTemplateIds: nucleiTemplateIds,
		CustomPocIds:      customPocIds,
	})
	if err != nil {
		w.logger.Error("GetTemplates HTTP failed: %v", err)
		return nil
	}

	if !resp.Success {
		w.logger.Error("GetTemplates failed: %s", resp.Msg)
		return nil
	}

	w.logger.Info("GetTemplatesByIds: requested nucleiIds=%d customPocIds=%d, fetched %d templates", len(nucleiTemplateIds), len(customPocIds), resp.Count)
	return resp.Templates
}

// getAllCustomPocs 获取所有自定义POC
func (w *Worker) getAllCustomPocs(ctx context.Context, severities []string) []string {
	// 通过 HTTP 接口获取所有自定义POC
	resp, err := w.httpClient.GetTemplates(ctx, &TemplatesReq{
		Severities:    severities,
		CustomPocOnly: true,
	})
	if err != nil {
		w.logger.Error("GetAllCustomPocs HTTP failed: %v", err)
		return nil
	}

	if !resp.Success {
		w.logger.Error("GetAllCustomPocs failed: %s", resp.Msg)
		return nil
	}

	w.logger.Info("GetAllCustomPocs: fetched %d custom POC templates", resp.Count)
	return resp.Templates
}

// parseAppName 解析应用名称，去除版本号和来源标�?
func parseAppName(app string) string {
	appName := app
	// 先去�?[source] 后缀
	if idx := strings.Index(appName, "["); idx > 0 {
		appName = appName[:idx]
	}
	// 再去除 :version 后缀
	if idx := strings.Index(appName, ":"); idx > 0 {
		appName = appName[:idx]
	}
	return strings.TrimSpace(appName)
}

// loadCustomFingerprints 加载自定义指纹到指纹扫描器
// activeScan: 是否启用主动扫描，如果启用则同时加载主动指纹
func (w *Worker) loadCustomFingerprints(ctx context.Context, fpScanner *scanner.FingerprintScanner, activeScan bool) {
	// 添加 panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("Load custom fingerprints panic recovered: %v, stack: %s", r, string(getStackTrace()))
		}
	}()

	// 通过 HTTP 接口获取被动指纹配置
	var passiveFingerprints []*model.Fingerprint
	passiveFpMap := make(map[string]*model.Fingerprint)

	resp, err := w.httpClient.GetFingerprints(ctx, &FingerprintsReq{
		EnabledOnly: true,
	})
	if err != nil {
		w.logger.Error("GetFingerprints HTTP failed: %v", err)
		// 不直接返回，继续尝试加载主动指纹
	} else if !resp.Success {
		w.logger.Error("GetFingerprints failed: %s", resp.Msg)
		// 不直接返回，继续尝试加载主动指纹
	} else {
		// 转换为model.Fingerprint（被动指纹）
		for _, fp := range resp.Fingerprints {
			mfp := &model.Fingerprint{
				Name:      fp.Name,
				Category:  fp.Category,
				Rule:      fp.Rule,
				Source:    fp.Source,
				Headers:   fp.Headers,
				Cookies:   fp.Cookies,
				HTML:      fp.Html,
				Scripts:   fp.Scripts,
				ScriptSrc: fp.ScriptSrc,
				Meta:      fp.Meta,
				CSS:       fp.Css,
				URL:       fp.Url,
				IsBuiltin: fp.IsBuiltin,
				Enabled:   fp.Enabled,
			}
			// 解析ID
			if fp.Id != "" {
				if oid, err := primitive.ObjectIDFromHex(fp.Id); err == nil {
					mfp.Id = oid
				}
			}
			passiveFingerprints = append(passiveFingerprints, mfp)
			// 存入映射（小写名称作为key，支持不区分大小写匹配）
			passiveFpMap[strings.ToLower(fp.Name)] = mfp
		}
	}

	// 如果启用主动扫描，加载主动指纹
	var activeFingerprints []*model.Fingerprint
	if activeScan {
		activeResp, err := w.httpClient.GetActiveFingerprints(ctx, true)
		if err != nil {
			w.logger.Warn("GetActiveFingerprints HTTP failed: %v", err)
		} else if activeResp.Success && len(activeResp.Fingerprints) > 0 {
			for _, afp := range activeResp.Fingerprints {
				// 创建主动指纹对象，直接使用API返回的规则（已包含关联的被动指纹规则）
				mfp := &model.Fingerprint{
					Name:        afp.Name,
					ActivePaths: afp.Paths,
					Enabled:     afp.Enabled,
					Type:        model.FingerprintTypeActive,
					// 使用API返回的匹配规则（服务端已关联被动指纹）
					Rule:      afp.Rule,
					Headers:   afp.Headers,
					Cookies:   afp.Cookies,
					HTML:      afp.Html,
					Scripts:   afp.Scripts,
					ScriptSrc: afp.ScriptSrc,
					Meta:      afp.Meta,
					CSS:       afp.Css,
					URL:       afp.Url,
				}

				// 如果API没有返回规则，尝试从本地被动指纹映射获取
				if mfp.Rule == "" && len(mfp.HTML) == 0 && len(mfp.Headers) == 0 {
					if passiveFp := passiveFpMap[strings.ToLower(afp.Name)]; passiveFp != nil {
						mfp.Rule = passiveFp.Rule
						mfp.Headers = passiveFp.Headers
						mfp.Cookies = passiveFp.Cookies
						mfp.HTML = passiveFp.HTML
						mfp.Scripts = passiveFp.Scripts
						mfp.ScriptSrc = passiveFp.ScriptSrc
						mfp.Meta = passiveFp.Meta
						mfp.CSS = passiveFp.CSS
						mfp.URL = passiveFp.URL
						mfp.Category = passiveFp.Category
						w.logger.Debug("Active fingerprint '%s' linked to local passive fingerprint with rule: %s", afp.Name, passiveFp.Rule)
					} else {
						w.logger.Warn("Active fingerprint '%s' has no matching rule", afp.Name)
					}
				} else if mfp.Rule != "" {
					w.logger.Debug("Active fingerprint '%s' loaded with rule from API: %s", afp.Name, mfp.Rule)
				}

				// 解析ID
				if afp.Id != "" {
					if oid, err := primitive.ObjectIDFromHex(afp.Id); err == nil {
						mfp.Id = oid
					}
				}
				activeFingerprints = append(activeFingerprints, mfp)
			}
			w.logger.Info("Loaded %d active fingerprints", len(activeFingerprints))
		}
	}

	// 创建自定义指纹引擎并设置到扫描器
	// 即使被动指纹为空，只要有主动指纹也要创建引擎
	if len(passiveFingerprints) > 0 || len(activeFingerprints) > 0 {
		var customEngine *scanner.CustomFingerprintEngine
		if len(activeFingerprints) > 0 {
			customEngine = scanner.NewCustomFingerprintEngineWithActive(passiveFingerprints, activeFingerprints)
		} else {
			customEngine = scanner.NewCustomFingerprintEngine(passiveFingerprints)
		}
		fpScanner.SetCustomFingerprintEngine(customEngine)
		w.logger.Info("Loaded %d passive fingerprints, %d active fingerprints into scanner", len(passiveFingerprints), len(activeFingerprints))
	} else {
		w.logger.Info("No fingerprints found")
	}
}

// filterSkippedHostsAssets 过滤掉因端口阈值超限被跳过的主机的资产
