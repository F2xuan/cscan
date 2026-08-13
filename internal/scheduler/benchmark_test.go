package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// newBenchScheduler 连接本地 Redis 构造调度器；Redis 不可用则跳过。
func newBenchScheduler(b *testing.B) (*Scheduler, context.Context) {
	b.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	b.Cleanup(func() { _ = rdb.Close() })

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		b.Skip("Redis 不可用，跳过基准测试")
	}
	return NewScheduler(rdb), ctx
}

// benchTasks 构造指定数量的基准任务
func benchTasks(prefix string, n int) []*TaskInfo {
	tasks := make([]*TaskInfo, n)
	for i := 0; i < n; i++ {
		tasks[i] = &TaskInfo{
			TaskId:      fmt.Sprintf("%s-%d", prefix, i),
			Priority:    i % 5,
		}
	}
	return tasks
}

// BenchmarkPushTaskBatch 基准测试：批量推送任务到队列
func BenchmarkPushTaskBatch(b *testing.B) {
	s, ctx := newBenchScheduler(b)

	for _, size := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("BatchSize_%d", size), func(b *testing.B) {
			tasks := benchTasks("bench-push", size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := s.PushTaskBatch(ctx, tasks); err != nil {
					b.Fatalf("PushTaskBatch failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkPopTask 基准测试：从队列弹出任务（队列耗尽后测量空弹开销）
func BenchmarkPopTask(b *testing.B) {
	s, ctx := newBenchScheduler(b)

	if err := s.PushTaskBatch(ctx, benchTasks("bench-pop", 1000)); err != nil {
		b.Fatalf("prefill failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.PopTask(ctx); err != nil {
			b.Fatalf("PopTask failed: %v", err)
		}
	}
}

// BenchmarkCalculatePriorityScore 基准测试：优先级分数计算
func BenchmarkCalculatePriorityScore(b *testing.B) {
	s := NewScheduler(nil)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.calculatePriorityScore(i%5, now)
	}
}

// BenchmarkWorkerLoadScore 基准测试：Worker 负载评分
func BenchmarkWorkerLoadScore(b *testing.B) {
	for _, count := range []int{5, 10, 20, 50} {
		workers := make([]WorkerLoad, count)
		for i := 0; i < count; i++ {
			workers[i] = WorkerLoad{
				WorkerName:     fmt.Sprintf("worker-%d", i),
				CurrentTasks:   i % 10,
				MaxConcurrency: 20,
				CPUPercent:     float64(30 + i%50),
				MemPercent:     float64(40 + i%40),
				LastHeartbeat:  time.Now(),
			}
		}

		b.Run(fmt.Sprintf("Workers_%d", count), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				best := -1.0
				for j := range workers {
					if score := workers[j].LoadScore(); best < 0 || score < best {
						best = score
					}
				}
				_ = best
			}
		})
	}
}

// BenchmarkSplitTask 基准测试：任务分片（含 CIDR 展开与分片优先级计算）
func BenchmarkSplitTask(b *testing.B) {
	splitter := NewTaskSplitter(DefaultChunkConfig())

	for _, cidr := range []string{"10.0.0.0/24", "10.0.0.0/22", "10.0.0.0/20"} {
		b.Run(cidr, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := splitter.SplitTask("bench-task", cidr, nil, PriorityNormal); err != nil {
					b.Fatalf("SplitTask failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSplitTargets 基准测试：目标批量拆分
func BenchmarkSplitTargets(b *testing.B) {
	splitter := NewTargetSplitter(30)

	for _, cidr := range []string{"10.0.0.0/24", "10.0.0.0/22"} {
		b.Run(cidr, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = splitter.SplitTargets(cidr)
			}
		})
	}
}
