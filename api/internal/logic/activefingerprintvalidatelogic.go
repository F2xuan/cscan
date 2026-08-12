package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/scheduler"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

// ActiveFingerprintValidateLogic 验证主动指纹（通过RPC下发给Worker执行）
type ActiveFingerprintValidateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewActiveFingerprintValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActiveFingerprintValidateLogic {
	return &ActiveFingerprintValidateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ActiveFingerprintValidate 验证主动指纹（下发到Worker执行）
func (l *ActiveFingerprintValidateLogic) ActiveFingerprintValidate(req *types.ActiveFingerprintValidateReq) (*types.ActiveFingerprintValidateResp, error) {
	if req.Id == "" {
		return &types.ActiveFingerprintValidateResp{Code: 400, Msg: "主动指纹ID不能为空"}, nil
	}
	if req.Url == "" {
		return &types.ActiveFingerprintValidateResp{Code: 400, Msg: "目标URL不能为空"}, nil
	}

	// 验证主动指纹存在
	activeFp, err := l.svcCtx.ActiveFingerprintModel.FindById(l.ctx, req.Id)
	if err != nil {
		logx.Errorf("ActiveFingerprintValidate: find active fingerprint failed, id=%s, error=%v", req.Id, err)
		return &types.ActiveFingerprintValidateResp{Code: 500, Msg: "查询主动指纹失败"}, nil
	}
	if activeFp == nil {
		return &types.ActiveFingerprintValidateResp{Code: 404, Msg: "主动指纹不存在"}, nil
	}

	l.Logger.Infof("ActiveFingerprintValidate: activeFpId=%s, name=%s, url=%s", req.Id, activeFp.Name, req.Url)

	// 检查在线 Worker
	if err := checkOnlineWorkers(l.ctx, l.svcCtx); err != nil {
		return &types.ActiveFingerprintValidateResp{Code: 500, Msg: err.Error()}, nil
	}

	// 直接入队主动指纹验证任务
	taskId := uuid.New().String()
	taskConfig := map[string]interface{}{
		"taskType":   "active_fingerprint_validate",
		"url":        req.Url,
		"activeFpId": req.Id,
		"timeout":    60,
	}
	configBytes, _ := json.Marshal(taskConfig)

	task := &scheduler.TaskInfo{
		TaskId:      taskId,
		MainTaskId:  taskId,
		WorkspaceId: "default",
		TaskName:    "主动指纹验证",
		Config:      string(configBytes),
		Priority:    2,
	}

	if err := l.svcCtx.Scheduler.PushTask(l.ctx, task); err != nil {
		l.Logger.Errorf("ActiveFingerprintValidate: push task failed, taskId=%s, error=%v", taskId, err)
		return &types.ActiveFingerprintValidateResp{Code: 500, Msg: "任务下发失败"}, nil
	}

	persistTaskInfo(l.ctx, l.svcCtx, taskId, taskConfig)
	result, err := l.waitForActiveFingerprintValidateResult(taskId, 30*time.Second)
	if err != nil {
		l.Logger.Errorf("ActiveFingerprintValidate: wait result failed, taskId=%s, error=%v", taskId, err)
		return &types.ActiveFingerprintValidateResp{Code: 500, Msg: "等待验证结果超时: " + err.Error()}, nil
	}
	if result.Error != "" {
		return &types.ActiveFingerprintValidateResp{Code: 500, Msg: result.Error}, nil
	}

	// 转换结果
	var items []types.ActiveFingerprintValidateItem
	for _, pr := range result.PathResults {
		items = append(items, types.ActiveFingerprintValidateItem{
			Path:           pr.Path,
			StatusCode:     pr.StatusCode,
			Matched:        pr.Matched,
			MatchedRule:    pr.MatchedRule,
			MatchedDetails: pr.MatchedDetails,
		})
	}

	return &types.ActiveFingerprintValidateResp{
		Code:    0,
		Msg:     "验证完成",
		Matched: result.Matched,
		Results: items,
	}, nil
}

// waitForActiveFingerprintValidateResult 轮询等待主动指纹验证结果
func (l *ActiveFingerprintValidateLogic) waitForActiveFingerprintValidateResult(taskId string, timeout time.Duration) (*WorkerActiveFpResult, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		select {
		case <-l.ctx.Done():
			return nil, l.ctx.Err()
		case <-time.After(pollInterval):
		}

		statusKey := "cscan:task:status:" + taskId
		val, err := l.svcCtx.RedisClient.Get(l.ctx, statusKey).Result()
		if err != nil {
			continue
		}

		var statusData struct {
			State  string `json:"state"`
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(val), &statusData); err != nil {
			continue
		}

		if statusData.State == "SUCCESS" || statusData.State == "FAILURE" {
			var resultWrapper struct {
				Status string               `json:"status"`
				Result WorkerActiveFpResult `json:"result"`
				Error  string               `json:"error"`
			}
			if err := json.Unmarshal([]byte(statusData.Result), &resultWrapper); err != nil {
				return &WorkerActiveFpResult{Error: "解析结果失败: " + err.Error()}, nil
			}
			if resultWrapper.Error != "" {
				resultWrapper.Result.Error = resultWrapper.Error
			}
			return &resultWrapper.Result, nil
		}
	}

	return nil, fmt.Errorf("等待验证结果超时(%v)", timeout)
}

// WorkerActiveFpResult Worker返回的主动指纹验证结果
type WorkerActiveFpResult struct {
	Matched     bool               `json:"matched"`
	PathResults []WorkerPathResult `json:"pathResults"`
	Error       string             `json:"error"`
}

// WorkerPathResult 单个路径验证结果
type WorkerPathResult struct {
	Path           string `json:"path"`
	StatusCode     int    `json:"statusCode"`
	Matched        bool   `json:"matched"`
	MatchedRule    string `json:"matchedRule"`
	MatchedDetails string `json:"matchedDetails"`
	Error          string `json:"error"`
}
