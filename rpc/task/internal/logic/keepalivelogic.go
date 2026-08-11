package logic

import (
	"context"
	"encoding/json"
	"time"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type KeepAliveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewKeepAliveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KeepAliveLogic {
	return &KeepAliveLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Worker心跳
func (l *KeepAliveLogic) KeepAlive(in *pb.KeepAliveReq) (*pb.KeepAliveResp, error) {
	workerName := in.WorkerName

	// 更新Worker状态到Redis（字段名与 API WorkerList 保持一致）
	workerKey := "cscan:worker:" + workerName
	workerData := map[string]interface{}{
		"workerName":         workerName,
		"ip":                 in.Ip,
		"cpuLoad":            in.CpuLoad,
		"memUsed":            in.MemUsed,
		"taskStartedNumber":  in.TaskStartedNumber,
		"taskExecutedNumber": in.TaskExecutedNumber,
		"subCommandRunning":  in.SubCommandRunning,
		"isDaemon":           in.IsDaemon,
		"updateTime":         time.Now().Format("2006-01-02 15:04:05"),
		"status":             "online",
	}
	workerJson, _ := json.Marshal(workerData)
	// 设置60秒过期，如果Worker没有心跳则自动过期
	l.svcCtx.RedisClient.Set(l.ctx, workerKey, workerJson, 60*time.Second)

	// 添加到Worker集合
	workersKey := "cscan:workers"
	l.svcCtx.RedisClient.SAdd(l.ctx, workersKey, workerName)

	// 检查是否有控制命令
	controlKey := "cscan:worker:control:" + workerName
	controlData, err := l.svcCtx.RedisClient.Get(l.ctx, controlKey).Result()

	var resp pb.KeepAliveResp
	resp.Status = "ok"

	if err == nil && controlData != "" {
		var control map[string]bool
		if json.Unmarshal([]byte(controlData), &control) == nil {
			resp.ManualStopFlag = control["stop"]
			resp.ManualReloadFlag = control["reload"]
			resp.ManualInitEnvFlag = control["initEnv"]
			resp.ManualSyncFlag = control["sync"]
		}
		// 清除控制命令
		l.svcCtx.RedisClient.Del(l.ctx, controlKey)
	}

	return &resp, nil
}
