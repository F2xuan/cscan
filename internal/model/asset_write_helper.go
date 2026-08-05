package model

import (
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AssetWriteOptions controls how BuildAssetUpdateDoc constructs the update document.
type AssetWriteOptions struct {
	// IsManual indicates a user-driven write (AssetSave / AssetImport).
	// Manual writes allow clearing user-editable fields (Memo / ColorTag) and
	// force new=true on insert; they do not advance last_task_id.
	IsManual bool
	// TaskId is the current task id (empty for manual writes).
	TaskId string
	// IsDifferentTask indicates a cross-task scanner write. When true, the
	// helper advances update=true / new=false / last_task_id /
	// last_status_change_time. When false (same-task re-save), state fields
	// are untouched so update_time does not regress to a no-op.
	IsDifferentTask bool
	// AllowClearUserFields allows empty Memo / ColorTag to be written to
	// $set (clearing existing values). Only set for manual writes.
	AllowClearUserFields bool
}

// BuildAssetSetOnInsert builds the $setOnInsert block for a new asset.
// Fields: _id, create_time, first_seen_time, first_seen_task_id, new=true.
func BuildAssetSetOnInsert(now time.Time, taskId string) bson.M {
	return bson.M{
		"_id":                primitive.NewObjectID(),
		"create_time":        now,
		"first_seen_time":    now,
		"first_seen_task_id": taskId,
		"new":                true,
	}
}

// BuildAssetUpdateDoc builds the MongoDB update document for an asset write.
// Returns the update doc and the field-level diff (changes) against existing.
// existing == nil means the asset is new; the doc will include $setOnInsert.
// The helper never touches the database — callers own the write.
//
// Semantics:
//   - Business fields (title/server/banner/header/body/cert/screenshot/
//     icon_hash/cname/domain/category/source/org_id/ip) are only added to
//     $set when non-empty, so existing data is preserved on partial writes.
//   - User-editable fields (memo/color_tag) are always written when
//     opts.AllowClearUserFields is true (manual writes); otherwise they
//     follow the non-empty rule.
//   - labels and app are merged via $addToSet $each, never overwritten.
//   - update_time is only advanced when there is a real change OR the asset
//     is new, so no-op writes do not regress the field.
//   - State fields (update/new/last_task_id/last_status_change_time) are
//     gated on opts.IsDifferentTask. Manual writes force new=true via
//     $setOnInsert and do not touch last_task_id.
func BuildAssetUpdateDoc(newAsset *Asset, existing *Asset, opts AssetWriteOptions) (bson.M, []FieldChange) {
	if newAsset == nil {
		return nil, nil
	}

	now := time.Now()
	isNew := existing == nil
	changes := diffAssetChanges(existing, newAsset, opts)

	setFields := bson.M{}
	addToSet := bson.M{}

	// Core identity fields (always written; identity-bearing).
	setFields["host"] = newAsset.Host
	setFields["port"] = newAsset.Port
	setFields["authority"] = newAsset.Authority
	setFields["is_http"] = newAsset.IsHTTP

	// Business fields — omit-if-empty to preserve existing data.
	if newAsset.Service != "" {
		setFields["service"] = newAsset.Service
	}
	if newAsset.Source != "" {
		setFields["source"] = newAsset.Source
	}
	if newAsset.Category != "" {
		setFields["category"] = newAsset.Category
	}
	if newAsset.Server != "" {
		setFields["server"] = newAsset.Server
	}
	if newAsset.Banner != "" {
		setFields["banner"] = newAsset.Banner
	}
	if newAsset.Title != "" {
		setFields["title"] = newAsset.Title
	}
	if newAsset.HttpStatus != "" {
		setFields["status"] = newAsset.HttpStatus
	}
	if newAsset.HttpHeader != "" {
		setFields["header"] = newAsset.HttpHeader
	}
	if newAsset.HttpBody != "" {
		setFields["body"] = newAsset.HttpBody
	}
	if newAsset.Cert != "" {
		setFields["cert"] = newAsset.Cert
	}
	if newAsset.Screenshot != "" {
		setFields["screenshot"] = newAsset.Screenshot
	}
	if newAsset.IconHash != "" {
		setFields["icon_hash"] = newAsset.IconHash
	}
	if len(newAsset.IconHashBytes) > 0 {
		setFields["icon_hash_bytes"] = newAsset.IconHashBytes
	}
	if newAsset.CName != "" {
		setFields["cname"] = newAsset.CName
	}
	if newAsset.Domain != "" {
		setFields["domain"] = newAsset.Domain
	}
	if newAsset.OrgId != "" {
		setFields["org_id"] = newAsset.OrgId
	}
	if len(newAsset.Ip.IpV4) > 0 || len(newAsset.Ip.IpV6) > 0 {
		setFields["ip"] = newAsset.Ip
	}

	// User-editable fields — manual writes may clear; others omit-if-empty.
	if opts.AllowClearUserFields {
		setFields["memo"] = newAsset.Memo
		setFields["color"] = newAsset.ColorTag
	} else {
		if newAsset.Memo != "" {
			setFields["memo"] = newAsset.Memo
		}
		if newAsset.ColorTag != "" {
			setFields["color"] = newAsset.ColorTag
		}
	}

	// app and labels merge via $addToSet, never overwrite.
	if len(newAsset.App) > 0 {
		addToSet["app"] = bson.M{"$each": newAsset.App}
	}
	if len(newAsset.Labels) > 0 {
		addToSet["labels"] = bson.M{"$each": newAsset.Labels}
	}

	// update_time: only advance when there is a real change or the asset is new.
	// Avoids no-op writes regressing the field.
	hasChange := len(changes) > 0 || len(addToSet) > 0
	if isNew || hasChange || opts.IsDifferentTask {
		setFields["update_time"] = now
	}

	// State fields — gated on IsDifferentTask (scanner cross-task write).
	// Manual writes use $setOnInsert for new=true and skip last_task_id.
	if opts.IsDifferentTask && !isNew {
		setFields["update"] = true
		setFields["new"] = false
		setFields["last_task_id"] = existing.TaskId
		setFields["last_status_change_time"] = now
	}
	if opts.IsManual && !isNew {
		// Manual edit to an existing asset — record status change time.
		setFields["last_status_change_time"] = now
	}

	// Always carry current TaskId forward when provided.
	if newAsset.TaskId != "" {
		setFields["taskId"] = newAsset.TaskId
	}

	update := bson.M{"$set": setFields}
	if isNew {
		update["$setOnInsert"] = BuildAssetSetOnInsert(now, opts.TaskId)
	}
	if len(addToSet) > 0 {
		update["$addToSet"] = addToSet
	}
	return update, changes
}

// DiffAssetChanges returns the field-level diff between existing and new.
// Returns nil when existing is nil (new asset) or when no field changed.
// Coverage: Title / Service / HttpStatus / Server / Banner / IconHash /
// HttpHeader / HttpBody / Screenshot / Cert / CName / Domain / Category /
// Source / OrgId / Ip / App (sorted join) / Labels (sorted join) /
// Memo / ColorTag (only when opts.AllowClearUserFields).
func DiffAssetChanges(existing *Asset, newAsset *Asset, opts AssetWriteOptions) []FieldChange {
	return diffAssetChanges(existing, newAsset, opts)
}

// diffAssetChanges returns the field-level diff between existing and new.
// Returns nil when existing is nil (new asset) or when no field changed.
// A field with an empty new value is treated as "not provided" — omit-if-empty
// in $set means we will not write it, so we do not report it as a change.
// Coverage: Title / Service / HttpStatus / Server / Banner / IconHash /
// HttpHeader / HttpBody / Screenshot / Cert / CName / Domain / Category /
// Source / OrgId / Ip / App (sorted join) / Labels (sorted join) /
// Memo / ColorTag (only when opts.AllowClearUserFields).
func diffAssetChanges(existing *Asset, newAsset *Asset, opts AssetWriteOptions) []FieldChange {
	if existing == nil {
		return nil
	}
	var changes []FieldChange

	addChange := func(field, oldV, newV string, maxLen int) {
		if oldV == newV {
			return
		}
		// empty new value = "not provided" — omit-if-empty skips the write,
		// so don't report a change for it (unless manual clear is allowed).
		if newV == "" && !opts.AllowClearUserFields {
			return
		}
		changes = append(changes, FieldChange{
			Field:    field,
			OldValue: truncateForChange(oldV, maxLen),
			NewValue: truncateForChange(newV, maxLen),
		})
	}

	addChange("title", existing.Title, newAsset.Title, 200)
	addChange("service", existing.Service, newAsset.Service, 0)
	addChange("httpStatus", existing.HttpStatus, newAsset.HttpStatus, 0)
	addChange("server", existing.Server, newAsset.Server, 0)
	addChange("banner", existing.Banner, newAsset.Banner, 200)
	addChange("iconHash", existing.IconHash, newAsset.IconHash, 0)
	addChange("header", existing.HttpHeader, newAsset.HttpHeader, 500)
	addChange("body", existing.HttpBody, newAsset.HttpBody, 500)
	addChange("screenshot", existing.Screenshot, newAsset.Screenshot, 0)
	addChange("cert", existing.Cert, newAsset.Cert, 500)
	addChange("cname", existing.CName, newAsset.CName, 0)
	addChange("domain", existing.Domain, newAsset.Domain, 0)
	addChange("category", existing.Category, newAsset.Category, 0)
	addChange("source", existing.Source, newAsset.Source, 0)
	addChange("org_id", existing.OrgId, newAsset.OrgId, 0)

	// App: sorted join compares order-independent.
	if sortedJoin(existing.App) != sortedJoin(newAsset.App) {
		changes = append(changes, FieldChange{
			Field:    "app",
			OldValue: truncateForChange(sortedJoin(existing.App), 500),
			NewValue: truncateForChange(sortedJoin(newAsset.App), 500),
		})
	}

	// Labels: sorted join compares order-independent.
	if sortedJoin(existing.Labels) != sortedJoin(newAsset.Labels) {
		changes = append(changes, FieldChange{
			Field:    "labels",
			OldValue: truncateForChange(sortedJoin(existing.Labels), 500),
			NewValue: truncateForChange(sortedJoin(newAsset.Labels), 500),
		})
	}

	// Ip: compare IPv4 / IPv6 name lists.
	if ipChanged(existing.Ip, newAsset.Ip) {
		changes = append(changes, FieldChange{
			Field:    "ip",
			OldValue: truncateForChange(ipSummary(existing.Ip), 200),
			NewValue: truncateForChange(ipSummary(newAsset.Ip), 200),
		})
	}

	// User-editable fields only matter for manual writes.
	if opts.AllowClearUserFields {
		addChange("memo", existing.Memo, newAsset.Memo, 0)
		addChange("color", existing.ColorTag, newAsset.ColorTag, 0)
	}

	return changes
}

func ipChanged(a, b IP) bool {
	if len(a.IpV4) != len(b.IpV4) || len(a.IpV6) != len(b.IpV6) {
		return true
	}
	for i := range a.IpV4 {
		if a.IpV4[i].IPName != b.IpV4[i].IPName {
			return true
		}
	}
	for i := range a.IpV6 {
		if a.IpV6[i].IPName != b.IpV6[i].IPName {
			return true
		}
	}
	return false
}

func ipSummary(ip IP) string {
	var parts []string
	for _, v := range ip.IpV4 {
		parts = append(parts, v.IPName)
	}
	for _, v := range ip.IpV6 {
		parts = append(parts, v.IPName)
	}
	return strings.Join(parts, ", ")
}

// sortedJoin sorts and joins a string slice for order-independent comparison.
func sortedJoin(arr []string) string {
	if len(arr) == 0 {
		return ""
	}
	sorted := make([]string, len(arr))
	copy(sorted, arr)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

// truncateForChange truncates a string for change records.
func truncateForChange(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
