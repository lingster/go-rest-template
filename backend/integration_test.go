package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/lingster/go-rest-template/backend/app"
	"github.com/lingster/go-rest-template/backend/testutil"
)

const (
	adminEmail    = "admin@example.com"
	adminPassword = "admin-password-1"
	password      = "correct horse 42"
)

var (
	handler http.Handler
	mailer  = &testutil.MemMailer{}
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gorest-it-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	cfg := testutil.Config()
	cfg.DatabaseURL = "file:" + dir + "/test.db"
	cfg.AdminEmail, cfg.AdminPassword = adminEmail, adminPassword
	a, err := app.New(cfg, app.Options{Mailer: mailer, Assets: assetsFS})
	if err != nil {
		panic(err)
	}
	handler = a.Handler()
	os.Exit(m.Run())
}

type resp struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   map[string]interface{}
}

func (r resp) str(path ...string) string {
	var cur interface{} = r.Body
	for _, p := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur = m[p]
	}
	s, _ := cur.(string)
	return s
}

func (r resp) errCode() string { return r.str("error", "code") }

func call(t *testing.T, method, path string, body interface{}, token string) resp {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	r := resp{Status: rec.Code, Header: rec.Header(), Raw: rec.Body.Bytes()}
	_ = json.Unmarshal(r.Raw, &r.Body)
	return r
}

func expect(t *testing.T, r resp, status int, code string) {
	t.Helper()
	if r.Status != status || (code != "" && r.errCode() != code) {
		t.Fatalf("expected %d %q, got %d %s", status, code, r.Status, r.Raw)
	}
}

// register + verify + login; returns tokens.
func newVerifiedUser(t *testing.T, prefix string) (email, access, refresh string) {
	t.Helper()
	email = testutil.Email(prefix)
	expect(t, call(t, "POST", "/api/v1/auth/register", map[string]string{"email": email, "password": password, "name": "Test"}, ""), 201, "")
	tok := mailer.LastToken(email, "Verify")
	expect(t, call(t, "POST", "/api/v1/auth/verify-email", map[string]string{"token": tok}, ""), 200, "")
	r := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	expect(t, r, 200, "")
	return email, r.str("tokens", "access_token"), r.str("tokens", "refresh_token")
}

func TestRegisterVerifyLoginRefreshLogout(t *testing.T) {
	email := testutil.Email("flow")
	r := call(t, "POST", "/api/v1/auth/register", map[string]string{"email": strings.ToUpper(email), "password": password, "name": "Ada"}, "")
	expect(t, r, 201, "")
	if r.str("user", "email") != email || r.str("user", "role") != "user" || r.str("message") == "" {
		t.Fatalf("bad register body: %s", r.Raw)
	}
	// duplicate
	expect(t, call(t, "POST", "/api/v1/auth/register", map[string]string{"email": email, "password": password, "name": "x"}, ""), 409, "conflict")
	// validation
	v := call(t, "POST", "/api/v1/auth/register", map[string]string{"email": "nope", "password": "short", "name": ""}, "")
	expect(t, v, 422, "validation_error")
	details, _ := v.Body["error"].(map[string]interface{})["details"].(map[string]interface{})
	if details["email"] == nil || details["password"] == nil || details["name"] == nil {
		t.Fatalf("missing details: %s", v.Raw)
	}
	// unverified login blocked; wrong password is invalid_credentials
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 403, "email_not_verified")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "wrong-password"}, ""), 401, "invalid_credentials")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": "ghost@example.com", "password": password}, ""), 401, "invalid_credentials")

	// verify (single use)
	tok := mailer.LastToken(email, "Verify")
	if tok == "" {
		t.Fatal("no verification email captured")
	}
	expect(t, call(t, "POST", "/api/v1/auth/verify-email", map[string]string{"token": tok}, ""), 200, "")
	expect(t, call(t, "POST", "/api/v1/auth/verify-email", map[string]string{"token": tok}, ""), 400, "invalid_token")

	// login
	r = call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	expect(t, r, 200, "")
	access, refresh := r.str("tokens", "access_token"), r.str("tokens", "refresh_token")
	if r.Body["mfa_required"] != false || r.str("tokens", "token_type") != "Bearer" || r.Body["tokens"].(map[string]interface{})["expires_in"].(float64) != 900 {
		t.Fatalf("bad auth response: %s", r.Raw)
	}

	// /me
	me := call(t, "GET", "/api/v1/me", nil, access)
	expect(t, me, 200, "")
	if me.str("email") != email || me.Body["email_verified"] != true || me.Body["passkey_count"].(float64) != 0 {
		t.Fatalf("bad /me: %s", me.Raw)
	}
	expect(t, call(t, "GET", "/api/v1/me", nil, ""), 401, "unauthorized")
	expect(t, call(t, "GET", "/api/v1/me", nil, "garbage"), 401, "invalid_token")
	if up := call(t, "PATCH", "/api/v1/me", map[string]string{"name": "Ada L"}, access); up.str("name") != "Ada L" {
		t.Fatalf("patch me: %s", up.Raw)
	}

	// refresh rotates; old token reuse revokes the family
	r2 := call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "")
	expect(t, r2, 200, "")
	newRefresh := r2.str("refresh_token")
	if newRefresh == "" || newRefresh == refresh {
		t.Fatal("refresh token not rotated")
	}
	expect(t, call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, ""), 401, "invalid_token")
	expect(t, call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": newRefresh}, ""), 401, "invalid_token")

	// logout
	r = call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	rt := r.str("tokens", "refresh_token")
	lo := call(t, "POST", "/api/v1/auth/logout", map[string]string{"refresh_token": rt}, "")
	if lo.Status != 204 || len(lo.Raw) != 0 {
		t.Fatalf("logout: %d %s", lo.Status, lo.Raw)
	}
	expect(t, call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": rt}, ""), 401, "invalid_token")
}

func TestForgotAndResetPassword(t *testing.T) {
	email, _, refresh := newVerifiedUser(t, "reset")
	// no enumeration: both 202; only the real account gets mail
	expect(t, call(t, "POST", "/api/v1/auth/forgot-password", map[string]string{"email": "nobody@example.com"}, ""), 202, "")
	if mailer.Count("nobody@example.com") != 0 {
		t.Fatal("mail sent to unknown address")
	}
	expect(t, call(t, "POST", "/api/v1/auth/forgot-password", map[string]string{"email": email}, ""), 202, "")
	tok := mailer.LastToken(email, "Reset")
	if tok == "" {
		t.Fatal("no reset email")
	}
	expect(t, call(t, "POST", "/api/v1/auth/reset-password", map[string]string{"token": tok, "password": "short"}, ""), 422, "validation_error")
	expect(t, call(t, "POST", "/api/v1/auth/reset-password", map[string]string{"token": tok, "password": "brand new password"}, ""), 200, "")
	expect(t, call(t, "POST", "/api/v1/auth/reset-password", map[string]string{"token": tok, "password": "another password"}, ""), 400, "invalid_token")
	// old sessions are revoked, old password dead, new one works
	expect(t, call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, ""), 401, "")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 401, "invalid_credentials")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "brand new password"}, ""), 200, "")
}

func TestPasskeyLoginBeginIndistinguishable(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "pk") // exists, no passkeys
	shape := func(body interface{}) (string, bool, string) {
		r := call(t, "POST", "/api/v1/auth/passkey/login/begin", body, "")
		if r.Status != 200 || r.str("session_id") == "" {
			t.Fatalf("expected 200 with session: %d %s", r.Status, r.Raw)
		}
		pk, _ := r.Body["options"].(map[string]interface{})["publicKey"].(map[string]interface{})
		_, hasAllow := pk["allowCredentials"]
		return pk["rpId"].(string), hasAllow, r.str("options", "publicKey", "userVerification")
	}
	a1, a2, a3 := shape(map[string]string{"email": email})
	b1, b2, b3 := shape(map[string]string{"email": "ghost@example.com"})
	c1, c2, c3 := shape(nil)
	if a1 != b1 || b1 != c1 || a2 != b2 || b2 != c2 || a3 != b3 || b3 != c3 {
		t.Fatalf("responses differ: (%v %v %v) (%v %v %v) (%v %v %v)", a1, a2, a3, b1, b2, b3, c1, c2, c3)
	}
}

func TestChangePasswordRevokesSessionsAndNotifies(t *testing.T) {
	email, access, refresh := newVerifiedUser(t, "chpw2")
	before := mailer.Count(email)
	expect(t, call(t, "POST", "/api/v1/me/change-password", map[string]string{"current_password": password, "new_password": "a brand new one"}, access), 200, "")
	expect(t, call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, ""), 401, "invalid_token")
	if mailer.Count(email) != before+1 || mailer.LastSubject(email) != "Your password was changed" {
		t.Fatalf("notification email missing (subject %q)", mailer.LastSubject(email))
	}
	// the current access token keeps working until it expires
	expect(t, call(t, "GET", "/api/v1/me", nil, access), 200, "")
}

func TestChangePassword(t *testing.T) {
	email, access, _ := newVerifiedUser(t, "chpw")
	expect(t, call(t, "POST", "/api/v1/me/change-password", map[string]string{"current_password": "nope", "new_password": "a brand new one"}, access), 401, "invalid_credentials")
	expect(t, call(t, "POST", "/api/v1/me/change-password", map[string]string{"current_password": password, "new_password": "a brand new one"}, access), 200, "")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "a brand new one"}, ""), 200, "")
}

func TestTwoFactorLogin(t *testing.T) {
	email, access, _ := newVerifiedUser(t, "mfa")

	setup := call(t, "POST", "/api/v1/me/2fa/totp/setup", nil, access)
	expect(t, setup, 200, "")
	secret := setup.str("secret")
	if !strings.HasPrefix(setup.str("otpauth_url"), "otpauth://") {
		t.Fatalf("bad setup: %s", setup.Raw)
	}
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/enable", map[string]string{"code": "000000"}, access), 401, "invalid_mfa_code")
	// Each TOTP step is accepted once, so successive uses move forward: previous, current, next step.
	code, _ := totp.GenerateCode(secret, time.Now().Add(-30*time.Second))
	en := call(t, "POST", "/api/v1/me/2fa/totp/enable", map[string]string{"code": code}, access)
	expect(t, en, 200, "")
	codes, _ := en.Body["recovery_codes"].([]interface{})
	if len(codes) != 10 {
		t.Fatalf("expected 10 recovery codes: %s", en.Raw)
	}

	// password login now yields a challenge
	ch := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	expect(t, ch, 200, "")
	if ch.Body["mfa_required"] != true || ch.str("mfa_token") == "" || ch.Body["tokens"] != nil {
		t.Fatalf("expected mfa challenge: %s", ch.Raw)
	}
	mfaTok := ch.str("mfa_token")

	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": "000000"}, ""), 401, "invalid_mfa_code")
	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": "bad", "code": code}, ""), 401, "invalid_token")
	code, _ = totp.GenerateCode(secret, time.Now())
	ok := call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": code}, "")
	expect(t, ok, 200, "")
	if ok.str("tokens", "access_token") == "" || ok.str("user", "email") != email || ok.Body["user"].(map[string]interface{})["totp_enabled"] != true {
		t.Fatalf("bad 2fa login: %s", ok.Raw)
	}
	// replay of the same code (same step) is rejected, as is an older step
	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": code}, ""), 401, "invalid_mfa_code")
	old, _ := totp.GenerateCode(secret, time.Now().Add(-30*time.Second))
	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": old}, ""), 401, "invalid_mfa_code")
	// an mfa token is not an access token
	expect(t, call(t, "GET", "/api/v1/me", nil, mfaTok), 401, "invalid_token")

	// recovery code works once
	rc := codes[0].(string)
	ok = call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": rc}, "")
	expect(t, ok, 200, "")
	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": rc}, ""), 401, "invalid_mfa_code")

	// disable
	access2 := ok.str("tokens", "access_token")
	code, _ = totp.GenerateCode(secret, time.Now().Add(30*time.Second))
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/disable", map[string]string{"password": "wrong", "code": code}, access2), 401, "invalid_credentials")
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/disable", map[string]string{"password": password, "code": code}, access2), 200, "")
	r := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	if r.Body["mfa_required"] != false {
		t.Fatalf("2fa still required: %s", r.Raw)
	}
}

func TestAdminAccessControlAndManagement(t *testing.T) {
	email, userAccess, _ := newVerifiedUser(t, "plain")
	admin := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": adminEmail, "password": adminPassword}, "")
	expect(t, admin, 200, "")
	adminAccess := admin.str("tokens", "access_token")
	adminID := admin.str("user", "id")
	if admin.str("user", "role") != "admin" {
		t.Fatal("seeded user is not admin")
	}

	for _, p := range []string{"/api/v1/admin/stats", "/api/v1/admin/users", "/api/v1/admin/audit-logs"} {
		expect(t, call(t, "GET", p, nil, ""), 401, "unauthorized")
		expect(t, call(t, "GET", p, nil, userAccess), 403, "forbidden")
		expect(t, call(t, "GET", p, nil, adminAccess), 200, "")
	}

	st := call(t, "GET", "/api/v1/admin/stats", nil, adminAccess)
	if st.Body["users_total"].(float64) < 2 || st.Body["users_active"] == nil || st.Body["users_verified"] == nil || st.Body["users_2fa"] == nil {
		t.Fatalf("bad stats: %s", st.Raw)
	}

	list := call(t, "GET", "/api/v1/admin/users?q="+email+"&per_page=5", nil, adminAccess)
	expect(t, list, 200, "")
	items := list.Body["items"].([]interface{})
	if len(items) != 1 || list.Body["total"].(float64) != 1 || list.Body["page"].(float64) != 1 || list.Body["per_page"].(float64) != 5 {
		t.Fatalf("bad list: %s", list.Raw)
	}
	uid := items[0].(map[string]interface{})["id"].(string)

	expect(t, call(t, "GET", "/api/v1/admin/users/"+uid, nil, adminAccess), 200, "")
	expect(t, call(t, "GET", "/api/v1/admin/users/does-not-exist", nil, adminAccess), 404, "not_found")
	expect(t, call(t, "PATCH", "/api/v1/admin/users/"+uid, map[string]string{"role": "root"}, adminAccess), 422, "validation_error")

	// disable the user: login and existing access token both stop working
	up := call(t, "PATCH", "/api/v1/admin/users/"+uid, map[string]interface{}{"is_active": false}, adminAccess)
	expect(t, up, 200, "")
	if up.Body["is_active"] != false {
		t.Fatalf("not disabled: %s", up.Raw)
	}
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 403, "account_disabled")
	expect(t, call(t, "GET", "/api/v1/me", nil, userAccess), 403, "account_disabled")
	call(t, "PATCH", "/api/v1/admin/users/"+uid, map[string]interface{}{"is_active": true, "role": "admin"}, adminAccess)
	expect(t, call(t, "GET", "/api/v1/admin/stats", nil, userAccess), 200, "")

	// self-protection
	expect(t, call(t, "DELETE", "/api/v1/admin/users/"+adminID, nil, adminAccess), 403, "forbidden")
	expect(t, call(t, "PATCH", "/api/v1/admin/users/"+adminID, map[string]interface{}{"is_active": false}, adminAccess), 403, "forbidden")

	// revoke sessions + delete
	rs := call(t, "POST", "/api/v1/admin/users/"+uid+"/revoke-sessions", nil, adminAccess)
	if rs.Status != 204 {
		t.Fatalf("revoke: %d %s", rs.Status, rs.Raw)
	}
	if d := call(t, "DELETE", "/api/v1/admin/users/"+uid, nil, adminAccess); d.Status != 204 {
		t.Fatalf("delete: %d %s", d.Status, d.Raw)
	}
	expect(t, call(t, "GET", "/api/v1/admin/users/"+uid, nil, adminAccess), 404, "not_found")

	// audit trail
	al := call(t, "GET", "/api/v1/admin/audit-logs?user_id="+uid+"&per_page=50", nil, adminAccess)
	expect(t, al, 200, "")
	seen := map[string]bool{}
	for _, it := range al.Body["items"].([]interface{}) {
		seen[it.(map[string]interface{})["action"].(string)] = true
	}
	for _, want := range []string{"register", "login.success", "admin.user.update", "admin.user.delete", "admin.user.revoke_sessions", "login.failure"} {
		if !seen[want] {
			t.Errorf("audit log missing %q (have %v)", want, seen)
		}
	}
}

func TestMiscEndpointsAndHeaders(t *testing.T) {
	h := call(t, "GET", "/healthz", nil, "")
	if h.Status != 200 || h.str("status") != "ok" || h.str("version") == "" {
		t.Fatalf("healthz: %d %s", h.Status, h.Raw)
	}
	if h.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing security headers")
	}

	nf := call(t, "GET", "/api/v1/nope", nil, "")
	expect(t, nf, 404, "not_found")
	if !strings.HasPrefix(nf.Header.Get("Content-Type"), "application/json") {
		t.Errorf("404 content type %q", nf.Header.Get("Content-Type"))
	}
	expect(t, call(t, "POST", "/api/v1/auth/login", nil, ""), 422, "validation_error")

	// CORS
	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("cors preflight: %d %v", rec.Code, rec.Header())
	}
	req = httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("disallowed origin got CORS header")
	}

	// swagger spec + UI served from the embedded FS
	sw := call(t, "GET", "/swagger/swagger.json", nil, "")
	if sw.Status != 200 || sw.Body["swagger"] == nil || sw.Body["paths"] == nil {
		t.Fatalf("swagger.json: %d", sw.Status)
	}
	ui := call(t, "GET", "/swagger/", nil, "")
	if ui.Status != 200 || strings.Contains(string(ui.Raw), "<script>") {
		t.Fatalf("swagger ui: %d", ui.Status)
	}
	if js := call(t, "GET", "/swagger/swagger-initializer.js", nil, ""); js.Status != 200 {
		t.Fatalf("initializer: %d", js.Status)
	}
}

var (
	nonceRe  = regexp.MustCompile(`nonce-([A-Za-z0-9_\-]+)`)
	inlineRe = regexp.MustCompile(`(?i)(\sstyle=|\son[a-z]+=|<script>|<style>)`)
)

func TestAdminUICSP(t *testing.T) {
	for _, p := range []string{"/admin-ui", "/admin-ui/", "/admin-ui/dashboard", "/admin-ui/users", "/admin-ui/audit",
		"/admin-ui/dashboard/", "/admin-ui/users/", "/admin-ui/audit/"} {
		r := call(t, "GET", p, nil, "")
		if r.Status != 200 {
			t.Fatalf("%s: %d %s", p, r.Status, r.Raw)
		}
		csp := r.Header.Get("Content-Security-Policy")
		m := nonceRe.FindStringSubmatch(csp)
		if m == nil || !strings.Contains(csp, "script-src 'self' 'nonce-") || strings.Contains(csp, "unsafe-inline") {
			t.Fatalf("%s: bad CSP %q", p, csp)
		}
		body := string(r.Raw)
		if !strings.Contains(body, `nonce="`+m[1]+`"`) {
			t.Errorf("%s: nonce not rendered into the page", p)
		}
		if inlineRe.MatchString(body) {
			t.Errorf("%s: inline script/style/handler found", p)
		}
		if strings.Contains(body, "http://") && strings.Contains(body, "cdn") {
			t.Errorf("%s: external resource", p)
		}
	}
	// different nonce per request
	a := nonceRe.FindString(call(t, "GET", "/admin-ui", nil, "").Header.Get("Content-Security-Policy"))
	b := nonceRe.FindString(call(t, "GET", "/admin-ui", nil, "").Header.Get("Content-Security-Policy"))
	if a == b {
		t.Error("nonce reused across requests")
	}
	for _, p := range []string{"/static/admin/admin.js", "/static/admin/admin.css"} {
		if r := call(t, "GET", p, nil, ""); r.Status != 200 {
			t.Errorf("%s: %d", p, r.Status)
		}
	}
}
