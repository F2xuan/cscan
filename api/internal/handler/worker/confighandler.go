package worker

import (
	"encoding/json"
	"net/http"
	"strings"

	"cscan/api/internal/svc"
	"cscan/internal/model"
	"cscan/pkg/response"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.mongodb.org/mongo-driver/bson"
)

// ==================== Templates Config Types ====================

// WorkerTemplatesReq 模板获取请求
type WorkerTemplatesReq struct {
	// 按标签获取
	Tags       []string `json:"tags,omitempty"`
	Severities []string `json:"severities,omitempty"`
	// 按ID获取
	NucleiTemplateIds []string `json:"nucleiTemplateIds,omitempty"`
	CustomPocIds      []string `json:"customPocIds,omitempty"`
	// 获取所有自定义POC
	CustomPocOnly bool `json:"customPocOnly,omitempty"`
}

// WorkerTemplatesResp 模板获取响应
type WorkerTemplatesResp struct {
	Code      int      `json:"code"`
	Msg       string   `json:"msg"`
	Success   bool     `json:"success"`
	Templates []string `json:"templates"`
	Count     int32    `json:"count"`
}

// ==================== Fingerprints Config Types ====================

// WorkerFingerprintsReq 指纹获取请求
type WorkerFingerprintsReq struct {
	EnabledOnly bool `json:"enabledOnly"`
}

// WorkerFingerprintDocument 指纹文档
type WorkerFingerprintDocument struct {
	Id        string            `json:"id"`
	Name      string            `json:"name"`
	Category  string            `json:"category"`
	Rule      string            `json:"rule"`
	Source    string            `json:"source"`
	Headers   map[string]string `json:"headers"`
	Cookies   map[string]string `json:"cookies"`
	Html      []string          `json:"html"`
	Scripts   []string          `json:"scripts"`
	ScriptSrc []string          `json:"scriptSrc"`
	Meta      map[string]string `json:"meta"`
	Css       []string          `json:"css"`
	Url       []string          `json:"url"`
	IsBuiltin bool              `json:"isBuiltin"`
	Enabled   bool              `json:"enabled"`
}

// WorkerFingerprintsResp 指纹获取响应
type WorkerFingerprintsResp struct {
	Code         int                         `json:"code"`
	Msg          string                      `json:"msg"`
	Success      bool                        `json:"success"`
	Fingerprints []WorkerFingerprintDocument `json:"fingerprints"`
	Count        int32                       `json:"count"`
}

// ==================== Subfinder Config Types ====================

// WorkerSubfinderReq Subfinder配置获取请求
type WorkerSubfinderReq struct {
	WorkspaceId string `json:"workspaceId"`
}

// WorkerSubfinderProvider Subfinder数据源
type WorkerSubfinderProvider struct {
	Id          string   `json:"id"`
	Provider    string   `json:"provider"`
	Keys        []string `json:"keys"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
}

// WorkerSubfinderResp Subfinder配置获取响应
type WorkerSubfinderResp struct {
	Code      int                       `json:"code"`
	Msg       string                    `json:"msg"`
	Success   bool                      `json:"success"`
	Providers []WorkerSubfinderProvider `json:"providers"`
	Count     int32                     `json:"count"`
}

// ==================== HttpService Config Types ====================

// WorkerHttpServiceReq HTTP服务映射获取请求
type WorkerHttpServiceReq struct {
	EnabledOnly bool `json:"enabledOnly"`
}

// WorkerHttpServiceMapping HTTP服务映射
type WorkerHttpServiceMapping struct {
	Id          string `json:"id"`
	ServiceName string `json:"serviceName"`
	IsHttp      bool   `json:"isHttp"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// WorkerHttpServiceResp HTTP服务映射获取响应
type WorkerHttpServiceResp struct {
	Code     int                        `json:"code"`
	Msg      string                     `json:"msg"`
	Success  bool                       `json:"success"`
	Mappings []WorkerHttpServiceMapping `json:"mappings"`
	Count    int32                      `json:"count"`
}

// WorkerHttpServiceConfig HTTP服务端口配置
type WorkerHttpServiceConfig struct {
	HttpPorts   []int  `json:"httpPorts"`
	HttpsPorts  []int  `json:"httpsPorts"`
	Description string `json:"description"`
}

// WorkerHttpServiceSettingsResp HTTP服务设置完整响应
type WorkerHttpServiceSettingsResp struct {
	Code     int                        `json:"code"`
	Msg      string                     `json:"msg"`
	Success  bool                       `json:"success"`
	Config   WorkerHttpServiceConfig    `json:"config"`
	Mappings []WorkerHttpServiceMapping `json:"mappings"`
}

// ==================== Active Fingerprints Config Types ====================

// WorkerActiveFingerprintsReq 主动指纹获取请求
type WorkerActiveFingerprintsReq struct {
	EnabledOnly bool `json:"enabledOnly"`
}

// WorkerActiveFingerprintDocument 主动指纹文档
type WorkerActiveFingerprintDocument struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`  // 应用名称（用于关联被动指纹）
	Paths       []string `json:"paths"` // 主动探测路径列表
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	// 关联的被动指纹规则（用于匹配响应）
	Rule      string            `json:"rule,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Cookies   map[string]string `json:"cookies,omitempty"`
	Html      []string          `json:"html,omitempty"`
	Scripts   []string          `json:"scripts,omitempty"`
	ScriptSrc []string          `json:"scriptSrc,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
	Css       []string          `json:"css,omitempty"`
	Url       []string          `json:"url,omitempty"`
}

// WorkerActiveFingerprintsResp 主动指纹获取响应
type WorkerActiveFingerprintsResp struct {
	Code         int                               `json:"code"`
	Msg          string                            `json:"msg"`
	Success      bool                              `json:"success"`
	Fingerprints []WorkerActiveFingerprintDocument `json:"fingerprints"`
	Count        int32                             `json:"count"`
}

// ==================== Templates Handler ====================

// WorkerConfigTemplatesHandler 模板配置获取接口
// POST /api/v1/worker/config/templates
func WorkerConfigTemplatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerTemplatesReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerTemplatesResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		ctx := r.Context()
		var templates []string

		// 优先按ID获取
		if len(req.NucleiTemplateIds) > 0 || len(req.CustomPocIds) > 0 {
			if len(req.NucleiTemplateIds) > 0 {
				nucleiTemplates, err := svcCtx.NucleiTemplateModel.FindByIds(ctx, req.NucleiTemplateIds)
				if err != nil {
					logx.Errorf("[WorkerConfigTemplates] FindByIds nuclei error: %v", err)
					response.Error(w, err)
					return
				}
				for _, t := range nucleiTemplates {
					if t.Content != "" && t.Enabled {
						templates = append(templates, t.Content)
					}
				}
			}
			if len(req.CustomPocIds) > 0 {
				customPocs, err := svcCtx.CustomPocModel.FindByIds(ctx, req.CustomPocIds)
				if err != nil {
					logx.Errorf("[WorkerConfigTemplates] FindByIds custom poc error: %v", err)
					response.Error(w, err)
					return
				}
				for _, p := range customPocs {
					if p.Content != "" && p.Enabled {
						templates = append(templates, p.Content)
					}
				}
			}
		} else if req.CustomPocOnly {
			// 获取所有自定义POC（按严重级别过滤）
			filter := bson.M{"enabled": true}
			if len(req.Severities) > 0 {
				filter["severity"] = bson.M{"$in": req.Severities}
			}
			customPocs, err := svcCtx.CustomPocModel.FindWithFilter(ctx, filter, 0, 0)
			if err != nil {
				logx.Errorf("[WorkerConfigTemplates] FindWithFilter custom poc error: %v", err)
				response.Error(w, err)
				return
			}
			for _, p := range customPocs {
				if p.Content != "" {
					templates = append(templates, p.Content)
				}
			}
		} else {
			// 按标签获取 Nuclei 模板
			filter := bson.M{"enabled": true}
			if len(req.Tags) > 0 {
				filter["tags"] = bson.M{"$in": req.Tags}
			}
			if len(req.Severities) > 0 {
				filter["severity"] = bson.M{"$in": req.Severities}
			}
			nucleiTemplates, err := svcCtx.NucleiTemplateModel.FindEnabledByFilter(ctx, filter)
			if err != nil {
				logx.Errorf("[WorkerConfigTemplates] FindEnabledByFilter nuclei error: %v", err)
				response.Error(w, err)
				return
			}
			for _, t := range nucleiTemplates {
				if t.Content != "" {
					templates = append(templates, t.Content)
				}
			}
			// 按标签获取自定义POC
			if len(req.Tags) > 0 {
				customFilter := bson.M{
					"enabled": true,
					"tags":    bson.M{"$in": req.Tags},
				}
				if len(req.Severities) > 0 {
					customFilter["severity"] = bson.M{"$in": req.Severities}
				}
				customPocs, err := svcCtx.CustomPocModel.FindWithFilter(ctx, customFilter, 0, 0)
				if err != nil {
					logx.Errorf("[WorkerConfigTemplates] FindWithFilter custom poc error: %v", err)
				} else {
					for _, p := range customPocs {
						if p.Content != "" {
							templates = append(templates, p.Content)
						}
					}
				}
			}
		}

		logx.Infof("[WorkerConfigTemplates] returned %d templates", len(templates))
		httpx.OkJson(w, &WorkerTemplatesResp{
			Code:      0,
			Msg:       "success",
			Success:   true,
			Templates: templates,
			Count:     int32(len(templates)),
		})
	}
}

// ==================== Fingerprints Handler ====================

// WorkerConfigFingerprintsHandler 指纹配置获取接口
// POST /api/v1/worker/config/fingerprints
func WorkerConfigFingerprintsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerFingerprintsReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerFingerprintsResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		ctx := r.Context()
		filter := bson.M{}
		if req.EnabledOnly {
			filter["enabled"] = true
		}

		fps, err := svcCtx.FingerprintModel.Find(ctx, filter, 0, 0)
		if err != nil {
			logx.Errorf("[WorkerConfigFingerprints] Find error: %v", err)
			response.Error(w, err)
			return
		}

		fingerprints := make([]WorkerFingerprintDocument, 0, len(fps))
		for _, fp := range fps {
			fingerprints = append(fingerprints, WorkerFingerprintDocument{
				Id:        fp.Id.Hex(),
				Name:      fp.Name,
				Category:  fp.Category,
				Rule:      fp.Rule,
				Source:    fp.Source,
				Headers:   fp.Headers,
				Cookies:   fp.Cookies,
				Html:      fp.HTML,
				Scripts:   fp.Scripts,
				ScriptSrc: fp.ScriptSrc,
				Meta:      fp.Meta,
				Css:       fp.CSS,
				Url:       fp.URL,
				IsBuiltin: fp.IsBuiltin,
				Enabled:   fp.Enabled,
			})
		}

		httpx.OkJson(w, &WorkerFingerprintsResp{
			Code:         0,
			Msg:          "success",
			Success:      true,
			Fingerprints: fingerprints,
			Count:        int32(len(fingerprints)),
		})
	}
}

// ==================== Subfinder Handler ====================

// WorkerConfigSubfinderHandler Subfinder配置获取接口
// POST /api/v1/worker/config/subfinder
func WorkerConfigSubfinderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerSubfinderReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerSubfinderResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		providers, err := svcCtx.SubfinderProviderModel.FindEnabled(r.Context())
		if err != nil {
			logx.Errorf("[WorkerConfigSubfinder] FindEnabled error: %v", err)
			response.Error(w, err)
			return
		}

		result := make([]WorkerSubfinderProvider, 0, len(providers))
		for _, p := range providers {
			result = append(result, WorkerSubfinderProvider{
				Id:          p.Id.Hex(),
				Provider:    p.Provider,
				Keys:        p.Keys,
				Status:      p.Status,
				Description: p.Description,
			})
		}

		httpx.OkJson(w, &WorkerSubfinderResp{
			Code:      0,
			Msg:       "success",
			Success:   true,
			Providers: result,
			Count:     int32(len(result)),
		})
	}
}

// ==================== HttpService Handler ====================

// WorkerConfigHttpServiceHandler HTTP服务映射获取接口
// POST /api/v1/worker/config/httpservice
func WorkerConfigHttpServiceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerHttpServiceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerHttpServiceResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		ctx := r.Context()
		var mappingDocs []model.HttpServiceMapping
		var err error

		if req.EnabledOnly {
			mappingDocs, err = svcCtx.HttpServiceMappingModel.FindEnabled(ctx)
		} else {
			mappingDocs, err = svcCtx.HttpServiceMappingModel.FindAll(ctx)
		}
		if err != nil {
			logx.Errorf("[WorkerConfigHttpService] Find error: %v", err)
			response.Error(w, err)
			return
		}

		mappings := make([]WorkerHttpServiceMapping, 0, len(mappingDocs))
		for _, m := range mappingDocs {
			mappings = append(mappings, WorkerHttpServiceMapping{
				Id:          m.Id.Hex(),
				ServiceName: m.ServiceName,
				IsHttp:      m.IsHttp,
				Description: m.Description,
				Enabled:     m.Enabled,
			})
		}

		httpx.OkJson(w, &WorkerHttpServiceResp{
			Code:     0,
			Msg:      "success",
			Success:  true,
			Mappings: mappings,
			Count:    int32(len(mappings)),
		})
	}
}

// WorkerConfigHttpServiceSettingsHandler HTTP服务设置获取接口（包含端口配置和服务映射）
// POST /api/v1/worker/config/httpservice/settings
func WorkerConfigHttpServiceSettingsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 获取端口配置
		config, err := svcCtx.HttpServiceModel.GetConfig(ctx)
		if err != nil {
			logx.Errorf("[WorkerConfigHttpServiceSettings] GetConfig error: %v", err)
			// 使用默认配置
			config = &model.HttpServiceConfig{
				HttpPorts:  []int{80, 8080, 8000, 8888, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 8088, 8089, 8090, 9000, 9001, 9080, 3000, 3001, 5000, 5001, 8008, 8009, 8181, 8200, 8300, 8400, 8500, 8600, 8800, 8880, 8983, 9090, 9091, 9200, 9300, 10000},
				HttpsPorts: []int{443, 8443, 9443, 4443, 10443},
			}
		}

		// 获取服务映射
		mappingDocs, err := svcCtx.HttpServiceModel.GetEnabledMappings(ctx)
		if err != nil {
			logx.Errorf("[WorkerConfigHttpServiceSettings] GetEnabledMappings error: %v", err)
			mappingDocs = []model.HttpServiceMapping{}
		}

		// 转换映射数据
		mappings := make([]WorkerHttpServiceMapping, 0, len(mappingDocs))
		for _, m := range mappingDocs {
			mappings = append(mappings, WorkerHttpServiceMapping{
				Id:          m.Id.Hex(),
				ServiceName: m.ServiceName,
				IsHttp:      m.IsHttp,
				Description: m.Description,
				Enabled:     m.Enabled,
			})
		}

		httpx.OkJson(w, &WorkerHttpServiceSettingsResp{
			Code:    0,
			Msg:     "success",
			Success: true,
			Config: WorkerHttpServiceConfig{
				HttpPorts:   config.HttpPorts,
				HttpsPorts:  config.HttpsPorts,
				Description: config.Description,
			},
			Mappings: mappings,
		})
	}
}

// ==================== Active Fingerprints Handler ====================

// WorkerConfigActiveFingerprintsHandler 主动指纹配置获取接口
// POST /api/v1/worker/config/activefingerprints
func WorkerConfigActiveFingerprintsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerActiveFingerprintsReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerActiveFingerprintsResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		ctx := r.Context()
		var docs []WorkerActiveFingerprintDocument

		// 获取主动指纹
		var activeFingerprints []model.ActiveFingerprint
		var err error
		if req.EnabledOnly {
			activeFingerprints, err = svcCtx.ActiveFingerprintModel.FindEnabled(ctx)
		} else {
			activeFingerprints, err = svcCtx.ActiveFingerprintModel.FindAll(ctx)
		}
		if err != nil {
			logx.Errorf("[WorkerConfigActiveFingerprints] Find error: %v", err)
			response.Error(w, err)
			return
		}

		// 收集所有主动指纹的名称，用于批量查询关联的被动指纹
		names := make([]string, 0, len(activeFingerprints))
		for _, fp := range activeFingerprints {
			names = append(names, fp.Name)
		}

		// 批量获取关联的被动指纹规则
		passiveFpMap := make(map[string]*model.Fingerprint)
		if len(names) > 0 && svcCtx.FingerprintModel != nil {
			passiveFingerprints, err := svcCtx.FingerprintModel.FindByNames(ctx, names)
			if err != nil {
				logx.Infof("[WorkerConfigActiveFingerprints] FindByNames error: %v", err)
				// 不返回错误，继续处理（主动指纹仍然可以返回，只是没有匹配规则）
			} else {
				for _, pf := range passiveFingerprints {
					// 使用小写名称作为key，支持不区分大小写匹配
					passiveFpMap[strings.ToLower(pf.Name)] = pf
				}
				logx.Debugf("[WorkerConfigActiveFingerprints] Found %d passive fingerprints for %d active fingerprints", len(passiveFingerprints), len(names))
			}
		}

		// 构建返回数据，包含关联的被动指纹规则
		for _, fp := range activeFingerprints {
			doc := WorkerActiveFingerprintDocument{
				Id:          fp.Id.Hex(),
				Name:        fp.Name,
				Paths:       fp.Paths,
				Description: fp.Description,
				Enabled:     fp.Enabled,
			}

			// 查找关联的被动指纹，复制其匹配规则（使用小写名称匹配）
			if passiveFp, ok := passiveFpMap[strings.ToLower(fp.Name)]; ok {
				doc.Rule = passiveFp.Rule
				doc.Headers = passiveFp.Headers
				doc.Cookies = passiveFp.Cookies
				doc.Html = passiveFp.HTML
				doc.Scripts = passiveFp.Scripts
				doc.ScriptSrc = passiveFp.ScriptSrc
				doc.Meta = passiveFp.Meta
				doc.Css = passiveFp.CSS
				doc.Url = passiveFp.URL
			}

			docs = append(docs, doc)
		}

		httpx.OkJson(w, &WorkerActiveFingerprintsResp{
			Code:         0,
			Msg:          "success",
			Success:      true,
			Fingerprints: docs,
			Count:        int32(len(docs)),
		})
	}
}

// ==================== POC Config Types ====================

// WorkerPocReq POC获取请求
type WorkerPocReq struct {
	PocId   string `json:"pocId"`
	PocType string `json:"pocType"` // nuclei, custom
}

// WorkerPocResp POC获取响应
type WorkerPocResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
	Content string `json:"content"`
	PocId   string `json:"pocId"`
	PocType string `json:"pocType"`
}

// ==================== POC Handler ====================

// WorkerConfigPocHandler POC配置获取接口
// POST /api/v1/worker/config/poc
func WorkerConfigPocHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerPocReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerPocResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if req.PocId == "" {
			httpx.OkJson(w, &WorkerPocResp{Code: 400, Msg: "pocId不能为空"})
			return
		}

		ctx := r.Context()

		if req.PocType == "custom" {
			poc, err := svcCtx.CustomPocModel.FindById(ctx, req.PocId)
			if err != nil {
				logx.Errorf("[WorkerConfigPoc] FindById custom poc error: %v", err)
				response.Error(w, err)
				return
			}
			if poc == nil {
				httpx.OkJson(w, &WorkerPocResp{Code: 404, Msg: "自定义POC不存在", Success: false})
				return
			}
			httpx.OkJson(w, &WorkerPocResp{
				Code:    0,
				Msg:     "success",
				Success: true,
				Content: poc.Content,
				PocId:   poc.Id.Hex(),
				PocType: "custom",
			})
			return
		}

		// 默认查询 Nuclei 模板：先按 template_id (FindByIds)，再按 template_id 字段
		templates, err := svcCtx.NucleiTemplateModel.FindByIds(ctx, []string{req.PocId})
		if err == nil && len(templates) > 0 {
			t := templates[0]
			httpx.OkJson(w, &WorkerPocResp{
				Code:    0,
				Msg:     "success",
				Success: true,
				Content: t.Content,
				PocId:   t.Id.Hex(),
				PocType: "nuclei",
			})
			return
		}

		// 按 template_id 字段查询
		template, err := svcCtx.NucleiTemplateModel.FindByTemplateId(ctx, req.PocId)
		if err != nil {
			logx.Errorf("[WorkerConfigPoc] FindByTemplateId error: %v", err)
			response.Error(w, err)
			return
		}
		if template == nil {
			httpx.OkJson(w, &WorkerPocResp{Code: 404, Msg: "Nuclei模板不存在", Success: false})
			return
		}

		httpx.OkJson(w, &WorkerPocResp{
			Code:    0,
			Msg:     "success",
			Success: true,
			Content: template.Content,
			PocId:   template.Id.Hex(),
			PocType: "nuclei",
		})
	}
}

// ==================== DirScan Dict Config Types ====================

// WorkerDirScanDictReq 目录扫描字典获取请求
type WorkerDirScanDictReq struct {
	DictIds []string `json:"dictIds"` // 字典ID列表
}

// WorkerDirScanDictItem 目录扫描字典项
type WorkerDirScanDictItem struct {
	Id    string   `json:"id"`
	Name  string   `json:"name"`
	Paths []string `json:"paths"` // 解析后的路径列表
}

// WorkerDirScanDictResp 目录扫描字典获取响应
type WorkerDirScanDictResp struct {
	Code  int                     `json:"code"`
	Msg   string                  `json:"msg"`
	Dicts []WorkerDirScanDictItem `json:"dicts"`
	Count int                     `json:"count"`
}

// ==================== DirScan Dict Handler ====================

// WorkerConfigDirScanDictHandler 目录扫描字典配置获取接口
// POST /api/v1/worker/config/dirscandict
func WorkerConfigDirScanDictHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerDirScanDictReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerDirScanDictResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if len(req.DictIds) == 0 {
			httpx.OkJson(w, &WorkerDirScanDictResp{Code: 400, Msg: "dictIds不能为空"})
			return
		}

		ctx := r.Context()
		dictModel := model.NewDirScanDictModel(svcCtx.MongoDB)

		// 获取字典
		dicts, err := dictModel.FindByIds(ctx, req.DictIds)
		if err != nil {
			logx.Errorf("[WorkerConfigDirScanDict] FindByIds error: %v", err)
			httpx.OkJson(w, &WorkerDirScanDictResp{Code: 500, Msg: "获取字典失败"})
			return
		}

		// 转换数据，解析路径
		items := make([]WorkerDirScanDictItem, 0, len(dicts))
		for _, d := range dicts {
			paths := parseDictPaths(d.Content)
			items = append(items, WorkerDirScanDictItem{
				Id:    d.Id.Hex(),
				Name:  d.Name,
				Paths: paths,
			})
		}

		httpx.OkJson(w, &WorkerDirScanDictResp{
			Code:  0,
			Msg:   "success",
			Dicts: items,
			Count: len(items),
		})
	}
}

// parseDictPaths 解析字典内容，返回路径列表
func parseDictPaths(content string) []string {
	var paths []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		paths = append(paths, line)
	}
	return paths
}

// ==================== Subdomain Dict Config Types ====================

// WorkerSubdomainDictReq 子域名字典获取请求
type WorkerSubdomainDictReq struct {
	DictIds []string `json:"dictIds"` // 字典ID列表
}

// WorkerSubdomainDictItem 子域名字典项
type WorkerSubdomainDictItem struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"` // 字典内容（每行一个前缀）
}

// WorkerSubdomainDictResp 子域名字典获取响应
type WorkerSubdomainDictResp struct {
	Code  int                       `json:"code"`
	Msg   string                    `json:"msg"`
	Dicts []WorkerSubdomainDictItem `json:"dicts"`
	Count int                       `json:"count"`
}

// ==================== Subdomain Dict Handler ====================

// WorkerConfigSubdomainDictHandler 子域名字典配置获取接口
// POST /api/v1/worker/config/subdomaindict
func WorkerConfigSubdomainDictHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerSubdomainDictReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerSubdomainDictResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		if len(req.DictIds) == 0 {
			httpx.OkJson(w, &WorkerSubdomainDictResp{Code: 400, Msg: "dictIds不能为空"})
			return
		}

		ctx := r.Context()
		dictModel := model.NewSubdomainDictModel(svcCtx.MongoDB)

		// 获取字典
		dicts, err := dictModel.FindByIds(ctx, req.DictIds)
		if err != nil {
			logx.Errorf("[WorkerConfigSubdomainDict] FindByIds error: %v", err)
			httpx.OkJson(w, &WorkerSubdomainDictResp{Code: 500, Msg: "获取字典失败"})
			return
		}

		// 转换数据
		items := make([]WorkerSubdomainDictItem, 0, len(dicts))
		for _, d := range dicts {
			items = append(items, WorkerSubdomainDictItem{
				Id:      d.Id.Hex(),
				Name:    d.Name,
				Content: d.Content,
			})
		}

		httpx.OkJson(w, &WorkerSubdomainDictResp{
			Code:  0,
			Msg:   "success",
			Dicts: items,
			Count: len(items),
		})
	}
}

// ==================== Weakpass Dict Config Types ====================

// WorkerWeakpassDictReq 弱口令字典获取请求
type WorkerWeakpassDictReq struct {
	DictIds  []string `json:"dictIds"`  // 字典ID列表
	Services []string `json:"services"` // 目标服务列表（用于过滤）
}

// WorkerWeakpassDictItem 弱口令字典项
type WorkerWeakpassDictItem struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Service   string `json:"service"` // 服务类型
	Content   string `json:"content"` // 字典内容（用户名:密码 格式）
	WordCount int    `json:"wordCount"`
}

// WorkerWeakpassDictResp 弱口令字典获取响应
type WorkerWeakpassDictResp struct {
	Code  int                      `json:"code"`
	Msg   string                   `json:"msg"`
	Dicts []WorkerWeakpassDictItem `json:"dicts"`
	Count int                      `json:"count"`
}

// ==================== Weakpass Dict Handler ====================

// WorkerConfigWeakpassDictHandler 弱口令字典配置获取接口
// POST /api/v1/worker/config/weakpassdict
func WorkerConfigWeakpassDictHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req WorkerWeakpassDictReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.OkJson(w, &WorkerWeakpassDictResp{Code: 400, Msg: "参数解析失败"})
			return
		}

		ctx := r.Context()
		dictModel := model.NewWeakpassDictModel(svcCtx.MongoDB)

		var dicts []model.WeakpassDict
		var err error

		if len(req.DictIds) > 0 {
			// 优先按字典ID获取
			dicts, err = dictModel.FindByIds(ctx, req.DictIds)
		} else if len(req.Services) > 0 {
			// 根据服务类型获取对应的字典
			// 如果指定了服务，只返回匹配服务的字典和通用字典
			var allDicts []model.WeakpassDict
			allDicts, err = dictModel.FindEnabled(ctx, "")
			if err == nil {
				// 过滤出匹配服务的字典
				serviceSet := make(map[string]bool)
				for _, svc := range req.Services {
					serviceSet[svc] = true
				}
				for _, d := range allDicts {
					if d.Service == "common" || serviceSet[d.Service] {
						dicts = append(dicts, d)
					}
				}
			}
		} else {
			// 没有指定字典ID和服务，返回所有启用的字典
			dicts, err = dictModel.FindEnabled(ctx, "")
		}

		if err != nil {
			logx.Errorf("[WorkerConfigWeakpassDict] Find error: %v", err)
			httpx.OkJson(w, &WorkerWeakpassDictResp{Code: 500, Msg: "获取字典失败"})
			return
		}

		// 转换数据
		items := make([]WorkerWeakpassDictItem, 0, len(dicts))
		for _, d := range dicts {
			items = append(items, WorkerWeakpassDictItem{
				Id:        d.Id.Hex(),
				Name:      d.Name,
				Service:   d.Service,
				Content:   d.Content,
				WordCount: d.WordCount,
			})
		}

		httpx.OkJson(w, &WorkerWeakpassDictResp{
			Code:  0,
			Msg:   "success",
			Dicts: items,
			Count: len(items),
		})
	}
}

// ==================== JSFinder Config Types ====================

// WorkerJSFinderConfigResp JSFinder 配置响应（供 Worker 拉取最新 4 份清单）
type WorkerJSFinderConfigResp struct {
	Code                 int      `json:"code"`
	Msg                  string   `json:"msg"`
	HighRiskRoutes       []string `json:"highRiskRoutes"`
	AuthRequiredKeywords []string `json:"authRequiredKeywords"`
	SensitiveKeywords    []string `json:"sensitiveKeywords"`
	DomainBlacklist      []string `json:"domainBlacklist"`
}

// WorkerConfigJSFinderHandler JSFinder 配置获取接口
// POST /api/v1/worker/config/jsfinder
func WorkerConfigJSFinderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		m := model.NewJSFinderConfigModel(svcCtx.MongoDB)
		doc, err := m.Get(ctx)
		if err != nil {
			logx.Errorf("[WorkerConfigJSFinder] Get error: %v", err)
			httpx.OkJson(w, &WorkerJSFinderConfigResp{Code: 500, Msg: "获取JSFinder配置失败"})
			return
		}

		httpx.OkJson(w, &WorkerJSFinderConfigResp{
			Code:                 0,
			Msg:                  "success",
			HighRiskRoutes:       doc.HighRiskRoutes,
			AuthRequiredKeywords: doc.AuthRequiredKeywords,
			SensitiveKeywords:    doc.SensitiveKeywords,
			DomainBlacklist:      doc.DomainBlacklist,
		})
	}
}
