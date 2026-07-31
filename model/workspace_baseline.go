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

// DefaultNewAssetWindowDays 默认"新增窗口"天数：首次发现后 N 天内视为新增资产。
const DefaultNewAssetWindowDays = 7

// WorkspaceBaseline 记录每个工作空间的首次扫描基线，用于抑制首次扫描的全量新增通知（G2）。
// 集合: workspace_baseline（全局单集合 + workspace_id 过滤，参照 crontask 模式）。
type WorkspaceBaseline struct {
	Id                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WorkspaceId          string             `bson:"workspace_id" json:"workspaceId"`
	BaselineEstablishedAt time.Time          `bson:"baseline_established_at,omitempty" json:"baselineEstablishedAt,omitempty"`
	BaselineTaskId       string             `bson:"baseline_task_id,omitempty" json:"baselineTaskId,omitempty"`
	NewAssetWindowDays   int                `bson:"new_asset_window_days" json:"newAssetWindowDays"`
	CreateTime           time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime           time.Time          `bson:"update_time" json:"updateTime"`
}

// IsEstablished 基线是否已建立（非首次扫描）
func (b *WorkspaceBaseline) IsEstablished() bool {
	return b != nil && !b.BaselineEstablishedAt.IsZero()
}

// WindowDays 返回新增窗口天数（未配置时取默认）
func (b *WorkspaceBaseline) WindowDays() int {
	if b == nil || b.NewAssetWindowDays <= 0 {
		return DefaultNewAssetWindowDays
	}
	return b.NewAssetWindowDays
}

type WorkspaceBaselineModel struct {
	coll *mongo.Collection
}

// NewWorkspaceBaselineModel 创建基线模型（全局单集合 workspace_baseline）
func NewWorkspaceBaselineModel(db *mongo.Database) *WorkspaceBaselineModel {
	coll := db.Collection("workspace_baseline")
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "workspace_id", Value: 1}}},
	}
	if err := ensureIndexes(coll, indexes); err != nil {
		logx.Errorf("[WorkspaceBaseline] ensureIndexes failed: %v", err)
	}
	return &WorkspaceBaselineModel{coll: coll}
}

// Get 返回工作空间基线；不存在时返回 nil（表示尚未建立）
func (m *WorkspaceBaselineModel) Get(ctx context.Context, workspaceId string) (*WorkspaceBaseline, error) {
	var doc WorkspaceBaseline
	err := m.coll.FindOne(ctx, bson.M{"workspace_id": workspaceId}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// Establish 建立基线（首次扫描完成时调用），幂等（重复调用仅刷新时间戳）。
// 返回建立后的基线。
func (m *WorkspaceBaselineModel) Establish(ctx context.Context, workspaceId, taskId string) (*WorkspaceBaseline, error) {
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"baseline_established_at": now,
			"baseline_task_id":       taskId,
			"new_asset_window_days":  DefaultNewAssetWindowDays,
			"update_time":            now,
		},
		"$setOnInsert": bson.M{
			"workspace_id": workspaceId,
			"create_time": now,
		},
	}
	opts := options.Update().SetUpsert(true)
	if _, err := m.coll.UpdateOne(ctx, bson.M{"workspace_id": workspaceId}, update, opts); err != nil {
		return nil, err
	}
	return m.Get(ctx, workspaceId)
}
