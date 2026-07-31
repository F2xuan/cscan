package swagger

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// RouteInfo 描述一个 REST 路由的最小元信息（足够生成 OpenAPI 3.0 文档）。
type RouteInfo struct {
	Method   string   `json:"-"`
	Path     string   `json:"-"`
	Group    string   `json:"-"` // 业务分组，如 "user"、"asset"
	Summary  string   `json:"-"`
	AuthTier AuthTier `json:"-"` // 鉴权层级：public/worker/auth/admin/console/terminal/container
}

// collectedRoutes 用于在路由注册处累积所有路由信息
var (
	collectedMu     sync.Mutex
	collectedRoutes []RouteInfo
)

// Collect 在路由注册阶段记录一个路由（默认 TierAuth 兼容旧调用方）。
func Collect(method, path string) {
	CollectEx(method, path, TierAuth)
}

// CollectEx 是 Collect 的扩展版本，要求调用方显式传入鉴权层级。
func CollectEx(method, path string, tier AuthTier) {
	collectedMu.Lock()
	defer collectedMu.Unlock()
	collectedRoutes = append(collectedRoutes, RouteInfo{
		Method:   method,
		Path:     path,
		Group:    groupOf(path),
		AuthTier: tier,
	})
}

func groupOf(path string) string {
	const prefix = "/api/v1/"
	if !strings.HasPrefix(path, prefix) {
		return "public"
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, "/")
	if idx < 0 {
		return rest
	}
	return rest[:idx]
}

// 标签中文描述表（按 metadata 中 Tag 去重后写入 OpenAPI tags[].description）
var tagDescriptions = map[string]string{
	"系统":     "系统健康检查与基础信息",
	"用户认证":   "用户登录与公共配置读取",
	"用户管理":   "用户账号、个人中心、个人 API Token 与扫描配置",
	"资产管理":   "网络资产的发现、查询、更新与下钻视图（站点 / 域名 / IP / 应用 / 图标 / 截图 / 历史）",
	"任务管理":   "扫描任务的创建、控制、日志、配置模板、分片与定时调度",
}

// specCache 生成的 OpenAPI 文档在一次进程内只构建一次。
var (
	specOnce  sync.Once
	specCache []byte
)

// SpecHandler 暴露 OpenAPI 3.0 JSON，路径 /swagger/doc.json
func SpecHandler(w http.ResponseWriter, r *http.Request) {
	specOnce.Do(buildSpec)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(specCache)
}

// ResetSpec 重置缓存（用于开发热重载或测试）
func ResetSpec() {
	specOnce = sync.Once{}
	specCache = nil
}

// buildSpec 聚合路由 + 元数据 + 反射器，构造完整 OpenAPI 3.0 文档缓存到 specCache。
func buildSpec() {
	collectedMu.Lock()
	routes := append([]RouteInfo(nil), collectedRoutes...)
	collectedMu.Unlock()

	r := NewReflector()
	r.SeedReflector()

	paths := make(map[string]map[string]interface{}, len(routes))
	tagSet := make(map[string]struct{}, 32)

	for _, rt := range routes {
		if !strings.HasPrefix(rt.Path, "/api/v1/") &&
			!strings.HasPrefix(rt.Path, "/api/open/") &&
			rt.Path != "/health" {
			continue
		}
		oaPath := openapiPath(rt.Path)
		if paths[oaPath] == nil {
			paths[oaPath] = make(map[string]interface{})
		}
		method := strings.ToLower(rt.Method)
		if method == "" {
			method = "post"
		}
		if paths[oaPath][method] != nil {
			continue // 跳过重复
		}

		meta, hasMeta := LookupMeta(rt.Method, rt.Path)
		tag := fallbackGroup(rt)
		summary := rt.Method + " " + rt.Path
		description := ""
		var reqType, respType string
		var security []map[string][]string
		var errors []int
		deprecated := false

		if hasMeta {
			tag = meta.Tag
			if tag == "" {
				tag = fallbackGroup(rt)
			}
			if tagDescriptions[tag] == "" && meta.TagDesc != "" {
				tagDescriptions[tag] = meta.TagDesc
			}
			if meta.Summary != "" {
				summary = meta.Summary
			}
			description = meta.Description
			reqType = meta.ReqType
			respType = meta.RespType
			security = securityFor(meta.Security)
			errors = meta.Errors
			deprecated = meta.Deprecated
			if meta.Tag != "" {
				tagDescriptions[tag] = firstNonEmpty(meta.TagDesc, tagDescriptions[tag])
			}
		} else {
			tier := rt.AuthTier
			if tier == "" {
				tier = inferTierFromPath(rt.Path)
			}
			security = securityFor(tier)
		}

		tagSet[tag] = struct{}{}

		op := map[string]interface{}{
			"tags":        []string{tag},
			"summary":     summary,
			"operationId": method + "_" + strings.ReplaceAll(strings.Trim(rt.Path, "/"), "/", "_"),
			"responses":   buildResponses(r, respType, security, errors),
		}
		if description != "" {
			op["description"] = description
		}
		if deprecated {
			op["deprecated"] = true
		}
		if len(security) > 0 {
			op["security"] = security
		}

		// 请求参数处理
		isGet := method == "get" || method == "delete"
		hasPathParams := strings.Contains(rt.Path, ":")

		if reqType != "" {
			if ref := r.MustRef(reqType); ref != nil {
				if isGet {
					// GET 请求：尝试将结构体字段转为 query 参数
					op["parameters"] = buildQueryParameters(r, reqType)
				} else {
					// POST/PUT: requestBody
					op["requestBody"] = buildRequestBody(ref)
				}
			}
		} else if method == "post" && !hasPathParams && !isOpenAPI(rt.Path) {
			// POST 且无 path 参数但元数据未声明 reqType 时，注入一个开放 JSON 对象
			op["requestBody"] = map[string]interface{}{
				"required": false,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": true,
							"description":          "动态 JSON Body（接口未在 metadata 中声明请求体结构）",
						},
					},
				},
			}
		}

		// path parameters（如 /static/avatars/:filename）
		if hasPathParams {
			pathParams := pathParameters(rt.Path)
			if existing, ok := op["parameters"].([]map[string]interface{}); ok {
				op["parameters"] = append(existing, pathParams...)
			} else {
				op["parameters"] = pathParams
			}
		}

		// 添加 x-code-samples 扩展（可选）
		paths[oaPath][method] = op
	}

	tagList := make([]map[string]string, 0, len(tagSet))
	tagNames := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tagNames = append(tagNames, t)
	}
	sort.Strings(tagNames)
	for _, t := range tagNames {
		desc := tagDescriptions[t]
		if desc == "" {
			desc = t
		}
		tagList = append(tagList, map[string]string{"name": t, "description": desc})
	}

	schemas := r.Schemas()
	components := map[string]interface{}{
		"securitySchemes": map[string]interface{}{
			"BearerAuth": map[string]interface{}{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
				"description":  "JWT Token，由 /api/v1/login 返回。PAT（cscan_pat_*）也可作为 Bearer 使用。",
			},
			"WorkerKey": map[string]interface{}{
				"type":        "apiKey",
				"in":          "header",
				"name":        "X-Worker-Key",
				"description": "Worker Install Key，由管理员通过 /api/v1/worker/install/command 生成，仅 Worker 进程使用。",
			},
		},
	}
	if len(schemas) > 0 {
		components["schemas"] = schemas
	}

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "CSCAN API",
			"version":     "v1",
			"description": "CSCAN 分布式网络资产扫描平台 REST API。\n\n所有业务接口前缀 `/api/v1/*`，除健康检查与登录外均统一 POST + JSON Body。\n\n响应统一信封：`{code, msg, data}`，业务错误回 HTTP 200 + body.code，鉴权失败回 HTTP 401 / 403。",
			"contact": map[string]interface{}{
				"name":  "CSCAN Team",
				"email": "s*****@*******",
			},
			"license": map[string]interface{}{
				"name": "Internal Use Only",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         "/",
				"description": "当前服务器",
			},
		},
		"tags":       tagList,
		"paths":      paths,
		"components": components,
	}

	buf, _ := json.MarshalIndent(spec, "", "  ")
	specCache = buf
}

// isOpenAPI 判断是否为开放 API 路径（/api/open/ 前缀）
func isOpenAPI(path string) bool {
	return strings.HasPrefix(path, "/api/open/")
}

// inferTierFromPath 在 metadata 缺失时，根据路径前缀推断鉴权层级。
func inferTierFromPath(path string) AuthTier {
	switch {
	case path == "/health":
		return TierPublic
	case strings.HasPrefix(path, "/api/v1/worker/console"):
		return TierConsole
	case strings.HasPrefix(path, "/api/v1/worker/task/") || strings.HasPrefix(path, "/api/v1/worker/config/") || strings.HasPrefix(path, "/api/v1/worker/heartbeat") || strings.HasPrefix(path, "/api/v1/worker/offline") || strings.HasPrefix(path, "/api/v1/worker/jsfinder/save"):
		return TierWorker
	case strings.HasPrefix(path, "/api/v1/container/logs/stream"):
		return TierContainer
	case path == "/api/v1/worker/console/terminal":
		return TierTerminal
	case strings.HasPrefix(path, "/api/v1/worker/install/") || strings.HasPrefix(path, "/api/v1/worker/delete") || strings.HasPrefix(path, "/api/v1/worker/restart") || strings.HasPrefix(path, "/api/v1/worker/rename") || strings.HasPrefix(path, "/api/v1/worker/concurrency"):
		return TierAdmin
	case strings.HasPrefix(path, "/api/v1/worker"):
		return TierAuth
	case strings.HasPrefix(path, "/api/v1/user/create"), strings.HasPrefix(path, "/api/v1/user/update"), strings.HasPrefix(path, "/api/v1/user/delete"):
		return TierAdmin
	case strings.HasPrefix(path, "/api/v1/login"), strings.HasPrefix(path, "/api/v1/theme/config/get"), strings.HasPrefix(path, "/api/v1/worker/download"), strings.HasPrefix(path, "/api/v1/worker/validate"), strings.HasPrefix(path, "/api/v1/worker/ws"):
		return TierPublic
	case strings.HasPrefix(path, "/api/open/"):
		return TierAuth
	}
	return TierAuth
}

func fallbackGroup(rt RouteInfo) string {
	if rt.Path == "/health" {
		return "系统"
	}
	g := rt.Group
	switch g {
	case "user":
		return "用户管理"
	case "asset", "assets":
		return "资产管理"
	case "task":
		return "任务管理"
	case "vul":
		return "漏洞管理"
	case "worker":
		return "Worker 管理"
	case "fingerprint":
		return "指纹管理"
	case "poc":
		return "POC 管理"
	case "onlineapi":
		return "在线搜索"
	case "dirscan":
		return "目录扫描"
	case "subdomain":
		return "子域名字典"
	case "weakpass":
		return "弱口令字典"
	case "notify":
		return "通知配置"
	case "report":
		return "报告管理"
	case "ai":
		return "AI 辅助"
	case "subfinder":
		return "Subfinder 配置"
	case "jsfinder":
		return "JSFinder"
	case "container":
		return "容器日志"
	case "workspace":
		return "工作空间"
	case "organization":
		return "组织管理"
	case "blacklist":
		return "黑名单"
	case "theme":
		return "主题配置"
	case "httpservice":
		return "HTTP 服务映射"
	case "health", "system":
		return "系统"
	case "branding":
		return "品牌配置"
	case "open":
		return "开放 API"
	case "cert":
		return "证书管理"
	}
	return g
}

// securityFor 把 AuthTier 映射为 OpenAPI security 数组。
// nil 表示无需鉴权（公开）。
func securityFor(tier AuthTier) []map[string][]string {
	switch tier {
	case TierPublic:
		return nil
	case TierWorker:
		return []map[string][]string{{"WorkerKey": {}}}
	case TierAuth:
		return []map[string][]string{{"BearerAuth": {}}}
	case TierAdmin:
		return []map[string][]string{{"BearerAuth": {}}}
	case TierConsole:
		return []map[string][]string{{"BearerAuth": {}}}
	case TierTerminal:
		return []map[string][]string{{"BearerAuth": {}}}
	case TierContainer:
		return []map[string][]string{{"BearerAuth": {}}}
	}
	return []map[string][]string{{"BearerAuth": {}}}
}

// buildRequestBody 构造 OpenAPI requestBody 节点
func buildRequestBody(schemaRef map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": schemaRef,
			},
		},
	}
}

// buildResponses 构造 OpenAPI responses 节点。
func buildResponses(r *Reflector, respType string, security []map[string][]string, errors []int) map[string]interface{} {
	resp := map[string]interface{}{}

	// 200 成功响应
	successSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"code": map[string]interface{}{
				"type":        "integer",
				"description": "业务码，0 表示成功",
				"example":     0,
			},
			"msg": map[string]interface{}{
				"type":        "string",
				"description": "提示信息",
				"example":     "success",
			},
		},
		"required": []string{"code", "msg"},
	}

	if respType != "" {
		// 有具名响应类型：data 字段引用该类型
		if ref := r.MustRef(respType); ref != nil {
			props := successSchema["properties"].(map[string]interface{})
			props["data"] = ref
		}
	} else {
		// 无具名响应：data 为可选 object
		props := successSchema["properties"].(map[string]interface{})
		props["data"] = map[string]interface{}{
			"type":        "object",
			"description": "可选数据载荷",
			"nullable":    true,
		}
	}

	resp["200"] = map[string]interface{}{
		"description": "成功响应",
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": successSchema,
			},
		},
	}

	// 401 / 403 仅在有鉴权时出现
	if len(security) > 0 {
		resp["401"] = map[string]interface{}{
			"description": "未授权：缺少 token 或 token 无效",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"code": map[string]interface{}{"type": "integer", "example": 401},
							"msg":  map[string]interface{}{"type": "string", "example": "未授权"},
						},
					},
				},
			},
		}
		resp["403"] = map[string]interface{}{
			"description": "禁止访问：权限不足",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"code": map[string]interface{}{"type": "integer", "example": 403},
							"msg":  map[string]interface{}{"type": "string", "example": "禁止访问"},
						},
					},
				},
			},
		}
	}

	// 业务错误码
	added500 := false
	added400 := false
	for _, code := range errors {
		switch code {
		case 400:
			if !added400 {
				resp["400"] = map[string]interface{}{
					"description": "请求参数错误",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"code": map[string]interface{}{"type": "integer", "example": 400},
									"msg":  map[string]interface{}{"type": "string", "example": "参数错误"},
								},
							},
						},
					},
				}
				added400 = true
			}
		case 500:
			if !added500 {
				resp["500"] = map[string]interface{}{
					"description": "服务器内部错误",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"code": map[string]interface{}{"type": "integer", "example": 500},
									"msg":  map[string]interface{}{"type": "string", "example": "服务器错误"},
								},
							},
						},
					},
				}
				added500 = true
			}
		}
	}
	return resp
}

// buildQueryParameters 尝试将请求结构体字段转换为 query 参数
func buildQueryParameters(r *Reflector, typeName string) []map[string]interface{} {
	schema, ok := r.schemas[typeName]
	if !ok {
		return nil
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}
	required, _ := schema["required"].([]string)
	requiredSet := make(map[string]bool)
	for _, f := range required {
		requiredSet[f] = true
	}

	var params []map[string]interface{}
	for name, propRaw := range props {
		prop, ok := propRaw.(map[string]interface{})
		if !ok {
			continue
		}
		// 跳过嵌套对象和数组（query 参数不适合复杂类型）
		propType, _ := prop["type"].(string)
		if propType == "object" || propType == "array" {
			continue
		}
		param := map[string]interface{}{
			"name":     name,
			"in":       "query",
			"required": requiredSet[name],
			"schema": map[string]interface{}{
				"type": propType,
			},
		}
		if desc, ok := prop["description"].(string); ok && desc != "" {
			param["description"] = desc
		}
		if example, ok := prop["example"]; ok {
			param["example"] = example
		}
		if def, ok := prop["default"]; ok {
			param["schema"].(map[string]interface{})["default"] = def
		}
		if format, ok := prop["format"].(string); ok && format != "" {
			param["schema"].(map[string]interface{})["format"] = format
		}
		params = append(params, param)
	}
	return params
}

// pathParameters 把 :param 风格路径补出 OpenAPI parameters 节点。
func pathParameters(p string) []map[string]interface{} {
	parts := strings.Split(p, "/")
	var out []map[string]interface{}
	for _, seg := range parts {
		if strings.HasPrefix(seg, ":") {
			name := seg[1:]
			out = append(out, map[string]interface{}{
				"name":     name,
				"in":       "path",
				"required": true,
				"schema": map[string]interface{}{
					"type":    "string",
					"example": name,
				},
				"description": "路径参数: " + name,
			})
		}
	}
	return out
}

// UIHandler 暴露内嵌的 swagger-ui HTML，路径 /swagger-ui
func UIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(SwaggerUIHTML))
}

// openapiPath 将 :param 风格路径转换为 OpenAPI {param} 风格
func openapiPath(p string) string {
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == ':' {
			out = append(out, '{')
			i++
			for i < len(p) && p[i] != '/' {
				out = append(out, p[i])
				i++
			}
			out = append(out, '}')
			if i < len(p) {
				out = append(out, p[i])
			}
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
