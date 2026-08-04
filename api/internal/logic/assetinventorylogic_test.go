package logic

import (
	"testing"

	"cscan/api/internal/types"

	"go.mongodb.org/mongo-driver/bson"
)

func newInventoryLogic() *AssetInventoryLogic {
	// buildInventoryFilter 仅依赖入参 req，不触碰 ctx/svcCtx，故可直接零值构造。
	return &AssetInventoryLogic{}
}

// TestBuildInventoryFilter_RequireRecognitionOrShotTrue 验证接入后的死参数：
// requireRecognitionOrShot=true 时仅返回「有截图/图标/指纹/技术栈」的资产。
func TestBuildInventoryFilter_RequireRecognitionOrShotTrue(t *testing.T) {
	l := newInventoryLogic()
	f := l.buildInventoryFilter(&types.AssetInventoryReq{RequireRecognitionOrShot: true})

	and, ok := f["$and"].([]bson.M)
	if !ok || len(and) != 1 {
		t.Fatalf("期望 $and 含 1 个条件，实际 %#v", f["$and"])
	}
	orCond, ok := and[0]["$or"].([]bson.M)
	if !ok {
		t.Fatalf("期望 $and[0] 为 $or 条件，实际 %#v", and[0])
	}

	fields := map[string]bool{}
	for _, c := range orCond {
		for k := range c {
			fields[k] = true
		}
	}
	for _, want := range []string{"screenshot", "icon_hash", "fingerprints", "app"} {
		if !fields[want] {
			t.Errorf("识别/截图过滤条件缺少字段 %q，实际 %v", want, fields)
		}
	}
}

// TestBuildInventoryFilter_RequireRecognitionOrShotFalse 验证关闭时不再注入该条件。
func TestBuildInventoryFilter_RequireRecognitionOrShotFalse(t *testing.T) {
	l := newInventoryLogic()
	f := l.buildInventoryFilter(&types.AssetInventoryReq{RequireRecognitionOrShot: false})
	if _, ok := f["$and"]; ok {
		t.Errorf("requireRecognitionOrShot=false 不应注入 $and 条件，实际 %#v", f["$and"])
	}
}

// TestBuildInventoryFilter_BasicFilters 验证关键词/端口/状态码/标签/服务/图标哈希等基础过滤条件。
func TestBuildInventoryFilter_BasicFilters(t *testing.T) {
	l := newInventoryLogic()
	req := &types.AssetInventoryReq{
		Query:       "example",
		Ports:       []int{80, 443},
		StatusCodes: []string{"200"},
		Labels:      []string{"prod"},
		Service:     "http",
		IconHash:    "deadbeef",
	}
	f := l.buildInventoryFilter(req)

	// 关键词 -> host/title/domain/ipv4/ipv6 5 个正则 $or
	or, ok := f["$or"].([]bson.M)
	if !ok {
		t.Fatalf("关键词期望生成 $or，实际 %#v", f)
	}
	if len(or) != 5 {
		t.Errorf("关键词 $or 期望 5 个字段，实际 %d", len(or))
	}

	assertIn := func(key string) bson.M {
		v, ok := f[key].(bson.M)
		if !ok {
			t.Fatalf("期望字段 %q 为 bson.M，实际 %#v", key, f[key])
		}
		return v
	}
	if _, ok := assertIn("port")["$in"]; !ok {
		t.Error("port 应使用 $in")
	}
	if _, ok := assertIn("status")["$in"]; !ok {
		t.Error("status 应使用 $in")
	}
	if _, ok := assertIn("labels")["$in"]; !ok {
		t.Error("labels 应使用 $in")
	}
	if _, ok := assertIn("service")["$regex"]; !ok {
		t.Error("service 应使用 $regex")
	}
	if f["icon_hash"] != "deadbeef" {
		t.Errorf("icon_hash 应透传，实际 %v", f["icon_hash"])
	}
}
