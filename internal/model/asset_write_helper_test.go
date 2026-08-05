package model

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func setStr(m bson.M, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func setHas(m bson.M, key string) bool {
	_, ok := m[key]
	return ok
}

func TestBuildAssetUpdateDoc_NewAsset_IncludesSetOnInsert(t *testing.T) {
	asset := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Title:     "Hello",
	}
	opts := AssetWriteOptions{TaskId: "task-1"}
	update, _ := BuildAssetUpdateDoc(asset, nil, opts)

	setOnInsert, ok := update["$setOnInsert"].(bson.M)
	if !ok {
		t.Fatalf("expected $setOnInsert for new asset")
	}
	for _, key := range []string{"_id", "create_time", "first_seen_time", "first_seen_task_id", "new"} {
		if !setHas(setOnInsert, key) {
			t.Errorf("$setOnInsert missing key %q", key)
		}
	}
	if setOnInsert["first_seen_task_id"] != "task-1" {
		t.Errorf("first_seen_task_id mismatch: %v", setOnInsert["first_seen_task_id"])
	}
}

func TestBuildAssetUpdateDoc_EmptyValueDoesNotOverwrite(t *testing.T) {
	existing := &Asset{
		Authority:  "example.com:80",
		Host:       "example.com",
		Port:       80,
		Title:      "OldTitle",
		HttpHeader: "X-Old: 1",
	}
	asset := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Title:     "", // empty — must not overwrite
	}
	opts := AssetWriteOptions{TaskId: "task-2"}
	update, _ := BuildAssetUpdateDoc(asset, existing, opts)
	setFields := update["$set"].(bson.M)
	if setHas(setFields, "title") {
		t.Errorf("title must be omitted when new value is empty")
	}
	if setHas(setFields, "header") {
		t.Errorf("header must be omitted when new value is empty")
	}
	// update_time should not advance on no-op
	if setHas(setFields, "update_time") {
		t.Errorf("update_time must not advance on no-op write; set keys=%v", keysOf(setFields))
	}
}

func keysOf(m bson.M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestBuildAssetUpdateDoc_ManualCanClearMemo(t *testing.T) {
	existing := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Memo:      "旧备注",
	}
	asset := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Memo:      "", // clear
	}
	opts := AssetWriteOptions{IsManual: true, AllowClearUserFields: true}
	update, _ := BuildAssetUpdateDoc(asset, existing, opts)
	setFields := update["$set"].(bson.M)
	if !setHas(setFields, "memo") {
		t.Errorf("manual write must include memo when AllowClearUserFields=true")
	}
	if got := setStr(setFields, "memo"); got != "" {
		t.Errorf("memo must be empty (clear), got %q", got)
	}
	// changes should record the memo clear
	var hasMemoChange bool
	for _, c := range updateChangesLookup(update) {
		_ = c
	}
	if changes := diffAssetChanges(existing, asset, opts); len(changes) == 0 {
		t.Errorf("expected memo change to be detected")
	} else {
		hasMemoChange = false
		for _, c := range changes {
			if c.Field == "memo" {
				hasMemoChange = true
			}
		}
		if !hasMemoChange {
			t.Errorf("expected memo change in diff")
		}
	}
}

func TestBuildAssetUpdateDoc_StateFields_GatedByIsDifferentTask(t *testing.T) {
	existing := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		TaskId:    "old-task",
		Title:     "Old",
	}
	asset := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		TaskId:    "new-task",
		Title:     "New",
	}

	// Same task — should NOT mutate state fields
	update, _ := BuildAssetUpdateDoc(asset, existing, AssetWriteOptions{TaskId: "new-task", IsDifferentTask: false})
	if setFields, ok := update["$set"].(bson.M); ok {
		for _, key := range []string{"update", "new", "last_task_id", "last_status_change_time"} {
			if setHas(setFields, key) {
				t.Errorf("same-task write must not set %q", key)
			}
		}
	}

	// Different task — should advance state fields
	update2, _ := BuildAssetUpdateDoc(asset, existing, AssetWriteOptions{TaskId: "new-task", IsDifferentTask: true})
	setFields2 := update2["$set"].(bson.M)
	for _, key := range []string{"update", "new", "last_task_id", "last_status_change_time"} {
		if !setHas(setFields2, key) {
			t.Errorf("cross-task write must set %q", key)
		}
	}
	if setFields2["last_task_id"] != "old-task" {
		t.Errorf("last_task_id must be existing.TaskId, got %v", setFields2["last_task_id"])
	}
}

func TestDiffAssetChanges_CoversLargeFields(t *testing.T) {
	existing := &Asset{
		Authority:  "example.com:80",
		HttpHeader: "X-Old: 1",
		HttpBody:   "<html>old</html>",
		Screenshot: "old.png",
		Cert:       "oldcert",
	}
	updated := &Asset{
		Authority:  "example.com:80",
		HttpHeader: "X-New: 2",
		HttpBody:   "<html>new</html>",
		Screenshot: "new.png",
		Cert:       "newcert",
	}
	changes := DiffAssetChanges(existing, updated, AssetWriteOptions{})
	fields := map[string]bool{}
	for _, c := range changes {
		fields[c.Field] = true
	}
	for _, want := range []string{"header", "body", "screenshot", "cert"} {
		if !fields[want] {
			t.Errorf("diff missing field %q; got changes=%v", want, changes)
		}
	}
}

func TestDiffAssetChanges_NoneWhenEqual(t *testing.T) {
	asset := &Asset{
		Authority: "example.com:80",
		Title:    "Same",
		App:      []string{"nginx"},
	}
	changes := DiffAssetChanges(asset, asset, AssetWriteOptions{})
	if len(changes) != 0 {
		t.Errorf("expected no changes for identical assets, got %d", len(changes))
	}
}

func TestSortedJoin_OrderIndependent(t *testing.T) {
	a := sortedJoin([]string{"b", "a", "c"})
	b := sortedJoin([]string{"c", "a", "b"})
	if a != b {
		t.Errorf("sortedJoin not order-independent: a=%q b=%q", a, b)
	}
	if a != "a, b, c" {
		t.Errorf("sortedJoin wrong: got %q", a)
	}
}

func TestTruncateForChange(t *testing.T) {
	if got := truncateForChange("hello", 0); got != "hello" {
		t.Errorf("maxLen=0 should return full string, got %q", got)
	}
	if got := truncateForChange("hello", 10); got != "hello" {
		t.Errorf("short string should be unchanged, got %q", got)
	}
	long := strings.Repeat("a", 300)
	if got := truncateForChange(long, 50); len(got) != 53 {
		t.Errorf("expected len 53 (50 + ellipsis), got %d", len(got))
	}
}

func TestIpChanged(t *testing.T) {
	a := IP{IpV4: []IPV4{{IPName: "1.1.1.1"}}}
	b := IP{IpV4: []IPV4{{IPName: "2.2.2.2"}}}
	if !ipChanged(a, b) {
		t.Error("expected ipChanged true for different IPv4")
	}
	if ipChanged(a, a) {
		t.Error("expected ipChanged false for identical IPs")
	}
}

func TestBuildAssetUpdateDoc_LabelsUseAddToSet(t *testing.T) {
	existing := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Labels:    []string{"existing"},
	}
	asset := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Labels:    []string{"new1", "new2"},
	}
	update, _ := BuildAssetUpdateDoc(asset, existing, AssetWriteOptions{})
	addToSet, ok := update["$addToSet"].(bson.M)
	if !ok {
		t.Fatalf("expected $addToSet when Labels non-empty")
	}
	labelsEach, ok := addToSet["labels"].(bson.M)
	if !ok {
		t.Fatalf("expected labels with $each")
	}
	if _, ok := labelsEach["$each"]; !ok {
		t.Errorf("missing $each on labels")
	}
	// $set must NOT contain labels (would overwrite)
	if setHas(update["$set"].(bson.M), "labels") {
		t.Errorf("labels must NOT be in $set (would overwrite)")
	}
}

func TestBuildAssetUpdateDoc_HasChangeAdvancesUpdateTime(t *testing.T) {
	existing := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Title:     "Old",
	}
	updated := &Asset{
		Authority: "example.com:80",
		Host:      "example.com",
		Port:      80,
		Title:     "New",
	}
	update, _ := BuildAssetUpdateDoc(updated, existing, AssetWriteOptions{})
	setFields := update["$set"].(bson.M)
	if !setHas(setFields, "update_time") {
		t.Errorf("update_time must advance when there is a real change")
	}
	// Should be a recent time (within the last 5 seconds)
	now := time.Now()
	ut, ok := setFields["update_time"].(time.Time)
	if !ok {
		t.Errorf("update_time not time.Time: %T", setFields["update_time"])
		return
	}
	if now.Sub(ut) > 5*time.Second {
		t.Errorf("update_time too old: %v", ut)
	}
}

// updateChangesLookup is a no-op helper kept for future use; returns the
// FieldChange slice decoded from update's $set / $setOnInsert if available.
func updateChangesLookup(update bson.M) []FieldChange {
	// BuildAssetUpdateDoc returns changes via the second return value;
	// this helper exists to keep tests future-proof if needed.
	return nil
}
