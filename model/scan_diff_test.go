package model

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// newTestScanDiffModel 仅在设置了 CSCAN_TEST_MONGO_URI 时连接测试库，否则返回 nil。
// 避免在无 MongoDB 的环境下让 model 包测试直接失败。
func newTestScanDiffModel(t *testing.T) (*ScanDiffModel, func()) {
	uri := os.Getenv("CSCAN_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("CSCAN_TEST_MONGO_URI not set, skip ScanDiff DB test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	db := client.Database("cscan_test")
	cleanup := func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}
	return NewScanDiffModel(db, "default"), cleanup
}

func TestScanDiff_DeleteOlderThan(t *testing.T) {
	m, cleanup := newTestScanDiffModel(t)
	if m == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	old := now.Add(-100 * 24 * time.Hour)

	docs := []ScanDiff{
		{TaskId: "t1", WorkspaceId: "default", DiffType: ScanDiffTypeAsset, ChangeType: ScanDiffChangeAdded, TargetKey: "a.example.com:80", CreateTime: old},
		{TaskId: "t1", WorkspaceId: "default", DiffType: ScanDiffTypeAsset, ChangeType: ScanDiffChangeAdded, TargetKey: "b.example.com:80", CreateTime: now},
		{TaskId: "t1", WorkspaceId: "default", DiffType: ScanDiffTypeVul, ChangeType: ScanDiffChangeAdded, TargetKey: "c.example.com:443:x", CreateTime: old},
	}
	if err := m.BatchInsert(ctx, docs); err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}

	// 保留期 90 天：old 的 2 条应被清理，now 的 1 条保留
	deleted, err := m.DeleteOlderThan(ctx, "default", now.Add(-ScanDiffRetentionDays*24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining, err := m.coll.CountDocuments(ctx, bson.M{"workspace_id": "default"})
	if err != nil {
		t.Fatalf("count remaining failed: %v", err)
	}
	if remaining != 1 {
		t.Errorf("expected 1 remaining, got %d", remaining)
	}
}

func TestScanDiff_BatchInsert_And_Count(t *testing.T) {
	m, cleanup := newTestScanDiffModel(t)
	if m == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	docs := []ScanDiff{
		{TaskId: "t2", WorkspaceId: "default", DiffType: ScanDiffTypeAsset, ChangeType: ScanDiffChangeAdded, TargetKey: "x.example.com:80"},
		{TaskId: "t2", WorkspaceId: "default", DiffType: ScanDiffTypeAsset, ChangeType: ScanDiffChangeUpdated, TargetKey: "y.example.com:80", Changes: []FieldChange{{Field: "title", OldValue: "a", NewValue: "b"}}},
	}
	if err := m.BatchInsert(ctx, docs); err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}

	added, err := m.CountByTaskIdAndType(ctx, "default", "t2", ScanDiffTypeAsset, ScanDiffChangeAdded)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if added != 1 {
		t.Errorf("expected 1 added, got %d", added)
	}

	stat, err := m.Stat(ctx, "default", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	total := int64(0)
	for _, s := range stat {
		total += s.Count
	}
	if total != 2 {
		t.Errorf("expected stat total 2, got %d", total)
	}
}
