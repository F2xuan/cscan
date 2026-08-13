package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestScheduler 创建基于 miniredis 的测试调度器
func newTestScheduler(t *testing.T, enableBucket bool) (*Scheduler, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis 启动失败: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := NewScheduler(rdb)
	s.SetEnablePriorityBucket(enableBucket)
	return s, mr
}

// TestPriorityBucketPushPop 验证分桶 Push 和 Pop 的基本正确性
func TestPriorityBucketPushPop(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	// 推送 3 个不同优先级的任务
	tasks := []*TaskInfo{
		{TaskId: "low", Priority: PriorityLow, Config: "{}"},
		{TaskId: "urgent", Priority: PriorityUrgent, Config: "{}"},
		{TaskId: "normal", Priority: PriorityNormal, Config: "{}"},
	}
	for _, task := range tasks {
		if err := s.PushTask(ctx, task); err != nil {
			t.Fatalf("PushTask %s 失败: %v", task.TaskId, err)
		}
	}

	// 验证分桶 Key 存在
	for _, p := range []int{PriorityUrgent, PriorityNormal, PriorityLow} {
		key := priorityBucketKey(p)
		if !mr.Exists(key) {
			t.Errorf("分桶 Key %s 不存在", key)
		}
	}

	// 按优先级顺序弹出：urgent -> normal -> low
	expected := []string{"urgent", "normal", "low"}
	for _, want := range expected {
		task, err := s.PopTask(ctx)
		if err != nil {
			t.Fatalf("PopTask 失败: %v", err)
		}
		if task == nil {
			t.Fatalf("PopTask 返回 nil，期望 %s", want)
		}
		if task.TaskId != want {
			t.Errorf("PopTask 返回 %s，期望 %s", task.TaskId, want)
		}
	}

	// 队列应为空
	task, err := s.PopTask(ctx)
	if err != nil {
		t.Fatalf("PopTask 失败: %v", err)
	}
	if task != nil {
		t.Errorf("PopTask 应返回 nil，实际返回 %+v", task)
	}
}

// TestPriorityBucketOrderStrict 验证优先级严格顺序：P4(Urgent) 先于 P3(High)，P3 先于 P2 ...
func TestPriorityBucketOrderStrict(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	// 同时推送 5 个优先级的任务（同时间戳）
	now := time.Now()
	priorities := []int{PriorityBackground, PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent}
	for _, p := range priorities {
		task := &TaskInfo{
			TaskId:   fmt.Sprintf("p%d", p),
			Priority: p,
			Config:   "{}",
		}
		// 直接用 pushToPriorityBucket 避免 PushTask 覆盖时间戳
		score := s.calculatePriorityScore(p, now)
		if err := s.pushToPriorityBucket(ctx, task, score); err != nil {
			t.Fatalf("pushToPriorityBucket 失败: %v", err)
		}
	}

	// 验证弹出顺序：P4(Urgent) -> P0(Background)，分数越小越先弹出
	expectedOrder := []string{"p4", "p3", "p2", "p1", "p0"}
	for i, want := range expectedOrder {
		task, err := s.PopTask(ctx)
		if err != nil {
			t.Fatalf("第 %d 次 PopTask 失败: %v", i, err)
		}
		if task == nil {
			t.Fatalf("第 %d 次 PopTask 返回 nil", i)
		}
		if task.TaskId != want {
			t.Errorf("第 %d 次 PopTask 返回 %s，期望 %s", i, task.TaskId, want)
		}
	}
}

// TestPriorityBucketEmptyAllBuckets 验证所有分桶为空时 PopTask 返回 nil
func TestPriorityBucketEmptyAllBuckets(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()

	task, err := s.PopTask(context.Background())
	if err != nil {
		t.Fatalf("PopTask 失败: %v", err)
	}
	if task != nil {
		t.Errorf("空队列 PopTask 应返回 nil，实际 %+v", task)
	}
}

// TestPriorityBucketClampPriority 验证 priority 越界时被 clamp 到合法范围
func TestPriorityBucketClampPriority(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	// priority=100 应被 clamp 到 P4（Urgent，最先弹出）
	// priority=-5 应被 clamp 到 P0（Background，最后弹出）
	tasks := []*TaskInfo{
		{TaskId: "too-high", Priority: 100, Config: "{}"}, // -> P4 Urgent
		{TaskId: "too-low", Priority: -5, Config: "{}"},  // -> P0 Background
	}
	for _, task := range tasks {
		if err := s.PushTask(ctx, task); err != nil {
			t.Fatalf("PushTask 失败: %v", err)
		}
	}

	// P4（Urgent）应先弹出 -> too-high
	task, err := s.PopTask(ctx)
	if err != nil {
		t.Fatalf("第一次 PopTask 失败: %v", err)
	}
	if task == nil || task.TaskId != "too-high" {
		t.Errorf("期望 too-high，实际 %+v", task)
	}

	// P0（Background）后弹出 -> too-low
	task, err = s.PopTask(ctx)
	if err != nil {
		t.Fatalf("第二次 PopTask 失败: %v", err)
	}
	if task == nil || task.TaskId != "too-low" {
		t.Errorf("期望 too-low，实际 %+v", task)
	}
}

// TestPopForWorkerFromBuckets 验证 Worker 弹出：先专属队列后分桶
func TestPopForWorkerFromBuckets(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	// 1. 推送到公共分桶（urgent 和 normal）
	s.PushTask(ctx, &TaskInfo{TaskId: "normal-task", Priority: PriorityNormal, Config: "{}"})
	s.PushTask(ctx, &TaskInfo{TaskId: "urgent-task", Priority: PriorityUrgent, Config: "{}"})

	// 2. 推送到 worker1 的专属队列（优先级低于 urgent，但应先被 worker1 取走）
	workerTask := &TaskInfo{TaskId: "worker-task", Priority: PriorityBackground, Workers: []string{"worker1"}}
	s.PushTask(ctx, workerTask)

	// 3. worker1 弹出：应先取到专属队列的 worker-task
	task, err := s.PopTaskForWorker(ctx, "worker1")
	if err != nil {
		t.Fatalf("PopTaskForWorker 失败: %v", err)
	}
	if task == nil {
		t.Fatal("期望 worker-task，实际 nil")
	}
	if task.TaskId != "worker-task" {
		t.Errorf("期望 worker-task，实际 %s", task.TaskId)
	}

	// 4. worker1 再次弹出：应取到公共分桶的 urgent-task（P4 Urgent 优先于 P2 Normal）
	task, err = s.PopTaskForWorker(ctx, "worker1")
	if err != nil {
		t.Fatalf("第二次 PopTaskForWorker 失败: %v", err)
	}
	if task == nil {
		t.Fatal("期望 urgent-task，实际 nil")
	}
	if task.TaskId != "urgent-task" {
		t.Errorf("期望 urgent-task，实际 %s", task.TaskId)
	}

	// 5. worker1 第三次弹出：应取到 normal-task
	task, err = s.PopTaskForWorker(ctx, "worker1")
	if err != nil {
		t.Fatalf("第三次 PopTaskForWorker 失败: %v", err)
	}
	if task == nil {
		t.Fatal("期望 normal-task，实际 nil")
	}
	if task.TaskId != "normal-task" {
		t.Errorf("期望 normal-task，实际 %s", task.TaskId)
	}
}

// TestPopForWorkerFromBucketsEmpty 验证空队列时 Worker 弹出返回 nil
func TestPopForWorkerFromBucketsEmpty(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()

	task, err := s.PopTaskForWorker(context.Background(), "worker1")
	if err != nil {
		t.Fatalf("PopTaskForWorker 失败: %v", err)
	}
	if task != nil {
		t.Errorf("空队列应返回 nil，实际 %+v", task)
	}
}

// TestPopForWorkerFromBucketsAddToProcessing 验证弹出后任务被加入 processing 集合
func TestPopForWorkerFromBucketsAddToProcessing(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	s.PushTask(ctx, &TaskInfo{TaskId: "task-123", Priority: PriorityNormal, Config: "{}"})

	task, err := s.PopTaskForWorker(ctx, "worker1")
	if err != nil {
		t.Fatalf("PopTaskForWorker 失败: %v", err)
	}
	if task == nil {
		t.Fatal("期望 task-123，实际 nil")
	}

	// 验证 processing 集合包含 task-123
	members, err := s.rdb.SMembers(ctx, s.processingKey).Result()
	if err != nil {
		t.Fatalf("SMembers 失败: %v", err)
	}
	found := false
	for _, m := range members {
		if m == "task-123" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("processing 集合应包含 task-123，实际 %v", members)
	}
}

// TestBackwardCompatDisableBucket 验证关闭分桶时保持原单 ZSet 行为
func TestBackwardCompatDisableBucket(t *testing.T) {
	s, mr := newTestScheduler(t, false) // 关闭分桶
	defer mr.Close()
	ctx := context.Background()

	// 推送 3 个不同优先级的任务
	tasks := []*TaskInfo{
		{TaskId: "low", Priority: PriorityLow, Config: "{}"},
		{TaskId: "urgent", Priority: PriorityUrgent, Config: "{}"},
		{TaskId: "normal", Priority: PriorityNormal, Config: "{}"},
	}
	for _, task := range tasks {
		if err := s.PushTask(ctx, task); err != nil {
			t.Fatalf("PushTask %s 失败: %v", task.TaskId, err)
		}
	}

	// 验证任务在 cscan:task:queue（单 ZSet）中，不在分桶 Key 中
	if !mr.Exists(s.queueKey) {
		t.Errorf("单 ZSet Key %s 应存在", s.queueKey)
	}
	for _, p := range []int{PriorityUrgent, PriorityNormal, PriorityLow} {
		bucketKey := priorityBucketKey(p)
		if mr.Exists(bucketKey) {
			t.Errorf("分桶 Key %s 不应存在（分桶已关闭）", bucketKey)
		}
	}

	// 按优先级弹出（单 ZSet 内 score 决定顺序）
	expected := []string{"urgent", "normal", "low"}
	for _, want := range expected {
		task, err := s.PopTask(ctx)
		if err != nil {
			t.Fatalf("PopTask 失败: %v", err)
		}
		if task == nil {
			t.Fatalf("PopTask 返回 nil，期望 %s", want)
		}
		if task.TaskId != want {
			t.Errorf("PopTask 返回 %s，期望 %s", task.TaskId, want)
		}
	}
}

// TestPushTaskBatchWithBucket 验证批量推送走分桶路径
func TestPushTaskBatchWithBucket(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	tasks := []*TaskInfo{
		{TaskId: "t1", Priority: PriorityUrgent, Config: "{}"},
		{TaskId: "t2", Priority: PriorityNormal, Config: "{}"},
		{TaskId: "t3", Priority: PriorityUrgent, Config: "{}"},
		{TaskId: "t4", Priority: PriorityBackground, Config: "{}"},
	}
	if err := s.PushTaskBatch(ctx, tasks); err != nil {
		t.Fatalf("PushTaskBatch 失败: %v", err)
	}

	// 验证分桶 Key 存在
	for _, p := range []int{PriorityUrgent, PriorityNormal, PriorityBackground} {
		if !mr.Exists(priorityBucketKey(p)) {
			t.Errorf("分桶 Key %s 应存在", priorityBucketKey(p))
		}
	}

	// 弹出顺序：P4 Urgent（t1, t3）-> P2 Normal（t2）-> P0 Background（t4）
	// t1 和 t3 同优先级，按入队顺序弹出
	expected := []string{"t1", "t3", "t2", "t4"}
	for i, want := range expected {
		task, err := s.PopTask(ctx)
		if err != nil {
			t.Fatalf("第 %d 次 PopTask 失败: %v", i, err)
		}
		if task == nil {
			t.Fatalf("第 %d 次 PopTask 返回 nil", i)
		}
		if task.TaskId != want {
			t.Errorf("第 %d 次 PopTask 返回 %s，期望 %s", i, task.TaskId, want)
		}
	}
}

// TestBucketPriorityFromWorker 验证 worker.TaskPriority 到 scheduler 分桶优先级的映射
func TestBucketPriorityFromWorker(t *testing.T) {
	cases := []struct {
		workerPriority int
		expected       int
	}{
		{4, PriorityUrgent},    // worker.Urgent -> scheduler.P0
		{3, PriorityHigh},       // worker.High -> scheduler.P1
		{2, PriorityNormal},     // worker.Normal -> scheduler.P2
		{1, PriorityLow},        // worker.Low -> scheduler.P3
		{0, PriorityBackground}, // 未设置 -> scheduler.P4
		{-1, PriorityBackground}, // 负值 -> scheduler.P4
		{99, PriorityNormal},    // 未知正值 -> 默认 Normal
	}
	for _, c := range cases {
		got := BucketPriorityFromWorker(c.workerPriority)
		if got != c.expected {
			t.Errorf("BucketPriorityFromWorker(%d) = %d，期望 %d", c.workerPriority, got, c.expected)
		}
	}
}

// TestCalculatePriorityScoreMicrosecondPrecision 验证微秒级时间戳解决同秒顺序问题
func TestCalculatePriorityScoreMicrosecondPrecision(t *testing.T) {
	s, _ := newTestScheduler(t, false)

	// 同一秒内两个任务，时间戳微秒级不同
	t1 := time.Date(2026, 7, 18, 15, 30, 45, 1000, time.UTC)   // 1 微秒
	t2 := time.Date(2026, 7, 18, 15, 30, 45, 2000, time.UTC)   // 2 微秒

	score1 := s.calculatePriorityScore(PriorityNormal, t1)
	score2 := s.calculatePriorityScore(PriorityNormal, t2)

	// t1 应小于 t2（分数越小优先级越高，但 t1 时间早所以分数小）
	if score1 >= score2 {
		t.Errorf("同秒任务 t1（早）分数 %.0f 应小于 t2（晚）分数 %.0f", score1, score2)
	}

	// 验证微秒级精度：差值应远小于 1 秒但大于 0
	diff := score2 - score1
	if diff <= 0 || diff > 1_000_000 {
		t.Errorf("同秒任务分数差 %.0f 应在 (0, 1000000] 范围内", diff)
	}
}

// TestPriorityBucketTaskInfoIntegrity 验证分桶路径下任务的完整性（JSON 反序列化）
func TestPriorityBucketTaskInfoIntegrity(t *testing.T) {
	s, mr := newTestScheduler(t, true)
	defer mr.Close()
	ctx := context.Background()

	original := &TaskInfo{
		TaskId:      "integrity-test",
		MainTaskId:  "main-001",
		TaskName:    "scan-task",
		Config:      `{"portscan":{"enable":true,"ports":"80,443"}}`,
		Priority:    PriorityHigh,
		Workers:     nil,
	}

	if err := s.PushTask(ctx, original); err != nil {
		t.Fatalf("PushTask 失败: %v", err)
	}

	task, err := s.PopTask(ctx)
	if err != nil {
		t.Fatalf("PopTask 失败: %v", err)
	}
	if task == nil {
		t.Fatal("PopTask 返回 nil")
	}

	// 验证字段完整性
	if task.TaskId != original.TaskId {
		t.Errorf("TaskId = %s，期望 %s", task.TaskId, original.TaskId)
	}
	if task.MainTaskId != original.MainTaskId {
		t.Errorf("MainTaskId = %s，期望 %s", task.MainTaskId, original.MainTaskId)
	}
	if task.Config != original.Config {
		t.Errorf("Config = %s，期望 %s", task.Config, original.Config)
	}
	if task.Priority != original.Priority {
		t.Errorf("Priority = %d，期望 %d", task.Priority, original.Priority)
	}

	// 验证 Config 可被正常 JSON 解析
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(task.Config), &cfg); err != nil {
		t.Errorf("Config JSON 解析失败: %v", err)
	}
}
