package httpx_test

import (
	"net/http/httptest"
	"testing"

	"github.com/lingster/go-rest-template/backend/httpx"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name  string
		xff   []string
		trust bool
		want  string
	}{
		{"no proxy trust ignores header", []string{"1.2.3.4"}, false, "10.0.0.1"},
		{"single entry", []string{"1.2.3.4"}, true, "1.2.3.4"},
		{"spoofed leftmost entry is ignored", []string{"6.6.6.6, 1.2.3.4"}, true, "1.2.3.4"},
		{"repeated headers use the last", []string{"6.6.6.6", "1.2.3.4"}, true, "1.2.3.4"},
		{"proxy that appends a port", []string{"1.2.3.4:5678"}, true, "1.2.3.4"},
		{"ipv6 with port", []string{"[2001:db8::1]:443"}, true, "2001:db8::1"},
		{"garbage falls back to peer", []string{"1.2.3.4, not-an-ip"}, true, "10.0.0.1"},
		{"no header falls back to peer", nil, true, "10.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.1:5555"
		for _, v := range c.xff {
			r.Header.Add("X-Forwarded-For", v)
		}
		if got := httpx.ClientIP(r, c.trust); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
