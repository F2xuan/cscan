package svc

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cscan/api/internal/config"
	svcsync "cscan/api/internal/svc/sync"
	"cscan/model"
	"cscan/pkg/cache"
	"cscan/rpc/task/pb"
	"cscan/scheduler"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

type ServiceContext struct {
	Config                  config.Config
	MongoClient             *mongo.Client
	MongoDB                 *mongo.Database
	RedisClient             *redis.Client
	// WorkerDefaultKey 默认 Worker 认证密钥（来自环境变量 CSCAN_WORKER_KEY）。
	// 与 Redis install_key 独立，互不影响。为空时仅校验 Redis install_key。
	WorkerDefaultKey string
	TaskRpcClient           pb.TaskServiceClient
	UserModel               *model.UserModel
	UserTokenModel          *model.UserTokenModel
	WorkspaceModel          *model.WorkspaceModel
	OrganizationModel       *model.OrganizationModel
	ProfileModel            *model.TaskProfileModel
	TagMappingModel         *model.TagMappingModel
	CustomPocModel          *model.CustomPocModel
	NucleiTemplateModel     *model.NucleiTemplateModel
	FingerprintModel        *model.FingerprintModel
	HttpServiceMappingModel *model.HttpServiceMappingModel
	HttpServiceModel        *model.HttpServiceModel // 新的HTTP服务设置模型
	ActiveFingerprintModel  *model.ActiveFingerprintModel
	CommandHistoryModel     *model.CommandHistoryModel
	AuditLogModel           *model.AuditLogModel
	NotifyConfigModel       *model.NotifyConfigModel
	ScanTemplateModel       *model.ScanTemplateModel
	CronTaskModel           *model.CronTaskModel
	WeakpassDictModel       *model.WeakpassDictModel

	// 调度器
	Scheduler *scheduler.Scheduler

	// 同步服务
	SyncMethods *svcsync.SyncMethods

	// 扫描结果服务
	ScanResultService *ScanResultService
	HistoryService    *HistoryService

	// Docker 容器服务(可选;docker.sock 不可达时为 nil,容器接口返回 503)
	DockerService *DockerService

	// 容器日志采集器(后台写本地文件,可选)
	LogCollector *LogCollector

	// Worker 日志写入器（有界 channel + flush，接收 Worker 同步来的日志写文件）
	WorkerLogWriter *WorkerLogWriter
	// Worker 日志读取器（读取 Worker 日志文件）
	WorkerLogReader *WorkerLogReader

	// TriggerWorkerLogSync 触发指定 Worker 的日志同步（由 routes.go 注入）
	// 用户点击刷新按钮时调用，向 Worker 发送 LOG_SYNC_REQ
	TriggerWorkerLogSync func(workerName string)

	// 弱口令复验立即触发（由 cscan.go 注入；T3.3 runNow 端点调用，解耦 scheduler 依赖）
	RunWeakPassReverify func(ctx context.Context, workspaceId string) error

	// 敏感信息（暴露面）复验立即触发（由 cscan.go 注入；T3.4 runNow 端点复用，解耦 scheduler 依赖）
	RunExposureReverify func(ctx context.Context, workspaceId string) error

	// 缓存的模板元数据（并发安全）
	templateMu         sync.RWMutex
	TemplateCategories []string
	TemplateTags       []string
	TemplateStats      map[string]int

	// 查询聚合结果缓存（filterOptions/iconStat/appStat/siteStat/vulStat/assetStat/workspaceIds/orgMap）
	// 短 TTL（30~60s）+ singleflight 防击穿，扫描完成可主动失效
	QueryCache *cache.LocalCache
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	// MongoDB连接
	logx.Infof("Connecting to MongoDB: %s", c.Mongo.Uri)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 配置MongoDB连接池和超时
	clientOptions := options.Client().
		ApplyURI(c.Mongo.Uri).
		SetMaxPoolSize(100).                         // 最大连接数
		SetMinPoolSize(10).                          // 最小连接数
		SetMaxConnIdleTime(30 * time.Second).        // 空闲连接超时
		SetConnectTimeout(10 * time.Second).         // 连接超时
		SetServerSelectionTimeout(10 * time.Second). // 服务器选择超时
		SetSocketTimeout(30 * time.Second)           // Socket超时

	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connect MongoDB: %w", err)
	}

	// 测试 MongoDB 连接
	if err := mongoClient.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}
	logx.Info("MongoDB connected successfully")

	mongoDB := mongoClient.Database(c.Mongo.DbName)

	// Redis连接 - 使用go-zero配置，增加连接池和超时设置
	logx.Infof("Connecting to Redis: %s", c.Redis.Host)
	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Host,
		Password:     c.Redis.Pass,
		DB:           0,
		PoolSize:     100,             // 连接池大小
		MinIdleConns: 10,              // 最小空闲连接数
		MaxRetries:   3,               // 最大重试次数
		DialTimeout:  5 * time.Second, // 连接超时
		ReadTimeout:  3 * time.Second, // 读超时
		WriteTimeout: 3 * time.Second, // 写超时
		PoolTimeout:  4 * time.Second, // 连接池超时
	})

	// 测试 Redis 连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	logx.Info("Redis connected successfully")

	// 创建RPC客户端（增加消息大小限制到50MB，支持大量指纹数据传输）
	logx.Infof("Connecting to RPC: %v", c.TaskRpc.Endpoints)
	rpcClient := zrpc.MustNewClient(c.TaskRpc, zrpc.WithDialOption(
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(50*1024*1024), // 50MB
			grpc.MaxCallSendMsgSize(50*1024*1024), // 50MB
		),
	))
	taskRpcClient := pb.NewTaskServiceClient(rpcClient.Conn())

	svcCtx := &ServiceContext{
		Config:                  c,
		MongoClient:             mongoClient,
		MongoDB:                 mongoDB,
		RedisClient:             rdb,
		WorkerDefaultKey:        os.Getenv("CSCAN_WORKER_KEY"),
		TaskRpcClient:           taskRpcClient,
		UserModel:               model.NewUserModel(mongoDB),
		UserTokenModel:          model.NewUserTokenModel(mongoDB),
		WorkspaceModel:          model.NewWorkspaceModel(mongoDB),
		OrganizationModel:       model.NewOrganizationModel(mongoDB),
		ProfileModel:            model.NewTaskProfileModel(mongoDB),
		TagMappingModel:         model.NewTagMappingModel(mongoDB),
		CustomPocModel:          model.NewCustomPocModel(mongoDB),
		NucleiTemplateModel:     model.NewNucleiTemplateModel(mongoDB),
		FingerprintModel:        model.NewFingerprintModel(mongoDB),
		HttpServiceMappingModel: model.NewHttpServiceMappingModel(mongoDB),
		HttpServiceModel:        model.NewHttpServiceModel(mongoDB),
		ActiveFingerprintModel:  model.NewActiveFingerprintModel(mongoDB),
		CommandHistoryModel:     model.NewCommandHistoryModel(mongoDB),
		AuditLogModel:           model.NewAuditLogModel(mongoDB),
		NotifyConfigModel:       model.NewNotifyConfigModel(mongoDB),
		ScanTemplateModel:       model.NewScanTemplateModel(mongoDB),
		CronTaskModel:           model.NewCronTaskModel(mongoDB),
		WeakpassDictModel:       model.NewWeakpassDictModel(mongoDB),
		Scheduler:               scheduler.NewScheduler(rdb),
		ScanResultService:       NewScanResultService(mongoDB),
		HistoryService:          NewHistoryService(mongoDB),
		TemplateCategories:      []string{},
		TemplateTags:            []string{},
		TemplateStats:           map[string]int{},
		QueryCache:              cache.NewLocalCache(60 * time.Second),
	}

	// 初始化 Docker 服务(可选,失败仅记录告警)
	if ds, err := NewDockerService(c.Docker); err != nil {
		logx.Errorf("[Docker] service unavailable: %v", err)
	} else {
		svcCtx.DockerService = ds
	}

	// 初始化容器日志采集器(可选,后台持续写入本地文件)
	if lc, err := NewLogCollector(c.Docker, c.Docker.LogDir, c.Docker.RetentionDays); err != nil {
		logx.Errorf("[LogCollector] unavailable: %v", err)
	} else {
		svcCtx.LogCollector = lc
		lc.Start()
	}

	// 初始化 Worker 日志写入器/读取器
	workerLogBaseDir := filepath.Dir(c.Docker.LogDir) // log
	svcCtx.WorkerLogWriter = NewWorkerLogWriter(workerLogBaseDir)
	svcCtx.WorkerLogReader = NewWorkerLogReader(workerLogBaseDir)

	// 初始化同步服务
	svcCtx.SyncMethods = svcsync.NewSyncMethods(
		svcCtx.NucleiTemplateModel,
		svcCtx.FingerprintModel,
		svcCtx.CustomPocModel,
		svcCtx.ActiveFingerprintModel,
		model.NewDirScanDictModel(svcCtx.MongoDB),
		model.NewSubdomainDictModel(svcCtx.MongoDB),
	)

	// 设置HTTP服务模型（用于启动时导入）
	svcCtx.SyncMethods.SetHttpServiceModel(svcCtx.HttpServiceModel)

	// 设置黑名单模型（用于启动时导入默认黑名单）
	svcCtx.SyncMethods.SetBlacklistModel(model.NewBlacklistConfigModel(svcCtx.MongoDB))

	// 设置弱口令字典模型（用于启动时导入默认字典）
	svcCtx.SyncMethods.SetWeakpassDictModel(svcCtx.WeakpassDictModel)

	// 初始化内置扫描模板
	svcsync.InitBuiltinTemplates(svcCtx.ScanTemplateModel)

	// 初始化 JSFinder 全局配置（不存在则写入内置默认值）
	svcsync.InitJSFinderConfig(model.NewJSFinderConfigModel(svcCtx.MongoDB))

	// 为已存在的内置模板补全 jsfinder 字段（标准扫描默认开启）
	svcsync.MigrateBuiltinTemplatesAddJSFinder(svcCtx.ScanTemplateModel)

	return svcCtx, nil
}

// ValidateWorkerKey 校验 Worker 密钥（双密钥接受）。
// 优先匹配环境变量 CSCAN_WORKER_KEY（默认 Worker 用，纯内存比较，无外部依赖），
// 再匹配 Redis install_key（手动探针用，可刷新、UI 展示）。
// 返回 (valid, infraError)：infraError=true 表示 Redis 基础设施故障（调用方应返回 503）。
func (s *ServiceContext) ValidateWorkerKey(ctx context.Context, providedKey string) (valid bool, infraError bool) {
	if providedKey == "" {
		return false, false
	}

	// 1. 优先校验环境变量默认密钥（内存比较，永不产生 infraError）
	if s.WorkerDefaultKey != "" &&
		subtle.ConstantTimeCompare([]byte(providedKey), []byte(s.WorkerDefaultKey)) == 1 {
		return true, false
	}

	// 2. 再校验 Redis install_key（手动探针用）
	keyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	storedKey, err := s.RedisClient.Get(keyCtx, "cscan:worker:install_key").Result()
	cancel()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			// install_key 未配置：默认密钥也未命中 → 无效（401），非基础设施故障
			return false, false
		}
		// Redis 基础设施故障 → infraError（503）
		logx.Errorf("[WorkerAuth] Redis unavailable during worker key validation: %v", err)
		return false, true
	}
	if storedKey == "" {
		return false, false
	}
	if subtle.ConstantTimeCompare([]byte(providedKey), []byte(storedKey)) == 1 {
		return true, false
	}
	return false, false
}

// GetAssetModel 根据workspaceId获取资产模型
func (s *ServiceContext) GetAssetModel(workspaceId string) *model.AssetModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewAssetModel(s.MongoDB, workspaceId)
}

// GetMainTaskModel 根据workspaceId获取主任务模型
func (s *ServiceContext) GetMainTaskModel(workspaceId string) *model.MainTaskModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewMainTaskModel(s.MongoDB, workspaceId)
}

// GetVulModel 根据workspaceId获取漏洞模型
func (s *ServiceContext) GetVulModel(workspaceId string) *model.VulModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewVulModel(s.MongoDB, workspaceId)
}

// GetAssetHistoryModel 根据workspaceId获取资产历史模型
func (s *ServiceContext) GetAssetHistoryModel(workspaceId string) *model.AssetHistoryModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewAssetHistoryModel(s.MongoDB, workspaceId)
}

// GetAssetTargetMetaModel 根据workspaceId获取顶层资产元信息模型
func (s *ServiceContext) GetAssetTargetMetaModel(workspaceId string) *model.AssetTargetMetaModel {
	if workspaceId == "" {
		workspaceId = "default"
	}
	return model.NewAssetTargetMetaModel(s.MongoDB, workspaceId)
}

// GetDirScanResultModel 获取目录扫描结果模型
func (s *ServiceContext) GetDirScanResultModel() *model.DirScanResultModel {
	return model.NewDirScanResultModel(s.MongoDB)
}

// RefreshTemplateCache 刷新模板元数据缓存
func (s *ServiceContext) RefreshTemplateCache() {
	ctx := context.Background()

	categories, err := s.NucleiTemplateModel.GetCategories(ctx)
	if err == nil {
		s.templateMu.Lock()
		s.TemplateCategories = categories
		s.templateMu.Unlock()
	}

	tags := []string{}

	stats, err := s.NucleiTemplateModel.GetStats(ctx)
	if err == nil {
		s.templateMu.Lock()
		s.TemplateStats = stats
		s.templateMu.Unlock()
	}

	s.templateMu.Lock()
	s.TemplateTags = tags
	s.templateMu.Unlock()

	s.templateMu.RLock()
	logx.Infof("[NucleiCache] Refreshed: %d categories, stats: %v", len(s.TemplateCategories), s.TemplateStats)
	s.templateMu.RUnlock()
}

// SyncNucleiTemplates 同步Nuclei模板
func (s *ServiceContext) SyncNucleiTemplates() {
	s.SyncMethods.SyncNucleiTemplates()
}

// SyncWappalyzerFingerprints 同步Wappalyzer指纹
func (s *ServiceContext) SyncWappalyzerFingerprints() {
	s.SyncMethods.SyncWappalyzerFingerprints()
}

// ImportCustomPocAndFingerprints 导入自定义POC和指纹
func (s *ServiceContext) ImportCustomPocAndFingerprints() {
	s.SyncMethods.ImportCustomPocAndFingerprints()
}

func (s *ServiceContext) GetJSFinderResultModel(workspaceId string) *model.JSFinderResultModel {
	return model.NewJSFinderResultModel(s.MongoDB, workspaceId)
}

// GetCertModel 返回指定工作空间的证书多租户模型（ARL 风格，集合 {workspaceId}_cert）
func (s *ServiceContext) GetCertModel(workspaceId string) *model.CertModel {
	return model.NewCertModel(s.MongoDB, workspaceId)
}

// GetReverifyConfigModel 返回复验配置模型（T3.3/T3.4，单集合 + workspace_id 隔离）
func (s *ServiceContext) GetReverifyConfigModel() *model.ReverifyConfigModel {
	return model.NewReverifyConfigModel(s.MongoDB)
}
