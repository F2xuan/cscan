package logic

import (
	"context"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const assetTargetCertsLimit = 500

type AssetTargetCertsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetTargetCertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetTargetCertsLogic {
	return &AssetTargetCertsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetTargetCerts 目标关联的 TLS 证书列表（cert 集合按 host 过滤），
// 支撑详情页 Inventory 的 TLS 子 Tab 与 Services 列的证书徽章。
func (l *AssetTargetCertsLogic) AssetTargetCerts(req *types.AssetTargetCertsReq) (*types.AssetTargetCertsResp, error) {
	if req.TargetId == "" {
		return nil, xerr.NewParamError("targetId is empty")
	}
	tType, tValue, err := model.DecodeTargetID(req.TargetId)
	if err != nil {
		return nil, err
	}

	certModel := l.svcCtx.GetCertModel()
	if certModel == nil {
		return nil, xerr.NewServerError("cert model not available")
	}

	filter := bson.M{"host": hostFilterForTarget(tType, tValue)}
	if q := strings.TrimSpace(req.Query); q != "" {
		filter["$or"] = bson.A{
			bson.M{"host": bson.M{"$regex": ".*" + regexpEscape(q) + ".*", "$options": "i"}},
			bson.M{"subject_dn": bson.M{"$regex": ".*" + regexpEscape(q) + ".*", "$options": "i"}},
			bson.M{"issuer_dn": bson.M{"$regex": ".*" + regexpEscape(q) + ".*", "$options": "i"}},
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "not_after", Value: -1}}).
		SetLimit(assetTargetCertsLimit)
	docs, err := certModel.Find(l.ctx, filter, opts)
	if err != nil {
		l.Logger.Errorf("[AssetTargetCerts] Find fail: %v", err)
		return nil, xerr.NewServerError("")
	}

	now := time.Now()
	list := make([]types.AssetTargetCertItem, 0, len(docs))
	for _, c := range docs {
		status := "valid"
		switch {
		case !c.NotAfter.IsZero() && c.NotAfter.Before(now):
			status = "expired"
		case !c.NotAfter.IsZero() && c.NotAfter.Before(now.Add(30*24*time.Hour)):
			status = "expiring"
		}
		sans := c.SANs
		if sans == nil {
			sans = []string{}
		}
		list = append(list, types.AssetTargetCertItem{
			Id:         c.Id.Hex(),
			Host:       c.Host,
			Port:       c.Port,
			Authority:  c.Authority,
			SubjectCN:  c.Subject.CommonName,
			SubjectDN:  c.SubjectDN,
			IssuerOrg:  c.Issuer.Organization,
			IssuerDN:   c.IssuerDN,
			SigAlg:     c.SigAlg,
			NotBefore:  tsMilli(c.NotBefore),
			NotAfter:   tsMilli(c.NotAfter),
			SANs:       sans,
			Status:     status,
			SelfSigned: c.IsSelfSigned,
			CreateTime: tsMilli(c.CreateTime),
		})
	}
	return &types.AssetTargetCertsResp{Code: 0, Msg: "success", List: list}, nil
}
