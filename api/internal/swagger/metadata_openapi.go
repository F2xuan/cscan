package swagger

import (
	"net/http"

	"cscan/api/internal/types"
)

func init() {
	registerOpenAPI()
}

// 开放 API（/api/open/v1/）元数据：第三方系统只读查询资产/漏洞/证书。
// 复用 PAT 鉴权（readonly scope），按 token 限流；所有端点只读，不暴露证据链等大字段。

func registerOpenAPI() {
	tag := "OpenAPI"
	tagDesc := "开放 API（只读）"

	register(http.MethodGet, "/api/open/v1/assets", Meta{
		Tag:         tag,
		TagDesc:     tagDesc,
		Summary:     "查询资产列表",
		Description: "分页查询资产，支持关键字、分类、风险等级过滤。仅返回安全投影字段（不含 body/cert/screenshot 等）。",
		ReqType:     "OpenAssetsReq",
		RespType:    "OpenAssetsResp",
		Security:    TierAuth,
		Errors:      []int{400, 401, 403, 429},
	})
	register(http.MethodGet, "/api/open/v1/assets/:id", Meta{
		Tag:         tag,
		TagDesc:     tagDesc,
		Summary:     "查询资产详情",
		Description: "按资产 id 查询单个资产的安全投影详情。",
		ReqType:     "OpenAssetDetailReq",
		RespType:    "OpenAssetDetailResp",
		Security:    TierAuth,
		Errors:      []int{400, 401, 403, 429},
	})
	register(http.MethodGet, "/api/open/v1/vulns", Meta{
		Tag:         tag,
		TagDesc:     tagDesc,
		Summary:     "查询漏洞列表",
		Description: "分页查询漏洞，支持关键字、严重级别、状态过滤。已剔除 request/response 等证据链。",
		ReqType:     "OpenVulnsReq",
		RespType:    "OpenVulnsResp",
		Security:    TierAuth,
		Errors:      []int{400, 401, 403, 429},
	})
	register(http.MethodGet, "/api/open/v1/vulns/:id", Meta{
		Tag:         tag,
		TagDesc:     tagDesc,
		Summary:     "查询漏洞详情",
		Description: "按漏洞 id 查询单个漏洞的安全投影详情。",
		ReqType:     "OpenVulnDetailReq",
		RespType:    "OpenVulnDetailResp",
		Security:    TierAuth,
		Errors:      []int{400, 401, 403, 429},
	})
	register(http.MethodGet, "/api/open/v1/certs", Meta{
		Tag:         tag,
		TagDesc:     tagDesc,
		Summary:     "查询证书列表",
		Description: "分页查询证书监控结果，支持关键字、状态过滤。",
		ReqType:     "OpenCertsReq",
		RespType:    "OpenCertsResp",
		Security:    TierAuth,
		Errors:      []int{400, 401, 403, 429},
	})
	register(http.MethodGet, "/api/open/v1/certs/:id", Meta{
		Tag:         tag,
		TagDesc:     tagDesc,
		Summary:     "查询证书详情",
		Description: "按证书 id 查询单个证书的安全投影详情。",
		ReqType:     "OpenCertDetailReq",
		RespType:    "OpenCertDetailResp",
		Security:    TierAuth,
		Errors:      []int{400, 401, 403, 429},
	})

	RegisterTypes([]interface{}{
		types.OpenAssetsReq{},
		types.OpenAssetsResp{},
		types.OpenAssetDetailReq{},
		types.OpenAssetDetailResp{},
		types.OpenVulnsReq{},
		types.OpenVulnsResp{},
		types.OpenVulnDetailReq{},
		types.OpenVulnDetailResp{},
		types.OpenCertsReq{},
		types.OpenCertsResp{},
		types.OpenCertDetailReq{},
		types.OpenCertDetailResp{},
		types.OpenAsset{},
		types.OpenVul{},
		types.OpenCert{},
		types.OpenListData{},
		types.OpenPageReq{},
	})
}
