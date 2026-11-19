package ratelimit_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/lingster/go-rest-template/backend/ratelimit"
)

func TestFixedWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := ratelimit.NewFixedWindow(2, time.Minute)
	l.Now = func() time.Time { return now }
	if !l.Allow("a") || !l.Allow("a") || l.Allow("a") {
		t.Fatal("expected exactly 2 calls per window")
	}
	if !l.Allow("b") {
		t.Fatal("keys must be independent")
	}
	now = now.Add(time.Minute)
	if !l.Allow("a") {
		t.Fatal("window must reset")
	}
}

func TestFixedWindowBoundedUnderKeyFlood(t *testing.T) {
	l := ratelimit.NewFixedWindow(1, time.Hour)
	for i := 0; i < 250_000; i++ {
		l.Allow(fmt.Sprintf("k%d", i))
	}
	// No assertion on internals: the call must stay fast and memory bounded (hard cap resets the table).
	if !l.Allow("fresh") {
		t.Fatal("new key must be allowed")
	}
}

func TestClientKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":              "203.0.113.7",
		"2001:db8:1:2:aaaa::1":     "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff:ffff::": "2001:db8:1:2::/64",
		"::ffff:203.0.113.7":       "::ffff:203.0.113.7",
		"not-an-ip":                "not-an-ip",
	}
	for in, want := range cases {
		if got := ratelimit.ClientKey(in); got != want {
			t.Errorf("ClientKey(%q) = %q, want %q", in, got, want)
		}
	}
}
