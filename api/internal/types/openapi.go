package types

// 开放 API（/api/open/v1/）请求与响应类型。
// 独立文件，避免改动 types.go；所有端点只读，响应仅含安全投影字段（剔除 body/cert/screenshot/request/response 等）。

// OpenPageReq 开放 API 通用分页/过滤参数（全部可选，缺省走服务端默认）
type OpenPageReq struct {
	Page     int    `form:"page,optional"`     // 页码，从 1 开始
	PageSize int    `form:"pageSize,optional"` // 每页大小，默认 20，最大 100
	Keyword  string `form:"keyword,optional"`  // 关键字模糊匹配（资产 authority/host、漏洞 url、证书 subject 等）
}

// OpenListData 通用列表载荷
type OpenListData struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// OpenAssetsReq 资产列表查询
type OpenAssetsReq struct {
	OpenPageReq
	Category  string `form:"category,optional"`  // 资产分类过滤
	RiskLevel string `form:"riskLevel,optional"` // 风险等级过滤 critical/high/medium/low/info/unknown
}

// OpenAssetsResp 资产列表响应
type OpenAssetsResp struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data *OpenListData  `json:"data,omitempty"`
}

// OpenAssetDetailReq 资产详情
type OpenAssetDetailReq struct {
	Id string `form:"id"` // 资产文档 id
}

// OpenAssetDetailResp 资产详情响应
type OpenAssetDetailResp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data *OpenAsset  `json:"data,omitempty"`
}

// OpenVulnsReq 漏洞列表查询
type OpenVulnsReq struct {
	OpenPageReq
	Severity string `form:"severity,optional"` // 严重级别过滤 critical/high/medium/low/info/unknown
	Status   string `form:"status,optional"`   // 状态过滤 open/fixed/ignored
}

// OpenVulnsResp 漏洞列表响应
type OpenVulnsResp struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data *OpenListData `json:"data,omitempty"`
}

// OpenVulnDetailReq 漏洞详情
type OpenVulnDetailReq struct {
	Id string `form:"id"`
}

// OpenVulnDetailResp 漏洞详情响应
type OpenVulnDetailResp struct {
	Code int       `json:"code"`
	Msg  string    `json:"msg"`
	Data *OpenVul `json:"data,omitempty"`
}

// OpenCertsReq 证书列表查询
type OpenCertsReq struct {
	OpenPageReq
	Keyword string `form:"keyword,optional"` // 按 host/authority/subjectDN/issuerDN/SAN 模糊匹配
}

// OpenCertsResp 证书列表响应
type OpenCertsResp struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data *OpenListData `json:"data,omitempty"`
}

// OpenCertDetailReq 证书详情
type OpenCertDetailReq struct {
	Id string `form:"id"`
}

// OpenCertDetailResp 证书详情响应
type OpenCertDetailResp struct {
	Code int       `json:"code"`
	Msg  string    `json:"msg"`
	Data *OpenCert `json:"data,omitempty"`
}

// ---- 安全投影 DTO：仅含开放 API 允许外发的字段 ----

// OpenAsset 资产安全投影（剔除 httpHeader/httpBody/cert/screenshot/banner/memo/icon 等大字段与敏感内容）
type OpenAsset struct {
	Id             string   `json:"id"`
	Authority      string   `json:"authority"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Category       string   `json:"category"`
	Domain         string   `json:"domain"`
	Service        string   `json:"service"`
	Server         string   `json:"server"`
	Title          string   `json:"title"`
	App            []string `json:"app"`
	Fingerprints   []string `json:"fingerprints"`
	HttpStatus     string   `json:"httpStatus"`
	Labels         []string `json:"labels"`
	OrgId          string   `json:"orgId"`
	ColorTag       string   `json:"colorTag"`
	IsCDN          bool     `json:"isCdn"`
	CName          string   `json:"cname"`
	IsCloud        bool     `json:"isCloud"`
	IsHTTP         bool     `json:"isHttp"`
	TaskId         string   `json:"taskId"`
	Source         string   `json:"source"`
	RiskScore      float64  `json:"riskScore"`
	RiskLevel      string   `json:"riskLevel"`
	CreateTime     string   `json:"createTime"`
	UpdateTime     string   `json:"updateTime"`
	FirstSeenTime  string   `json:"firstSeenTime"`
}

// OpenVul 漏洞安全投影（剔除 request/response/curlCommand/matcherName/extractedResults/pocFile 等证据链）
type OpenVul struct {
	Id            string    `json:"id"`
	Authority     string    `json:"authority"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	Url           string    `json:"url"`
	Source        string    `json:"source"`
	Severity      string    `json:"severity"`
	VulName       string    `json:"vulName"`
	Tags          []string  `json:"tags"`
	CvssScore     float64   `json:"cvssScore"`
	CveId         string    `json:"cveId"`
	CweId         string    `json:"cweId"`
	Remediation   string    `json:"remediation"`
	References    []string  `json:"references"`
	Status        string    `json:"status"`
	RiskSource    string    `json:"riskSource"`
	CreateTime    string    `json:"createTime"`
	UpdateTime    string    `json:"updateTime"`
	FirstSeenTime string    `json:"firstSeenTime"`
	LastSeenTime  string    `json:"lastSeenTime"`
}

// OpenCert 证书安全投影（ARL 风格结构化字段）
type OpenCert struct {
	Id           string            `json:"id"`
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Authority    string            `json:"authority"`
	Subject      CertNameInfo      `json:"subject"`
	SubjectDN    string            `json:"subjectDN"`
	Issuer       CertNameInfo      `json:"issuer"`
	IssuerDN     string            `json:"issuerDN"`
	SerialNumber string            `json:"serialNumber"`
	SigAlg       string            `json:"sigAlg"`
	NotBefore    string            `json:"notBefore"`
	NotAfter     string            `json:"notAfter"`
	Version      int               `json:"version"`
	SANs         []string          `json:"sans"`
	Fingerprints map[string]string `json:"fingerprints"`
	IsSelfSigned bool              `json:"isSelfSigned"`
	CreateTime   string            `json:"createTime"`
}
