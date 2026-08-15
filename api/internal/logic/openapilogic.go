package logic

import (
	"context"
	"errors"
	"regexp"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OpenApiLogic 开放 API 逻辑集合（只读，安全投影）。
// 所有方法要求调用方已通过 PAT 鉴权且持有 readonly scope（由路由层保证）。
// 每个 Logic 持有自己的 request ctx，并发安全。

const (
	openMaxPageSize = 100
	openDefPageSize = 20
)

func openPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = openDefPageSize
	}
	if pageSize > openMaxPageSize {
		pageSize = openMaxPageSize
	}
	return page, pageSize
}

// keywordFilter 构造大小写不敏感的模糊匹配 $or 子句。
func keywordFilter(fields []string, kw string) bson.M {
	if kw == "" {
		return nil
	}
	escaped := regexp.QuoteMeta(kw)
	or := make([]bson.M, 0, len(fields))
	for _, f := range fields {
		or = append(or, bson.M{f: bson.M{"$regex": escaped, "$options": "i"}})
	}
	return bson.M{"$or": or}
}

func mergeFilter(base bson.M, extra ...bson.M) bson.M {
	for _, e := range extra {
		for k, v := range e {
			if v != nil {
				base[k] = v
			}
		}
	}
	return base
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ---- 资产 ----

type OpenAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAssetsLogic {
	return &OpenAssetsLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenAssetsLogic) Assets(req *types.OpenAssetsReq) (*types.OpenAssetsResp, error) {
	page, pageSize := openPage(req.Page, req.PageSize)
	filter := bson.M{}
	if kw := keywordFilter([]string{"authority", "host", "domain", "title", "server"}, req.Keyword); kw != nil {
		filter = mergeFilter(filter, kw)
	}
	if req.Category != "" {
		filter["category"] = req.Category
	}
	if req.RiskLevel != "" {
		filter["risk_level"] = req.RiskLevel
	}
	m := model.NewAssetModel(l.svcCtx.MongoDB)
	docs, err := m.Find(l.ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}
	total, err := m.Count(l.ctx, filter)
	if err != nil {
		return nil, err
	}
	items := make([]*types.OpenAsset, 0, len(docs))
	for _, d := range docs {
		items = append(items, toOpenAsset(d))
	}
	return &types.OpenAssetsResp{
		Code: 0,
		Msg:  "ok",
		Data: &types.OpenListData{Items: items, Total: total, Page: page, PageSize: pageSize},
	}, nil
}

type OpenAssetDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenAssetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAssetDetailLogic {
	return &OpenAssetDetailLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenAssetDetailLogic) AssetDetail(req *types.OpenAssetDetailReq) (*types.OpenAssetDetailResp, error) {
	if req.Id == "" {
		return nil, errors.New("缺少资产 id")
	}
	m := model.NewAssetModel(l.svcCtx.MongoDB)
	doc, err := m.FindById(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("资产不存在")
	}
	return &types.OpenAssetDetailResp{Code: 0, Msg: "ok", Data: toOpenAsset(*doc)}, nil
}

func toOpenAsset(d model.Asset) *types.OpenAsset {
	return &types.OpenAsset{
		Id:            d.Id.Hex(),
		Authority:     d.Authority,
		Host:          d.Host,
		Port:          d.Port,
		Category:      d.Category,
		Domain:        d.Domain,
		Service:       d.Service,
		Server:        d.Server,
		Title:         d.Title,
		App:           d.App,
		Fingerprints:  d.Fingerprints,
		HttpStatus:    d.HttpStatus,
		Labels:        d.Labels,
		OrgId:         d.OrgId,
		ColorTag:      d.ColorTag,
		IsCDN:         d.IsCDN,
		CName:         d.CName,
		IsCloud:       d.IsCloud,
		IsHTTP:        d.IsHTTP,
		TaskId:        d.TaskId,
		Source:        d.Source,
		RiskScore:     d.RiskScore,
		RiskLevel:     d.RiskLevel,
		CreateTime:    fmtTime(d.CreateTime),
		UpdateTime:    fmtTime(d.UpdateTime),
		FirstSeenTime: fmtTime(d.FirstSeenTime),
	}
}

// ---- 漏洞 ----

type OpenVulnsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenVulnsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenVulnsLogic {
	return &OpenVulnsLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenVulnsLogic) Vulns(req *types.OpenVulnsReq) (*types.OpenVulnsResp, error) {
	page, pageSize := openPage(req.Page, req.PageSize)
	filter := bson.M{}
	if kw := keywordFilter([]string{"url", "authority", "host", "vul_name"}, req.Keyword); kw != nil {
		filter = mergeFilter(filter, kw)
	}
	if req.Severity != "" {
		filter["severity"] = req.Severity
	}
	if req.Status != "" {
		filter["status"] = req.Status
	}
	m := model.NewVulModel(l.svcCtx.MongoDB)
	docs, err := m.Find(l.ctx, filter, page, pageSize)
	if err != nil {
		return nil, err
	}
	total, err := m.Count(l.ctx, filter)
	if err != nil {
		return nil, err
	}
	items := make([]*types.OpenVul, 0, len(docs))
	for _, d := range docs {
		items = append(items, toOpenVul(d))
	}
	return &types.OpenVulnsResp{
		Code: 0,
		Msg:  "ok",
		Data: &types.OpenListData{Items: items, Total: total, Page: page, PageSize: pageSize},
	}, nil
}

type OpenVulnDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenVulnDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenVulnDetailLogic {
	return &OpenVulnDetailLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenVulnDetailLogic) VulnDetail(req *types.OpenVulnDetailReq) (*types.OpenVulnDetailResp, error) {
	if req.Id == "" {
		return nil, errors.New("缺少漏洞 id")
	}
	m := model.NewVulModel(l.svcCtx.MongoDB)
	doc, err := m.FindById(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("漏洞不存在")
	}
	return &types.OpenVulnDetailResp{Code: 0, Msg: "ok", Data: toOpenVul(*doc)}, nil
}

func toOpenVul(d model.Vul) *types.OpenVul {
	return &types.OpenVul{
		Id:            d.Id.Hex(),
		Authority:     d.Authority,
		Host:          d.Host,
		Port:          d.Port,
		Url:           d.Url,
		Source:        d.Source,
		Severity:      d.Severity,
		VulName:       d.VulName,
		Tags:          d.Tags,
		CvssScore:     d.CvssScore,
		CveId:         d.CveId,
		CweId:         d.CweId,
		Remediation:   d.Remediation,
		References:    d.References,
		Status:        d.Status,
		RiskSource:    d.RiskSource,
		CreateTime:    fmtTime(d.CreateTime),
		UpdateTime:    fmtTime(d.UpdateTime),
		FirstSeenTime: fmtTime(d.FirstSeenTime),
		LastSeenTime:  fmtTime(d.LastSeenTime),
	}
}

// ---- 证书 ----

type OpenCertsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenCertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenCertsLogic {
	return &OpenCertsLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenCertsLogic) Certs(req *types.OpenCertsReq) (*types.OpenCertsResp, error) {
	page, pageSize := openPage(req.Page, req.PageSize)
	filter := bson.M{}
	if kw := keywordFilter([]string{"subject_dn", "issuer_dn", "authority", "host", "sans"}, req.Keyword); kw != nil {
		filter = mergeFilter(filter, kw)
	}
	m := model.NewCertModel(l.svcCtx.MongoDB)
	opt := options.Find().
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "not_after", Value: 1}})
	docs, err := m.Find(l.ctx, filter, opt)
	if err != nil {
		return nil, err
	}
	total, err := m.Count(l.ctx, filter)
	if err != nil {
		return nil, err
	}
	items := make([]*types.OpenCert, 0, len(docs))
	for _, d := range docs {
		items = append(items, toOpenCert(d))
	}
	return &types.OpenCertsResp{
		Code: 0,
		Msg:  "ok",
		Data: &types.OpenListData{Items: items, Total: total, Page: page, PageSize: pageSize},
	}, nil
}

type OpenCertDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenCertDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenCertDetailLogic {
	return &OpenCertDetailLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenCertDetailLogic) CertDetail(req *types.OpenCertDetailReq) (*types.OpenCertDetailResp, error) {
	if req.Id == "" {
		return nil, errors.New("缺少证书 id")
	}
	m := model.NewCertModel(l.svcCtx.MongoDB)
	doc, err := m.FindByID(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("证书不存在")
	}
	return &types.OpenCertDetailResp{Code: 0, Msg: "ok", Data: toOpenCert(doc)}, nil
}

func toOpenCert(d *model.Cert) *types.OpenCert {
	if d == nil {
		return nil
	}
	return &types.OpenCert{
		Id:           d.Id.Hex(),
		Host:         d.Host,
		Port:         d.Port,
		Authority:    d.Authority,
		Subject:      types.CertNameInfo(d.Subject),
		SubjectDN:    d.SubjectDN,
		Issuer:       types.CertNameInfo(d.Issuer),
		IssuerDN:     d.IssuerDN,
		SerialNumber: d.SerialNumber,
		SigAlg:       d.SigAlg,
		NotBefore:    fmtTime(d.NotBefore),
		NotAfter:     fmtTime(d.NotAfter),
		Version:      d.Version,
		SANs:         d.SANs,
		Fingerprints: d.Fingerprints,
		IsSelfSigned: d.IsSelfSigned,
		CreateTime:   fmtTime(d.CreateTime),
	}
}
