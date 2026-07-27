package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"
	"cscan/scheduler"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type CheckTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckTaskLogic {
	return &CheckTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 检查任务状态 - 从Redis队列中获取待执行的任务
// 优先从 Worker 专属队列获取任务，然后从公共队列获取
//
// 当 svcCtx.Scheduler.IsPriorityBucketEnabled()=true 时，公共队列走 5 级分桶路径
// 用单 Lua 脚本原子完成 Worker 专属队列 + p4~p0 分桶弹出 + taskInfo/execution 持久化
// 修复历史问题：原 CheckTask 只读 cscan:task:queue 单 ZSet，分桶开启后任务无法被消费
func (l *CheckTaskLogic) CheckTask(in *pb.CheckTaskReq) (*pb.CheckTaskResp, error) {
	// 说明：不在函数入口回写 l.ctx 加超时。原实现 l.ctx = ctx 会把 5s 超时的 ctx
	// 留在 struct 字段，defer cancel() 后任何逃逸使用 l.ctx 的代码会拿到已取消的 context。
	// 各子函数直接使用 l.ctx；MongoDB 短超时由调用方逐处控制。

	workerName := in.TaskId // TaskId 实际上是 Worker 名称

	publicQueueKey := "cscan:task:queue"
	workerQueueKey := "cscan:task:queue:worker:" + strings.ToLower(workerName)
	processingKey := "cscan:task:processing"

	// 分桶路径：单脚本原子完成 Worker 专属队列 + 5 级公共分桶弹出
	if l.svcCtx.Scheduler != nil && l.svcCtx.Scheduler.IsPriorityBucketEnabled() {
		return l.popTaskFromBuckets(workerQueueKey, publicQueueKey, processingKey, workerName)
	}

	// 默认路径：Worker 专属队列 → 公共单 ZSet 队列（两次调用）
	// 1. 优先从 Worker 专属队列获取任务（使用 ZPopMin 原子操作）
	task, err := l.popTaskFromQueue(workerQueueKey, processingKey, workerName)
	if err != nil {
		l.Logger.Errorf("CheckTask: failed to pop from worker queue: %v", err)
	}
	if task != nil {
		return task, nil
	}

	// 2. 从公共队列获取任务（使用 ZPopMin 原子操作）
	task, err = l.popTaskFromQueue(publicQueueKey, processingKey, workerName)
	if err != nil {
		l.Logger.Errorf("CheckTask: failed to pop from public queue: %v", err)
	}
	if task != nil {
		return task, nil
	}

	return &pb.CheckTaskResp{IsExist: false}, nil
}

// atomicPopTaskScript 原子化弹出任务 Lua 脚本
// 修复历史问题：原 ZPopMin + SAdd + RecordTaskStart + Set 为 4 个独立 Redis 调用
// 中间崩溃会导致任务处于"已弹出但未记录"的孤儿状态，虽 TaskRecoveryManager 会兜底但恢复信息缺失
//
// 脚本原子完成：
//  1. ZPOPMIN 从队列弹出优先级最高的任务
//  2. SADD 加入 processing 集合
//  3. SET 记录 taskInfo 到 cscan:task:info:{taskId}（String 形式，原始 JSON），TTL 24h
//  4. SET 记录 execution info 到 cscan:task:execution:{taskId}，TTL 1h
//  5. 异常 member 移入 cscan:task:deadletter 死信队列（避免阻塞原队列）
//
// 修复 C6：死信 PUBLISH 移出原子 Lua,避免订阅消费慢或缓冲满时阻塞整个 pop 事务。
// Lua 仅做 ZADD 死信并返回 "__DL__" 前缀哨兵,由调用方在 Lua 返回后再 PUBLISH。
//
// 参数：
//
//	KEYS[1] = queueKey（队列名）
//	KEYS[2] = processingKey（处理中集合）
//	ARGV[1] = taskInfoKey 前缀（cscan:task:info:）
//	ARGV[2] = executionKey 前缀（cscan:task:execution:）
//	ARGV[3] = workerName
//	ARGV[4] = 24h TTL（秒）
//	ARGV[5] = 当前时间戳（RFC3339，用于 execution info）
//
// 返回：任务 JSON 字符串，或 "__DL__"+member 哨兵（死信），或 nil（队列为空）
var atomicPopTaskScript = redis.NewScript(`
	local queueKey = KEYS[1]
	local processingKey = KEYS[2]
	local taskInfoPrefix = ARGV[1]
	local execPrefix = ARGV[2]
	local workerName = ARGV[3]
	local ttlSeconds = tonumber(ARGV[4])
	local nowStr = ARGV[5]

	local result = redis.call('ZPOPMIN', queueKey, 1)
	if #result == 0 then
		return nil
	end
	local member = result[1]
	local score = result[2]

	-- 解析任务 JSON 提取 taskId
	local data = nil
	pcall(function() data = cjson.decode(member) end)
	if not data or not data.taskId or data.taskId == '' then
		-- 修复 C2：decode 失败时移入死信队列，避免放回后形成无限循环阻塞整个队列
		redis.call('ZADD', 'cscan:task:deadletter', score, member)
		-- 修复 C6：返回 "__DL__" 前缀哨兵,由调用方在 Lua 外执行 PUBLISH,避免阻塞 pop 事务
		return '__DL__' .. member
	end
	local taskId = data.taskId

	-- 原子加入 processing 集合
	redis.call('SADD', processingKey, taskId)

	-- 持久化 taskInfo（24h TTL，供 TaskRecoveryManager 恢复读取）
	local taskInfoKey = taskInfoPrefix .. taskId
	redis.call('SET', taskInfoKey, member, 'EX', ttlSeconds)

	-- 记录 execution info（与 TaskRecoveryManager.saveTaskExecutionInfo 格式一致）
	local execKey = execPrefix .. taskId
	local execInfo = cjson.encode({
		taskId = taskId,
		workerName = workerName,
		startTime = nowStr,
		lastUpdate = nowStr,
		phase = "started",
		progress = 0,
		retryCount = 0,
		maxRetries = 3
	})
	redis.call('SET', execKey, execInfo, 'EX', 3600)

	return member
`)

// publishDeadLetterAlert 在 pop 脚本外执行死信告警 PUBLISH,避免阻塞 pop 事务。
// PUBLISH 失败仅记录日志,不影响后续 pop。
func (l *CheckTaskLogic) publishDeadLetterAlert(member string) {
	if err := l.svcCtx.RedisClient.Publish(l.ctx, "cscan:task:deadletter:alert", member).Err(); err != nil {
		l.Logger.Errorf("CheckTask: publish deadletter alert failed: %v", err)
	}
}

// popTaskFromQueue 从指定队列原子获取一个任务
// 修复历史问题：原实现 ZPopMin + SAdd + RecordTaskStart + Set 非原子，崩溃窗口存在孤儿任务
// 现改为单次 Lua 脚本原子完成，且省去 RecordTaskStart 的一次 RPC 往返
func (l *CheckTaskLogic) popTaskFromQueue(queueKey, processingKey, workerName string) (*pb.CheckTaskResp, error) {
	// 执行原子化 Lua 脚本
	result, err := atomicPopTaskScript.Run(l.ctx, l.svcCtx.RedisClient,
		[]string{queueKey, processingKey},
		"cscan:task:info:",
		"cscan:task:execution:",
		workerName,
		86400, // 24h
		time.Now().Format(time.RFC3339),
	).Result()

	if err == redis.Nil || result == nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	taskData, ok := result.(string)
	if !ok {
		l.Logger.Errorf("CheckTask: unexpected script result type: %T", result)
		return nil, nil
	}

	// 修复 C6：死信哨兵在 Lua 外 PUBLISH,避免阻塞 pop 事务
	if strings.HasPrefix(taskData, "__DL__") {
		l.publishDeadLetterAlert(strings.TrimPrefix(taskData, "__DL__"))
		return nil, nil
	}

	var task scheduler.TaskInfo
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		l.Logger.Errorf("CheckTask: failed to parse task: %v", err)
		return nil, nil
	}

	l.Logger.Infof("CheckTask: atomically assigned task %s to worker %s from queue %s", task.TaskId, workerName, queueKey)

	// 立即更新主任务状态为 STARTED
	l.updateMainTaskToStarted(task.MainTaskId, task.WorkspaceId)

	return &pb.CheckTaskResp{
		IsExist:     true,
		IsFinished:  false,
		TaskId:      task.TaskId,
		MainTaskId:  task.MainTaskId,
		WorkspaceId: task.WorkspaceId,
		Config:      task.Config,
	}, nil
}

// atomicPopFromBucketsScript 分桶路径原子化弹出任务 Lua 脚本
// 修复历史问题：原 CheckTask 只读 cscan:task:queue，enablePriorityBucket=true 时分桶任务无法被消费
// 脚本原子完成：
//  1. ZPOPMIN 从 Worker 专属队列弹出（优先）
//  2. 若为空，按 p4 Urgent -> p0 Background 顺序跨 5 个公共分桶弹出
//  3. SADD 加入 processing 集合
//  4. SET taskInfo（24h TTL，供 TaskRecoveryManager 恢复读取）
//  5. SET execution info（1h TTL）
//
// 参数：
//
//	KEYS[1] = workerQueueKey（Worker 专属队列）
//	KEYS[2] = bucketPrefix（公共分桶前缀，如 "cscan:task:queue"）
//	KEYS[3] = processingKey（处理中集合）
//	ARGV[1] = taskInfoKey 前缀（cscan:task:info:）
//	ARGV[2] = executionKey 前缀（cscan:task:execution:）
//	ARGV[3] = workerName
//	ARGV[4] = 24h TTL（秒）
//	ARGV[5] = 当前时间戳（RFC3339）
//
// 返回：任务 JSON 字符串，或 nil（队列均为空）
var atomicPopFromBucketsScript = redis.NewScript(`
	local workerQueueKey = KEYS[1]
	local bucketPrefix = KEYS[2]
	local processingKey = KEYS[3]
	local taskInfoPrefix = ARGV[1]
	local execPrefix = ARGV[2]
	local workerName = ARGV[3]
	local ttlSeconds = tonumber(ARGV[4])
	local nowStr = ARGV[5]

	-- 1. 先从 Worker 专属队列弹出
	local sourceKey = workerQueueKey
	local result = redis.call('ZPOPMIN', sourceKey, 1)

	-- 2. 若为空，跨 5 个公共分桶按 p4 -> p0 顺序弹出（urgent 先，background 后）
	if #result == 0 then
		for i = 4, 0, -1 do
			sourceKey = bucketPrefix .. ":p" .. i
			result = redis.call('ZPOPMIN', sourceKey, 1)
			if #result > 0 then
				break
			end
		end
	end

	if #result == 0 then
		return nil
	end

	local member = result[1]
	local score = result[2]

	-- 解析任务 JSON 提取 taskId
	local data = nil
	pcall(function() data = cjson.decode(member) end)
	if not data or not data.taskId or data.taskId == '' then
		-- 修复 C2：decode 失败时移入死信队列，避免放回后形成无限循环阻塞分桶队列
		-- 修复 C6：返回 "__DL__" 前缀哨兵,由调用方在 Lua 外执行 PUBLISH,避免阻塞 pop 事务
		redis.call('ZADD', 'cscan:task:deadletter', score, member)
		return '__DL__' .. member
	end
	local taskId = data.taskId

	-- 原子加入 processing 集合
	redis.call('SADD', processingKey, taskId)

	-- 持久化 taskInfo（24h TTL，供 TaskRecoveryManager 恢复读取）
	local taskInfoKey = taskInfoPrefix .. taskId
	redis.call('SET', taskInfoKey, member, 'EX', ttlSeconds)

	-- 记录 execution info（与 TaskRecoveryManager.saveTaskExecutionInfo 格式一致）
	local execKey = execPrefix .. taskId
	local execInfo = cjson.encode({
		taskId = taskId,
		workerName = workerName,
		startTime = nowStr,
		lastUpdate = nowStr,
		phase = "started",
		progress = 0,
		retryCount = 0,
		maxRetries = 3
	})
	redis.call('SET', execKey, execInfo, 'EX', 3600)

	return member
`)

// popTaskFromBuckets 分桶路径原子弹出：Worker 专属队列 + 5 级公共分桶
// 修复历史问题：原 CheckTask 未适配分桶，enablePriorityBucket=true 时任务无法被消费
// 同时对齐 atomicPopTaskScript 的 taskInfo/execution 持久化，确保 TaskRecoveryManager 可恢复
func (l *CheckTaskLogic) popTaskFromBuckets(workerQueueKey, bucketPrefix, processingKey, workerName string) (*pb.CheckTaskResp, error) {
	result, err := atomicPopFromBucketsScript.Run(l.ctx, l.svcCtx.RedisClient,
		[]string{workerQueueKey, bucketPrefix, processingKey},
		"cscan:task:info:",
		"cscan:task:execution:",
		workerName,
		86400, // 24h
		time.Now().Format(time.RFC3339),
	).Result()

	if err == redis.Nil || result == nil {
		return &pb.CheckTaskResp{IsExist: false}, nil
	}
	if err != nil {
		return nil, err
	}

	taskData, ok := result.(string)
	if !ok {
		l.Logger.Errorf("CheckTask: unexpected script result type: %T", result)
		return nil, nil
	}

	// 修复 C6：死信哨兵在 Lua 外 PUBLISH,避免阻塞 pop 事务
	if strings.HasPrefix(taskData, "__DL__") {
		l.publishDeadLetterAlert(strings.TrimPrefix(taskData, "__DL__"))
		return nil, nil
	}

	var task scheduler.TaskInfo
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		l.Logger.Errorf("CheckTask: failed to parse task: %v", err)
		return nil, nil
	}

	l.Logger.Infof("CheckTask: atomically assigned task %s to worker %s from bucket path", task.TaskId, workerName)

	// 立即更新主任务状态为 STARTED
	l.updateMainTaskToStarted(task.MainTaskId, task.WorkspaceId)

	return &pb.CheckTaskResp{
		IsExist:     true,
		IsFinished:  false,
		TaskId:      task.TaskId,
		MainTaskId:  task.MainTaskId,
		WorkspaceId: task.WorkspaceId,
		Config:      task.Config,
	}, nil
}

// updateMainTaskToStarted 更新主任务状态为 STARTED
func (l *CheckTaskLogic) updateMainTaskToStarted(mainTaskId, workspaceId string) {
	if mainTaskId == "" || workspaceId == "" {
		l.Logger.Errorf("CheckTask: updateMainTaskToStarted called with empty params: mainTaskId='%s', workspaceId='%s'", mainTaskId, workspaceId)
		return
	}

	l.Logger.Infof("CheckTask: updating main task status to STARTED, mainTaskId=%s, workspaceId=%s", mainTaskId, workspaceId)

	// MongoDB 操作使用 5s 超时保护，避免无 deadline 时挂起（局部 ctx，不回写 l.ctx）
	mongoCtx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	taskModel := l.svcCtx.GetMainTaskModel(workspaceId)
	task, err := taskModel.FindById(mongoCtx, mainTaskId)
	if err != nil {
		l.Logger.Errorf("CheckTask: failed to find main task %s in workspace %s: %v", mainTaskId, workspaceId, err)
		return
	}
	if task == nil {
		l.Logger.Errorf("CheckTask: main task %s not found in workspace %s", mainTaskId, workspaceId)
		return
	}

	l.Logger.Infof("CheckTask: found main task, id=%s, taskId=%s, current status='%s'", task.Id.Hex(), task.TaskId, task.Status)

	// PENDING、CREATED 或空状态都更新为 STARTED
	if task.Status == "PENDING" || task.Status == "CREATED" || task.Status == "" {
		now := time.Now()
		update := bson.M{
			"status":     "STARTED",
			"start_time": now,
		}
		if err := taskModel.Update(mongoCtx, mainTaskId, update); err != nil {
			l.Logger.Errorf("CheckTask: failed to update main task status: %v", err)
		} else {
			l.Logger.Infof("CheckTask: main task %s status updated from '%s' to STARTED successfully", mainTaskId, task.Status)
		}
	} else {
		l.Logger.Infof("CheckTask: main task %s status is '%s', not updating to STARTED", mainTaskId, task.Status)
	}
}
