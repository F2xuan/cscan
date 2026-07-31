package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ============== SMTP Email Provider ==============

// SMTPConfig SMTP配置
type SMTPConfig struct {
	Server          string   `json:"server"`
	Port            int      `json:"port"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	FromAddress     string   `json:"fromAddress"`
	ToAddresses     []string `json:"toAddresses"`
	Subject         string   `json:"subject"`
	UseTLS          bool     `json:"useTLS"`
	SkipVerify      bool     `json:"skipVerify"`
	MessageTemplate string   `json:"messageTemplate"`
}

// SMTPProvider SMTP邮件通知
type SMTPProvider struct {
	config SMTPConfig
}

// NewSMTPProvider 创建SMTP提供者
func NewSMTPProvider(config SMTPConfig) *SMTPProvider {
	return &SMTPProvider{config: config}
}

func (p *SMTPProvider) Name() string { return "smtp" }

func (p *SMTPProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.Server == "" || len(p.config.ToAddresses) == 0 {
		return fmt.Errorf("smtp config incomplete")
	}

	subject := p.config.Subject
	if subject == "" {
		subject = fmt.Sprintf("扫描任务完成: %s", result.TaskName)
	}

	body := FormatMessage(result, p.config.MessageTemplate)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		p.config.FromAddress,
		strings.Join(p.config.ToAddresses, ","),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", p.config.Server, p.config.Port)
	auth := smtp.PlainAuth("", p.config.Username, p.config.Password, p.config.Server)

	// 修复 C-34：原实现用 goroutine + 15s timer 包装 smtp.SendMail/sendWithTLS，
	// 但 tls.Dial 和 smtp.SendMail 内部使用 net.Dial（无超时），SMTP 服务器不可达时
	// goroutine 会阻塞到 OS 级 TCP 超时（数分钟），timer 返回后 goroutine 仍然泄漏。
	// 现使用带 10s 超时的 net.DialTimeout 建立连接，并在连接上设置 I/O deadline，
	// 彻底消除 goroutine 泄漏。
	if p.config.UseTLS {
		return p.sendWithTLS(addr, auth, msg)
	}
	return p.sendPlain(addr, auth, msg)
}

// sendPlain 使用明文 SMTP 发送（带连接超时和 I/O deadline）
func (p *SMTPProvider) sendPlain(addr string, auth smtp.Auth, msg string) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("smtp dial timeout: %w", err)
	}
	// 设置 I/O deadline，防止 SMTP 交互阶段挂起
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer conn.Close()

	client, err := smtp.NewClient(conn, p.config.Server)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return err
	}
	if err = client.Mail(p.config.FromAddress); err != nil {
		return err
	}
	for _, to := range p.config.ToAddresses {
		if err = client.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	return w.Close()
}

func (p *SMTPProvider) sendWithTLS(addr string, auth smtp.Auth, msg string) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: p.config.SkipVerify,
		ServerName:         p.config.Server,
	}

	// 修复 C-34：使用 net.DialTimeout + tls.Client 替代 tls.Dial（无超时）
	rawConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("smtp tls dial timeout: %w", err)
	}
	conn := tls.Client(rawConn, tlsConfig)
	if err := conn.Handshake(); err != nil {
		rawConn.Close()
		return fmt.Errorf("smtp tls handshake failed: %w", err)
	}
	// 设置 I/O deadline，防止 SMTP 交互阶段挂起
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer conn.Close()

	client, err := smtp.NewClient(conn, p.config.Server)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return err
	}
	if err = client.Mail(p.config.FromAddress); err != nil {
		return err
	}
	for _, to := range p.config.ToAddresses {
		if err = client.Rcpt(to); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	return w.Close()
}

// ============== 飞书 (Feishu/Lark) Provider ==============

// FeishuConfig 飞书配置
type FeishuConfig struct {
	WebhookURL      string `json:"webhookUrl"`
	Secret          string `json:"secret"` // 签名密钥（可选）
	MessageTemplate string `json:"messageTemplate"`
}

// FeishuProvider 飞书通知
type FeishuProvider struct {
	config FeishuConfig
}

// NewFeishuProvider 创建飞书提供者
func NewFeishuProvider(config FeishuConfig) *FeishuProvider {
	return &FeishuProvider{config: config}
}

func (p *FeishuProvider) Name() string { return "feishu" }

func (p *FeishuProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("feishu webhook url is empty")
	}

	payload := p.buildCard(result)

	// 如果配置了签名密钥
	if p.config.Secret != "" {
		timestamp := time.Now().Unix()
		sign := p.genFeishuSign(timestamp)
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = sign
	}

	return postJSON(ctx, p.config.WebhookURL, payload)
}

// buildCard 构造飞书交互式卡片（T4.5：标题色随严重度、概览 + 最紧急项 + 跳转按钮）
func (p *FeishuProvider) buildCard(result *NotifyResult) map[string]interface{} {
	elements := []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": cardBodyFor(result, p.config.MessageTemplate),
			},
		},
	}
	if jump := primaryJumpURL(result); jump != "" {
		elements = append(elements, map[string]interface{}{
			"tag": "action",
			"actions": []map[string]interface{}{
				{
					"tag":  "button",
					"text": map[string]interface{}{"tag": "plain_text", "content": "查看详情"},
					"type": "primary",
					"url":  jump,
				},
			},
		})
	}
	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": cardTitle(result),
				},
				"template": cardHeaderColor(result),
			},
			"elements": elements,
		},
	}
}

func (p *FeishuProvider) genFeishuSign(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, p.config.Secret)
	h := hmac.New(sha256.New, []byte(p.config.Secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ============== 钉钉 (DingTalk) Provider ==============

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	WebhookURL      string `json:"webhookUrl"`
	Secret          string `json:"secret"` // 签名密钥
	MessageTemplate string `json:"messageTemplate"`
}

// DingTalkProvider 钉钉通知
type DingTalkProvider struct {
	config DingTalkConfig
}

// NewDingTalkProvider 创建钉钉提供者
func NewDingTalkProvider(config DingTalkConfig) *DingTalkProvider {
	return &DingTalkProvider{config: config}
}

func (p *DingTalkProvider) Name() string { return "dingtalk" }

func (p *DingTalkProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("dingtalk webhook url is empty")
	}

	payload := p.buildActionCard(result)

	webhookURL := p.config.WebhookURL
	// 如果配置了签名密钥
	if p.config.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := p.genDingTalkSign(timestamp)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
	}

	return postJSON(ctx, webhookURL, payload)
}

// buildActionCard 构造钉钉 ActionCard（T4.5：标题 + 概览 + 最紧急项 + 跳转按钮）
func (p *DingTalkProvider) buildActionCard(result *NotifyResult) map[string]interface{} {
	actionCard := map[string]interface{}{
		"title":          cardTitle(result),
		"text":           cardBodyFor(result, p.config.MessageTemplate),
		"btnOrientation": "0",
	}
	if jump := primaryJumpURL(result); jump != "" {
		actionCard["singleTitle"] = "查看详情"
		actionCard["singleURL"] = jump
	} else if result.ReportURL != "" {
		actionCard["singleTitle"] = "查看报告"
		actionCard["singleURL"] = result.ReportURL
	}
	return map[string]interface{}{
		"msgtype":    "actionCard",
		"actionCard": actionCard,
	}
}

func (p *DingTalkProvider) genDingTalkSign(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, p.config.Secret)
	h := hmac.New(sha256.New, []byte(p.config.Secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ============== 企业微信 (WeCom) Provider ==============

// WeComConfig 企业微信配置
type WeComConfig struct {
	WebhookURL      string `json:"webhookUrl"`
	MessageTemplate string `json:"messageTemplate"`
}

// WeComProvider 企业微信通知
type WeComProvider struct {
	config WeComConfig
}

// NewWeComProvider 创建企业微信提供者
func NewWeComProvider(config WeComConfig) *WeComProvider {
	return &WeComProvider{config: config}
}

func (p *WeComProvider) Name() string { return "wecom" }

func (p *WeComProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("wecom webhook url is empty")
	}

	payload := p.buildMarkdown(result)

	return postJSON(ctx, p.config.WebhookURL, payload)
}

// buildMarkdown 构造企业微信 markdown 消息（T4.5：标题 + 概览 + 最紧急项 + 跳转链接）
func (p *WeComProvider) buildMarkdown(result *NotifyResult) map[string]interface{} {
	content := fmt.Sprintf("# %s\n\n%s", cardTitle(result), cardBodyFor(result, p.config.MessageTemplate))
	if jump := primaryJumpURL(result); jump != "" {
		content += fmt.Sprintf("\n[查看详情](%s)\n", jump)
	} else if result.ReportURL != "" {
		content += fmt.Sprintf("\n[查看报告](%s)\n", result.ReportURL)
	}
	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": content,
		},
	}
}

// ============== Slack Provider ==============

// SlackConfig Slack配置
type SlackConfig struct {
	WebhookURL      string `json:"webhookUrl"`
	Channel         string `json:"channel"`
	Username        string `json:"username"`
	MessageTemplate string `json:"messageTemplate"`
}

// SlackProvider Slack通知
type SlackProvider struct {
	config SlackConfig
}

// NewSlackProvider 创建Slack提供者
func NewSlackProvider(config SlackConfig) *SlackProvider {
	return &SlackProvider{config: config}
}

func (p *SlackProvider) Name() string { return "slack" }

func (p *SlackProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("slack webhook url is empty")
	}

	content := FormatMessage(result, p.config.MessageTemplate)

	payload := map[string]interface{}{
		"text": content,
	}
	if p.config.Channel != "" {
		payload["channel"] = p.config.Channel
	}
	if p.config.Username != "" {
		payload["username"] = p.config.Username
	}

	return postJSON(ctx, p.config.WebhookURL, payload)
}

// ============== Discord Provider ==============

// DiscordConfig Discord配置
type DiscordConfig struct {
	WebhookURL      string `json:"webhookUrl"`
	Username        string `json:"username"`
	MessageTemplate string `json:"messageTemplate"`
}

// DiscordProvider Discord通知
type DiscordProvider struct {
	config DiscordConfig
}

// NewDiscordProvider 创建Discord提供者
func NewDiscordProvider(config DiscordConfig) *DiscordProvider {
	return &DiscordProvider{config: config}
}

func (p *DiscordProvider) Name() string { return "discord" }

func (p *DiscordProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("discord webhook url is empty")
	}

	content := FormatMessage(result, p.config.MessageTemplate)

	payload := map[string]interface{}{
		"content": content,
	}
	if p.config.Username != "" {
		payload["username"] = p.config.Username
	}

	return postJSON(ctx, p.config.WebhookURL, payload)
}

// ============== Telegram Provider ==============

// TelegramConfig Telegram配置
type TelegramConfig struct {
	BotToken        string `json:"botToken"`
	ChatID          string `json:"chatId"`
	ParseMode       string `json:"parseMode"` // Markdown, HTML, MarkdownV2
	MessageTemplate string `json:"messageTemplate"`
}

// TelegramProvider Telegram通知
type TelegramProvider struct {
	config TelegramConfig
}

// NewTelegramProvider 创建Telegram提供者
func NewTelegramProvider(config TelegramConfig) *TelegramProvider {
	return &TelegramProvider{config: config}
}

func (p *TelegramProvider) Name() string { return "telegram" }

func (p *TelegramProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.BotToken == "" || p.config.ChatID == "" {
		return fmt.Errorf("telegram config incomplete")
	}

	content := FormatMessage(result, p.config.MessageTemplate)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", p.config.BotToken)

	payload := map[string]interface{}{
		"chat_id": p.config.ChatID,
		"text":    content,
	}
	if p.config.ParseMode != "" {
		payload["parse_mode"] = p.config.ParseMode
	}

	return postJSON(ctx, apiURL, payload)
}

// ============== Microsoft Teams Provider ==============

// TeamsConfig Teams配置
type TeamsConfig struct {
	WebhookURL      string `json:"webhookUrl"`
	MessageTemplate string `json:"messageTemplate"`
}

// TeamsProvider Teams通知
type TeamsProvider struct {
	config TeamsConfig
}

// NewTeamsProvider 创建Teams提供者
func NewTeamsProvider(config TeamsConfig) *TeamsProvider {
	return &TeamsProvider{config: config}
}

func (p *TeamsProvider) Name() string { return "teams" }

func (p *TeamsProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.WebhookURL == "" {
		return fmt.Errorf("teams webhook url is empty")
	}

	content := FormatMessage(result, p.config.MessageTemplate)

	// Teams Adaptive Card 格式
	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": "0076D7",
		"summary":    "扫描任务完成",
		"sections": []map[string]interface{}{
			{
				"activityTitle": "扫描任务完成通知",
				"text":          content,
			},
		},
	}

	return postJSON(ctx, p.config.WebhookURL, payload)
}

// ============== Gotify Provider ==============

// GotifyConfig Gotify配置
type GotifyConfig struct {
	ServerURL       string `json:"serverUrl"`
	Token           string `json:"token"`
	Priority        int    `json:"priority"`
	MessageTemplate string `json:"messageTemplate"`
}

// GotifyProvider Gotify通知
type GotifyProvider struct {
	config GotifyConfig
}

// NewGotifyProvider 创建Gotify提供者
func NewGotifyProvider(config GotifyConfig) *GotifyProvider {
	return &GotifyProvider{config: config}
}

func (p *GotifyProvider) Name() string { return "gotify" }

func (p *GotifyProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.ServerURL == "" || p.config.Token == "" {
		return fmt.Errorf("gotify config incomplete")
	}

	content := FormatMessage(result, p.config.MessageTemplate)

	apiURL := fmt.Sprintf("%s/message?token=%s", strings.TrimSuffix(p.config.ServerURL, "/"), p.config.Token)

	priority := p.config.Priority
	if priority == 0 {
		priority = 5
	}

	payload := map[string]interface{}{
		"title":    fmt.Sprintf("扫描任务完成: %s", result.TaskName),
		"message":  content,
		"priority": priority,
	}

	return postJSON(ctx, apiURL, payload)
}

// ============== Custom Webhook Provider ==============

// WebhookConfig 自定义Webhook配置
type WebhookConfig struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"` // GET, POST
	Headers         map[string]string `json:"headers"`
	MessageTemplate string            `json:"messageTemplate"`
	BodyTemplate    string            `json:"bodyTemplate"` // 自定义请求体模板
}

// WebhookProvider 自定义Webhook通知
type WebhookProvider struct {
	config WebhookConfig
}

// NewWebhookProvider 创建Webhook提供者
func NewWebhookProvider(config WebhookConfig) *WebhookProvider {
	return &WebhookProvider{config: config}
}

func (p *WebhookProvider) Name() string { return "webhook" }

func (p *WebhookProvider) Send(ctx context.Context, result *NotifyResult) error {
	if p.config.URL == "" {
		return fmt.Errorf("webhook url is empty")
	}

	method := p.config.Method
	if method == "" {
		method = "POST"
	}

	var body io.Reader
	if method == "POST" {
		if p.config.BodyTemplate != "" {
			// 使用自定义模板
			content := FormatMessage(result, p.config.BodyTemplate)
			body = strings.NewReader(content)
		} else {
			// 默认JSON格式
			payload := map[string]interface{}{
				"taskId":     result.TaskId,
				"taskName":   result.TaskName,
				"status":     result.Status,
				"assetCount": result.AssetCount,
				"vulCount":   result.VulCount,
				"duration":   result.Duration,
				"startTime":  result.StartTime.Format(time.RFC3339),
				"endTime":    result.EndTime.Format(time.RFC3339),
				"reportUrl":  result.ReportURL,
				"message":    FormatMessage(result, p.config.MessageTemplate),
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshal webhook payload: %w", err)
			}
			body = bytes.NewReader(data)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, p.config.URL, body)
	if err != nil {
		return err
	}

	// 设置默认Content-Type
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置自定义Headers
	for k, v := range p.config.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ============== 卡片构造（T4.5：飞书/钉钉/企微移动端卡片） ==============

// cardTitle 卡片标题：含状态 emoji 与最高严重度标签
func cardTitle(result *NotifyResult) string {
	emoji := "✅"
	if result.Status == "FAILURE" {
		emoji = "❌"
	}
	name := result.TaskName
	if name == "" {
		name = result.TaskId
	}
	if sev := topRiskSeverity(result.HighRiskInfo); sev != "" {
		return fmt.Sprintf("%s [%s] %s", emoji, riskSeverityLabel(sev), name)
	}
	return fmt.Sprintf("%s %s", emoji, name)
}

// cardHeaderColor 飞书 interactive 卡片 header 配色，随最高严重度变化
func cardHeaderColor(result *NotifyResult) string {
	if result.Status == "FAILURE" {
		return "red"
	}
	switch topRiskSeverity(result.HighRiskInfo) {
	case "critical":
		return "red"
	case "high":
		return "orange"
	case "medium":
		return "blue"
	case "low", "info":
		return "grey"
	default:
		return "blue"
	}
}

// topRiskSeverity 返回最高严重度（无明细时退看 HighRiskVulSeverities）
func topRiskSeverity(info *HighRiskInfo) string {
	if info == nil {
		return ""
	}
	best := ""
	bestRank := 0
	consider := func(sev string) {
		if r := severityRank(sev); r > bestRank {
			bestRank = r
			best = sev
		}
	}
	for _, r := range info.NewRisks {
		consider(r.Severity)
	}
	if best == "" {
		for s := range info.HighRiskVulSeverities {
			consider(s)
		}
	}
	return best
}

// topRiskKind 返回最高严重度风险的 kind（用于决定跳转页面）
func topRiskKind(info *HighRiskInfo) string {
	if info == nil || len(info.NewRisks) == 0 {
		return ""
	}
	top := info.NewRisks[0]
	bestRank := severityRank(top.Severity)
	for _, r := range info.NewRisks[1:] {
		if rank := severityRank(r.Severity); rank > bestRank {
			bestRank = rank
			top = r
		}
	}
	return top.Kind
}

// topNRisks 按严重度降序取前 n 条
func topNRisks(risks []RiskSummary, n int) []RiskSummary {
	if len(risks) == 0 {
		return nil
	}
	sorted := make([]RiskSummary, len(risks))
	copy(sorted, risks)
	sort.SliceStable(sorted, func(i, j int) bool {
		return severityRank(sorted[i].Severity) > severityRank(sorted[j].Severity)
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// riskKindLabel 风险类型中文标签
func riskKindLabel(kind string) string {
	switch kind {
	case "vuln":
		return "漏洞"
	case "weakpass":
		return "弱口令"
	case "cert":
		return "证书"
	case "asset":
		return "资产"
	default:
		return kind
	}
}

// riskSeverityLabel 严重度中文标签
func riskSeverityLabel(sev string) string {
	switch sev {
	case "critical":
		return "严重"
	case "high":
		return "高危"
	case "medium":
		return "中危"
	case "low":
		return "低危"
	case "info":
		return "提示"
	default:
		return ""
	}
}

// frontendBase 从报告 URL 推导前端根地址（scheme://host）
func frontendBase(reportURL string) string {
	if reportURL == "" {
		return ""
	}
	u, err := url.Parse(reportURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// primaryJumpURL 主跳转链接：指向与最高严重度风险对应的详情页；无则回退报告页
func primaryJumpURL(result *NotifyResult) string {
	base := frontendBase(result.ReportURL)
	if base == "" {
		return result.ReportURL // 可能为空 → 上层不渲染按钮
	}
	switch topRiskKind(result.HighRiskInfo) {
	case "cert":
		return base + "/asset-management/risk/cert"
	case "vuln", "weakpass":
		return base + "/asset-management/risk/vuln"
	case "asset":
		return base + "/asset-management"
	default:
		if result.ReportURL != "" {
			return result.ReportURL
		}
		return base + "/asset-management"
	}
}

// buildCardBody 卡片正文（markdown）：概览 + 最紧急 1-3 项 + 报告链接
func buildCardBody(result *NotifyResult) string {
	var b strings.Builder
	b.WriteString("**概览**\n")
	b.WriteString(fmt.Sprintf("发现资产 %d · 发现漏洞 %d", result.AssetCount, result.VulCount))
	if hi := result.HighRiskInfo; hi != nil {
		if len(hi.NewRisks) > 0 {
			b.WriteString(fmt.Sprintf(" · 新增风险 %d", len(hi.NewRisks)))
		}
		if hi.NewAssetCount > 0 {
			b.WriteString(fmt.Sprintf(" · 新增资产 %d", hi.NewAssetCount))
		}
		if hi.FixedVulCount > 0 {
			b.WriteString(fmt.Sprintf(" · 已修复 %d", hi.FixedVulCount))
		}
	}
	b.WriteString("\n")
	if hi := result.HighRiskInfo; hi != nil && len(hi.NewRisks) > 0 {
		top := topNRisks(hi.NewRisks, 3)
		b.WriteString("\n**最紧急项**\n")
		for i, r := range top {
			line := fmt.Sprintf("%d. [%s] %s", i+1, riskKindLabel(r.Kind), r.Name)
			if r.Target != "" {
				line += " @ " + r.Target
			}
			if r.Severity != "" {
				line += " (" + r.Severity + ")"
			}
			b.WriteString(line + "\n")
		}
	}
	if result.ReportURL != "" {
		b.WriteString(fmt.Sprintf("\n[查看完整报告](%s)\n", result.ReportURL))
	}
	return b.String()
}

// cardBodyFor 有自定义模板时用模板，否则用统一卡片正文
func cardBodyFor(result *NotifyResult, template string) string {
	if template != "" {
		return FormatMessage(result, template)
	}
	return buildCardBody(result)
}

// ============== Helper Functions ==============

func postJSON(ctx context.Context, url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}
