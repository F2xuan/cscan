package model

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// JSFinderConfig 全局 JSFinder 配置
type JSFinderConfig struct {
	Id                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	HighRiskRoutes       []string           `bson:"high_risk_routes" json:"highRiskRoutes"`             // 高危路由关键词，命中则跳过未授权检测
	AuthRequiredKeywords []string           `bson:"auth_required_keywords" json:"authRequiredKeywords"` // 鉴权关键词，响应包含视为已正确鉴权（非未授权）
	SensitiveKeywords    []string           `bson:"sensitive_keywords" json:"sensitiveKeywords"`        // 敏感数据关键词，响应包含视为信息泄漏
	DomainBlacklist      []string           `bson:"domain_blacklist" json:"domainBlacklist"`            // 域名黑名单，命中则跳过抓取与未授权检测
	CreateTime           time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime           time.Time          `bson:"update_time" json:"updateTime"`
}

// JSFinderConfigModel JSFinder 配置模型
type JSFinderConfigModel struct {
	coll *mongo.Collection
}

// NewJSFinderConfigModel 创建 JSFinder 配置模型
func NewJSFinderConfigModel(db *mongo.Database) *JSFinderConfigModel {
	return &JSFinderConfigModel{coll: db.Collection("jsfinder_config")}
}

// DefaultHighRiskRoutes 默认高危路由关键词
func DefaultHighRiskRoutes() []string {
	return []string{
		"delete",
		"remove",
		"drop",
		"destroy",
		"update",
		"modify",
		"edit",
		"patch",
		"create",
		"add",
		"insert",
		"save",
		"write",
		"upload",
		"import",
		"export",
		"exec",
		"execute",
		"run",
		"kill",
		"stop",
		"shutdown",
		"restart",
		"reset",
		"clear",
		"truncate",
		"purge",
		"wipe",
		"logout",
		"signout",
		"revoke",
	}
}

// DefaultAuthRequiredKeywords 默认鉴权关键词（响应包含视为已正确鉴权）
func DefaultAuthRequiredKeywords() []string {
	return []string{
		"请登录",
		"请先登录",
		"未登录",
		"登录过期",
		"登录失效",
		"鉴权失败",
		"权限不足",
		"无权访问",
		"无权限",
		"拒绝访问",
		"token失效",
		"token过期",
		"token无效",
		"token不能为空",
		"login required",
		"please login",
		"please log in",
		"sign in required",
		"unauthorized",
		"unauthenticated",
		"authentication required",
		"access denied",
		"permission denied",
		"forbidden",
		"not authorized",
		"401",
		"403",
		"invalid token",
		"token expired",
		"token missing",
	}
}

// DefaultSensitiveKeywords 默认敏感数据关键词（响应包含视为信息泄漏）
func DefaultSensitiveKeywords() []string {
	return []string{
		"password",
		"passwd",
		"secret",
		"token",
		"access_token",
		"refresh_token",
		"api_key",
		"apikey",
		"access_key",
		"accesskey",
		"secret_key",
		"secretkey",
		"private_key",
		"privatekey",
		"client_secret",
		"clientsecret",
		"AKID",
		"AccessKeyId",
		"SecretAccessKey",
		"phone",
		"mobile",
		"telephone",
		"idcard",
		"id_card",
		"identity_card",
		"身份证",
		"email",
		"mail",
		"openid",
		"unionid",
		"jwt",
		"bearer",
		"credit_card",
		"creditcard",
		"cvv",
		"ssn",
		"passport",
	}
}

// DefaultDomainBlacklist 默认域名黑名单
func DefaultDomainBlacklist() []string {
	return []string{
		"w3.org",
		"www.w3.org",
		"googleapis.com",
		"ajax.googleapis.com",
		"fonts.googleapis.com",
		"gstatic.com",
		"cdnjs.cloudflare.com",
		"cdnjs.com",
		"jsdelivr.net",
		"cdn.jsdelivr.net",
		"unpkg.com",
		"element-plus.org",
		"element.eleme.io",
		"element-plus.gitee.io",
		"vuejs.org",
		"cn.vuejs.org",
		"router.vuejs.org",
		"reactjs.org",
		"react.dev",
		"ant.design",
		"antdv.com",
		"github.com",
		"raw.githubusercontent.com",
		"githubusercontent.com",
		"npmjs.com",
		"registry.npmjs.org",
		"bootstrapcdn.com",
		"cdn.bootcss.com",
		"bootcdn.net",
		"polyfill.io",
		"code.jquery.com",
		"jquery.com",
		"sentry.io",
		"sentry-cdn.com",
		"baidu.com",
		"hm.baidu.com",
		"google-analytics.com",
		"googletagmanager.com",
		"vite.dev",
		// —— 以下为补充：规范文档/标准草案/浏览器Bug追踪器等外链，不应作为未授权扫描对象 ——
		"tc39.es", // ECMA-262 等规范
		"tc39.github.io",
		"github.com/tc39",
		"w3c.github.io", // W3C 规范草案
		"w3.org",
		"html.spec.whatwg.org", // HTML Living Standard
		"whatwg.org",
		"whatwg.cn",
		"spec.whatwg.org",
		"bugs.chromium.org", // Chromium bug tracker
		"chromium.googlesource.com",
		"crbug.com",
		"developer.mozilla.org", // MDN
		"developer.chrome.com",
		"web.dev",
		"developers.google.com",
		"datatracker.ietf.org",
		"tools.ietf.org",
		"rfc-editor.org",
		"ietf.org",
		"www.iana.org",
		"iana.org",
		"mozilla.org",
		"www.mozilla.org",
		"wiki.mozilla.org",
		"support.mozilla.org",
		"bugzilla.mozilla.org",
		"dxr.mozilla.org",
		"searchfox.org",
		"chromereleases.googleblog.com",
		"blog.chromium.org",
		"web.archive.org",
		"archive.org",
		"wikipedia.org",
		"en.wikipedia.org",
		"zh.wikipedia.org",
		"schema.org",
		"www.schema.org",
		"openid.net",
		"oauth.net",
		"jwt.io",
		"www.rfc-editor.org",
	}
}

// NewDefaultJSFinderConfig 返回内置默认配置（非持久化）
func NewDefaultJSFinderConfig() *JSFinderConfig {
	return &JSFinderConfig{
		HighRiskRoutes:       DefaultHighRiskRoutes(),
		AuthRequiredKeywords: DefaultAuthRequiredKeywords(),
		SensitiveKeywords:    DefaultSensitiveKeywords(),
		DomainBlacklist:      DefaultDomainBlacklist(),
	}
}

// Get 获取 JSFinder 配置（仅一条记录），不存在时返回内置默认值
func (m *JSFinderConfigModel) Get(ctx context.Context) (*JSFinderConfig, error) {
	var doc JSFinderConfig
	err := m.coll.FindOne(ctx, bson.M{}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return NewDefaultJSFinderConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// Save 保存 JSFinder 配置（Upsert）
func (m *JSFinderConfigModel) Save(ctx context.Context, doc *JSFinderConfig) error {
	now := time.Now()
	doc.UpdateTime = now

	var current JSFinderConfig
	findErr := m.coll.FindOne(ctx, bson.M{}).Decode(&current)
	if findErr != nil && findErr != mongo.ErrNoDocuments {
		return findErr
	}

	if findErr == nil && !current.Id.IsZero() {
		update := bson.M{
			"high_risk_routes":       doc.HighRiskRoutes,
			"auth_required_keywords": doc.AuthRequiredKeywords,
			"sensitive_keywords":     doc.SensitiveKeywords,
			"domain_blacklist":       doc.DomainBlacklist,
			"update_time":            now,
		}
		_, err := m.coll.UpdateOne(ctx, bson.M{"_id": current.Id}, bson.M{"$set": update})
		return err
	}

	doc.Id = primitive.NewObjectID()
	doc.CreateTime = now
	_, err := m.coll.InsertOne(ctx, doc)
	return err
}

// EnsureDefault 若不存在配置文档则插入内置默认值
func (m *JSFinderConfigModel) EnsureDefault(ctx context.Context) error {
	count, err := m.coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return m.Save(ctx, NewDefaultJSFinderConfig())
}

// MergeMissingDomainBlacklist 合并补充黑名单条目（幂等）
// 仅追加 DefaultDomainBlacklist 中存在但数据库记录中缺失的条目，保留用户自定义条目
func (m *JSFinderConfigModel) MergeMissingDomainBlacklist(ctx context.Context) error {
	doc, err := m.Get(ctx)
	if err != nil {
		return err
	}

	existing := make(map[string]bool, len(doc.DomainBlacklist))
	for _, d := range doc.DomainBlacklist {
		existing[strings.ToLower(strings.TrimSpace(d))] = true
	}

	var added []string
	for _, d := range DefaultDomainBlacklist() {
		if !existing[strings.ToLower(d)] {
			added = append(added, d)
		}
	}

	if len(added) == 0 {
		return nil
	}

	merged := append(append([]string{}, doc.DomainBlacklist...), added...)
	update := bson.M{
		"domain_blacklist": merged,
		"update_time":      time.Now(),
	}
	_, err = m.coll.UpdateOne(ctx, bson.M{"_id": doc.Id}, bson.M{"$set": update})
	return err
}
