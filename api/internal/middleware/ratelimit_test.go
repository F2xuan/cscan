package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenRateLimiter(t *testing.T) {
	limit := 2
	limiter := NewTokenRateLimiter(limit, time.Second)
	next := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	h := limiter.Handle(next)

	key := "tok-openapi-1"
	allowed := func() int {
		r := httptest.NewRequest(http.MethodGet, "/x", nil).
			WithContext(context.WithValue(context.Background(), TokenIdKey, key))
		w := httptest.NewRecorder()
		h(w, r)
		return w.Code
	}

	// 前 limit 次应通过
	for i := 0; i < limit; i++ {
		if code := allowed(); code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, code)
		}
	}
	// 第 limit+1 次应被限流
	if code := allowed(); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", code)
	}

	// 不同 key 不受影响
	other := httptest.NewRequest(http.MethodGet, "/x", nil).
		WithContext(context.WithValue(context.Background(), TokenIdKey, "tok-openapi-2"))
	w := httptest.NewRecorder()
	h(w, other)
	if w.Code != http.StatusOK {
		t.Fatalf("different key: expected 200, got %d", w.Code)
	}
}

func TestTokenRateLimiterFallback(t *testing.T) {
	limiter := NewTokenRateLimiter(1, time.Second)
	next := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	h := limiter.Handle(next)

	// 无 tokenId 时回退 userId
	r1 := httptest.NewRequest(http.MethodGet, "/x", nil).
		WithContext(context.WithValue(context.Background(), UserIdKey, "user-9"))
	w1 := httptest.NewRecorder()
	h(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("user-key first: expected 200, got %d", w1.Code)
	}
	r2 := httptest.NewRequest(http.MethodGet, "/x", nil).
		WithContext(context.WithValue(context.Background(), UserIdKey, "user-9"))
	w2 := httptest.NewRecorder()
	h(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("user-key second: expected 429, got %d", w2.Code)
	}
}
