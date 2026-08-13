package model

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const testMongoURI = "mongodb://localhost:27017"
const testDBName = "cscan_model_test"

// mongoTestDB 连接测试库；若本地无 MongoDB 则跳过（与现有集成测试约定一致）。
func mongoTestDB(t *testing.T) (*mongo.Database, func()) {
	t.Helper()
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(testMongoURI))
	if err != nil {
		t.Skipf("MongoDB 不可用，跳过 DB 测试: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB 不可用，跳过 DB 测试: %v", err)
	}
	db := client.Database(testDBName)
	cleanup := func() {
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}
	return db, cleanup
}

// TestAggregateInventoryPaged_PaginationSortProjection 覆盖清单分页重写后的核心行为：
//  1. 空过滤走 EstimatedDocumentCount（O(1)），total 准确；
//  2. 默认按 update_time 降序排序；
//  3. 投影排除 body/header/banner，但保留 screenshot；
//  4. skip/limit 分页正确。
func TestAggregateInventoryPaged_PaginationSortProjection(t *testing.T) {
	db, cleanup := mongoTestDB(t)
	defer cleanup()
	ctx := context.Background()

	wsId := "inv_ws"
	coll := db.Collection(wsId + "_asset")
	defer coll.Drop(ctx)

	now := time.Now()
	docs := make([]interface{}, 0, 25)
	for i := 0; i < 25; i++ {
		docs = append(docs, &Asset{
			Id:         primitive.NewObjectID(),
			Authority:  "host" + string(rune('a'+i%26)) + ":80",
			Host:       "host" + string(rune('a'+i%26)),
			Port:       80,
			Title:      "t",
			HttpBody:   "BIGBODY_" + string(rune('a'+i%26)),
			HttpHeader: "HDR",
			Banner:     "BNR",
			Screenshot: "shot_" + string(rune('a'+i%26)) + ".png",
			UpdateTime: now.Add(time.Duration(i) * time.Minute),
			CreateTime: now,
		})
	}
	if _, err := coll.InsertMany(ctx, docs); err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}

	m := NewAssetModel(db, wsId)

	// 空过滤 -> EstimatedDocumentCount，total == 25
	total, list, err := m.AggregateInventoryPaged(ctx, []string{wsId}, bson.M{}, 0, 10, "-update_time")
	if err != nil {
		t.Fatalf("AggregateInventoryPaged 失败: %v", err)
	}
	if total != 25 {
		t.Errorf("空过滤期望 total=25，实际 %d", total)
	}
	if len(list) != 10 {
		t.Errorf("首页期望 10 条，实际 %d", len(list))
	}

	// 默认降序：第一条应为 update_time 最新者
	if len(list) > 1 && list[0].UpdateTime.Before(list[1].UpdateTime) {
		t.Errorf("期望按 update_time 降序，但首条早于次条")
	}

	// 投影：body/header/banner 被排除，screenshot 保留
	if list[0].HttpBody != "" {
		t.Errorf("body 应被投影排除，实际 %q", list[0].HttpBody)
	}
	if list[0].HttpHeader != "" {
		t.Errorf("header 应被投影排除，实际 %q", list[0].HttpHeader)
	}
	if list[0].Banner != "" {
		t.Errorf("banner 应被投影排除，实际 %q", list[0].Banner)
	}
	if list[0].Screenshot == "" {
		t.Errorf("screenshot 应保留")
	}

	// 第 2、3 页
	total2, list2, err := m.AggregateInventoryPaged(ctx, bson.M{}, 10, 10, "-update_time")
	if err != nil {
		t.Fatalf("第 2 页失败: %v", err)
	}
	if total2 != 25 {
		t.Errorf("分页间 total 应稳定，实际 %d", total2)
	}
	if len(list2) != 10 {
		t.Errorf("第 2 页期望 10 条，实际 %d", len(list2))
	}
	_, list3, _ := m.AggregateInventoryPaged(ctx, []string{wsId}, bson.M{}, 20, 10, "-update_time")
	if len(list3) != 5 {
		t.Errorf("第 3 页期望 5 条（余量），实际 %d", len(list3))
	}
}

// TestAggregateInventoryPaged_FilteredCount 覆盖带过滤时走 CountDocuments（精确计数）而非估算。
func TestAggregateInventoryPaged_FilteredCount(t *testing.T) {
	db, cleanup := mongoTestDB(t)
	defer cleanup()
	ctx := context.Background()

	wsId := "inv_ws2"
	coll := db.Collection(wsId + "_asset")
	defer coll.Drop(ctx)

	now := time.Now()
	docs := []interface{}{
		&Asset{Id: primitive.NewObjectID(), Host: "a", Port: 80, HttpStatus: "200", UpdateTime: now},
		&Asset{Id: primitive.NewObjectID(), Host: "b", Port: 80, HttpStatus: "404", UpdateTime: now},
		&Asset{Id: primitive.NewObjectID(), Host: "c", Port: 443, HttpStatus: "200", UpdateTime: now},
	}
	if _, err := coll.InsertMany(ctx, docs); err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	m := NewAssetModel(db)
	filter := bson.M{"status": "200"}
	total, list, err := m.AggregateInventoryPaged(ctx, filter, 0, 10, "-update_time")
	if err != nil {
		t.Fatalf("AggregateInventoryPaged 失败: %v", err)
	}
	if total != 2 {
		t.Errorf("过滤计数期望 total=2，实际 %d", total)
	}
	if len(list) != 2 {
		t.Errorf("过滤结果期望 2 条，实际 %d", len(list))
	}
}

// TestAggregateInventoryPaged_EmptyWsIds 覆盖 wsIds 为空时安全返回（不 panic）。
func TestAggregateInventoryPaged_EmptyWsIds(t *testing.T) {
	db, cleanup := mongoTestDB(t)
	defer cleanup()
	ctx := context.Background()
	m := NewAssetModel(db)
	total, list, err := m.AggregateInventoryPaged(ctx, bson.M{}, 0, 10, "-update_time")
	if err != nil {
		t.Fatalf("空 wsIds 不应返回错误，实际 %v", err)
	}
	if total != 0 || list != nil {
		t.Errorf("空 wsIds 应返回 (0, nil)，实际 (%d, %v)", total, list)
	}
}
