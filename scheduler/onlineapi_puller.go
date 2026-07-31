package scheduler

import (
	"context"

	"cscan/model"
	"cscan/pkg/notify"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// OnlineAPIPullSender 新资产通知发送函数（由 API 层注入，复用 NotifyManager）。
// 保留类型签名以兼容外部调用；实际自动拉取已废弃，该 sender 不会再被调用。
type OnlineAPIPullSender func(ctx context.Context, result *notify.NotifyResult) error

// OnlineAPIPuller 在线 API 定时拉取器（已废弃）。
//
// 历史上（T3.1）该组件周期性扫描各工作空间的 APIConfig，按 NextRunTime 触发
// Fofa/Hunter/Quake 自动拉取。该功能现已迁移至 Redis 订阅模式：
//   - 订阅频道 "cscan:cron:execute_space" 由 createAndPushSpaceEngineTask 处理；
//   - 自动拉取配置字段（AutoPullEnabled/CronSpec/Queries/...）已从 model.APIConfig 中移除。
//
// 为避免破坏现有调用方（api/cscan.go 中的 NewOnlineAPIPuller / SetOnlineAPIPuller），
// 保留结构体与构造器签名，但 Run 方法退化为空操作。
type OnlineAPIPuller struct {
	db             *mongo.Database
	workspaceModel *model.WorkspaceModel
	sender         OnlineAPIPullSender
}

// NewOnlineAPIPuller 创建在线 API 定时拉取器（已废弃，返回空操作实例）。
func NewOnlineAPIPuller(db *mongo.Database, wm *model.WorkspaceModel, sender OnlineAPIPullSender) *OnlineAPIPuller {
	logx.Infof("[OnlineAPIPuller] deprecated: auto-pull disabled, use Redis channel cscan:cron:execute_space instead")
	return &OnlineAPIPuller{
		db:             db,
		workspaceModel: wm,
		sender:         sender,
	}
}

// Run 保留空实现以兼容 SchedulerService 调用路径；实际不再执行任何拉取。
func (p *OnlineAPIPuller) Run(ctx context.Context) error {
	return nil
}
