// Package ratelimit provides the in-memory limiters used by the HTTP middleware (per client IP) and by
// the services (per account). Counters are per process: behind several instances enforce limits at the
// edge as well, or swap in a shared store behind the Limiter interface.
package ratelimit

import (
	"net"
	"sync"
	"time"
)

// Limiter decides whether a key may proceed.
type Limiter interface{ Allow(key string) bool }

// maxKeys bounds memory: past this many live keys the table is swept, and if every key is still
// live (a flood of distinct keys) it is reset rather than allowed to grow without limit.
const maxKeys = 100_000

// FixedWindow is a per-key fixed window counter: at most limit calls per key per window.
type FixedWindow struct {
	limit     int
	window    time.Duration
	mu        sync.Mutex
	hits      map[string]*windowCount
	lastSweep time.Time
	// Now is the clock (overridable in tests).
	Now func() time.Time
}

type windowCount struct {
	start time.Time
	n     int
}

// NewFixedWindow allows limit calls per key per window.
func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{limit: limit, window: window, hits: map[string]*windowCount{}, Now: time.Now}
}

// Allow records a call for key and reports whether it is within the limit.
func (l *FixedWindow) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Now()
	l.sweepLocked(now)
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.hits[key] = &windowCount{start: now, n: 1}
		return l.limit > 0
	}
	w.n++
	return w.n <= l.limit
}

// sweepLocked drops expired windows, at most once per window so a large table cannot turn every
// request into an O(n) scan.
func (l *FixedWindow) sweepLocked(now time.Time) {
	if len(l.hits) < maxKeys {
		return
	}
	if now.Sub(l.lastSweep) >= l.window {
		l.lastSweep = now
		for k, w := range l.hits {
			if now.Sub(w.start) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	if len(l.hits) >= 2*maxKeys { // hard cap between sweeps: fail open rather than exhaust memory
		l.hits = map[string]*windowCount{}
	}
}

// ClientKey turns a client IP into a rate-limit key. IPv6 clients are grouped by /64, the smallest
// block normally assigned to a single subscriber, so rotating addresses inside it does not reset
// the limit.
func ClientKey(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return ip
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}
