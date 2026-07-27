package model

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Vul struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Authority  string             `bson:"authority" json:"authority"`
	Host       string             `bson:"host" json:"host"`
	Port       int                `bson:"port" json:"port"`
	Url        string             `bson:"url" json:"url"`
	PocFile    string             `bson:"pocfile" json:"pocFile"`
	Source     string             `bson:"source" json:"source"`
	Severity   string             `bson:"severity" json:"severity"` // 严重级别: critical/high/medium/low/info/unknown
	Extra      string             `bson:"extra" json:"extra"`
	Result     string             `bson:"result" json:"result"`
	TaskId     string             `bson:"task_id" json:"taskId"`
	VulName    string             `bson:"vul_name,omitempty" json:"vulName,omitempty"`
	Tags       []string           `bson:"tags,omitempty" json:"tags,omitempty"`
	CreateTime time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime time.Time          `bson:"update_time" json:"updateTime"`

	// 漏洞知识库关联字段
	CvssScore   float64  `bson:"cvss_score,omitempty" json:"cvssScore,omitempty"`
	CveId       string   `bson:"cve_id,omitempty" json:"cveId,omitempty"`
	CweId       string   `bson:"cwe_id,omitempty" json:"cweId,omitempty"`
	Remediation string   `bson:"remediation,omitempty" json:"remediation,omitempty"`
	References  []string `bson:"references,omitempty" json:"references,omitempty"`

	// 证据链字段
	MatcherName       string   `bson:"matcher_name,omitempty" json:"matcherName,omitempty"`
	ExtractedResults  []string `bson:"extracted_results,omitempty" json:"extractedResults,omitempty"`
	CurlCommand       string   `bson:"curl_command,omitempty" json:"curlCommand,omitempty"`
	Request           string   `bson:"request,omitempty" json:"request,omitempty"`
	Response          string   `bson:"response,omitempty" json:"response,omitempty"`
	ResponseTruncated bool     `bson:"response_truncated,omitempty" json:"responseTruncated,omitempty"`

	// 时间追踪字段
	FirstSeenTime time.Time `bson:"first_seen_time,omitempty" json:"firstSeenTime,omitempty"`
	LastSeenTime  time.Time `bson:"last_seen_time,omitempty" json:"lastSeenTime,omitempty"`
	ScanCount     int       `bson:"scan_count,omitempty" json:"scanCount,omitempty"`

	// 风险层标记：is_risk=true 时该漏洞计入"风险"视角，false 时仅在暴露面视角展示
	// 缺失字段视为 false（暴露面视角默认）；risk 层查询显式 filter is_risk=true
	IsRisk         bool      `bson:"is_risk,omitempty" json:"isRisk,omitempty"`
	RiskAssessedAt time.Time `bson:"risk_assessed_at,omitempty" json:"riskAssessedAt,omitempty"`
	RiskSource    string     `bson:"risk_source,omitempty" json:"riskSource,omitempty"` // manual / auto:cvss / auto:weakpass / auto:info-leak
}

type VulModel struct {
	coll *mongo.Collection
}

func NewVulModel(db *mongo.Database, workspaceId string) *VulModel {
	return &VulModel{
		coll: db.Collection(workspaceId + "_vul"),
	}
}

func (m *VulModel) Insert(ctx context.Context, doc *Vul) error {
	if doc.Id.IsZero() {
		doc.Id = primitive.NewObjectID()
	}
	now := time.Now()
	doc.CreateTime = now
	doc.UpdateTime = now
	_, err := m.coll.InsertOne(ctx, doc)
	return err
}

func (m *VulModel) FindById(ctx context.Context, id string) (*Vul, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc Vul
	err = m.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func (m *VulModel) Find(ctx context.Context, filter bson.M, page, pageSize int) ([]Vul, error) {
	opts := options.Find()
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}
	opts.SetSort(bson.D{{Key: "create_time", Value: -1}})
	// 列表场景排除超大字段（请求/响应/curl 命令），列表接口不需要这些证据链数据
	// 详情接口应使用 FindById 获取完整字段
	opts.SetProjection(bson.D{
		{Key: "request", Value: 0},
		{Key: "response", Value: 0},
		{Key: "curl_command", Value: 0},
	})

	cursor, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []Vul
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (m *VulModel) Count(ctx context.Context, filter bson.M) (int64, error) {
	return m.coll.CountDocuments(ctx, filter)
}

// EstimatedCount 使用集合元数据快速估算文档总数（O(1)），仅适用于空 filter 场景
func (m *VulModel) EstimatedCount(ctx context.Context) (int64, error) {
	return m.coll.EstimatedDocumentCount(ctx)
}

type VulSeverityStats struct {
	Total    int64
	Critical int64
	High     int64
	Medium   int64
	Low      int64
	Info     int64
	Week     int64
	Month    int64
}

// AggregateStats 聚合统计漏洞总数、严重级别分布以及近7/30天数量
func (m *VulModel) AggregateStats(ctx context.Context, now time.Time) (*VulSeverityStats, error) {
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)

	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "critical", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$severity", "critical"}}}, 1, 0}}}}}},
			{Key: "high", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$severity", "high"}}}, 1, 0}}}}}},
			{Key: "medium", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$severity", "medium"}}}, 1, 0}}}}}},
			{Key: "low", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$severity", "low"}}}, 1, 0}}}}}},
			{Key: "info", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$severity", "info"}}}, 1, 0}}}}}},
			{Key: "week", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$gte", Value: bson.A{"$create_time", weekAgo}}}, 1, 0}}}}}},
			{Key: "month", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$gte", Value: bson.A{"$create_time", monthAgo}}}, 1, 0}}}}}},
		}}},
	}

	cursor, err := m.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stats := &VulSeverityStats{}
	if cursor.Next(ctx) {
		if err := cursor.Decode(stats); err != nil {
			return nil, err
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// CountByTaskId 根据任务ID统计漏洞数量
func (m *VulModel) CountByTaskId(ctx context.Context, taskId string) (int64, error) {
	return m.coll.CountDocuments(ctx, bson.M{"task_id": taskId})
}

func (m *VulModel) Delete(ctx context.Context, id string) (int64, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return 0, err
	}
	res, err := m.coll.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (m *VulModel) DeleteByTaskId(ctx context.Context, taskId string) error {
	_, err := m.coll.DeleteMany(ctx, bson.M{"task_id": taskId})
	return err
}

// Upsert 插入或更新漏洞（基于 host+port+pocFile+url 去重）
func (m *VulModel) Upsert(ctx context.Context, doc *Vul) error {
	now := time.Now()
	filter := bson.M{
		"host":    doc.Host,
		"port":    doc.Port,
		"pocfile": doc.PocFile,
		"url":     doc.Url,
	}
	update := bson.M{
		"$set": bson.M{
			"authority":   doc.Authority,
			"source":      doc.Source,
			"severity":    doc.Severity,
			"extra":       doc.Extra,
			"result":      doc.Result,
			"task_id":     doc.TaskId,
			"update_time": now,
			// 新增字段 - 漏洞知识库关联
			"cvss_score":  doc.CvssScore,
			"cve_id":      doc.CveId,
			"cwe_id":      doc.CweId,
			"remediation": doc.Remediation,
			"references":  doc.References,
			// 新增字段 - 证据链
			"matcher_name":       doc.MatcherName,
			"extracted_results":  doc.ExtractedResults,
			"curl_command":       doc.CurlCommand,
			"request":            doc.Request,
			"response":           doc.Response,
			"response_truncated": doc.ResponseTruncated,
			// 新增字段 - 漏洞名称和标签
			"vul_name": doc.VulName,
			"tags":     doc.Tags,
			// 新增字段 - 时间追踪
			"last_seen_time": now,
		},
		"$inc": bson.M{
			"scan_count": 1, // 新增：扫描计数
		},
		"$setOnInsert": bson.M{
			"_id":             primitive.NewObjectID(),
			"create_time":     now,
			"first_seen_time": now, // 新增：首次发现时间
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := m.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// DeleteByFilter 按条件批量删除漏洞
func (m *VulModel) DeleteByFilter(ctx context.Context, filter bson.M) (int64, error) {
	res, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// BatchDelete 批量删除漏洞
func (m *VulModel) BatchDelete(ctx context.Context, ids []string) (int64, error) {
	oids := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		if oid, err := primitive.ObjectIDFromHex(id); err == nil {
			oids = append(oids, oid)
		}
	}
	if len(oids) == 0 {
		return 0, nil
	}
	result, err := m.coll.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": oids}})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// Clear 清空所有漏洞
func (m *VulModel) Clear(ctx context.Context) (int64, error) {
	result, err := m.coll.DeleteMany(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// FindByHostPort 根据host和port查找漏洞列表（用于风险评分计算）
func (m *VulModel) FindByHostPort(ctx context.Context, host string, port int) ([]Vul, error) {
	filter := bson.M{
		"host": host,
		"port": port,
	}
	opts := options.Find()
	opts.SetSort(bson.D{{Key: "create_time", Value: -1}})

	cursor, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []Vul
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}
