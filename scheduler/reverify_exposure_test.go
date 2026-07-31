package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestProbeExposure 验证敏感信息复验的探测分类逻辑（纯函数，无需 DB）：
// 验收标准 3（不可达不误判为已修复）→ exposurePending；
// 验收标准 4（软 404 内容特征兜底）→ 200 但原泄露内容已消失 → exposureResolved。
func TestProbeExposure(t *testing.T) {
	const secret = "AKIA-SECRET-12345"
	mux := http.NewServeMux()
	mux.HandleFunc("/leak", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("config: " + secret))
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/forbidden", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx := context.Background()
	timeout := 3 * time.Second

	cases := []struct {
		name     string
		target   exposureTarget
		expected exposureOutcome
	}{
		{"still-leaking-200", exposureTarget{url: srv.URL + "/leak", extracted: []string{secret}}, exposureVerified},
		{"content-gone-200", exposureTarget{url: srv.URL + "/leak", extracted: []string{"MISSING-XYZ"}}, exposureResolved},
		{"not-found-404", exposureTarget{url: srv.URL + "/gone"}, exposureResolved},
		{"forbidden-403", exposureTarget{url: srv.URL + "/forbidden"}, exposureVerified},
		{"server-error-500", exposureTarget{url: srv.URL + "/boom"}, exposurePending},
		{"unreachable", exposureTarget{url: "http://nonexistent.invalid.local.test/"}, exposurePending},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := probeExposure(ctx, c.target, timeout); got != c.expected {
				t.Errorf("probeExposure(%s) = %v, want %v", c.name, got, c.expected)
			}
		})
	}
}
