package svc

import (
	"context"

	"cscan/pkg/notify"

	"github.com/zeromicro/go-zero/core/logx"
)

// SendReverifyNotify 发送复验修复确认通知（T3.3 / T3.4 共用）。
// 按工作空间已启用通道发送；无可用通道时静默跳过。
func (s *ServiceContext) SendReverifyNotify(ctx context.Context, result *notify.NotifyResult) error {
	mgr := notify.NewNotifyManager()
	configs, err := s.NotifyConfigModel.FindEnabled(ctx)
	if err != nil {
		return err
	}
	items := make([]notify.ConfigItem, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, notify.ConfigItem{
			Provider:        cfg.Provider,
			Config:          cfg.Config,
			Status:          cfg.Status,
			MessageTemplate: cfg.MessageTemplate,
			WebURL:          cfg.WebURL,
		})
	}
	if len(items) == 0 {
		return nil // 无可用告警通道，静默跳过
	}
	if err := mgr.LoadConfigs(items); err != nil {
		return err
	}
	if err := mgr.Send(ctx, result); err != nil {
		logx.Errorf("[ReverifyNotify] 发送修复通知失败: %v", err)
		return err
	}
	return nil
}
