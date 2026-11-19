package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lingster/go-rest-template/backend/httpx"
)

func TestRefreshCookieSecureSetReadClear(t *testing.T) {
	c := httpx.RefreshCookie{Secure: true, Domain: "example.com", TTL: time.Hour}
	rec := httptest.NewRecorder()
	c.Set(rec, "tok")
	set := rec.Result().Cookies()
	if len(set) != 1 {
		t.Fatalf("expected one cookie, got %v", set)
	}
	got := set[0]
	if got.Name != "__Secure-rt" || got.Value != "tok" || !got.Secure || !got.HttpOnly ||
		got.SameSite != http.SameSiteStrictMode || got.Path != httpx.RefreshCookiePath ||
		got.Domain != "example.com" || got.MaxAge != 3600 {
		t.Fatalf("bad cookie: %+v", got)
	}

	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "rt", Value: "plain"}) // the non-Secure name is ignored
	if v := c.Read(req); v != "" {
		t.Fatalf("read the wrong cookie: %q", v)
	}
	req.AddCookie(&http.Cookie{Name: "__Secure-rt", Value: "tok"})
	if v := c.Read(req); v != "tok" {
		t.Fatalf("read %q", v)
	}

	rec = httptest.NewRecorder()
	c.Clear(rec)
	if cl := rec.Result().Cookies()[0]; cl.Name != "__Secure-rt" || cl.MaxAge >= 0 || cl.Path != httpx.RefreshCookiePath {
		t.Fatalf("bad clear: %+v", cl)
	}
}

func TestCheckCookieRequest(t *testing.T) {
	allowed := httpx.NewOrigins([]string{"https://app.example.com/", "*"})
	req := func(header, origin string) *http.Request {
		r := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
		if header != "" {
			r.Header.Set(httpx.SessionTransportHeader, header)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	cases := []struct {
		header, origin string
		ok             bool
	}{
		{"cookie", "https://app.example.com", true},
		{"Cookie", "", true},
		{"", "https://app.example.com", false},
		{"body", "https://app.example.com", false},
		{"cookie", "https://evil.example", false}, // "*" never allows cookie requests
	}
	for _, tc := range cases {
		if err := httpx.CheckCookieRequest(req(tc.header, tc.origin), allowed); (err == nil) != tc.ok {
			t.Errorf("header=%q origin=%q: err=%v", tc.header, tc.origin, err)
		}
	}
}
