package swagger

import (
	"reflect"
	"testing"
	"time"

	"cscan/api/internal/types"
)

func TestParseJSONTag(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantN   string
		wantOpt bool
		wantOmi bool
		wantDef string
	}{
		{"plain", "page", "page", false, false, ""},
		{"default", "page,default=1", "page", false, false, "1"},
		{"optional", "host,optional", "host", true, false, ""},
		{"omitempty", "iconData,omitempty", "iconData", false, true, ""},
		{"combined", "pageSize,default=20", "pageSize", false, false, "20"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, opts := parseJSONTag(c.input)
			if n != c.wantN {
				t.Fatalf("name: got %q want %q", n, c.wantN)
			}
			if opts.optional != c.wantOpt {
				t.Errorf("optional: got %v want %v", opts.optional, c.wantOpt)
			}
			if opts.omitempty != c.wantOmi {
				t.Errorf("omitempty: got %v want %v", opts.omitempty, c.wantOmi)
			}
			if opts.defaultVal != c.wantDef {
				t.Errorf("defaultVal: got %q want %q", opts.defaultVal, c.wantDef)
			}
		})
	}
}

func TestReflector_LoginReq(t *testing.T) {
	r := NewReflector()
	r.RegisterBySample(types.LoginReq{})
	s, ok := r.Schemas()["LoginReq"]
	if !ok {
		t.Fatal("LoginReq 未注册")
	}
	props, _ := s["properties"].(SchemaRef)
	if _, exists := props["username"]; !exists {
		t.Error("缺少 username 字段")
	}
	if _, exists := props["password"]; !exists {
		t.Error("缺少 password 字段")
	}
	req, ok := s["required"].([]string)
	if !ok || len(req) != 2 {
		t.Fatalf("required 应为 [username password], got %#v", s["required"])
	}
}

func TestReflector_AssetListReq_DefaultsAndOptional(t *testing.T) {
	r := NewReflector()
	r.RegisterBySample(types.AssetListReq{})
	s := r.Schemas()["AssetListReq"]
	props := s["properties"].(SchemaRef)
	page := props["page"].(SchemaRef)
	if page["type"] != "integer" {
		t.Errorf("page.type: got %v want integer", page["type"])
	}
	if page["default"] != int64(1) {
		t.Errorf("page.default: got %v want 1", page["default"])
	}
	host := props["host"].(SchemaRef)
	if host["type"] != "string" {
		t.Errorf("host.type: got %v want string", host["type"])
	}
	// 有 default 不应被加入 required
	req, _ := s["required"].([]string)
	for _, n := range req {
		if n == "page" || n == "host" {
			t.Errorf("不应把 %q 列为 required", n)
		}
	}
}

func TestReflector_NestedStructAndArray(t *testing.T) {
	r := NewReflector()
	r.RegisterBySample(types.AssetListResp{})
	// AssetListResp 内嵌 []Asset 数组，Asset 应该被递归注册
	if _, ok := r.Schemas()["Asset"]; !ok {
		t.Fatal("嵌套类型 Asset 未被收集")
	}
	assetList := r.Schemas()["AssetListResp"]
	props := assetList["properties"].(SchemaRef)
	listProp := props["list"].(SchemaRef)
	if listProp["type"] != "array" {
		t.Errorf("list.type: got %v want array", listProp["type"])
	}
	items := listProp["items"].(SchemaRef)
	if _, ok := items["$ref"]; !ok {
		t.Errorf("list.items 应为 $ref, got %v", items)
	}
}

func TestReflector_Map(t *testing.T) {
	r := NewReflector()
	r.RegisterBySample(types.AssetStatResp{})
	s := r.Schemas()["AssetStatResp"]
	props := s["properties"].(SchemaRef)
	rd := props["riskDistribution"].(SchemaRef)
	if rd["type"] != "object" {
		t.Errorf("riskDistribution.type: got %v want object", rd["type"])
	}
	ap, _ := rd["additionalProperties"].(SchemaRef)
	if ap["type"] != "integer" {
		t.Errorf("additionalProperties.type: got %v want integer", ap["type"])
	}
}

func TestReflector_TimeTime(t *testing.T) {
	r := NewReflector()
	// 注册一个含 time.Time 的样本（直接构造）
	r.RegisterBySample(time.Now())
	// time.Time 不是具名 struct，不应出现在 schemas 中
	if _, ok := r.Schemas()["Time"]; ok {
		t.Error("time.Time 不应被作为命名结构体注册")
	}
}

func TestReflector_PointerField(t *testing.T) {
	r := NewReflector()
	r.RegisterBySample(types.Asset{})
	asset := r.Schemas()["Asset"]
	props := asset["properties"].(SchemaRef)
	ip := props["ip"].(SchemaRef)
	// 字段为 *IPInfo，应展开为对 IPInfo 的 $ref
	if _, ok := ip["$ref"]; !ok {
		t.Errorf("ip 应为 $ref, got %v", ip)
	}
	if _, ok := r.Schemas()["IPInfo"]; !ok {
		t.Error("IPInfo 未被收集")
	}
}

func TestCoerceDefault(t *testing.T) {
	cases := []struct {
		in   string
		want interface{}
	}{
		{"1", int64(1)},
		{"20", int64(20)},
		{"true", true},
		{"false", false},
		{"", ""},
		{"abc", "abc"},
	}
	for _, c := range cases {
		got := coerceDefault(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("coerceDefault(%q): got %#v want %#v", c.in, got, c.want)
		}
	}
}
