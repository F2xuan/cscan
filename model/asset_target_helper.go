package model

import (
	"context"
	"strings"
	"time"

	"cscan/pkg/utils"
)

// ResolveAssetTarget 将一条 asset 的 (host, domain) 归并到顶层资产 (type, value)。
// 规则与 tools/migrate_asset_target_meta.go 的 resolveTarget 保持一致：
//   - host 为 IP 且 domain 非空 → domain 的根域名（domain 类型）
//   - host 为 IP 且 domain 为空 → host（ip 类型）
//   - host 为域名 → host 的根域名（domain 类型）
//
// 无法解析时返回空串。
func ResolveAssetTarget(host, domain string) (AssetTargetType, string) {
	host = strings.TrimSpace(host)
	domain = strings.TrimSpace(domain)
	if host == "" {
		return "", ""
	}
	if utils.IsIPAddress(host) {
		if domain != "" {
			if root := utils.GetRootDomain(domain); root != "" {
				return AssetTargetTypeDomain, root
			}
		}
		return AssetTargetTypeIP, host
	}
	root := utils.GetRootDomain(host)
	if root == "" {
		return "", ""
	}
	return AssetTargetTypeDomain, root
}

// EnsureForAsset 按 (host, domain) 解析顶层资产并 upsert 到 meta 集合。
// 供手动新增（API AssetSave）与扫描结果保存（RPC SaveTaskResult）共用，
// 确保 asset 出现在顶层资产列表中。labels 为 nil 时不覆盖既有标签。
func (m *AssetTargetMetaModel) EnsureForAsset(ctx context.Context, wsId, host, domain string, labels []string) error {
	tType, tValue := ResolveAssetTarget(host, domain)
	if tType == "" || tValue == "" {
		return nil
	}
	doc := &AssetTargetMeta{
		Id:           EncodeTargetID(tType, tValue),
		WorkspaceId:  wsId,
		TargetType:   string(tType),
		TargetValue:  tValue,
		Labels:       labels,
		LastScanTime: time.Now(),
	}
	return m.Upsert(ctx, doc)
}
