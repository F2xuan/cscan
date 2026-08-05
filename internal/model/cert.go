package model

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CertNameInfo 证书主体/颁发者的结构化字段（对齐 ARL cert 集合的可检索性）
type CertNameInfo struct {
	Country      string `bson:"country,omitempty" json:"country,omitempty"`
	Province     string `bson:"province,omitempty" json:"province,omitempty"`
	Locality     string `bson:"locality,omitempty" json:"locality,omitempty"`
	Organization string `bson:"organization,omitempty" json:"organization,omitempty"`
	OrgUnit      string `bson:"org_unit,omitempty" json:"orgUnit,omitempty"`
	CommonName   string `bson:"common_name,omitempty" json:"commonName,omitempty"`
	Email        string `bson:"email,omitempty" json:"email,omitempty"`
}

// Cert TLS 证书采集结果（ARL 风格：host+port+task_id 关联，结构化证书详情）。
// 集合命名：{workspaceId}_cert，多租户隔离。
// 不含监控语义（无 status/daysLeft/告警档位），仅作为指纹识别附加产出的证书快照。
type Cert struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WorkspaceId  string             `bson:"workspace_id" json:"workspaceId"`
	TaskId       string             `bson:"task_id,omitempty" json:"taskId,omitempty"`
	Host         string             `bson:"host" json:"host"`
	Port         int                `bson:"port" json:"port"`
	Authority    string             `bson:"authority" json:"authority"` // host:port
	Subject      CertNameInfo       `bson:"subject" json:"subject"`
	SubjectDN    string             `bson:"subject_dn" json:"subjectDN"`
	Issuer       CertNameInfo       `bson:"issuer" json:"issuer"`
	IssuerDN     string             `bson:"issuer_dn" json:"issuerDN"`
	SerialNumber string             `bson:"serial_number" json:"serialNumber"`
	SigAlg       string             `bson:"sig_alg" json:"sigAlg"`
	NotBefore    time.Time          `bson:"not_before" json:"notBefore"`
	NotAfter     time.Time          `bson:"not_after" json:"notAfter"`
	Version      int                `bson:"version" json:"version"`
	SANs         []string           `bson:"sans,omitempty" json:"sans,omitempty"`
	Fingerprints map[string]string  `bson:"fingerprints" json:"fingerprints"` // sha1 / sha256 / md5
	IsSelfSigned bool               `bson:"is_self_signed" json:"isSelfSigned"`
	CreateTime   time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime   time.Time          `bson:"update_time" json:"updateTime"`
}

// CertModel 证书模型
type CertModel struct {
	coll *mongo.Collection
}

// NewCertModel 多租户模型实例化
func NewCertModel(db *mongo.Database, workspaceId string) *CertModel {
	return &CertModel{coll: db.Collection("cert")}
}

// UpsertMany 批量 upsert：按 host+port+serial_number 去重，重复扫描刷新 updateTime。
func (m *CertModel) UpsertMany(ctx context.Context, results []*Cert) error {
	if len(results) == 0 {
		return nil
	}
	now := time.Now()
	models := make([]mongo.WriteModel, 0, len(results))
	for _, r := range results {
		filter := bson.M{
			"host":          r.Host,
			"port":          r.Port,
			"serial_number": r.SerialNumber,
		}
		update := bson.M{
			"$setOnInsert": bson.M{
				"workspace_id": r.WorkspaceId,
				"task_id":      r.TaskId,
				"create_time":  now,
			},
			"$set": bson.M{
				"host":           r.Host,
				"port":           r.Port,
				"authority":      r.Authority,
				"subject":        r.Subject,
				"subject_dn":     r.SubjectDN,
				"issuer":         r.Issuer,
				"issuer_dn":      r.IssuerDN,
				"serial_number":  r.SerialNumber,
				"sig_alg":        r.SigAlg,
				"not_before":     r.NotBefore,
				"not_after":      r.NotAfter,
				"version":        r.Version,
				"sans":           r.SANs,
				"fingerprints":   r.Fingerprints,
				"is_self_signed": r.IsSelfSigned,
				"update_time":    now,
			},
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true))
	}
	opts := options.BulkWrite().SetOrdered(false)
	_, err := m.coll.BulkWrite(ctx, models, opts)
	return err
}

// EnsureIndexes 确保索引存在
func (m *CertModel) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "host", Value: 1},
				{Key: "port", Value: 1},
				{Key: "serial_number", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetBackground(true),
		},
		{Keys: bson.D{{Key: "not_after", Value: 1}}},
		{Keys: bson.D{{Key: "issuer_dn", Value: 1}}},
		{Keys: bson.D{{Key: "subject_dn", Value: 1}}},
		{Keys: bson.D{{Key: "serial_number", Value: 1}}},
		{Keys: bson.D{{Key: "sans", Value: 1}}},
	}
	_, err := m.coll.Indexes().CreateMany(ctx, indexes)
	return err
}

// Find 查询列表
func (m *CertModel) Find(ctx context.Context, filter bson.M, opt *options.FindOptions) ([]*Cert, error) {
	cursor, err := m.coll.Find(ctx, filter, opt)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []*Cert
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindByID 按 _id 取单条
func (m *CertModel) FindByID(ctx context.Context, id string) (*Cert, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc Cert
	if err := m.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Count 按过滤条件统计证书数量
func (m *CertModel) Count(ctx context.Context, filter bson.M) (int64, error) {
	return m.coll.CountDocuments(ctx, filter)
}
