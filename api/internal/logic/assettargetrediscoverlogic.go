package logic

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"
	"cscan/pkg/utils"
	"cscan/pkg/xerr"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// rediscoverCandidateLimit 重放候选任务扫描窗口：按 create_time 倒序取最近 N 条命中目标串的任务
const rediscoverCandidateLimit = 100

type AssetTargetRediscoverLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetTargetRediscoverLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetTargetRediscoverLogic {
	return &AssetTargetRediscoverLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetTargetRediscover 重新发现目标：定位该目标最近一次扫描任务，
// 复用其扫描配置、以该目标对应的原始 token 为目标重放一次（新任务）。
func (l *AssetTargetRediscoverLogic) AssetTargetRediscover(req *types.AssetTargetRediscoverReq) (*types.AssetTargetRediscoverResp, error) {
	targetId := strings.TrimSpace(req.TargetId)
	if targetId == "" {
		return nil, xerr.NewParamError("targetId is empty")
	}
	tType, tValue, err := model.DecodeTargetID(targetId)
	if err != nil {
		return nil, err
	}

	taskModel := l.svcCtx.GetMainTaskModel()

	// 候选任务：target 字段子串命中（宽松预筛，Go 侧再按 token 精确校验），create_time 倒序
	pattern := "(^|[\\n\\r,;\\s.])" + regexp.QuoteMeta(tValue) + "($|[\\n\\r,;\\s])"
	candidates, err := taskModel.Find(l.ctx, bson.M{"target": bson.M{"$regex": pattern, "$options": "i"}}, 1, rediscoverCandidateLimit)
	if err != nil {
		l.Logger.Errorf("[AssetTargetRediscover] find candidates fail: %v", err)
		return nil, xerr.NewServerError("")
	}

	var oldTask *model.MainTask
	var replayTokens []string
	for i := range candidates {
		task := &candidates[i]
		tokens := tokensForTarget(task.Target, string(tType), tValue)
		if len(tokens) == 0 {
			continue
		}
		if task.Config == "" && task.ProfileId == "" {
			continue
		}
		oldTask = task
		replayTokens = tokens
		break
	}
	if oldTask == nil {
		return nil, xerr.NewCodeErrorMsg(xerr.NotFound, "该目标暂无历史扫描任务，无法重放")
	}

	replayTarget := strings.Join(replayTokens, "\n")

	// 构建重放配置：优先任务自带 Config，其次 Profile 兜底（口径与任务重试一致）
	taskConfig := map[string]interface{}{}
	if oldTask.Config != "" {
		var savedConfig map[string]interface{}
		if err := json.Unmarshal([]byte(oldTask.Config), &savedConfig); err == nil {
			for k, v := range savedConfig {
				taskConfig[k] = v
			}
		}
	} else if oldTask.ProfileId != "" {
		profile, err := l.svcCtx.ProfileModel.FindById(l.ctx, oldTask.ProfileId)
		if err != nil || profile == nil {
			return nil, xerr.NewCodeErrorMsg(xerr.NotFound, "任务配置不存在，无法重放")
		}
		if profile.Config != "" {
			var profileConfig map[string]interface{}
			if err := json.Unmarshal([]byte(profile.Config), &profileConfig); err == nil {
				for k, v := range profileConfig {
					taskConfig[k] = v
				}
			}
		}
	}

	// 旧 Config 内嵌的是原任务全量目标，必须收敛为本次重放目标
	taskConfig["target"] = replayTarget
	taskConfig = common.InjectPocConfig(l.ctx, l.svcCtx, taskConfig, l.Logger)
	configBytes, _ := json.Marshal(taskConfig)

	newTaskId := uuid.New().String()
	newTask := &model.MainTask{
		TaskId:      newTaskId,
		Name:        oldTask.Name + " (重放)",
		Target:      replayTarget,
		ProfileId:   oldTask.ProfileId,
		ProfileName: oldTask.ProfileName,
		OrgId:       oldTask.OrgId,
		Tags:        oldTask.Tags,
		Config:      string(configBytes),
		Status:      model.TaskStatusCreated,
		CreatedBy:   middleware.GetUserId(l.ctx),
	}
	if err := taskModel.Insert(l.ctx, newTask); err != nil {
		l.Logger.Errorf("[AssetTargetRediscover] insert task fail: %v", err)
		return nil, xerr.NewServerError("创建重放任务失败: " + err.Error())
	}

	// 登记顶层目标（pending），资产空间搜索立即可见重放状态
	l.svcCtx.GetAssetTargetMetaModel().RegisterScanTargets(l.ctx, utils.SplitTargetTokens(replayTarget), "pending")

	builder := common.NewTaskBuilder(l.ctx, l.svcCtx)
	batchCount, err := builder.BuildAndPushSubTasks(newTask, taskConfig)
	if err != nil {
		l.Logger.Errorf("[AssetTargetRediscover] start task %s fail: %v", newTaskId, err)
	} else {
		l.Logger.Infof("[AssetTargetRediscover] task %s started with %d batches (from task %s)", newTaskId, batchCount, oldTask.TaskId)
	}

	return &types.AssetTargetRediscoverResp{
		Code:    0,
		Msg:     "重放任务已创建",
		TaskId:  newTaskId,
		TaskKey: newTask.Id.Hex(),
	}, nil
}

// tokensForTarget 从任务 Target 字符串中挑出归属该顶层目标的原始 token。
// token 解析口径与 RegisterScanTargets 一致：URL 取 hostname、域名取根域名、IP 精确匹配。
func tokensForTarget(rawTarget, tType, tValue string) []string {
	var tokens []string
	seen := make(map[string]struct{})
	for _, tok := range utils.SplitTargetTokens(rawTarget) {
		pType, pValue, ok := utils.ParseScanTarget(tok)
		if !ok || pType != tType {
			continue
		}
		if pType == string(model.AssetTargetTypeDomain) {
			if root := utils.GetRootDomain(pValue); root != tValue {
				continue
			}
		} else if pValue != tValue {
			continue
		}
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		tokens = append(tokens, tok)
	}
	return tokens
}
