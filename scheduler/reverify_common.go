package scheduler

import (
	"context"
	"time"

	"cscan/pkg/notify"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
)

// 复验调度共享默认值（T3.3 弱口令 / T3.4 敏感信息共用，秒级 6 字段 cron）
const (
	defaultReverifyCronSpec    = "0 0 3 * * *" // 每日 03:00 复验
	defaultReverifyMaxTargets  = 200           // 单次复验目标上限，超出下个周期继续（禁止静默截断）
	defaultReverifyConcurrency = 1             // 串行执行，上一个验证完才能到下一个
)

// ReverifySender 复验结果通知发送函数（由 API 层注入，复用 NotifyManager 按已启用通道发送）
type ReverifySender func(ctx context.Context, result *notify.NotifyResult) error

// ReverifyStats 一次复验的聚合统计（弱口令与敏感信息共用）
type ReverifyStats struct {
	Total    int
	Resolved int // 已确认修复/不可访问（weakpass: MarkFixed / exposure: resolved）
	Verified int // 仍存在/仍暴露
	Pending  int // 目标不可达，待确认（不得误判为已修复）
}

// nextReverifyRunTime 依据 cron_spec 计算下次执行时间（6 字段秒级，解析失败回退默认每日）
func nextReverifyRunTime(spec string, from time.Time) time.Time {
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

// buildReverifyNotify 构造修复确认通知（共用 T1.4 的 FixedVulCount 进通知）
func buildReverifyNotify(taskId, taskName, wsId string, resolved int) *notify.NotifyResult {
	return &notify.NotifyResult{
		TaskId:      taskId,
		TaskName:    taskName,
		Status:      "SUCCESS",
		WorkspaceId: wsId,
		HighRiskInfo: &notify.HighRiskInfo{
			FixedVulCount: resolved,
		},
	}
}

// sendReverifyNotify 发送复验修复通知（sender 为空时静默跳过）
func sendReverifyNotify(ctx context.Context, sender ReverifySender, result *notify.NotifyResult, wsId, taskName string) {
	if sender == nil {
		return
	}
	if err := sender(ctx, result); err != nil {
		logx.Errorf("[reverify] workspace=%s %s 发送修复通知失败: %v", wsId, taskName, err)
	}
}
