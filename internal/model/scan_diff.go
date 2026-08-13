package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ScanDiff 记录某次扫描相对上一次任务的变化（新增/更新/已修复）。
// 它是"变化基线"的持久实体，替代原先只能靠 new=true 反查的近似做法（G1）。
// 集合: scan_diff
type ScanDiff struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskId      string             `bson:"task_id" json:"taskId"`
	DiffType    string             `bson:"diff_type" json:"diffType"`   // asset / vul
	ChangeType  string             `bson:"change_type" json:"changeType"` // added / updated / resolved
	Severity    string             `bson:"severity,omitempty" json:"severity,omitempty"` // 风险严重级别（漏洞变化记录携带，供 T1.4 排序/展示）
	TargetKey   string             `bson:"target_key" json:"targetKey"` // 资产 authority / 漏洞 host:port:pocfile
	Summary     string             `bson:"summary,omitempty" json:"summary,omitempty"`
	Changes     []FieldChange      `bson:"changes,omitempty" json:"changes,omitempty"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
}

// 变化类型常量
const (
	ScanDiffTypeAsset = "asset"
	ScanDiffTypeVul   = "vul"

	ScanDiffChangeAdded    = "added"
	ScanDiffChangeUpdated  = "updated"
	ScanDiffChangeResolved = "resolved"
)

// ScanDiffRetentionDays 默认保留期（天），可被配置覆盖
const ScanDiffRetentionDays = 90

// ScanDiffStatItem 聚合统计项
type ScanDiffStatItem struct {
	DiffType   string `bson:"diffType" json:"diffType"`
	ChangeType string `bson:"changeType" json:"changeType"`
	Count      int64  `bson:"count" json:"count"`
}

type ScanDiffModel struct {
	coll *mongo.Collection
}

// NewScanDiffModel 创建变化快照模型。
// 索引通过 ensureIndexes 创建（与 §1.3 约定一致，不登记到 model/indexes.go 死代码）。
func NewScanDiffModel(db *mongo.Database) *ScanDiffModel {
	coll := db.Collection("scan_diff")
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "task_id", Value: 1}}},
		{Keys: bson.D{{Key: "create_time", Value: -1}}},
		{Keys: bson.D{{Key: "change_type", Value: 1}, {Key: "create_time", Value: -1}}},
	}
	if err := ensureIndexes(coll, indexes); err != nil {
		logx.Errorf("[ScanDiff] ensureIndexes failed: %v", err)
	}
	return &ScanDiffModel{coll: coll}
}

// Insert 插入单条变化记录
func (m *ScanDiffModel) Insert(ctx context.Context, doc *ScanDiff) error {
	if doc.Id.IsZero() {
		doc.Id = primitive.NewObjectID()
	}
	if doc.CreateTime.IsZero() {
		doc.CreateTime = time.Now()
	}
	_, err := m.coll.InsertOne(ctx, doc)
	return err
}

// BatchInsert 批量插入变化记录（单次请求内聚合后批量写，避免逐条往返）
func (m *ScanDiffModel) BatchInsert(ctx context.Context, docs []ScanDiff) error {
	if len(docs) == 0 {
		return nil
	}
	now := time.Now()
	models := make([]mongo.WriteModel, 0, len(docs))
	for i := range docs {
		if docs[i].Id.IsZero() {
			docs[i].Id = primitive.NewObjectID()
		}
		if docs[i].CreateTime.IsZero() {
			docs[i].CreateTime = now
		}
		models = append(models, mongo.NewInsertOneModel().SetDocument(docs[i]))
	}
	_, err := m.coll.BulkWrite(ctx, models)
	return err
}

// FindByTaskId 按任务查询变化明细，支持按 diff_type / change_type 过滤与分页
func (m *ScanDiffModel) FindByTaskId(ctx context.Context, taskId, diffType, changeType string, page, pageSize int64) ([]ScanDiff, int64, error) {
	page, pageSize = NormalizePage(page, pageSize)
	filter := bson.M{"task_id": taskId}
	if diffType != "" {
		filter["diff_type"] = diffType
	}
	if changeType != "" {
		filter["change_type"] = changeType
	}
	total, err := m.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "create_time", Value: -1}})
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize)).SetLimit(int64(pageSize))
	}
	cur, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var docs []ScanDiff
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	return docs, total, nil
}

// CountByTaskIdAndType 统计某任务下指定类型的变化条数（用于通知口径对齐）
func (m *ScanDiffModel) CountByTaskIdAndType(ctx context.Context, taskId, diffType, changeType string) (int64, error) {
	filter := bson.M{"task_id": taskId}
	if diffType != "" {
		filter["diff_type"] = diffType
	}
	if changeType != "" {
		filter["change_type"] = changeType
	}
	return m.coll.CountDocuments(ctx, filter)
}

// FindByTimeRange 按时间范围查询变化记录
func (m *ScanDiffModel) FindByTimeRange(ctx context.Context, start, end time.Time, diffType, changeType string) ([]ScanDiff, error) {
	filter := bson.M{}
	if !start.IsZero() || !end.IsZero() {
		tr := bson.M{}
		if !start.IsZero() {
			tr["$gte"] = start
		}
		if !end.IsZero() {
			tr["$lte"] = end
		}
		filter["create_time"] = tr
	}
	if diffType != "" {
		filter["diff_type"] = diffType
	}
	if changeType != "" {
		filter["change_type"] = changeType
	}
	opts := options.Find().SetSort(bson.D{{Key: "create_time", Value: -1}})
	cur, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []ScanDiff
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// Stat 按 diff_type + change_type 聚合计数（用于工作台/通知统计）
func (m *ScanDiffModel) Stat(ctx context.Context, start, end time.Time) ([]ScanDiffStatItem, error) {
	match := bson.M{}
	if !start.IsZero() || !end.IsZero() {
		tr := bson.M{}
		if !start.IsZero() {
			tr["$gte"] = start
		}
		if !end.IsZero() {
			tr["$lte"] = end
		}
		match["create_time"] = tr
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "diff_type", Value: "$diff_type"},
				{Key: "change_type", Value: "$change_type"},
			}},
			{Key: "count", Value: bson.M{"$sum": 1}},
		}}},
	}
	cur, err := m.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type groupResult struct {
		Id struct {
			DiffType   string `bson:"diff_type"`
			ChangeType string `bson:"change_type"`
		} `bson:"_id"`
		Count int64 `bson:"count"`
	}
	var groups []groupResult
	if err := cur.All(ctx, &groups); err != nil {
		return nil, err
	}
	items := make([]ScanDiffStatItem, 0, len(groups))
	for _, g := range groups {
		items = append(items, ScanDiffStatItem{
			DiffType:   g.Id.DiffType,
			ChangeType: g.Id.ChangeType,
			Count:      g.Count,
		})
	}
	return items, nil
}

// DeleteOlderThan 清理超过保留期的变化记录，返回删除条数
func (m *ScanDiffModel) DeleteOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	filter := bson.M{
		"create_time": bson.M{"$lt": olderThan},
	}
	res, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}
