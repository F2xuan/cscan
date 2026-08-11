package worker

import (
	"testing"
	"time"

	"cscan/internal/scheduler"

	"github.com/stretchr/testify/assert"
)

func newTestWorker(concurrency int) *Worker {
	return &Worker{
		config:        WorkerConfig{Name: "test-worker", Concurrency: concurrency},
		logger:        NewWorkerLoggerLocal("test-worker"),
		taskChan:      make(chan *scheduler.TaskInfo, concurrency),
		stopChan:      make(chan struct{}),
		executorCount: concurrency,
	}
}

func TestApplyConcurrencyIncreaseSpawnsExecutorsWhenRunning(t *testing.T) {
	w := newTestWorker(3)
	w.isRunning = true

	w.applyConcurrency(8)

	assert.Equal(t, 8, w.config.Concurrency)
	assert.Equal(t, 8, w.executorCount)

	// 关闭 stopChan 应让补启的协程全部退出
	close(w.stopChan)
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("spawned executors did not exit after stopChan closed")
	}
}

func TestApplyConcurrencyNoopWhenUnchanged(t *testing.T) {
	w := newTestWorker(5)

	w.applyConcurrency(5)

	assert.Equal(t, 5, w.config.Concurrency)
	assert.Equal(t, 5, w.executorCount)
}

func TestApplyConcurrencyRejectsInvalidValues(t *testing.T) {
	w := newTestWorker(5)

	w.applyConcurrency(0)
	assert.Equal(t, 5, w.config.Concurrency)

	w.applyConcurrency(-3)
	assert.Equal(t, 5, w.config.Concurrency)
}
