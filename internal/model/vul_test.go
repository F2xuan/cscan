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

// newTestVulModel 仅在设置了 CSCAN_TEST_MONGO_URI 时连接真实库；否则测试 skip。
func newTestVulModel(t *testing.T) (*VulModel, func()) {
	uri := os.Getenv("CSCAN_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("CSCAN_TEST_MONGO_URI not set, skip Vul DB test")
	}
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("mongo connect failed: %v", err)
	}
	db := client.Database("cscan_test_vul_" + t.Name())
	cleanup := func() {
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}
	return NewVulModel(db), cleanup
}

// TestShouldResurrect 纯逻辑单测：仅 fixed 应复活为 open（T1.3）。
func TestShouldResurrect(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"open", false},
		{"ignored", false},
		{"fixed", true},
	}
	for _, c := range cases {
		if got := shouldResurrect(c.in); got != c.want {
			t.Errorf("shouldResurrect(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestVul_UpsertResurrectsFixed 验证修复态漏洞被再次扫到时自动回到 open 并清空 fixed_at。
func TestVul_UpsertResurrectsFixed(t *testing.T) {
	m, cleanup := newTestVulModel(t)
	defer cleanup()
	ctx := context.Background()

	doc := &Vul{
		Authority: "example.com:443",
		Host:      "example.com",
		Port:      443,
		PocFile:   "x",
		Url:       "https://example.com",
		Severity:  "high",
		Status:    VulStatusFixed,
		FixedAt:   time.Now().Add(-time.Hour),
	}
	if _, err := m.Upsert(ctx, doc); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	// 复扫：同 key 再次 upsert（模拟修复后又复现）
	if _, err := m.Upsert(ctx, doc); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	got, err := m.Find(ctx, bson.M{"host": "example.com", "port": 443}, 0, 0)
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expect 1 doc, got %d", len(got))
	}
	if got[0].Status != VulStatusOpen {
		t.Errorf("status = %q, want %q (fixed should resurrect to open)", got[0].Status, VulStatusOpen)
	}
	if !got[0].FixedAt.IsZero() {
		t.Errorf("fixed_at should be cleared after resurrection, got %v", got[0].FixedAt)
	}
}

// TestVul_MarkAndAggregateStatus 验证批量标记与按状态聚合计数一致。
func TestVul_MarkAndAggregateStatus(t *testing.T) {
	m, cleanup := newTestVulModel(t)
	defer cleanup()
	ctx := context.Background()

	docs := []*Vul{
		{Host: "a.com", Port: 80, PocFile: "p", Url: "http://a.com", Severity: "high", Status: VulStatusOpen},
		{Host: "b.com", Port: 80, PocFile: "p", Url: "http://b.com", Severity: "low", Status: VulStatusFixed},
		{Host: "c.com", Port: 80, PocFile: "p", Url: "http://c.com", Severity: "info", Status: VulStatusIgnored},
	}
	for _, d := range docs {
		if _, err := m.Upsert(ctx, d); err != nil {
			t.Fatalf("upsert failed: %v", err)
		}
	}
	// 取第一个文档 id 并标记为 fixed
	var firstID string
	if v, err := m.Find(ctx, bson.M{"host": "a.com", "port": 80}, 0, 0); err == nil && len(v) > 0 {
		firstID = v[0].Id.Hex()
	}
	if firstID == "" {
		t.Fatal("cannot find first doc id")
	}
	if _, err := m.MarkFixed(ctx, []string{firstID}, VulFixSourceManual); err != nil {
		t.Fatalf("MarkFixed failed: %v", err)
	}

	stats, err := m.AggregateStats(ctx, time.Now())
	if err != nil {
		t.Fatalf("AggregateStats failed: %v", err)
	}
	// 现在: a->fixed, b->fixed, c->ignored => open=0, fixed=2, ignored=1, total=3
	if stats.Open != 0 {
		t.Errorf("Open = %d, want 0", stats.Open)
	}
	if stats.Fixed != 2 {
		t.Errorf("Fixed = %d, want 2", stats.Fixed)
	}
	if stats.Ignored != 1 {
		t.Errorf("Ignored = %d, want 1", stats.Ignored)
	}
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}

	// CountByStatus 校验
	if c, err := m.CountByStatus(ctx, VulStatusFixed); err != nil || c != 2 {
		t.Errorf("CountByStatus(fixed) = %d (err=%v), want 2", c, err)
	}
	if c, err := m.CountByStatus(ctx, VulStatusIgnored); err != nil || c != 1 {
		t.Errorf("CountByStatus(ignored) = %d (err=%v), want 1", c, err)
	}

	// MarkIgnored 再标记 a 为 ignored，状态应切换
	if _, err := m.MarkIgnored(ctx, []string{firstID}); err != nil {
		t.Fatalf("MarkIgnored failed: %v", err)
	}
	if v, err := m.Find(ctx, bson.M{"host": "a.com", "port": 80}, 0, 0); err == nil && len(v) > 0 {
		if v[0].Status != VulStatusIgnored {
			t.Errorf("after MarkIgnored status = %q, want %q", v[0].Status, VulStatusIgnored)
		}
	}
}
