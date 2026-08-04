package logic

import (
	"context"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 弱口令 / 敏感信息持续复验配置默认值（T3.3 / T3.4，与 scheduler/reverify_weakpass.go 对齐）
const (
	defaultReverifyWeakPassEnabled = false
	defaultReverifyExposureEnabled = false
	defaultReverifyCronSpec        = "0 0 3 * * *" // 每日 03:00（秒级 6 字段）
	defaultReverifyMaxTargets      = 200
	defaultReverifyConcurrency     = 1 // 串行执行，上一个验证完才能到下一个
)

// ReverifyConfigGetLogic 获取复验配置
type ReverifyConfigGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReverifyConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReverifyConfigGetLogic {
	return &ReverifyConfigGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReverifyConfigGet 返回当前工作空间的复验配置；无配置时返回默认值（weakPassEnabled=false）。
func (l *ReverifyConfigGetLogic) ReverifyConfigGet(req *types.ReverifyConfigGetReq) (*types.ReverifyConfigGetResp, error) {
	// 单租户化：workspaceId 为空时回退到默认工作空间
	req.WorkspaceId = common.GetDefaultWorkspaceId(l.ctx, l.svcCtx, req.WorkspaceId)
	cfg, err := l.svcCtx.GetReverifyConfigModel().GetByWorkspace(l.ctx, req.WorkspaceId)
	if err != nil {
		return &types.ReverifyConfigGetResp{Code: 500, Msg: "查询失败: " + err.Error()}, nil
	}
	if cfg == nil {
		return &types.ReverifyConfigGetResp{
			Code: 0,
			Msg:  "success",
			Config: &types.ReverifyConfig{
				WorkspaceId:      req.WorkspaceId,
				WeakPassEnabled:  defaultReverifyWeakPassEnabled,
				ExposureEnabled:  defaultReverifyExposureEnabled,
				CronSpec:         defaultReverifyCronSpec,
				MaxTargetsPerRun: defaultReverifyMaxTargets,
				Concurrency:      defaultReverifyConcurrency,
			},
		}, nil
	}
	return &types.ReverifyConfigGetResp{Code: 0, Msg: "success", Config: toReverifyConfigType(cfg)}, nil
}

// ReverifyConfigSaveLogic 保存复验配置
type ReverifyConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReverifyConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReverifyConfigSaveLogic {
	return &ReverifyConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReverifyConfigSave 保存复验配置（空值回退默认值；不触碰运行状态字段）。
func (l *ReverifyConfigSaveLogic) ReverifyConfigSave(req *types.ReverifyConfigSaveReq) (*types.ReverifyConfigSaveResp, error) {
	// 单租户化：workspaceId 为空时回退到默认工作空间
	req.WorkspaceId = common.GetDefaultWorkspaceId(l.ctx, l.svcCtx, req.WorkspaceId)
	cronSpec := req.CronSpec
	if cronSpec == "" {
		cronSpec = defaultReverifyCronSpec
	}
	maxTargets := req.MaxTargetsPerRun
	if maxTargets <= 0 {
		maxTargets = defaultReverifyMaxTargets
	}
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = defaultReverifyConcurrency
	}

	doc := &model.ReverifyConfig{
		WeakPassEnabled:  req.WeakPassEnabled,
		ExposureEnabled:  req.ExposureEnabled,
		CronSpec:         cronSpec,
		MaxTargetsPerRun: maxTargets,
		Concurrency:      concurrency,
	}
	if err := l.svcCtx.GetReverifyConfigModel().Upsert(l.ctx, doc); err != nil {
		return &types.ReverifyConfigSaveResp{Code: 500, Msg: "保存失败: " + err.Error()}, nil
	}
	return &types.ReverifyConfigSaveResp{Code: 0, Msg: "保存成功", Config: toReverifyConfigType(doc)}, nil
}

// ReverifyRunNowLogic 立即触发弱口令复验（T3.3）
type ReverifyRunNowLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReverifyRunNowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReverifyRunNowLogic {
	return &ReverifyRunNowLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReverifyRunNow 经 svcCtx 注入的复验器，对单个工作空间立即复验（T3.3 弱口令 + T3.4 敏感信息，按各自开关触发）。
func (l *ReverifyRunNowLogic) ReverifyRunNow(req *types.ReverifyRunNowReq) (*types.ReverifyRunNowResp, error) {
	if l.svcCtx.RunWeakPassReverify == nil && l.svcCtx.RunExposureReverify == nil {
		return &types.ReverifyRunNowResp{Code: 500, Msg: "复验服务未初始化"}, nil
	}
	var errMsg []string
	if l.svcCtx.RunWeakPassReverify != nil {
		if err := l.svcCtx.RunWeakPassReverify(l.ctx, req.WorkspaceId); err != nil {
			errMsg = append(errMsg, "弱口令:"+err.Error())
		}
	}
	if l.svcCtx.RunExposureReverify != nil {
		if err := l.svcCtx.RunExposureReverify(l.ctx, req.WorkspaceId); err != nil {
			errMsg = append(errMsg, "敏感信息:"+err.Error())
		}
	}
	if len(errMsg) > 0 {
		return &types.ReverifyRunNowResp{Code: 500, Msg: "复验失败: " + strings.Join(errMsg, "; ")}, nil
	}
	return &types.ReverifyRunNowResp{Code: 0, Msg: "已触发复验"}, nil
}

// toReverifyConfigType 将 model 配置转换为 API 类型（运行状态字段一并回传）
func toReverifyConfigType(c *model.ReverifyConfig) *types.ReverifyConfig {
	if c == nil {
		return nil
	}
	return &types.ReverifyConfig{
		Id:               objectIDHex(c.Id),
		WorkspaceId:      "default",
		WeakPassEnabled:  c.WeakPassEnabled,
		ExposureEnabled:  c.ExposureEnabled,
		CronSpec:         c.CronSpec,
		MaxTargetsPerRun: c.MaxTargetsPerRun,
		Concurrency:      c.Concurrency,
		LastRunTime:      optTimeStr(c.LastRunTime),
		LastRunStatus:    c.LastRunStatus,
		LastRunCount:     c.LastRunCount,
		LastRunError:     c.LastRunError,
		NextRunTime:      optTimeStr(c.NextRunTime),
		CreateTime:       c.CreateTime.Format("2006-01-02 15:04:05"),
		UpdateTime:       c.UpdateTime.Format("2006-01-02 15:04:05"),
	}
}

// objectIDHex 从可能为 interface{} 的 _id 提取十六进制字符串
func objectIDHex(id interface{}) string {
	if oid, ok := id.(primitive.ObjectID); ok {
		return oid.Hex()
	}
	return ""
}

// optTimeStr 格式化可选时间字段；为零值返回空字符串
func optTimeStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
