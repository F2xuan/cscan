package swagger

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// typeCatalog 通过 RegisterBySample 在 metadata 子文件 init() 中递归注册 types 包内结构体。
// 由于 Go 反射不能枚举包内类型，此 slice 是入口点；具体注册分散到 metadata_xxx.go 中。
var typeCatalog []interface{}

// RegisterTypes 把 types 包内的具名结构体样本登记到反射器入口列表。
// 重复登记同一类型是安全的（按 type.Name 去重）。
func RegisterTypes(samples ...interface{}) {
	for _, s := range samples {
		if s == nil {
			continue
		}
		typeCatalog = append(typeCatalog, s)
	}
}

// SeedReflector 用 typeCatalog 内的样本批量注册结构体。
// 在 SpecHandler 生成 OpenAPI 前调用一次。
func (r *Reflector) SeedReflector() {
	for _, s := range typeCatalog {
		r.RegisterBySample(s)
	}
}

// SchemaRef 描述一个 OpenAPI 3.0 schema（内联或 $ref）。
type SchemaRef = map[string]interface{}

// Reflector 把 types 包结构体反射为 OpenAPI 3.0 schema，并收集 components.schemas。
type Reflector struct {
	mu      sync.Mutex
	schemas map[string]SchemaRef
	seen    map[reflect.Type]struct{}
}

func NewReflector() *Reflector {
	return &Reflector{
		schemas: make(map[string]SchemaRef),
		seen:    make(map[reflect.Type]struct{}),
	}
}

// Schemas 返回收集到的所有命名结构体 schema。
func (r *Reflector) Schemas() map[string]SchemaRef {
	out := make(map[string]SchemaRef, len(r.schemas))
	for k, v := range r.schemas {
		out[k] = v
	}
	return out
}

// SortedSchemaNames 返回稳定的 schema 名顺序，便于人工审阅与测试断言。
func (r *Reflector) SortedSchemaNames() []string {
	names := make([]string, 0, len(r.schemas))
	for k := range r.schemas {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// RegisterBySample 接受一个 types 包内的具名结构体或其指针实例，写入并递归收集相关结构体。
// 传入 nil 或非 struct 返回空 ref，不 panic。
func (r *Reflector) RegisterBySample(sample interface{}) SchemaRef {
	if sample == nil {
		return SchemaRef{"type": "object"}
	}
	t := reflect.TypeOf(sample)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return SchemaRef{"type": "object"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerNamedStruct(t)
	return SchemaRef{"$ref": "#/components/schemas/" + t.Name()}
}

// MustRef 通过名字返回结构体 $ref；若该结构体尚未注册，则返回内联空 object。
func (r *Reflector) MustRef(name string) SchemaRef {
	if name == "" {
		return SchemaRef{"type": "object"}
	}
	if _, ok := r.schemas[name]; ok {
		return SchemaRef{"$ref": "#/components/schemas/" + name}
	}
	return SchemaRef{"type": "object", "description": "未注册类型: " + name}
}

func (r *Reflector) registerNamedStruct(t reflect.Type) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	if t == reflect.TypeOf(time.Time{}) {
		return
	}
	name := t.Name()
	if name == "" {
		return
	}
	if _, ok := r.schemas[name]; ok {
		return
	}
	if _, seen := r.seen[t]; seen {
		return
	}
	r.seen[t] = struct{}{}
	// 占位，避免循环嵌套无限递归
	r.schemas[name] = SchemaRef{"type": "object", "description": name, "properties": SchemaRef{}}

	props := SchemaRef{}
	required := []string{}
	desc := getStructDescription(t)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		namePart, opts := parseJSONTag(jsonTag)
		if namePart == "" {
			namePart = f.Name
		}
		propSchema := r.schemaForType(f.Type)

		// 字段描述
		fieldDesc := getFieldDescription(f)
		if fieldDesc != "" {
			propSchema["description"] = fieldDesc
		}

		// 示例值
		if example := f.Tag.Get("example"); example != "" {
			propSchema["example"] = coerceExample(example, f.Type)
		}

		// 枚举值
		if enumStr := f.Tag.Get("enum"); enumStr != "" {
			enums := parseEnumValues(enumStr, f.Type)
			if len(enums) > 0 {
				propSchema["enum"] = enums
			}
		}

		// 格式
		if format := f.Tag.Get("format"); format != "" {
			propSchema["format"] = format
		}

		if !opts.optional && !opts.omitempty && !opts.hasDefault {
			required = append(required, namePart)
		}
		if opts.hasDefault {
			propSchema["default"] = coerceDefault(opts.defaultVal)
		}
		props[namePart] = propSchema
		// 递归收集引用到的命名结构体
		r.collectNested(f.Type)
	}
	schema := SchemaRef{
		"type":       "object",
		"properties": props,
	}
	if desc != "" {
		schema["description"] = desc
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	r.schemas[name] = schema
}

func (r *Reflector) collectNested(t reflect.Type) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() == reflect.Map {
		r.collectNested(t.Elem())
		return
	}
	if t.Kind() == reflect.Struct && t.Name() != "" && t != reflect.TypeOf(time.Time{}) {
		r.registerNamedStruct(t)
	}
}

// schemaForType 返回某 reflect.Type 对应的 OpenAPI schema（内联或 $ref）。
func (r *Reflector) schemaForType(t reflect.Type) SchemaRef {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return SchemaRef{"type": "string"}
	case reflect.Bool:
		return SchemaRef{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return SchemaRef{"type": "integer", "format": "int32"}
	case reflect.Int64:
		return SchemaRef{"type": "integer", "format": "int64"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return SchemaRef{"type": "integer", "format": "int32"}
	case reflect.Uint64:
		return SchemaRef{"type": "integer", "format": "int64"}
	case reflect.Float32:
		return SchemaRef{"type": "number", "format": "float"}
	case reflect.Float64:
		return SchemaRef{"type": "number", "format": "double"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return SchemaRef{"type": "string", "format": "byte"}
		}
		return SchemaRef{
			"type":  "array",
			"items": r.schemaForType(t.Elem()),
		}
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			return SchemaRef{
				"type":                 "object",
				"additionalProperties": r.schemaForType(t.Elem()),
			}
		}
		return SchemaRef{"type": "object", "description": "动态键 map"}
	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			return SchemaRef{"type": "string", "format": "date-time"}
		}
		if t.Name() != "" {
			if _, ok := r.schemas[t.Name()]; !ok {
				r.registerNamedStruct(t)
			}
			return SchemaRef{"$ref": "#/components/schemas/" + t.Name()}
		}
		props := SchemaRef{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := f.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			namePart, _ := parseJSONTag(tag)
			if namePart == "" {
				namePart = f.Name
			}
			propSchema := r.schemaForType(f.Type)
			if desc := getFieldDescription(f); desc != "" {
				propSchema["description"] = desc
			}
			if example := f.Tag.Get("example"); example != "" {
				propSchema["example"] = coerceExample(example, f.Type)
			}
			props[namePart] = propSchema
		}
		return SchemaRef{"type": "object", "properties": props}
	case reflect.Interface:
		return SchemaRef{"type": "object", "description": "动态结构 (interface{})"}
	default:
		return SchemaRef{"type": "object"}
	}
}

type jsonTagOpts struct {
	optional   bool
	omitempty  bool
	hasDefault bool
	defaultVal string
}

// parseJSONTag 解析 go-zero 风格 `json:"page,default=1"` 或 `json:"name,optional"`。
func parseJSONTag(tag string) (string, jsonTagOpts) {
	parts := strings.Split(tag, ",")
	name := strings.TrimSpace(parts[0])
	opts := jsonTagOpts{}
	if len(parts) < 2 {
		return name, opts
	}
	for i := 1; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		switch {
		case p == "optional":
			opts.optional = true
		case p == "omitempty":
			opts.omitempty = true
		case strings.HasPrefix(p, "default="):
			opts.hasDefault = true
			opts.defaultVal = strings.TrimPrefix(p, "default=")
		}
	}
	return name, opts
}

// coerceDefault 尝试把 default=N 的字符串转换为对应基础类型的字面量。
func coerceDefault(s string) interface{} {
	if s == "" {
		return s
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if n, err := atoi64(s); err == nil {
		return n
	}
	return s
}

// coerceExample 将 example tag 值转换为对应类型
func coerceExample(s string, t reflect.Type) interface{} {
	if s == "" {
		return s
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return s == "true"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := atoi64(s); err == nil {
			return n
		}
	case reflect.Float32, reflect.Float64:
		if f, err := parseFloat(s); err == nil {
			return f
		}
	}
	return s
}

// parseEnumValues 解析 enum tag "a|b|c" 为值数组
func parseEnumValues(s string, t reflect.Type) []interface{} {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "|")
	out := make([]interface{}, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if n, err := atoi64(p); err == nil {
				out = append(out, n)
				continue
			}
		case reflect.Bool:
			out = append(out, p == "true")
			continue
		}
		out = append(out, p)
	}
	return out
}

// getStructDescription 获取结构体的描述
// 注意：reflect.Type 无法直接访问 struct tag，需通过 StructField 或注释约定实现
func getStructDescription(t reflect.Type) string {
	// 结构体级别的描述暂通过类型名到描述的映射提供
	// 如需为特定结构体添加描述，可在此处扩展
	structDescriptions := map[string]string{}
	if desc, ok := structDescriptions[t.Name()]; ok {
		return desc
	}
	return ""
}

// getFieldDescription 获取字段描述（从 swagger/desc/comment tag）
func getFieldDescription(f reflect.StructField) string {
	if desc := f.Tag.Get("swagger"); desc != "" {
		return desc
	}
	if desc := f.Tag.Get("desc"); desc != "" {
		return desc
	}
	if desc := f.Tag.Get("comment"); desc != "" {
		return desc
	}
	return ""
}

func atoi64(s string) (int64, error) {
	var n int64
	var sign int64 = 1
	i := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		i = 1
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errBadInt
		}
		n = n*10 + int64(c-'0')
	}
	return n * sign, nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	var sign float64 = 1
	i := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		i = 1
	}
	dec := false
	var div float64 = 1
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if dec {
				return 0, errBadFloat
			}
			dec = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, errBadFloat
		}
		if dec {
			div *= 10
			f = f + float64(c-'0')/div
		} else {
			f = f*10 + float64(c-'0')
		}
	}
	return f * sign, nil
}

var errBadInt = &parseError{}
var errBadFloat = &parseError{msg: "not a float"}

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return "not an integer"
}
