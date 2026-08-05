package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"cscan/internal/model"
	"cscan/pkg/httpclient"

	"github.com/zeromicro/go-zero/core/logx"
)

// aiCallTimeout 单次 AI 研判调用超时预算
const aiCallTimeout = 60 * time.Second

// aiFailFastThreshold 连续失败熔断阈值：AI 服务连续失败达到该次数即判定中断，终止批量研判
const aiFailFastThreshold = 5

var (
	// aiClient AI 调用专用连接池客户端（无客户端级超时，由调用方 context 控制）
	aiClient     *http.Client
	aiClientOnce sync.Once
)

// aiHTTPClient 返回 AI 调用专用客户端。
// 默认客户端带 30s 硬超时，会截断慢速 AI 接口的响应；此处不设客户端级超时，
// 请求生命周期完全由调用方 context（aiCallTimeout）控制。
func aiHTTPClient() *http.Client {
	aiClientOnce.Do(func() {
		cfg := httpclient.DefaultPoolConfig()
		cfg.Timeout = 0
		aiClient = httpclient.NewPooledClient(cfg)
	})
	return aiClient
}

// AIClient 大模型调用客户端（OpenAI/Anthropic兼容）
type AIClient struct {
	protocol string // openai / anthropic
	baseUrl  string
	apiKey   string
	model    string
}

// NewAIClientFromConfig 从已有的AI配置构造客户端
// 配置来源于系统管理→AI配置页面存储的APIConfig（platform="ai"）
func NewAIClientFromConfig(cfg *model.APIConfig) *AIClient {
	// Key 字段存储格式: protocol|baseUrl|model
	parts := strings.Split(cfg.Key, "|")
	protocol, baseUrl, modelName := "openai", "", ""
	if len(parts) >= 3 {
		protocol = parts[0]
		baseUrl = parts[1]
		modelName = parts[2]
	}
	return &AIClient{
		protocol: protocol,
		baseUrl:  baseUrl,
		apiKey:   cfg.Secret,
		model:    modelName,
	}
}

// chatMessage 通用消息结构
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIRequest OpenAI协议请求体
type openAIRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature,omitempty"`
}

// openAIResponse OpenAI协议响应体
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// anthropicRequest Anthropic协议请求体
type anthropicRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

// anthropicResponse Anthropic协议响应体
type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat 调用大模型，返回文本内容
func (c *AIClient) Chat(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if c.baseUrl == "" {
		return "", fmt.Errorf("AI服务地址未配置")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("AI API密钥未配置")
	}

	client := aiHTTPClient()

	var url string
	var reqBody interface{}
	var headers = map[string]string{
		"Content-Type": "application/json",
	}

	base := strings.TrimRight(c.baseUrl, "/")

	if c.protocol == "anthropic" {
		// Anthropic协议
		url = base + "/v1/messages"
		headers["x-api-key"] = c.apiKey
		headers["anthropic-version"] = "2023-06-01"
		reqBody = anthropicRequest{
			Model:     c.model,
			Messages:  []chatMessage{{Role: "user", Content: prompt}},
			MaxTokens: maxTokens,
		}
	} else {
		// OpenAI兼容协议：baseUrl可能已包含/v1后缀，避免重复拼接
		suffix := "/chat/completions"
		matched, _ := regexp.MatchString(`(?i)/v1$`, base)
		if !matched {
			suffix = "/v1/chat/completions"
		}
		url = base + suffix
		headers["Authorization"] = "Bearer " + c.apiKey
		reqBody = openAIRequest{
			Model:       c.model,
			Messages:    []chatMessage{{Role: "user", Content: prompt}},
			MaxTokens:   maxTokens,
			Temperature: 0.1, // 低温度，研判结果更稳定
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求AI服务失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logx.Errorf("[AIClient] API返回非200状态: %d, body: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("AI服务返回错误(%d): %s", resp.StatusCode, truncateStr(string(respBody), 300))
	}

	if c.protocol == "anthropic" {
		var anthResp anthropicResponse
		if err := json.Unmarshal(respBody, &anthResp); err != nil {
			return "", fmt.Errorf("解析Anthropic响应失败: %w", err)
		}
		if anthResp.Error != nil {
			return "", fmt.Errorf("Anthropic API错误: %s", anthResp.Error.Message)
		}
		var texts []string
		for _, block := range anthResp.Content {
			if block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
		return strings.Join(texts, ""), nil
	}

	// OpenAI协议
	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return "", fmt.Errorf("解析OpenAI响应失败: %w", err)
	}
	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenAI API错误: %s", openAIResp.Error.Message)
	}
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("AI返回结果为空")
	}
	return openAIResp.Choices[0].Message.Content, nil
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
