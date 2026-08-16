package model

import (
	"testing"
	"time"
)

func TestNeedsRefresh(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		doc  *AssetTargetMeta
		want bool
	}{
		{"nil 文档", nil, true},
		{"从未计算过快照", &AssetTargetMeta{LastScanTime: now}, true},
		{
			"快照新鲜且资产无更新",
			&AssetTargetMeta{LastScanTime: now.Add(-time.Minute), RiskUpdatedAt: now},
			false,
		},
		{
			"快照超过 maxAge",
			&AssetTargetMeta{LastScanTime: now.Add(-time.Hour), RiskUpdatedAt: now.Add(-31 * time.Minute)},
			true,
		},
		{
			// 扫描完成后 last_scan_time 被资产反推同步推进，晚于快照计算时间 → 立即重算
			"资产在快照后有更新（扫描完成）",
			&AssetTargetMeta{LastScanTime: now, RiskUpdatedAt: now.Add(-time.Minute)},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NeedsRefresh(c.doc, 30*time.Minute); got != c.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, c.want)
			}
		})
	}
}
