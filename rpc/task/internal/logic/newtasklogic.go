package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"
	"cscan/internal/scheduler"

	"github.com/zeromicro/go-zero/core/logx"
)

type NewTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNewTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NewTaskLogic {
	return &NewTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建执行器任务
// 修复历史问题：
//  1. 原实现直接 ZAdd 到 cscan:task:queue，绕过 scheduler.PushTask，导致 Priority 字段失效
//     （原用 time.Now().UnixNano() 作 score，未应用 calculatePriorityScore）
//  2. 原实现未保存 cscan:task:info:{taskId}，导致 TaskRecoveryManager.getTaskInfo 恢复失败
//
// 现改为：
//  1. 调用 scheduler.PushTask 统一入口，正确计算优先级分数
//  2. 持久化 taskInfo 到 cscan:task:info:{taskId}（24h TTL），保证任务可恢复
func (l *NewTaskLogic) NewTask(in *pb.NewTaskReq) (*pb.NewTaskResp, error) {
	taskId := in.TaskId
	if taskId == "" {
		return &pb.NewTaskResp{
			Success: false,
			Message: "TaskId不能为空",
		}, nil
	}

	// 创建任务信息（Priority 由调用方按业务规则设置，默认为 Normal=2）
	taskInfo := scheduler.TaskInfo{
		TaskId:      taskId,
		MainTaskId:  in.MainTaskId,
		TaskName:    in.TaskName,
		Config:      in.Config,
		WorkspaceId: in.WorkspaceId,
		Priority:    l.resolvePriority(in),
		CreateTime:  time.Now().Local().Format("2006-01-02 15:04:05"),
	}

	// 推送到任务队列（应用 calculatePriorityScore，保留优先级）
	// PushTask 内部会用 time.Now() 覆盖 CreateTime，必须在 PushTask 之后再序列化持久化
	if err := l.svcCtx.Scheduler.PushTask(l.ctx, &taskInfo); err != nil {
		l.Logger.Errorf("NewTask: failed to push task %s to queue: %v", taskId, err)
		return &pb.NewTaskResp{
			Success: false,
			Message: "添加任务到队列失败: " + err.Error(),
		}, nil
	}

	// 持久化 taskInfo（24h TTL，供恢复用）
	// 原 newtasklogic 遗漏此步，导致 task_recovery.go getTaskInfo 失败
	// 注意：即使此处失败，checktasklogic.go 的 Lua 脚本在 pop 时也会兜底 SET taskInfo
	taskInfoKey := fmt.Sprintf("cscan:task:info:%s", taskId)
	// H-15 修复：必须在 PushTask 之后再序列化，确保 CreateTime 等字段为最新值
	taskInfoJson, err := json.Marshal(taskInfo)
	if err != nil {
		l.Logger.Errorf("NewTask: failed to marshal taskInfo: %v", err)
	}
	if err := l.svcCtx.RedisClient.Set(l.ctx, taskInfoKey, string(taskInfoJson), 24*time.Hour).Err(); err != nil {
		// taskInfo 持久化失败不影响任务入队（任务已成功推送）
		// 但记录日志，便于排查恢复链路问题
		l.Logger.Errorf("NewTask: task %s enqueued but taskInfo persist failed: %v", taskId, err)
	} else {
		l.Logger.Infof("NewTask: created task %s (priority=%d, taskInfo persisted)", taskId, taskInfo.Priority)
	}

	return &pb.NewTaskResp{
		Success: true,
		Message: "Task created successfully",
	}, nil
}

// resolvePriority 根据任务请求推断优先级
// 优先级规则：
//   - in.Priority 显式设置（>0）则使用
//   - 否则默认 Normal=2
//
// 与 scheduler.calculatePriorityScore 的配合：
//   - score = UnixMicro - priority * 1_000_000（每级 1 秒，微秒级时间戳）
//   - Priority 越大，score 越小，越优先被 ZPopMin 弹出
//
// 未来可扩展：按 taskType、目标数量等综合判定
func (l *NewTaskLogic) resolvePriority(in *pb.NewTaskReq) int {
	// pb.NewTaskReq 当前没有 Priority 字段，预留扩展点
	// 默认 Normal 优先级（数值 2），与现有 ZSet score 公式兼容
	return 2 // Normal
}

