package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AssetTargetType 顶层资产类型
type AssetTargetType string

const (
	AssetTargetTypeDomain AssetTargetType = "domain"
	AssetTargetTypeIP     AssetTargetType = "ip"
)

// AssetTargetMeta 顶层资产元信息（仅存用户附加字段，业务数据仍在 {wsId}_asset / {wsId}_vul 等集合）
//
// _id 编码为 "{type}:{value}"，例如 "domain:example.com" / "ip:1.2.3.4"，
// 保证 (type, value) 全局唯一且 IP/主域名严格分开。
type AssetTargetMeta struct {
	Id           string    `bson:"_id" json:"id"`
	WorkspaceId  string    `bson:"workspace_id" json:"workspaceId"`
	TargetType   string    `bson:"target_type" json:"targetType"`
	TargetValue  string    `bson:"target_value" json:"targetValue"`
	Labels       []string  `bson:"labels,omitempty" json:"labels"`
	Memo         string    `bson:"memo,omitempty" json:"memo"`
	ColorTag     string    `bson:"color_tag,omitempty" json:"colorTag"`
	LastScanTime time.Time `bson:"last_scan_time,omitempty" json:"lastScanTime"`
	FirstSeenTime time.Time `bson:"first_seen_time,omitempty" json:"firstSeenTime"`
	TaskCount    int       `bson:"task_count,omitempty" json:"taskCount"`
	CreateTime   time.Time `bson:"create_time" json:"createTime"`
	UpdateTime   time.Time `bson:"update_time" json:"updateTime"`

	// Phase 4 denormalize 字段：list 行内联 exposure/risk 气泡。
	// risk_updated_at 控制是否需要懒回填（>maxAge 或零值即转 detail 实时算并写回）。
	ExposureSubdomains int       `bson:"exp_subdomains,omitempty" json:"expSubdomains,omitempty"`
	ExposureIps        int       `bson:"exp_ips,omitempty"        json:"expIps,omitempty"`
	ExposurePorts      int       `bson:"exp_ports,omitempty"      json:"expPorts,omitempty"`
	ExposureSites      int       `bson:"exp_sites,omitempty"      json:"expSites,omitempty"`
	ExposureIcons      int       `bson:"exp_icons,omitempty"      json:"expIcons,omitempty"`
	ExposureApps       int       `bson:"exp_apps,omitempty"       json:"expApps,omitempty"`
	ExposureDirs       int       `bson:"exp_dirs,omitempty"       json:"expDirs,omitempty"`
	ExposureJs         int       `bson:"exp_js,omitempty"         json:"expJs,omitempty"`
	ExposureScreenshots int      `bson:"exp_screenshots,omitempty" json:"expScreenshots,omitempty"`
	RiskSensitiveInfo  int       `bson:"risk_sensitive_info,omitempty" json:"riskSensitiveInfo,omitempty"`
	RiskSensitiveDir   int       `bson:"risk_sensitive_dir,omitempty"  json:"riskSensitiveDir,omitempty"`
	RiskVulnHigh       int       `bson:"risk_vuln_high,omitempty"      json:"riskVulnHigh,omitempty"`
	RiskVulnTotal      int       `bson:"risk_vuln_total,omitempty"     json:"riskVulnTotal,omitempty"`
	RiskUpdatedAt      time.Time `bson:"risk_updated_at,omitempty"      json:"riskUpdatedAt,omitempty"`
}

// EncodeTargetID 将 (type, value) 编码为 _id。RFC 3986 限定 host 不含 ':'，
// 此处再防御性 replace 以避免边界数据意外碰撞。
func EncodeTargetID(t AssetTargetType, value string) string {
	v := strings.ReplaceAll(value, ":", "_")
	return fmt.Sprintf("%s:%s", t, v)
}

// DecodeTargetID 反解 _id 得到 (type, value)。若格式非法返回错误。
func DecodeTargetID(id string) (AssetTargetType, string, error) {
	idx := strings.Index(id, ":")
	if idx <= 0 || idx == len(id)-1 {
		return "", "", errors.New("invalid target id format")
	}
	t := AssetTargetType(id[:idx])
	if t != AssetTargetTypeDomain && t != AssetTargetTypeIP {
		return "", "", fmt.Errorf("unknown target type %q", t)
	}
	return t, id[idx+1:], nil
}

type AssetTargetMetaModel struct {
	coll *mongo.Collection
}

func NewAssetTargetMetaModel(db *mongo.Database, workspaceId string) *AssetTargetMetaModel {
	collName := workspaceId + "_asset_target_meta"
	coll := db.Collection(collName)

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "target_type", Value: 1}, {Key: "target_value", Value: 1}}},
		{Keys: bson.D{{Key: "labels", Value: 1}}},
		{Keys: bson.D{{Key: "last_scan_time", Value: -1}}},
	}
	if err := ensureIndexes(coll, indexes); err != nil {
		logx.Errorf("[AssetTargetMetaModel] create indexes failed for %s: %v", coll.Name(), err)
	}
	return &AssetTargetMetaModel{coll: coll}
}

func (m *AssetTargetMetaModel) FindByID(ctx context.Context, id string) (*AssetTargetMeta, error) {
	var doc AssetTargetMeta
	if err := m.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func (m *AssetTargetMetaModel) FindByIDs(ctx context.Context, ids []string) ([]AssetTargetMeta, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cursor, err := m.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []AssetTargetMeta
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// Upsert 创建或更新顶层资产元信息。doc.Id 与 doc.TargetType/TargetValue 必须一致；
// 调用方应通过 EncodeTargetID 构造 Id。
func (m *AssetTargetMetaModel) Upsert(ctx context.Context, doc *AssetTargetMeta) error {
	if doc.Id == "" {
		return errors.New("asset target meta id is empty")
	}
	if doc.TargetType != string(AssetTargetTypeDomain) && doc.TargetType != string(AssetTargetTypeIP) {
		return fmt.Errorf("invalid target type %q", doc.TargetType)
	}
	now := time.Now()
	setFields := bson.M{
		"target_type":    doc.TargetType,
		"target_value":   doc.TargetValue,
		"workspace_id":   doc.WorkspaceId,
		"update_time":   now,
	}
	if doc.Labels != nil {
		setFields["labels"] = doc.Labels
	}
	if doc.Memo != "" {
		setFields["memo"] = doc.Memo
	}
	if doc.ColorTag != "" {
		setFields["color_tag"] = doc.ColorTag
	}
	if !doc.LastScanTime.IsZero() {
		setFields["last_scan_time"] = doc.LastScanTime
	}
	if !doc.FirstSeenTime.IsZero() {
		setFields["first_seen_time"] = doc.FirstSeenTime
	}
	if doc.TaskCount > 0 {
		setFields["task_count"] = doc.TaskCount
	}
	update := bson.M{
		"$set": setFields,
		"$setOnInsert": bson.M{
			"create_time": now,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := m.coll.UpdateOne(ctx, bson.M{"_id": doc.Id}, update, opts)
	return err
}

// UpdateLabels 仅更新标签
func (m *AssetTargetMetaModel) UpdateLabels(ctx context.Context, id string, labels []string) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"labels": labels, "update_time": time.Now()}})
	return err
}

func (m *AssetTargetMetaModel) AddLabel(ctx context.Context, id, label string) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$addToSet": bson.M{"labels": label}, "$set": bson.M{"update_time": time.Now()}})
	return err
}

func (m *AssetTargetMetaModel) RemoveLabel(ctx context.Context, id, label string) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$pull": bson.M{"labels": label}, "$set": bson.M{"update_time": time.Now()}})
	return err
}

func (m *AssetTargetMetaModel) UpdateMemo(ctx context.Context, id, memo string) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"memo": memo, "update_time": time.Now()}})
	return err
}

func (m *AssetTargetMetaModel) UpdateColorTag(ctx context.Context, id, color string) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"color_tag": color, "update_time": time.Now()}})
	return err
}

func (m *AssetTargetMetaModel) UpdateLastScanTime(ctx context.Context, id string, t time.Time) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"last_scan_time": t, "update_time": time.Now()}})
	return err
}

// ExposureSnapshot 是 list/detail 调用方向 model 传递暴露面计数的轻量结构，
// 与 api/internal/types.AssetTargetExposureStats 同形但保持在 model 包避免循环依赖。
type ExposureSnapshot struct {
	Subdomains  int
	Ips         int
	Ports       int
	Sites       int
	Icons       int
	Apps        int
	Dirs        int
	Js          int
	Screenshots int
}

// RiskSnapshot 与 api/internal/types.AssetTargetRiskStats 同形（不含 top-N 列表字段，仅计数）。
type RiskSnapshot struct {
	SensitiveInfo int
	SensitiveDir  int
	VulnHigh      int
	VulnTotal     int
}

// UpdateDenormalized 把暴露面 + 风险计数 + risk_updated_at=now 一次性 $set 写回 meta 文档。
// 供 list 懒回填 / detail 顺带回填 / 迁移脚本调用，避免 list 行 N 次 detail 计算。
func (m *AssetTargetMetaModel) UpdateDenormalized(ctx context.Context, id string, exp ExposureSnapshot, risk RiskSnapshot) error {
	now := time.Now()
	setFields := bson.M{
		"exp_subdomains":     exp.Subdomains,
		"exp_ips":            exp.Ips,
		"exp_ports":          exp.Ports,
		"exp_sites":          exp.Sites,
		"exp_icons":          exp.Icons,
		"exp_apps":           exp.Apps,
		"exp_dirs":           exp.Dirs,
		"exp_js":             exp.Js,
		"exp_screenshots":    exp.Screenshots,
		"risk_sensitive_info": risk.SensitiveInfo,
		"risk_sensitive_dir":  risk.SensitiveDir,
		"risk_vuln_high":      risk.VulnHigh,
		"risk_vuln_total":     risk.VulnTotal,
		"risk_updated_at":    now,
		"update_time":        now,
	}
	_, err := m.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": setFields})
	return err
}

// NeedsRefresh 判断 meta 文档是否需要重新算 denormalize 字段。
// 触发条件：risk_updated_at 零值、超过 maxAge、或任意关键计数字段为 0（说明未初始化）。
func NeedsRefresh(d *AssetTargetMeta, maxAge time.Duration) bool {
	if d == nil {
		return true
	}
	if d.RiskUpdatedAt.IsZero() {
		return true
	}
	if time.Since(d.RiskUpdatedAt) > maxAge {
		return true
	}
	// 字段缺失（未初始化）也视为需要刷新
	return d.ExposureSites == 0 && d.ExposurePorts == 0 && d.RiskVulnTotal == 0
}

func (m *AssetTargetMetaModel) Delete(ctx context.Context, id string) error {
	_, err := m.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (m *AssetTargetMetaModel) DeleteByFilter(ctx context.Context, filter bson.M) (int64, error) {
	res, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// FindAll 分页查询，支持按 type/labels/关键字过滤
func (m *AssetTargetMetaModel) FindAll(ctx context.Context, targetType, query string, labels []string, page, pageSize int) ([]AssetTargetMeta, int64, error) {
	filter := bson.M{}
	if targetType != "" {
		filter["target_type"] = targetType
	}
	if len(labels) > 0 {
		filter["labels"] = bson.M{"$in": labels}
	}
	if query != "" {
		filter["target_value"] = bson.M{"$regex": query, "$options": "i"}
	}

	total, err := m.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().SetSort(bson.D{{Key: "update_time", Value: -1}})
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}

	cursor, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var docs []AssetTargetMeta
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	return docs, total, nil
}
