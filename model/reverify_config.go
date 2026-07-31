package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ReverifyConfig 复验配置（T3.3 弱口令 / T3.4 敏感信息持续复验共用，单集合 + workspace_id 隔离）。
// 与 cert_monitor_config 同范式：全局单集合，按 workspace_id 唯一。
type ReverifyConfig struct {
	Id               interface{} `bson:"_id,omitempty" json:"id"`
	WorkspaceId      string      `bson:"workspace_id" json:"workspaceId"`
	WeakPassEnabled  bool        `bson:"weakpass_enabled" json:"weakPassEnabled"`     // 弱口令复验启用
	ExposureEnabled  bool        `bson:"exposure_enabled" json:"exposureEnabled"`     // 敏感信息复验启用（T3.4）
	CronSpec         string      `bson:"cron_spec" json:"cronSpec"`                   // 周期（默认每日）
	MaxTargetsPerRun int         `bson:"max_targets_per_run" json:"maxTargetsPerRun"` // 单次复验目标上限（超出下个周期继续）
	Concurrency      int         `bson:"concurrency" json:"concurrency"`              // 并发上限
	LastRunTime      time.Time   `bson:"last_run_time,omitempty" json:"lastRunTime,omitempty"`
	LastRunStatus    string      `bson:"last_run_status,omitempty" json:"lastRunStatus,omitempty"` // success/failed/partial
	LastRunCount     int         `bson:"last_run_count,omitempty" json:"lastRunCount,omitempty"`
	LastRunError     string      `bson:"last_run_error,omitempty" json:"lastRunError,omitempty"`
	NextRunTime      time.Time   `bson:"next_run_time,omitempty" json:"nextRunTime,omitempty"`
	CreateTime       time.Time   `bson:"create_time" json:"createTime"`
	UpdateTime       time.Time   `bson:"update_time" json:"updateTime"`
}

// ReverifyConfigModel 复验配置模型
type ReverifyConfigModel struct {
	coll *mongo.Collection
}

// NewReverifyConfigModel 创建复验配置模型（集合 reverify_config，workspace_id 唯一索引）
func NewReverifyConfigModel(db *mongo.Database) *ReverifyConfigModel {
	coll := db.Collection("reverify_config")
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "workspace_id", Value: 1}}, Options: options.Index().SetUnique(true).SetBackground(true)},
	}
	if err := ensureIndexes(coll, indexes); err != nil {
		logx.Errorf("[ReverifyConfigModel] ensureIndexes failed: %v", err)
	}
	return &ReverifyConfigModel{coll: coll}
}

// GetByWorkspace 按工作空间获取复验配置（不存在返回 nil, nil）
func (m *ReverifyConfigModel) GetByWorkspace(ctx context.Context, workspaceId string) (*ReverifyConfig, error) {
	var cfg ReverifyConfig
	err := m.coll.FindOne(ctx, bson.M{"workspace_id": workspaceId}).Decode(&cfg)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

// Upsert 按 workspace_id 写入（存在则更新，不存在则插入）
// 修复 H-7：_id 和 create_time 放入 $setOnInsert（仅插入时设置），可变字段单独 $set。
// 原实现每次 Upsert 都生成新 ObjectID 并通过 $set 写入整个 cfg，对已存在文档会触发
// immutable field '_id' 错误，导致更新失败。
func (m *ReverifyConfigModel) Upsert(ctx context.Context, cfg *ReverifyConfig) error {
	now := time.Now()
	cfg.UpdateTime = now

	// 可变字段（每次更新都写入）
	setFields := bson.M{
		"workspace_id":        cfg.WorkspaceId,
		"weakpass_enabled":    cfg.WeakPassEnabled,
		"exposure_enabled":    cfg.ExposureEnabled,
		"cron_spec":           cfg.CronSpec,
		"max_targets_per_run": cfg.MaxTargetsPerRun,
		"concurrency":         cfg.Concurrency,
		"update_time":         now,
	}
	// 允许更新上次运行状态（若调用方传入了值）
	if !cfg.LastRunTime.IsZero() {
		setFields["last_run_time"] = cfg.LastRunTime
	}
	if cfg.LastRunStatus != "" {
		setFields["last_run_status"] = cfg.LastRunStatus
	}
	if cfg.LastRunCount != 0 {
		setFields["last_run_count"] = cfg.LastRunCount
	}
	if cfg.LastRunError != "" {
		setFields["last_run_error"] = cfg.LastRunError
	}
	if !cfg.NextRunTime.IsZero() {
		setFields["next_run_time"] = cfg.NextRunTime
	}

	// 仅插入时设置的不可变字段
	setOnInsert := bson.M{
		"create_time": now,
	}
	if cfg.CreateTime.IsZero() {
		setOnInsert["create_time"] = now
	} else {
		setOnInsert["create_time"] = cfg.CreateTime
	}

	_, err := m.coll.UpdateOne(ctx,
		bson.M{"workspace_id": cfg.WorkspaceId},
		bson.M{
			"$set":         setFields,
			"$setOnInsert": setOnInsert,
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// FindEnabledWeakPass 返回弱口令复验启用的配置（遍历复验）
func (m *ReverifyConfigModel) FindEnabledWeakPass(ctx context.Context) ([]ReverifyConfig, error) {
	cursor, err := m.coll.Find(ctx, bson.M{"weakpass_enabled": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var cfgs []ReverifyConfig
	if err = cursor.All(ctx, &cfgs); err != nil {
		return nil, err
	}
	return cfgs, nil
}

// FindEnabledExposure 返回敏感信息（暴露面）复验启用的配置（T3.4，与弱口令共用集合与端点）
func (m *ReverifyConfigModel) FindEnabledExposure(ctx context.Context) ([]ReverifyConfig, error) {
	cursor, err := m.coll.Find(ctx, bson.M{"exposure_enabled": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var cfgs []ReverifyConfig
	if err = cursor.All(ctx, &cfgs); err != nil {
		return nil, err
	}
	return cfgs, nil
}

// UpdateRunState 回写复验运行状态（不触碰配置字段与密钥，满足隔离约束）
func (m *ReverifyConfigModel) UpdateRunState(ctx context.Context, workspaceId string, lastRunTime time.Time, status string, count int, runErr string, nextRunTime time.Time) error {
	_, err := m.coll.UpdateOne(ctx,
		bson.M{"workspace_id": workspaceId},
		bson.M{"$set": bson.M{
			"last_run_time":   lastRunTime,
			"last_run_status": status,
			"last_run_count":  count,
			"last_run_error":  runErr,
			"next_run_time":   nextRunTime,
		}},
		options.Update().SetUpsert(false),
	)
	return err
}
