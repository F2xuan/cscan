package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen 熔断器开启状态错误
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrCircuitHalfOpen 熔断器半开状态错误
	ErrCircuitHalfOpen = errors.New("circuit breaker is half-open, limiting requests")
)

// State 熔断器状态
type State int

const (
	StateClosed   State = iota // 正常状态
	StateOpen                  // 熔断状态
	StateHalfOpen              // 半开状态
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config 熔断器配置
type Config struct {
	FailureThreshold    int           // 失败阈值
	SuccessThreshold    int           // 半开状态成功阈值
	Timeout             time.Duration // 熔断超时时间
	HalfOpenMaxRequests int           // 半开状态最大请求数
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config Config

	state           State
	failureCount    int
	successCount    int
	halfOpenReqs    int
	lastFailureTime time.Time

	mu sync.RWMutex

	// 回调
	OnStateChange func(from, to State)
}

// New 创建熔断器
func New(cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		config: cfg,
		state:  StateClosed,
	}
}

// NewWithName 创建带名称的熔断器
func NewWithName(name string, cfg Config) *CircuitBreaker {
	cb := New(cfg)
	return cb
}

// Execute 执行操作
func (cb *CircuitBreaker) Execute(fn func() error) (err error) {
	if err := cb.beforeExecute(); err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("circuit breaker execute panic: %v", r)
		}
		cb.afterExecute(err)
	}()

	err = fn()
	return err
}

// ExecuteWithFallback 执行操作（带降级）
func (cb *CircuitBreaker) ExecuteWithFallback(fn func() error, fallback func(error) error) error {
	err := cb.Execute(fn)
	if err != nil && fallback != nil {
		return fallback(err)
	}
	return err
}

func (cb *CircuitBreaker) beforeExecute() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateOpen:
		// 检查是否超时，可以转为半开
		if now.Sub(cb.lastFailureTime) > cb.config.Timeout {
			cb.transitionTo(StateHalfOpen)
			cb.halfOpenReqs = 1
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// 限制半开状态的请求数
		if cb.halfOpenReqs >= cb.config.HalfOpenMaxRequests {
			return ErrCircuitHalfOpen
		}
		cb.halfOpenReqs++
		return nil

	case StateClosed:
		return nil
	}

	return nil
}

func (cb *CircuitBreaker) afterExecute(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		cb.transitionTo(StateOpen)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++

	switch cb.state {
	case StateClosed:
		cb.failureCount = 0
	case StateHalfOpen:
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	}
}

func (cb *CircuitBreaker) transitionTo(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenReqs = 0

	if cb.OnStateChange != nil {
		go func() {
			defer func() {
				_ = recover()
			}()
			cb.OnStateChange(oldState, newState)
		}()
	}
}

// State 获取当前状态
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen 检查熔断器是否开启
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == StateOpen
}

// IsClosed 检查熔断器是否关闭
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.State() == StateClosed
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenReqs = 0
}

// Stats 获取统计信息
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":         cb.state.String(),
		"failure_count": cb.failureCount,
		"success_count": cb.successCount,
		"last_failure":  cb.lastFailureTime,
	}
}

// CircuitBreakerRegistry 熔断器注册表
type CircuitBreakerRegistry struct {
	breakers sync.Map
	config   Config
}

// NewRegistry 创建熔断器注册表
func NewRegistry(defaultConfig Config) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		config: defaultConfig,
	}
}

// Get 获取或创建熔断器
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	if cb, ok := r.breakers.Load(name); ok {
		return cb.(*CircuitBreaker)
	}

	cb := New(r.config)
	actual, _ := r.breakers.LoadOrStore(name, cb)
	return actual.(*CircuitBreaker)
}

// GetWithConfig 获取或创建带自定义配置的熔断器
func (r *CircuitBreakerRegistry) GetWithConfig(name string, cfg Config) *CircuitBreaker {
	if cb, ok := r.breakers.Load(name); ok {
		return cb.(*CircuitBreaker)
	}

	cb := New(cfg)
	actual, _ := r.breakers.LoadOrStore(name, cb)
	return actual.(*CircuitBreaker)
}

// Reset 重置指定熔断器
func (r *CircuitBreakerRegistry) Reset(name string) {
	if cb, ok := r.breakers.Load(name); ok {
		cb.(*CircuitBreaker).Reset()
	}
}

// ResetAll 重置所有熔断器
func (r *CircuitBreakerRegistry) ResetAll() {
	r.breakers.Range(func(key, value interface{}) bool {
		value.(*CircuitBreaker).Reset()
		return true
	})
}

// Stats 获取所有熔断器统计
func (r *CircuitBreakerRegistry) Stats() map[string]map[string]interface{} {
	stats := make(map[string]map[string]interface{})
	r.breakers.Range(func(key, value interface{}) bool {
		stats[key.(string)] = value.(*CircuitBreaker).Stats()
		return true
	})
	return stats
}
