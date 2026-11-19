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

func TestFailureCounterBlocksOnlyAfterLimitFailures(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := ratelimit.NewFixedWindow(3, time.Minute)
	l.Now = func() time.Time { return now }
	var fc ratelimit.FailureCounter = l
	for i := 0; i < 10; i++ { // checking never consumes budget
		if fc.Blocked("a") {
			t.Fatal("untouched key must not be blocked")
		}
	}
	fc.Fail("a")
	fc.Fail("a")
	if fc.Blocked("a") {
		t.Fatal("2 failures of 3 must not block")
	}
	fc.Fail("a")
	if !fc.Blocked("a") || !fc.Blocked("a") {
		t.Fatal("3 failures must block, and checking must stay blocked")
	}
	if fc.Blocked("b") {
		t.Fatal("keys must be independent")
	}
	now = now.Add(time.Minute)
	if fc.Blocked("a") {
		t.Fatal("window must reset")
	}
	fc.Fail("a")
	if fc.Blocked("a") {
		t.Fatal("a new window starts at one failure")
	}
}

func TestFailureCounterReset(t *testing.T) {
	l := ratelimit.NewFixedWindow(1, time.Hour)
	l.Fail("a")
	if !l.Blocked("a") {
		t.Fatal("expected blocked")
	}
	l.Reset("a")
	if l.Blocked("a") {
		t.Fatal("reset must clear the key")
	}
	l.Reset("never-seen") // no-op
}

func TestFailureCounterZeroLimitBlocksEverything(t *testing.T) {
	if !ratelimit.NewFixedWindow(0, time.Minute).Blocked("a") {
		t.Fatal("limit 0 must block, like Allow")
	}
}

func TestFailureCounterBoundedUnderKeyFlood(t *testing.T) {
	l := ratelimit.NewFixedWindow(1, time.Hour)
	for i := 0; i < 250_000; i++ {
		l.Fail(fmt.Sprintf("k%d", i))
	}
	l.Fail("fresh")
	if !l.Blocked("fresh") {
		t.Fatal("a recorded failure must count after the table was capped")
	}
}
