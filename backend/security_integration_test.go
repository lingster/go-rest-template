package main

import (
	"testing"
	"time"

	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/pquerna/otp/totp"
)

// staleAccessToken returns an access token for the user whose last sign-in was an hour ago, as a
// session kept alive by refresh-token rotation would carry.
func staleAccessToken(t *testing.T, email string) string {
	t.Helper()
	c := container.Get()
	u, err := c.Repos.Users.GetByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := c.Tokens.IssueAccess(u, time.Now().Add(-time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestAddingCredentialsRequiresRecentSignIn(t *testing.T) {
	email, fresh, _ := newVerifiedUser(t, "stepup")
	stale := staleAccessToken(t, email)

	// A stale (refreshed) session can still read the profile...
	expect(t, call(t, "GET", "/api/v1/me", nil, stale), 200, "")
	// ...but cannot plant a passkey or TOTP that would survive a password reset.
	expect(t, call(t, "POST", "/api/v1/me/passkeys/register/begin", nil, stale), 403, "reauthentication_required")
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/setup", nil, stale), 403, "reauthentication_required")

	expect(t, call(t, "POST", "/api/v1/me/passkeys/register/begin", nil, fresh), 200, "")
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/setup", nil, fresh), 200, "")
}

func TestRefreshKeepsOriginalSignInTime(t *testing.T) {
	_, access, refresh := newVerifiedUser(t, "authtime")
	tokens := container.Get().Tokens
	before, err := tokens.ParseAccess(access)
	if err != nil {
		t.Fatal(err)
	}
	r := call(t, "POST", "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "")
	expect(t, r, 200, "")
	after, err := tokens.ParseAccess(r.str("access_token"))
	if err != nil {
		t.Fatal(err)
	}
	if before.AuthTime == 0 || after.AuthTime != before.AuthTime {
		t.Fatalf("auth_time must survive rotation: before %d after %d", before.AuthTime, after.AuthTime)
	}
}

func TestPasswordResetRevokesAccessTokens(t *testing.T) {
	email, access, _ := newVerifiedUser(t, "resetrevoke")
	expect(t, call(t, "POST", "/api/v1/auth/forgot-password", map[string]string{"email": email}, ""), 202, "")
	tok := mailer.LastToken(email, "Reset")
	expect(t, call(t, "POST", "/api/v1/auth/reset-password", map[string]string{"token": tok, "password": "a whole new password"}, ""), 200, "")
	expect(t, call(t, "GET", "/api/v1/me", nil, access), 401, "unauthorized")
}

func TestMFABruteForceIsLimitedPerAccount(t *testing.T) {
	email, access, _ := newVerifiedUser(t, "mfabrute")
	setup := call(t, "POST", "/api/v1/me/2fa/totp/setup", nil, access)
	expect(t, setup, 200, "")
	enableAt := time.Now().Add(-30 * time.Second) // previous step: still inside the accepted skew
	code, _ := totp.GenerateCode(setup.str("secret"), enableAt)
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/enable", map[string]string{"code": code}, access), 200, "")

	ch := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	mfaTok := ch.str("mfa_token")
	for i := 0; i < services.FactorAttemptLimit; i++ {
		expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": "000000"}, ""), 401, "invalid_mfa_code")
	}
	// Further guesses are refused even with a fresh challenge (and, by construction, from any IP).
	ch = call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	good, _ := totp.GenerateCode(setup.str("secret"), time.Now())
	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": ch.str("mfa_token"), "code": good}, ""), 429, "rate_limited")
}

func TestCurrentPasswordGuessingIsLimited(t *testing.T) {
	_, access, _ := newVerifiedUser(t, "pwbrute")
	body := map[string]string{"current_password": "guess", "new_password": "a brand new one"}
	for i := 0; i < services.FactorAttemptLimit; i++ {
		expect(t, call(t, "POST", "/api/v1/me/change-password", body, access), 401, "invalid_credentials")
	}
	body["current_password"] = password
	expect(t, call(t, "POST", "/api/v1/me/change-password", body, access), 429, "rate_limited")
}

func TestForgotPasswordMailIsThrottledPerAddress(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "mailflood")
	before := mailer.Count(email)
	for i := 0; i < services.MailLimit+3; i++ {
		expect(t, call(t, "POST", "/api/v1/auth/forgot-password", map[string]string{"email": email}, ""), 202, "")
	}
	if got := mailer.Count(email) - before; got != services.MailLimit {
		t.Fatalf("expected %d reset mails, got %d", services.MailLimit, got)
	}
}

func TestPasskeyLoginRequiresUserVerificationAndNoAllowList(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "pkuv")
	r := call(t, "POST", "/api/v1/auth/passkey/login/begin", map[string]string{"email": email}, "")
	expect(t, r, 200, "")
	pk := r.Body["options"].(map[string]interface{})["publicKey"].(map[string]interface{})
	if _, ok := pk["allowCredentials"]; ok {
		t.Fatalf("allowCredentials leaks account existence: %s", r.Raw)
	}
	if pk["userVerification"] != "required" {
		t.Fatalf("user verification must be required: %s", r.Raw)
	}
}
