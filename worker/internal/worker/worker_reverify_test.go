package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cscan/internal/scheduler"
)

// TestProbeExposure 验证敏感信息复验的探测分类逻辑（纯函数，无需 DB）：
// 验收标准 3（不可达不误判为已修复）→ pending；
// 验收标准 4（软 404 内容特征兜底）→ 200 但原泄露内容已消失 → resolved。
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
		target   scheduler.ReverifyExposureTarget
		expected string
	}{
		{"still-leaking-200", scheduler.ReverifyExposureTarget{Url: srv.URL + "/leak", Extracted: []string{secret}}, "verified"},
		{"content-gone-200", scheduler.ReverifyExposureTarget{Url: srv.URL + "/leak", Extracted: []string{"MISSING-XYZ"}}, "resolved"},
		{"not-found-404", scheduler.ReverifyExposureTarget{Url: srv.URL + "/gone"}, "resolved"},
		{"forbidden-403", scheduler.ReverifyExposureTarget{Url: srv.URL + "/forbidden"}, "verified"},
		{"server-error-500", scheduler.ReverifyExposureTarget{Url: srv.URL + "/boom"}, "pending"},
		{"unreachable", scheduler.ReverifyExposureTarget{Url: "http://nonexistent.invalid.local.test/"}, "pending"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := probeExposure(ctx, c.target, timeout); got != c.expected {
				t.Errorf("probeExposure(%s) = %v, want %v", c.name, got, c.expected)
			}
		})
	}
}
