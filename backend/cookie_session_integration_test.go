package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/lingster/go-rest-template/backend/httpx"
)

const webOrigin = "http://localhost:5173" // allow-listed in testutil.Config

// webHeaders are the headers the webapp sends; cookie is the refresh cookie value ("" for none).
func webHeaders(cookie string) map[string]string {
	h := map[string]string{httpx.SessionTransportHeader: httpx.SessionTransportCookie, "Origin": webOrigin}
	if cookie != "" {
		h["Cookie"] = "rt=" + cookie
	}
	return h
}

// refreshCookie returns the rt cookie set by r, failing when there is none.
func refreshCookie(t *testing.T, r resp) *http.Cookie {
	t.Helper()
	for _, c := range (&http.Response{Header: r.Header}).Cookies() {
		if c.Name == "rt" {
			return c
		}
	}
	t.Fatalf("no rt cookie set: %v", r.Header.Values("Set-Cookie"))
	return nil
}

func expectNoCookie(t *testing.T, r resp) {
	t.Helper()
	if sc := r.Header.Values("Set-Cookie"); len(sc) != 0 {
		t.Fatalf("unexpected Set-Cookie: %v", sc)
	}
}

func expectCleared(t *testing.T, r resp) {
	t.Helper()
	if c := refreshCookie(t, r); c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("cookie not cleared: %+v", c)
	}
}

func expectNoBodyRefreshToken(t *testing.T, r resp, path ...string) {
	t.Helper()
	if strings.Contains(string(r.Raw), "refresh_token") || r.str(append(path, "access_token")...) == "" {
		t.Fatalf("expected access token only in body: %s", r.Raw)
	}
}

// cookieLogin signs a fresh verified user in through the webapp transport; returns email and cookie value.
func cookieLogin(t *testing.T, prefix string) (string, string) {
	t.Helper()
	email, _, _ := newVerifiedUser(t, prefix)
	r := callWith(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "", webHeaders(""))
	expect(t, r, 200, "")
	expectNoBodyRefreshToken(t, r, "tokens")
	return email, refreshCookie(t, r).Value
}

func TestCookieSessionLoginSetsCookie(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "cookielogin")
	r := callWith(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "", webHeaders(""))
	expect(t, r, 200, "")
	expectNoBodyRefreshToken(t, r, "tokens")
	if r.str("tokens", "token_type") != "Bearer" || r.str("user", "email") != email {
		t.Fatalf("bad auth response: %s", r.Raw)
	}
	c := refreshCookie(t, r)
	if c.Value == "" || !c.HttpOnly || c.Secure || c.SameSite != http.SameSiteStrictMode ||
		c.Path != "/api/v1/auth" || c.Domain != "" || c.MaxAge != int((720*time.Hour).Seconds()) {
		t.Fatalf("bad cookie attributes: %+v", c)
	}
}

func TestBodyTokenFlowUnchanged(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "bodyflow")
	r := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	expect(t, r, 200, "")
	expectNoCookie(t, r)
	rt := r.str("tokens", "refresh_token")
	if rt == "" {
		t.Fatalf("body refresh token missing: %s", r.Raw)
	}
	r = call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": rt}, "")
	expect(t, r, 200, "")
	expectNoCookie(t, r)
	if r.str("refresh_token") == "" {
		t.Fatalf("rotated body refresh token missing: %s", r.Raw)
	}
	lo := call(t, "POST", "/api/v1/auth/logout", map[string]string{"refresh_token": r.str("refresh_token")}, "")
	expect(t, lo, 204, "")
	expectNoCookie(t, lo)
	// no token at all is still a validation error for body clients
	expect(t, call(t, "POST", "/api/v1/auth/refresh", nil, ""), 422, "validation_error")
	expect(t, call(t, "POST", "/api/v1/auth/logout", map[string]string{}, ""), 422, "validation_error")
}

func TestCookieRefreshRotatesCookie(t *testing.T) {
	_, rt := cookieLogin(t, "cookierefresh")
	r := callWith(t, "POST", "/api/v1/auth/refresh", nil, "", webHeaders(rt))
	expect(t, r, 200, "")
	expectNoBodyRefreshToken(t, r)
	next := refreshCookie(t, r).Value
	if next == "" || next == rt {
		t.Fatal("refresh cookie not rotated")
	}
	if me := call(t, "GET", "/api/v1/me", nil, r.str("access_token")); me.Status != 200 {
		t.Fatalf("new access token rejected: %s", me.Raw)
	}
	// reuse of the old cookie fails, clears the cookie and revokes the family
	old := callWith(t, "POST", "/api/v1/auth/refresh", nil, "", webHeaders(rt))
	expect(t, old, 401, "invalid_token")
	expectCleared(t, old)
	expect(t, callWith(t, "POST", "/api/v1/auth/refresh", nil, "", webHeaders(next)), 401, "invalid_token")

	// cookie transport without any cookie: no session, and no Set-Cookie that could race a concurrent login
	none := callWith(t, "POST", "/api/v1/auth/refresh", nil, "", webHeaders(""))
	expect(t, none, 401, "invalid_token")
	expectNoCookie(t, none)
}

func TestCookieLogoutClearsCookie(t *testing.T) {
	_, rt := cookieLogin(t, "cookielogout")
	lo := callWith(t, "POST", "/api/v1/auth/logout", nil, "", webHeaders(rt))
	expect(t, lo, 204, "")
	expectCleared(t, lo)
	expect(t, callWith(t, "POST", "/api/v1/auth/refresh", nil, "", webHeaders(rt)), 401, "invalid_token")
	// idempotent without a cookie
	again := callWith(t, "POST", "/api/v1/auth/logout", nil, "", webHeaders(""))
	expect(t, again, 204, "")
	expectCleared(t, again)
}

func TestCookieRequestsNeedHeaderAndAllowedOrigin(t *testing.T) {
	_, rt := cookieLogin(t, "cookiecsrf")
	noHeader := map[string]string{"Cookie": "rt=" + rt, "Origin": webOrigin}
	evil := webHeaders(rt)
	evil["Origin"] = "https://evil.example"
	for _, path := range []string{"/api/v1/auth/refresh", "/api/v1/auth/logout"} {
		r := callWith(t, "POST", path, nil, "", noHeader)
		expect(t, r, 403, "forbidden")
		expectNoCookie(t, r)
		expect(t, callWith(t, "POST", path, nil, "", evil), 403, "forbidden")
	}
	// the rejected requests neither consumed nor revoked the session; no Origin (same-origin proxy) is fine
	noOrigin := webHeaders(rt)
	delete(noOrigin, "Origin")
	expect(t, callWith(t, "POST", "/api/v1/auth/refresh", nil, "", noOrigin), 200, "")
}

func TestCookieTransportMigratesBodyToken(t *testing.T) {
	_, _, legacy := newVerifiedUser(t, "cookiemigrate")
	r := callWith(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": legacy}, "", webHeaders(""))
	expect(t, r, 200, "")
	expectNoBodyRefreshToken(t, r)
	expect(t, callWith(t, "POST", "/api/v1/auth/refresh", nil, "", webHeaders(refreshCookie(t, r).Value)), 200, "")
}

func TestCookieTransportMFALogin(t *testing.T) {
	email, access, _ := newVerifiedUser(t, "cookiemfa")
	setup := call(t, "POST", "/api/v1/me/2fa/totp/setup", nil, access)
	expect(t, setup, 200, "")
	code, _ := totp.GenerateCode(setup.str("secret"), time.Now().Add(-30*time.Second))
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/enable", map[string]string{"code": code}, access), 200, "")

	ch := callWith(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "", webHeaders(""))
	expect(t, ch, 200, "")
	expectNoCookie(t, ch) // a challenge is not a session
	code, _ = totp.GenerateCode(setup.str("secret"), time.Now())
	ok := callWith(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": ch.str("mfa_token"), "code": code}, "", webHeaders(""))
	expect(t, ok, 200, "")
	expectNoBodyRefreshToken(t, ok, "tokens")
	refreshCookie(t, ok)
}

func TestCORSCredentialsOnlyForAllowedOrigins(t *testing.T) {
	preflight := func(origin string) http.Header {
		req := httptest.NewRequest("OPTIONS", "/api/v1/auth/refresh", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type, x-session-transport")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Header()
	}
	h := preflight(webOrigin)
	if h.Get("Access-Control-Allow-Origin") != webOrigin || h.Get("Access-Control-Allow-Credentials") != "true" ||
		!strings.Contains(h.Get("Access-Control-Allow-Headers"), httpx.SessionTransportHeader) {
		t.Fatalf("allowed origin preflight: %v", h)
	}
	h = preflight("https://evil.example")
	if h.Get("Access-Control-Allow-Origin") != "" || h.Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("disallowed origin got CORS headers: %v", h)
	}
}
