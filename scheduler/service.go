package scheduler

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// SyncInterface 同步接口
type SyncInterface interface {
	SyncNucleiTemplates()
	SyncWappalyzerFingerprints()
	ImportCustomPocAndFingerprints()
	RefreshTemplateCache()
}

// SchedulerService 调度器服务，实现 service.Service 接口
type SchedulerService struct {
	scheduler         *Scheduler
	cronManager       *CronManager
	rdb               *redis.Client
	syncMethods       SyncInterface

	// puller 自动拉取功能已废弃（T-空间引擎定时），保留字段以避免破坏外部调用
	// puller          *OnlineAPIPuller
	// pullerSweepSpec string

	reverifier         *WeakPassReverifier
	reverifierCronSpec string

	exposureReverifier         *ExposureReverifier
	exposureReverifierCronSpec string
}

// NewSchedulerService 创建调度器服务
func NewSchedulerService(sched *Scheduler, rdb *redis.Client, syncMethods SyncInterface, taskSrc CronTaskSource) *SchedulerService {
	cronManager := NewCronManager(sched, rdb, taskSrc)

	return &SchedulerService{
		scheduler:   sched,
		cronManager: cronManager,
		rdb:         rdb,
		syncMethods: syncMethods,
	}
}

// SetOnlineAPIPuller 已废弃：自动拉取功能迁移到 Redis 订阅的空间引擎拉取任务（cscan:cron:execute_space）。
// 保留空实现以避免调用方编译错误，实际不再注册任何定时扫描。
func (s *SchedulerService) SetOnlineAPIPuller(puller *OnlineAPIPuller, sweepSpec string) {
	// 自动拉取功能已废弃，不再注册 cron 任务。
	logx.Infof("[SchedulerService] SetOnlineAPIPuller called but auto-pull is deprecated; ignored.")
}

// SetWeakPassReverifier 注入弱口令持续复验器与周期（T3.3）。
// cronSpec 为空时使用默认每日 03:00。
func (s *SchedulerService) SetWeakPassReverifier(reverifier *WeakPassReverifier, cronSpec string) {
	if cronSpec == "" {
		cronSpec = defaultReverifyCronSpec
	}
	s.reverifier = reverifier
	s.reverifierCronSpec = cronSpec
}

// SetExposureReverifier 注入敏感信息持续复验器与周期（T3.4）。
// cronSpec 为空时使用默认每日 03:00（与弱口令复验共用默认周期）。
func (s *SchedulerService) SetExposureReverifier(reverifier *ExposureReverifier, cronSpec string) {
	if cronSpec == "" {
		cronSpec = defaultReverifyCronSpec
	}
	s.exposureReverifier = reverifier
	s.exposureReverifierCronSpec = cronSpec
}

// Start 启动服务
func (s *SchedulerService) Start() {
	logx.Info("Starting scheduler service...")

	// 启动调度器
	s.scheduler.Start()

	// 加载定时任务
	ctx := context.Background()
	s.cronManager.LoadTasks(ctx)

	// 启动定时任务消息订阅
	s.cronManager.StartMessageSubscriber(ctx)

	// 在线 API 定时拉取扫描已废弃（T-空间引擎定时）：由 Redis 订阅 cscan:cron:execute_space 处理，
	// 不再注册 puller sweep cron 任务。
	// if s.puller != nil {
	// 	spec := s.pullerSweepSpec
	// 	if spec == "" {
	// 		spec = "0 * * * * *"
	// 	}
	// 	if _, err := s.scheduler.AddCronTask(spec, func() {
	// 		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	// 		defer cancel()
	// 		if err := s.puller.Run(ctx); err != nil {
	// 			logx.Errorf("[SchedulerService] online api puller run failed: %v", err)
	// 		}
	// 	}); err != nil {
	// 		logx.Errorf("[SchedulerService] register online api puller sweep failed: %v", err)
	// 	} else {
	// 		logx.Infof("[SchedulerService] online api puller sweep registered with cron spec=%q", spec)
	// 	}
	// }

	// 修复 H-8：原实现用全局固定 cron 触发，忽略每个 workspace 的 CronSpec/NextRunTime。
	// 改为 sweep 模式：每分钟扫描所有启用配置，仅原子执行 NextRunTime <= now 的 workspace，
	// 执行后按各自 CronSpec 计算下一次时间，使用户配置的 CronSpec 真正生效。
	if s.reverifier != nil {
		if _, err := s.scheduler.AddCronTask("0 * * * * *", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			s.reverifier.RunDue(ctx)
		}); err != nil {
			logx.Errorf("[SchedulerService] register weakpass reverify sweep failed: %v", err)
		} else {
			logx.Info("[SchedulerService] weakpass reverify sweep registered (every 1 minute, per-workspace CronSpec respected)")
		}
	}

	if s.exposureReverifier != nil {
		if _, err := s.scheduler.AddCronTask("0 * * * * *", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			s.exposureReverifier.RunDue(ctx)
		}); err != nil {
			logx.Errorf("[SchedulerService] register exposure reverify sweep failed: %v", err)
		} else {
			logx.Info("[SchedulerService] exposure reverify sweep registered (every 1 minute, per-workspace CronSpec respected)")
		}
	}

	// 启动后台同步任务
	if s.syncMethods != nil {
		// 先加载缓存
		s.syncMethods.RefreshTemplateCache()

		// 异步同步模板和指纹
		go s.syncMethods.SyncNucleiTemplates()
		go s.syncMethods.SyncWappalyzerFingerprints()
		go s.syncMethods.ImportCustomPocAndFingerprints()
	}

	logx.Info("Scheduler service started")
}

// Stop 停止服务
func (s *SchedulerService) Stop() {
	logx.Info("Stopping scheduler service...")

	// 停止定时任务管理器中的所有任务和订阅
	if s.cronManager != nil {
		s.cronManager.Stop()
	}

	s.scheduler.Stop()
	logx.Info("Scheduler service stopped")
}

// GetScheduler 获取调度器
func (s *SchedulerService) GetScheduler() *Scheduler {
	return s.scheduler
}

// GetCronManager 获取定时任务管理器
func (s *SchedulerService) GetCronManager() *CronManager {
	return s.cronManager
}
