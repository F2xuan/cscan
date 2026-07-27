package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

// Subfinder 配置分组：子域名采集数据源（API Provider）配置。
func init() {
	tag := "Subfinder 配置"
	tagDesc := "Subfinder 子域名采集数据源（GitHub / Shodan / SecurityTrails 等）配置"

	register(http.MethodPost, "/api/v1/subfinder/provider/list", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "数据源列表",
		Description: "返回当前已配置的 Subfinder 数据源（API Key 脱敏），含 `provider / keys / status / description`。",
		RespType:    "SubfinderProviderListResp",
		Security:    TierAuth,
	})

	register(http.MethodPost, "/api/v1/subfinder/provider/save", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "保存数据源配置",
		Description: "保存或更新一个 Subfinder 数据源配置。\n\n**字段说明**\n\n- `provider`：数据源标识，如 `github`、`shodan`、`securitytrails`。\n- `keys`：API 密钥列表（可多个）。\n- `status`：`enable` / `disable`。\n- `description`：可选描述。\n\n**典型错误码**\n\n- 400 参数错误\n- 500 服务器错误",
		ReqType:     "SubfinderProviderSaveReq",
		Security:    TierAuth,
		Errors:      []int{400, 500},
	})

	register(http.MethodPost, "/api/v1/subfinder/provider/info", Meta{
		Tag: tag, TagDesc: tagDesc,
		Summary:     "支持的数据源元信息",
		Description: "返回 Subfinder 全部支持的数据源标识与字段约束，供前端构建新增表单。",
		RespType:    "SubfinderProviderInfoResp",
		Security:    TierAuth,
	})

	RegisterTypes(
		types.SubfinderProvider{},
		types.SubfinderProviderListResp{},
		types.SubfinderProviderSaveReq{},
		types.SubfinderProviderInfoResp{},
		types.SubfinderProviderMeta{},
	)
}
