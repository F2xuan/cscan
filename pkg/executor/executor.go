package executor

import (
	"context"
	"fmt"
	"time"
)

// TaskExecutor 统一的任务执行器
// - Context 完全穿透
// - 5 层超时管理（全局 > 阶段 > 单个任务 > 扫描器 > 网络请求）
type TaskExecutor struct {
	globalTimeout time.Duration
}

// NewTaskExecutor 创建任务执行器
func NewTaskExecutor(globalTimeout time.Duration) *TaskExecutor {
	return &TaskExecutor{globalTimeout: globalTimeout}
}

// ExecuteWithContext 执行任务，支持 Context 传播和完整的超时管理
//
// 超时优先级（从高到低）：
// 1. ctx 本身的 Deadline（全局约束，无法覆盖）
// 2. TaskConfig.Timeout（阶段级约束，e.g., 30min Nuclei）
// 3. 扫描器默认超时（scanner 级约束，e.g., 5min naabu）
// 4. 网络请求超时（http.Client 级约束，e.g., 30s httpx）
// 5. 内部重试超时（单次重试的 backoff）
//
// 使用示例：
//
//	executor.ExecuteWithContext(ctx, &TaskConfig{
//	    ID:      "task-123",
//	    Timeout: 30 * time.Minute,
//	    Name:    "Nuclei POC Verification",
//	}, func(taskCtx context.Context) error {
//	    // 在 taskCtx 下执行所有子操作
//	    return nucleiEngine.Run(taskCtx)
//	})
func (e *TaskExecutor) ExecuteWithContext(
	ctx context.Context,
	cfg *TaskConfig,
	fn func(context.Context) error,
) error {
	if cfg == nil {
		return fmt.Errorf("task config is nil")
	}

	// 确定有效的任务超时（取当前 ctx deadline 和 cfg.Timeout 的最小值）
	var taskDeadline time.Time
	if d, ok := ctx.Deadline(); ok {
		taskDeadline = d
	}
	if cfg.Timeout > 0 {
		cfgDeadline := time.Now().Add(cfg.Timeout)
		if taskDeadline.IsZero() || cfgDeadline.Before(taskDeadline) {
			taskDeadline = cfgDeadline
		}
	}

	// 为任务创建新的 Context，继承来自父 ctx 的约束
	var taskCtx context.Context
	var cancel context.CancelFunc
	if !taskDeadline.IsZero() {
		taskCtx, cancel = context.WithDeadline(ctx, taskDeadline)
	} else if e.globalTimeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, e.globalTimeout)
	} else {
		taskCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// 执行任务
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("task panicked: %v", r)
			}
		}()
		done <- fn(taskCtx)
	}()

	// 等待任务完成或超时
	select {
	case err := <-done:
		return err
	case <-taskCtx.Done():
		// taskCtx 已超时
		if taskCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("task %s exceeded timeout (%v)", cfg.ID, cfg.Timeout)
		}
		return taskCtx.Err()
	}
}

// ExecuteWithTimeout 执行任务，带超时（简化版）
func (e *TaskExecutor) ExecuteWithTimeout(
	ctx context.Context,
	taskID string,
	timeout time.Duration,
	fn func(context.Context) error,
) error {
	return e.ExecuteWithContext(ctx, &TaskConfig{
		ID:      taskID,
		Timeout: timeout,
		Name:    taskID,
	}, fn)
}

// TaskConfig 任务配置
type TaskConfig struct {
	ID      string        `json:"id"`
	Timeout time.Duration `json:"timeout"`
	Name    string        `json:"name"`
	// 可扩展其他配置字段
}

// TaskResult 任务执行结果
type TaskResult struct {
	TaskID   string        `json:"taskId"`
	Success  bool          `json:"success"`
	Error    error         `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	Output   []byte        `json:"output,omitempty"`
}
