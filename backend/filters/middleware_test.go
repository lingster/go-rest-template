package filters_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lingster/go-rest-template/backend/filters"
)

func TestRateLimitMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := filters.RateLimit(filters.NewFixedWindowLimiter(2, time.Minute), "/api/v1/auth/", false)(ok)

	hit := func(path, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for i := 0; i < 2; i++ {
		if r := hit("/api/v1/auth/login", "1.1.1.1"); r.Code != 200 {
			t.Fatalf("request %d: %d", i, r.Code)
		}
	}
	r := hit("/api/v1/auth/login", "1.1.1.1")
	if r.Code != 429 || !strings.Contains(r.Body.String(), `"rate_limited"`) {
		t.Fatalf("expected 429 rate_limited, got %d %s", r.Code, r.Body.String())
	}
	if hit("/api/v1/auth/login", "2.2.2.2").Code != 200 {
		t.Error("other IP must not be limited")
	}
	if hit("/api/v1/me", "1.1.1.1").Code != 200 {
		t.Error("non-auth path must not be limited")
	}
}

func TestRecoverMiddleware(t *testing.T) {
	h := filters.Recover()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"internal_error"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}
