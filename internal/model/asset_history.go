package model

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ScanResultHistory represents versioned scan results for historical tracking
type ScanResultHistory struct {
	Id              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WorkspaceId     string             `bson:"workspace_id" json:"workspaceId"`
	AssetId         string             `bson:"asset_id,omitempty" json:"assetId,omitempty"`
	Authority       string             `bson:"authority" json:"authority"`
	Host            string             `bson:"host" json:"host"`
	Port            int                `bson:"port" json:"port"`
	VersionId       string             `bson:"version_id" json:"versionId"`
	ScanTimestamp   time.Time          `bson:"scan_timestamp" json:"scanTimestamp"`
	DirScanResults  []DirScanResult    `bson:"dir_scan_results,omitempty" json:"dirScanResults,omitempty"`
	VulnScanResults []ScanResult       `bson:"vuln_scan_results,omitempty" json:"vulnScanResults,omitempty"`
	ChangesSummary  string             `bson:"changes_summary,omitempty" json:"changesSummary,omitempty"`
	ArchivedAt      time.Time          `bson:"archived_at" json:"archivedAt"`
	CreateTime      time.Time          `bson:"create_time" json:"createTime"`
}

// ScanResultHistoryModel provides database operations for ScanResultHistory
type ScanResultHistoryModel struct {
	coll *mongo.Collection
}

// NewScanResultHistoryModel creates a new ScanResultHistoryModel
func NewScanResultHistoryModel(db *mongo.Database, workspaceId string) *ScanResultHistoryModel {
	coll := db.Collection("asset_history")

	// Create indexes
	ctx := context.Background()
	indexes := []mongo.IndexModel{
		// Index for querying by workspace and asset
		{Keys: bson.D{
			{Key: "workspace_id", Value: 1},
			{Key: "asset_id", Value: 1},
			{Key: "scan_timestamp", Value: -1},
		}},
		// Index for querying by workspace, authority, host, port
		{Keys: bson.D{
			{Key: "workspace_id", Value: 1},
			{Key: "authority", Value: 1},
			{Key: "host", Value: 1},
			{Key: "port", Value: 1},
			{Key: "scan_timestamp", Value: -1},
		}},
		// Index for scan_timestamp to support temporal queries
		{Keys: bson.D{{Key: "scan_timestamp", Value: -1}}},
		// Index for archived_at
		{Keys: bson.D{{Key: "archived_at", Value: -1}}},
		// Index for version_id
		{Keys: bson.D{{Key: "version_id", Value: 1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &ScanResultHistoryModel{
		coll: coll,
	}
}

// Insert inserts a new historical scan result record
func (m *ScanResultHistoryModel) Insert(ctx context.Context, doc *ScanResultHistory) error {
	if doc.Id.IsZero() {
		doc.Id = primitive.NewObjectID()
	}
	now := time.Now()
	if doc.CreateTime.IsZero() {
		doc.CreateTime = now
	}
	if doc.ArchivedAt.IsZero() {
		doc.ArchivedAt = now
	}
	_, err := m.coll.InsertOne(ctx, doc)
	return err
}

// FindByAssetId retrieves historical scan results for a specific asset
func (m *ScanResultHistoryModel) FindByAssetId(ctx context.Context, workspaceId, assetId string, limit int) ([]ScanResultHistory, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"asset_id":     assetId,
	}
	return m.findWithFilter(ctx, filter, limit)
}

// FindByAuthority retrieves historical scan results by authority, host, and port
func (m *ScanResultHistoryModel) FindByAuthority(ctx context.Context, workspaceId, authority, host string, port int, limit int) ([]ScanResultHistory, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"authority":    authority,
		"host":         host,
		"port":         port,
	}
	return m.findWithFilter(ctx, filter, limit)
}

// FindByTimeRange retrieves historical scan results within a time range
func (m *ScanResultHistoryModel) FindByTimeRange(ctx context.Context, workspaceId, authority, host string, port int, startTime, endTime time.Time) ([]ScanResultHistory, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"authority":    authority,
		"host":         host,
		"port":         port,
		"scan_timestamp": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}
	return m.findWithFilter(ctx, filter, 0)
}

// FindByVersionId retrieves a specific version by version ID
func (m *ScanResultHistoryModel) FindByVersionId(ctx context.Context, workspaceId, versionId string) (*ScanResultHistory, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"version_id":   versionId,
	}
	var doc ScanResultHistory
	err := m.coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// findWithFilter is a helper method for querying with filters
func (m *ScanResultHistoryModel) findWithFilter(ctx context.Context, filter bson.M, limit int) ([]ScanResultHistory, error) {
	opts := options.Find()
	opts.SetSort(bson.D{{Key: "scan_timestamp", Value: -1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []ScanResultHistory
	for cursor.Next(ctx) {
		var doc ScanResultHistory
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

// Count counts historical records matching the filter
func (m *ScanResultHistoryModel) Count(ctx context.Context, filter bson.M) (int64, error) {
	return m.coll.CountDocuments(ctx, filter)
}

// CountByAssetId counts historical records for a specific asset
func (m *ScanResultHistoryModel) CountByAssetId(ctx context.Context, workspaceId, assetId string) (int64, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"asset_id":     assetId,
	}
	return m.Count(ctx, filter)
}

// CountByAuthority counts historical records by authority, host, and port
func (m *ScanResultHistoryModel) CountByAuthority(ctx context.Context, workspaceId, authority, host string, port int) (int64, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"authority":    authority,
		"host":         host,
		"port":         port,
	}
	return m.Count(ctx, filter)
}

// Delete deletes a historical record by ID
func (m *ScanResultHistoryModel) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = m.coll.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

// DeleteByAssetId deletes all historical records for a specific asset
func (m *ScanResultHistoryModel) DeleteByAssetId(ctx context.Context, workspaceId, assetId string) (int64, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"asset_id":     assetId,
	}
	result, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// DeleteByAuthority deletes all historical records by authority, host, and port
func (m *ScanResultHistoryModel) DeleteByAuthority(ctx context.Context, workspaceId, authority, host string, port int) (int64, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"authority":    authority,
		"host":         host,
		"port":         port,
	}
	result, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// DeleteOlderThan deletes historical records older than the specified time
func (m *ScanResultHistoryModel) DeleteOlderThan(ctx context.Context, workspaceId string, olderThan time.Time) (int64, error) {
	filter := bson.M{
		"workspace_id": workspaceId,
		"scan_timestamp": bson.M{
			"$lt": olderThan,
		},
	}
	result, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// Clear clears all historical records for a workspace
func (m *ScanResultHistoryModel) Clear(ctx context.Context, workspaceId string) (int64, error) {
	filter := bson.M{"workspace_id": workspaceId}
	result, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// DeleteByFilter deletes historical records matching the filter
func (m *ScanResultHistoryModel) DeleteByFilter(ctx context.Context, workspaceId string, filter bson.M) (int64, error) {
	// 添加 workspace_id 到过滤条件
	filter["workspace_id"] = workspaceId
	result, err := m.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
