package svc

import (
	"context"
	"fmt"
	"time"

	"cscan/model"
	"cscan/rpc/task/internal/config"
	"cscan/scheduler"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ServiceContext struct {
	Config                  config.Config
	MongoClient             *mongo.Client
	MongoDB                 *mongo.Database
	RedisClient             *redis.Client
	Scheduler               *scheduler.Scheduler            // 任务调度器（NewTask 统一入口，修复优先级失效问题）
	NucleiTemplateModel     *model.NucleiTemplateModel
	FingerprintModel        *model.FingerprintModel
	CustomPocModel          *model.CustomPocModel
	HttpServiceMappingModel *model.HttpServiceMappingModel
	SubfinderProviderModel  *model.SubfinderProviderModel
	NotifyConfigModel       *model.NotifyConfigModel
	TaskRecoveryManager     *scheduler.TaskRecoveryManager // 任务恢复管理器
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	logx.Infof("Connecting to MongoDB: %s", c.Mongo.Uri)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 配置 MongoDB 连接池参数，避免高并发下连接耗尽
	//   MaxPoolSize=100：单个客户端最多复用 100 个连接（mongo-driver 默认 100，显式声明）
	//   MinPoolSize=10：保持 10 个空闲连接，减少突发流量下的握手开销
	//   MaxConnIdleTime=5min：空闲连接超过 5 分钟回收，避免长时间占用资源
	clientOpts := options.Client().
		ApplyURI(c.Mongo.Uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(5 * time.Minute)

	mongoClient, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connect MongoDB: %w", err)
	}

	// 测试 MongoDB 连接
	if err := mongoClient.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}
	logx.Info("MongoDB connected successfully")

	mongoDB := mongoClient.Database(c.Mongo.DbName)

	// 使用go-zero Redis配置
	logx.Infof("Connecting to Redis: %s", c.RedisConf.Host)
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.RedisConf.Host,
		Password: c.RedisConf.Pass,
		DB:       0,
	})

	// 测试 Redis 连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	logx.Info("Redis connected successfully")

	// 创建任务调度器（NewTask 通过此入口推送任务，确保优先级分数计算一致）
	// 修复历史问题：原 NewTaskLogic 直接 ZAdd，绕过 calculatePriorityScore，Priority 字段失效
	taskScheduler := scheduler.NewScheduler(rdb)

	// 创建任务恢复管理器，共享 scheduler 实例
	// 修复历史问题：原 recoverTask 用 time.Now().Unix()，高优先级任务恢复后降级
	// 共享 scheduler 后可复用 calculatePriorityScore，保留原优先级
	recoveryManager := scheduler.NewTaskRecoveryManager(rdb, context.Background(), taskScheduler)
	recoveryManager.Start()
	logx.Info("Task recovery manager started")

	return &ServiceContext{
		Config:                  c,
		MongoClient:             mongoClient,
		MongoDB:                 mongoDB,
		RedisClient:             rdb,
		Scheduler:               taskScheduler,
		NucleiTemplateModel:     model.NewNucleiTemplateModel(mongoDB),
		FingerprintModel:        model.NewFingerprintModel(mongoDB),
		CustomPocModel:          model.NewCustomPocModel(mongoDB),
		HttpServiceMappingModel: model.NewHttpServiceMappingModel(mongoDB),
		SubfinderProviderModel:  model.NewSubfinderProviderModel(mongoDB),
		NotifyConfigModel:       model.NewNotifyConfigModel(mongoDB),
		TaskRecoveryManager:     recoveryManager,
	}, nil
}

func (s *ServiceContext) GetAssetModel(workspaceId string) *model.AssetModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewAssetModel(s.MongoDB, workspaceId)
}

func (s *ServiceContext) GetMainTaskModel(workspaceId string) *model.MainTaskModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewMainTaskModel(s.MongoDB, workspaceId)
}

func (s *ServiceContext) GetVulModel(workspaceId string) *model.VulModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewVulModel(s.MongoDB, workspaceId)
}

func (s *ServiceContext) GetExecutorTaskModel(workspaceId string) *model.ExecutorTaskModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewExecutorTaskModel(s.MongoDB, workspaceId)
}

func (s *ServiceContext) GetAssetHistoryModel(workspaceId string) *model.AssetHistoryModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewAssetHistoryModel(s.MongoDB, workspaceId)
}

// GetAssetTargetMetaModel 顶层资产元信息模型（per-workspace 集合 {wsId}_asset_target_meta）。
// 扫描结果保存时调用 EnsureForAsset 登记/刷新顶层资产，否则资产页只见手动新增。
func (s *ServiceContext) GetAssetTargetMetaModel(workspaceId string) *model.AssetTargetMetaModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewAssetTargetMetaModel(s.MongoDB, workspaceId)
}
