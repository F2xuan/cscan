package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// AI 辅助分组：POC 生成与 AI 配置管理。
// - generatePoc：调用 AI 生成 Nuclei YAML 模板草案。
// - config/get/save：按工作空间维护 AI 服务商配置（baseUrl / apiKey / model）。
func init() {
	tag := "AI 辅助"
	tagDesc := "AI 生成 POC 与 AI 服务商配置（OpenAI / Anthropic）"

	register(http.MethodPost, "/api/v1/ai/generatePoc", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "AI 生成 POC",
		Description: "调用 AI 根据漏洞描述生成 Nuclei YAML 模板草案。\n\n**字段说明**\n\n- `description`：漏洞描述，建议详尽。\n- `vulnType`：漏洞类型（如 SQLi / XSS / RCE）。\n- `cveId`：可选 CVE 编号。\n- `reference`：可选参考链接。\n\n**典型错误码**\n\n- 400 参数错误（至少需要 description / vulnType 之一）\n- 500 AI 调用失败",
		ReqType:     "GeneratePocReq",
		RespType:    "GeneratePocResp",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/ai/config/get", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "获取 AI 配置",
		Description: "返回当前工作空间的 AI 服务商配置（API Key 脱敏）。",
		RespType:    "AIConfigGetResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/ai/config/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存 AI 配置",
		Description: "保存当前工作空间的 AI 服务商配置。\n\n**字段说明**\n\n- `protocol`：`openai` / `anthropic`。\n- `baseUrl`：服务地址。\n- `apiKey`：API 密钥。\n- `model`：模型名称。\n\n**典型错误码**\n\n- 400 参数错误\n- 500 服务器错误",
		ReqType:     "AIConfigSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	RegisterTypes(
		types.GeneratePocReq{},
		types.GeneratePocResp{},
		types.GeneratePocData{},
		types.AIConfig{},
		types.AIConfigGetResp{},
		types.AIConfigSaveReq{},
	)
}
