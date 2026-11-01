// Package filters contains HTTP middleware (net/http) and Beego filters.
package filters

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/httpx"
	"github.com/lingster/go-rest-template/backend/services"
)

// Middleware wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares so that the first one is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder remembers the status code written.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.written {
		s.status, s.written = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.written {
		s.status, s.written = http.StatusOK, true
	}
	return s.ResponseWriter.Write(b)
}

// Recover turns panics into the JSON 500 error body.
func Recover() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: 200}
			defer func() {
				if p := recover(); p != nil {
					if p == http.ErrAbortHandler {
						panic(p)
					}
					log.Printf("panic: %v %s %s", p, r.Method, r.URL.Path)
					if !rec.written {
						httpx.WriteError(w, apperr.Internal())
					}
				}
			}()
			next.ServeHTTP(rec, r)
		})
	}
}

// RequestLogger logs one line per request.
func RequestLogger(trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r)
			log.Printf("%s %s -> %d (%s) ip=%s", r.Method, r.URL.Path, rec.status,
				time.Since(start).Round(time.Microsecond), httpx.ClientIP(r, trustProxy))
		})
	}
}

// SecurityHeaders sets hardening headers and a per-request CSP nonce (exposed through the request context).
func SecurityHeaders(cfg *config.Config) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			if cfg.IsProd() {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
				h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			} else {
				nonce := services.RandomToken(16)
				r = r.WithContext(httpx.WithNonce(r.Context(), nonce))
				h.Set("Content-Security-Policy", fmt.Sprintf(
					"default-src 'none'; script-src 'self' 'nonce-%[1]s'; style-src 'self' 'nonce-%[1]s'; "+
						"img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; "+
						"form-action 'self'; frame-ancestors 'none'", nonce))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS implements an allow-list based CORS policy (bearer tokens; no cookies, so no credentials).
func CORS(allowed []string) Middleware {
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		set[strings.TrimRight(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Add("Vary", "Origin")
				if set["*"] || set[origin] {
					h := w.Header()
					if set["*"] {
						h.Set("Access-Control-Allow-Origin", "*")
					} else {
						h.Set("Access-Control-Allow-Origin", origin)
					}
					h.Set("Access-Control-Expose-Headers", "Retry-After")
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
						h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						h.Set("Access-Control-Max-Age", "600")
						w.WriteHeader(http.StatusNoContent)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ForceJSONBody marks every API request body as JSON. Beego pre-parses
// application/x-www-form-urlencoded bodies (e.g. curl -d without a header), which would consume
// the JSON payload before the controller reads it.
func ForceJSONBody(prefix string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, prefix) && r.Body != nil && r.Method != http.MethodGet {
				if ct := r.Header.Get("Content-Type"); ct == "" || strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
					r.Header.Set("Content-Type", "application/json")
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Limiter decides whether a key may proceed.
type Limiter interface{ Allow(key string) bool }

// FixedWindowLimiter is a simple in-memory per-key fixed window rate limiter.
type FixedWindowLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   map[string]*windowCount
	now    func() time.Time
}

type windowCount struct {
	start time.Time
	n     int
}

func NewFixedWindowLimiter(limit int, window time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{limit: limit, window: window, hits: map[string]*windowCount{}, now: time.Now}
}

func (l *FixedWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.hits) > 10000 { // bound memory
		for k, w := range l.hits {
			if now.Sub(w.start) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.hits[key] = &windowCount{start: now, n: 1}
		return true
	}
	w.n++
	return w.n <= l.limit
}

// RateLimit applies limiter per client IP to requests whose path starts with prefix.
func RateLimit(limiter Limiter, prefix string, trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter != nil && strings.HasPrefix(r.URL.Path, prefix) && r.Method != http.MethodOptions &&
				!limiter.Allow(httpx.ClientIP(r, trustProxy)) {
				httpx.WriteError(w, apperr.RateLimited())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
