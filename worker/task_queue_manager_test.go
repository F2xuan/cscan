package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"cscan/scheduler"

	"github.com/stretchr/testify/assert"
)

// TestTaskQueueManagerEnqueueDequeueOrder 验证 TaskQueueManager 按优先级出队
func TestTaskQueueManagerEnqueueDequeueOrder(t *testing.T) {
	m := NewTaskQueueManager(10, 5*time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// 入队顺序：Low -> Normal -> High -> Urgent
	m.Enqueue(&scheduler.TaskInfo{TaskId: "low"}, PriorityLow)
	m.Enqueue(&scheduler.TaskInfo{TaskId: "normal"}, PriorityNormal)
	m.Enqueue(&scheduler.TaskInfo{TaskId: "high"}, PriorityHigh)
	m.Enqueue(&scheduler.TaskInfo{TaskId: "urgent"}, PriorityUrgent)

	// 出队顺序应为：Urgent -> High -> Normal -> Low
	expected := []string{"urgent", "high", "normal", "low"}
	for _, want := range expected {
		task, _ := m.Dequeue()
		if task == nil {
			t.Fatalf("Dequeue 返回 nil，期望 %s", want)
		}
		if task.TaskId != want {
			t.Errorf("Dequeue 返回 %s，期望 %s", task.TaskId, want)
		}
	}

	// 队列应空
	if task, _ := m.Dequeue(); task != nil {
		t.Errorf("空队列 Dequeue 应返回 nil，实际 %+v", task)
	}
}

// TestTaskQueueManagerFullAndDrop 验证队列满时丢弃低优先级任务
func TestTaskQueueManagerFullAndDrop(t *testing.T) {
	m := NewTaskQueueManager(3, 5*time.Minute)
	m.SetLogger(func(level, format string, args ...interface{}) {
		// 静默
	})

	// 填满队列（3 个 Low 任务）
	m.Enqueue(&scheduler.TaskInfo{TaskId: "low1"}, PriorityLow)
	m.Enqueue(&scheduler.TaskInfo{TaskId: "low2"}, PriorityLow)
	m.Enqueue(&scheduler.TaskInfo{TaskId: "low3"}, PriorityLow)

	assert.Equal(t, 3, m.Size())
	assert.True(t, m.IsFull())

	// 再入队一个 Urgent，应丢弃一个 Low
	ok := m.Enqueue(&scheduler.TaskInfo{TaskId: "urgent"}, PriorityUrgent)
	assert.True(t, ok, "入队 Urgent 应触发低优先级丢弃后成功")
	assert.Equal(t, 3, m.Size(), "队列大小仍为 3")

	// 第一个出队应为 urgent
	task, _ := m.Dequeue()
	assert.NotNil(t, task)
	assert.Equal(t, "urgent", task.TaskId)

	// 剩余两个应为 low1/low2（dropLowPriorityTaskLocked 丢弃队列末尾 low3）
	remaining := []string{}
	for i := 0; i < 2; i++ {
		if t, _ := m.Dequeue(); t != nil {
			remaining = append(remaining, t.TaskId)
		}
	}
	assert.ElementsMatch(t, []string{"low1", "low2"}, remaining)
}

// TestTaskQueueManagerExpire 验证过期任务被清理
func TestTaskQueueManagerExpire(t *testing.T) {
	m := NewTaskQueueManager(10, 50*time.Millisecond)
	m.SetLogger(func(level, format string, args ...interface{}) {})

	m.Enqueue(&scheduler.TaskInfo{TaskId: "expire-me"}, PriorityNormal)
	assert.Equal(t, 1, m.Size())

	// 等待超过 maxWaitTime
	time.Sleep(100 * time.Millisecond)

	// 手动触发清理
	m.cleanupExpiredTasks()

	assert.Equal(t, 0, m.Size(), "过期任务应被清理")
	assert.Equal(t, int64(1), m.GetStats().TotalExpired)
}

// TestTaskQueueManagerStats 验证统计信息正确
func TestTaskQueueManagerStats(t *testing.T) {
	m := NewTaskQueueManager(10, 5*time.Minute)

	m.Enqueue(&scheduler.TaskInfo{TaskId: "t1"}, PriorityNormal)
	m.Enqueue(&scheduler.TaskInfo{TaskId: "t2"}, PriorityUrgent)
	m.Dequeue()

	stats := m.GetStats()
	assert.Equal(t, int64(2), stats.TotalEnqueued)
	assert.Equal(t, int64(1), stats.TotalDequeued)
	assert.Equal(t, 1, stats.CurrentSize)
	assert.Equal(t, 1, stats.QueueSizes["normal"])
}

// TestWorkerSubmitTaskWithQueueEnabled 验证启用 taskQueue 时 SubmitTask 走优先级入队
func TestWorkerSubmitTaskWithQueueEnabled(t *testing.T) {
	w := &Worker{
		config: WorkerConfig{
			Name:                   "test",
			Concurrency:            5,
			EnableTaskQueueManager: true,
		},
		logger:    NewWorkerLoggerLocal("test"),
		taskChan:  make(chan *scheduler.TaskInfo, 5),
		stopChan:  make(chan struct{}),
		taskQueue: NewTaskQueueManager(10, 5*time.Minute),
	}

	// SubmitTask 应走 taskQueue 路径
	w.SubmitTask(&scheduler.TaskInfo{TaskId: "urgent-task", Config: `{"urgent":true}`})
	w.SubmitTask(&scheduler.TaskInfo{TaskId: "normal-task", Config: `{}`})

	// taskQueue 应有 2 个任务
	assert.Equal(t, 2, w.taskQueue.Size())

	// taskChan 应为空（未被填充）
	assert.Equal(t, 0, len(w.taskChan))

	// 出队顺序：urgent-task 优先
	task, _ := w.taskQueue.Dequeue()
	assert.NotNil(t, task)
	assert.Equal(t, "urgent-task", task.TaskId)
}

// TestWorkerSubmitTaskWithChannelFallback 验证未启用 taskQueue 时 SubmitTask 走 taskChan
func TestWorkerSubmitTaskWithChannelFallback(t *testing.T) {
	w := &Worker{
		config:   WorkerConfig{Name: "test", Concurrency: 5},
		logger:   NewWorkerLoggerLocal("test"),
		taskChan: make(chan *scheduler.TaskInfo, 5),
		stopChan: make(chan struct{}),
		// taskQueue 为 nil
	}

	w.SubmitTask(&scheduler.TaskInfo{TaskId: "channel-task"})

	assert.Equal(t, 1, len(w.taskChan))
	select {
	case task := <-w.taskChan:
		assert.Equal(t, "channel-task", task.TaskId)
	default:
		t.Fatal("taskChan 应有任务")
	}
}

// TestWorkerPullTaskSlotCheckWithQueue 验证启用 taskQueue 时 pendingCount 槽位检查
func TestWorkerPullTaskSlotCheckWithQueue(t *testing.T) {
	w := &Worker{
		config: WorkerConfig{
			Name:                   "test",
			Concurrency:            2,
			EnableTaskQueueManager: true,
		},
		logger:    NewWorkerLoggerLocal("test"),
		taskChan:  make(chan *scheduler.TaskInfo, 2),
		stopChan:  make(chan struct{}),
		taskQueue: NewTaskQueueManager(10, 5*time.Minute),
	}

	// 填满 taskQueue（达到 Concurrency 上限）
	w.taskQueue.Enqueue(&scheduler.TaskInfo{TaskId: "t1"}, PriorityNormal)
	w.taskQueue.Enqueue(&scheduler.TaskInfo{TaskId: "t2"}, PriorityNormal)

	// 此时 pendingCount=2，应达到上限
	pendingCount := len(w.taskChan) + w.taskQueue.Size()
	assert.Equal(t, 2, pendingCount)
	assert.True(t, pendingCount >= w.config.Concurrency)

	// 出队一个后应有余量
	w.taskQueue.Dequeue()
	pendingCount = len(w.taskChan) + w.taskQueue.Size()

	assert.Equal(t, 1, pendingCount)
	assert.False(t, pendingCount >= w.config.Concurrency)
}

// TestWorkerTaskSourceLabel 验证 taskSource 返回正确的来源标签
func TestWorkerTaskSourceLabel(t *testing.T) {
	w1 := &Worker{
		taskQueue: NewTaskQueueManager(10, 5*time.Minute),
	}
	assert.Equal(t, "taskQueue", w1.taskSource())

	w2 := &Worker{}
	assert.Equal(t, "taskChan", w2.taskSource())
}

// TestGetTaskPriority 验证任务优先级推断
func TestGetTaskPriority(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		expected TaskPriority
	}{
		{
			name:     "urgent 标记",
			config:   `{"urgent":true}`,
			expected: PriorityUrgent,
		},
		{
			name:     "priority 配置 urgent",
			config:   `{"priority":"urgent"}`,
			expected: PriorityUrgent,
		},
		{
			name:     "priority 配置 high",
			config:   `{"priority":"high"}`,
			expected: PriorityHigh,
		},
		{
			name:     "priority 配置 low",
			config:   `{"priority":"low"}`,
			expected: PriorityLow,
		},
		{
			name:     "POC 验证任务",
			config:   `{"taskType":"poc_validate"}`,
			expected: PriorityHigh,
		},
		{
			name:     "POC 批量验证任务",
			config:   `{"taskType":"poc_batch_validate"}`,
			expected: PriorityHigh,
		},
		{
			name:     "小批量目标（≤10）",
			config:   `{"target":"1.1.1.1"}`,
			expected: PriorityHigh,
		},
		{
			name:     "中批量目标（11-999）",
			config:   `{"target":"1.1.1.1\n2.2.2.2\n3.3.3.3\n4.4.4.4\n5.5.5.5\n6.6.6.6\n7.7.7.7\n8.8.8.8\n9.9.9.9\n10.10.10.10\n11.11.11.11"}`,
			expected: PriorityNormal,
		},
		// 注意：JSON 字符串字面量内的 \n 是两个字符（反斜杠+n），Go raw string 中保持原样
		// json.Unmarshal 会把 \n 转义序列解析为真正的换行符
		{
			name:     "大批量目标（≥1000）",
			config:   buildLargeTarget(1000),
			expected: PriorityLow,
		},
		{
			name:     "默认优先级",
			config:   `{}`,
			expected: PriorityNormal,
		},
		{
			name:     "无效 JSON",
			config:   `not-json`,
			expected: PriorityNormal,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := &scheduler.TaskInfo{Config: c.config}
			got := GetTaskPriority(task)
			if got != c.expected {
				t.Errorf("GetTaskPriority() = %d，期望 %d", got, c.expected)
			}
		})
	}
}

// buildLargeTarget 构造包含 n 行 IP 的 target 字段
// 注意：JSON 字符串内的换行符必须用 \n 转义（\\n 在 Go 字符串中表示 \n 字面量）
func buildLargeTarget(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = "10.0.0." + itoa(i%256)
	}
	// 用 strings.Join 构造，再用 \\n 转义
	target := strings.Join(parts, "\\n")
	return `{"target":"` + target + `"}`
}

// itoa 简易整数转字符串
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
