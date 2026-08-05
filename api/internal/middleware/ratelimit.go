package middleware

import (
	"net/http"
	"sync"
	"time"
)

// tokenBucket 固定窗口限流桶：在 window 时间窗内最多允许 limit 次请求，
// 超过即拒绝（返回 429）。进程内实现，按 tokenId 维度隔离（key 缺失时回退 userId/IP）。
type tokenBucket struct {
	limit    int
	window   time.Duration
	mu       sync.Mutex
	count    int
	windowStart time.Time
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.Sub(b.windowStart) >= b.window {
		// 窗口已滚动，重置计数
		b.windowStart = now
		b.count = 0
	}
	if b.count >= b.limit {
		return false
	}
	b.count++
	return true
}

// TokenRateLimiter 按 key 维度维护固定窗口限流桶。
type TokenRateLimiter struct {
	limit   int
	window  time.Duration
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	stopCh  chan struct{}
}

// NewTokenRateLimiter 创建限流器。limit 为窗口内最大请求数，window 为窗口时长。
// 修复 M-22：启动后台 goroutine 定期清理过期 bucket，避免 map 永久增长。
func NewTokenRateLimiter(limit int, window time.Duration) *TokenRateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	l := &TokenRateLimiter{
		limit:     limit,
		window:    window,
		buckets:   make(map[string]*tokenBucket),
		stopCh:    make(chan struct{}),
	}
	go l.evictLoop()
	return l
}

// Stop 停止后台清理 goroutine
func (l *TokenRateLimiter) Stop() {
	close(l.stopCh)
}

// evictLoop 定期清理已超过多个窗口未使用的 bucket，防止 map 无限增长
func (l *TokenRateLimiter) evictLoop() {
	// 清理周期为窗口时长的 2 倍
	ticker := time.NewTicker(l.window * 2)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.evict()
		}
	}
}

func (l *TokenRateLimiter) evict() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	// 超过 2 倍窗口时长未活动的 bucket 可安全删除
	expireBefore := now.Add(-2 * l.window)
	for k, b := range l.buckets {
		b.mu.Lock()
		expired := b.windowStart.Before(expireBefore)
		b.mu.Unlock()
		if expired {
			delete(l.buckets, k)
		}
	}
}

func (l *TokenRateLimiter) bucket(key string) *tokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{limit: l.limit, window: l.window, windowStart: time.Now()}
		l.buckets[key] = b
	}
	return b
}

// Handle 返回限流中间件：在 next 之前校验配额，超限返回 HTTP 429。
// key 优先级：tokenId > userId > 客户端 IP（与 PAT 认证链对应）。
func (l *TokenRateLimiter) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := GetTokenId(r.Context())
		if key == "" {
			key = GetUserId(r.Context())
		}
		if key == "" {
			key = clientIP(r)
		}
		if key == "" {
			// 无法识别调用方，直接放行（不应发生，auth 已拦截匿名请求）
			next(w, r)
			return
		}
		if !l.bucket(key).allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"code":429,"msg":"请求过于频繁，请稍后重试"}`))
			return
		}
		next(w, r)
	}
}

