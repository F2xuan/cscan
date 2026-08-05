package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewTaskExecutor(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)
	if executor == nil {
		t.Fatal("NewTaskExecutor returned nil")
	}
	if executor.globalTimeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", executor.globalTimeout)
	}
}

func TestExecuteWithContext_Success(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)

	err := executor.ExecuteWithContext(
		context.Background(),
		&TaskConfig{
			ID:      "test-001",
			Timeout: 10 * time.Second,
			Name:    "test task",
		},
		func(ctx context.Context) error {
			return nil
		},
	)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteWithContext_Error(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)
	expectedErr := errors.New("test error")

	err := executor.ExecuteWithContext(
		context.Background(),
		&TaskConfig{
			ID:      "test-002",
			Timeout: 10 * time.Second,
			Name:    "test task",
		},
		func(ctx context.Context) error {
			return expectedErr
		},
	)

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestExecuteWithContext_Timeout(t *testing.T) {
	executor := NewTaskExecutor(1 * time.Second)

	err := executor.ExecuteWithContext(
		context.Background(),
		&TaskConfig{
			ID:      "test-003",
			Timeout: 100 * time.Millisecond,
			Name:    "test task",
		},
		func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestExecuteWithContext_NilConfig(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)

	err := executor.ExecuteWithContext(
		context.Background(),
		nil,
		func(ctx context.Context) error {
			return nil
		},
	)

	if err == nil {
		t.Error("expected error for nil config, got nil")
	}
}

func TestExecuteWithContext_Panic(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)

	err := executor.ExecuteWithContext(
		context.Background(),
		&TaskConfig{
			ID:      "test-004",
			Timeout: 10 * time.Second,
			Name:    "test task",
		},
		func(ctx context.Context) error {
			panic("test panic")
		},
	)

	if err == nil {
		t.Error("expected panic error, got nil")
	}
}

func TestExecuteWithTimeout_Success(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)

	err := executor.ExecuteWithTimeout(
		context.Background(),
		"test-005",
		10*time.Second,
		func(ctx context.Context) error {
			return nil
		},
	)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteWithContext_ContextCancellation(t *testing.T) {
	executor := NewTaskExecutor(5 * time.Minute)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := executor.ExecuteWithContext(
		ctx,
		&TaskConfig{
			ID:      "test-006",
			Timeout: 10 * time.Second,
			Name:    "test task",
		},
		func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	)

	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}
