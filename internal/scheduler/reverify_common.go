package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
)

// 复验调度共享默认值（T3.3 弱口令 / T3.4 敏感信息共用，秒级 6 字段 cron）
const (
	defaultReverifyCronSpec   = "0 0 3 * * *" // 每日 03:00 复验
	defaultReverifyMaxTargets = 200           // 单次复验目标上限，超出下个周期继续（禁止静默截断）
)

// NextReverifyRunTime 依据 cron_spec 计算下次执行时间（6 字段秒级，解析失败回退默认每日）
func NextReverifyRunTime(spec string, from time.Time) time.Time {
	if spec == "" {
		spec = defaultReverifyCronSpec
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(spec)
	if err != nil {
		sched, err = parser.Parse(defaultReverifyCronSpec)
		if err != nil {
			return from.Add(24 * time.Hour)
		}
	}
	return sched.Next(from)
}
