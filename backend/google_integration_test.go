package main

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/lingster/go-rest-template/backend/testutil"
)

const googleWebClientID = "web-client.apps.googleusercontent.com"

var (
	googleFake = &testutil.FakeGoogleVerifier{}
)

// googleToken registers fake claims and returns the token to post.
func googleToken(email, name string) (token, sub string) {
	sub = "sub-" + testutil.Email("g")
	token = "tok-" + sub
	googleFake.Add(token, services.GoogleClaims{Subject: sub, Email: email, EmailVerified: true, Name: name})
	return token, sub
}

// googleTokenHD is like googleToken but the ID token also carries the Workspace hosted-domain claim.
func googleTokenHD(email, name, hd string) (token, sub string) {
	sub = "sub-" + testutil.Email("g")
	token = "tok-" + sub
	googleFake.Add(token, services.GoogleClaims{Subject: sub, Email: email, EmailVerified: true, Name: name, HostedDomain: hd})
	return token, sub
}

func googleLogin(t *testing.T, token string) resp {
	t.Helper()
	return call(t, "POST", "/api/v1/auth/google", map[string]string{"id_token": token}, "")
}

func auditActions(t *testing.T, userID string) []string {
	t.Helper()
	page, err := container.Get().Audit.List(userID, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range page.Items {
		out = append(out, l.Action)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestGoogleProvidersEndpoint(t *testing.T) {
	r := call(t, "GET", "/api/v1/auth/providers", nil, "")
	expect(t, r, 200, "")
	g, _ := r.Body["google"].(map[string]interface{})
	if g["enabled"] != true || g["web_client_id"] != googleWebClientID {
		t.Fatalf("bad providers: %s", r.Raw)
	}
}

func TestGoogleNewUser(t *testing.T) {
	email := testutil.Email("gnew")
	tok, _ := googleToken(strings.ToUpper(email), "Grace Hopper")
	r := googleLogin(t, tok)
	expect(t, r, 200, "")
	if r.Body["mfa_required"] != false || r.str("tokens", "access_token") == "" {
		t.Fatalf("expected session: %s", r.Raw)
	}
	u := r.Body["user"].(map[string]interface{})
	ids, _ := u["identities"].([]interface{})
	if u["email"] != email || u["name"] != "Grace Hopper" || u["email_verified"] != true || u["has_password"] != false ||
		len(ids) != 1 || ids[0] != "google" {
		t.Fatalf("bad user: %s", r.Raw)
	}
	me := call(t, "GET", "/api/v1/me", nil, r.str("tokens", "access_token"))
	expect(t, me, 200, "")
	if me.Body["has_password"] != false {
		t.Fatalf("bad /me: %s", me.Raw)
	}
	acts := auditActions(t, u["id"].(string))
	if !contains(acts, "auth.google_login") || !contains(acts, "login.success") {
		t.Fatalf("missing audit entries: %v", acts)
	}

	// password login for a password-less account fails generically
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 401, "invalid_credentials")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": ""}, ""), 422, "validation_error")

	// change-password is not possible without a password; the reset flow sets one
	access := r.str("tokens", "access_token")
	expect(t, call(t, "POST", "/api/v1/me/change-password", map[string]string{"current_password": "x", "new_password": "a new password!"}, access), 409, "password_not_set")
	expect(t, call(t, "POST", "/api/v1/auth/forgot-password", map[string]string{"email": email}, ""), 202, "")
	reset := mailer.LastToken(email, "Reset")
	expect(t, call(t, "POST", "/api/v1/auth/reset-password", map[string]string{"token": reset, "password": "a new password!"}, ""), 200, "")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "a new password!"}, ""), 200, "")
	me = call(t, "GET", "/api/v1/me", nil, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "a new password!"}, "").str("tokens", "access_token"))
	if me.Body["has_password"] != true {
		t.Fatalf("has_password should now be true: %s", me.Raw)
	}
}

func TestGoogleLinkExistingVerifiedUser(t *testing.T) {
	email, access, _ := newVerifiedUser(t, "glink")
	tok, _ := googleTokenHD(strings.ToUpper(email), "Other Name", "example.com")
	r := googleLogin(t, tok)
	expect(t, r, 200, "")
	u := r.Body["user"].(map[string]interface{})
	if u["has_password"] != true || u["name"] != "Test" {
		t.Fatalf("existing account must be kept as is: %s", r.Raw)
	}
	if ids, _ := u["identities"].([]interface{}); len(ids) != 1 || ids[0] != "google" {
		t.Fatalf("identity not linked: %s", r.Raw)
	}
	if !contains(auditActions(t, u["id"].(string)), "auth.identity_linked") {
		t.Fatal("missing auth.identity_linked audit entry")
	}
	// the password still works, and the earlier session is untouched
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 200, "")
	expect(t, call(t, "GET", "/api/v1/me", nil, access), 200, "")
}

func TestGoogleLinkUnverifiedAccountDropsAttackerCredentials(t *testing.T) {
	email := testutil.Email("gpre")
	// an attacker pre-registers the victim's address with a password they know...
	expect(t, call(t, "POST", "/api/v1/auth/register", map[string]string{"email": email, "password": password, "name": "Mallory"}, ""), 202, "")
	c := container.Get()
	u, err := c.Repos.Users.GetByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	// ...and has planted a session, TOTP, recovery codes and a passkey
	attackerAccess := issueSession(t, u)
	u.TOTPEnabled, u.TOTPSecret = true, "attacker-secret"
	if err := c.Repos.Users.Update(u, "TOTPEnabled", "TOTPSecret"); err != nil {
		t.Fatal(err)
	}
	if err := c.Repos.Recovery.ReplaceForUser(u.ID, []*models.RecoveryCode{{ID: services.NewID(), UserID: u.ID, CodeHash: "h", CreatedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Repos.Passkeys.Create(&models.WebAuthnCredential{ID: services.NewID(), UserID: u.ID, Name: "evil", CredentialID: "cred-" + u.ID, CredentialJSON: "{}", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	tok, _ := googleTokenHD(email, "Victim", "example.com")
	r := googleLogin(t, tok)
	expect(t, r, 200, "")
	if r.Body["mfa_required"] != false || r.str("user", "id") != u.ID || r.Body["user"].(map[string]interface{})["email_verified"] != true {
		t.Fatalf("victim should get a plain verified session: %s", r.Raw)
	}

	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 401, "invalid_credentials")
	expect(t, call(t, "GET", "/api/v1/me", nil, attackerAccess), 401, "")
	got, err := c.Repos.Users.GetByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TOTPEnabled || got.TOTPSecret != "" || got.HasPassword() || !got.EmailVerified {
		t.Fatalf("attacker credentials survived: %+v", got)
	}
	if n, _ := c.Repos.Recovery.CountUnused(u.ID); n != 0 {
		t.Fatalf("recovery codes survived: %d", n)
	}
	if n, _ := c.Repos.Passkeys.CountByUser(u.ID); n != 0 {
		t.Fatalf("passkeys survived: %d", n)
	}
	acts := auditActions(t, u.ID)
	if !contains(acts, services.AuditUnverifiedReset) || !contains(acts, services.AuditIdentityLinked) {
		t.Fatalf("missing audit entries: %v", acts)
	}
}

// issueSession mints an access token for u as a prior (attacker) session.
func issueSession(t *testing.T, u *models.User) string {
	t.Helper()
	tokens, err := container.Get().Tokens.Issue(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tokens.AccessToken
}

func TestGoogleLinkOnlyWhenGoogleIsAuthoritativeForTheDomain(t *testing.T) {
	cases := []struct {
		name, domain, hd string
		link             bool
	}{
		{"gmail without hd", "gmail.com", "", true},
		{"googlemail without hd", "googlemail.com", "", true},
		{"workspace hd matches domain", "corp-example.com", "corp-example.com", true},
		{"workspace hd matches case-insensitively", "corp-example.com", "CORP-EXAMPLE.COM", true},
		{"other domain without hd", "example.com", "", false},
		{"hd of a different domain", "example.com", "evil.com", false},
		{"hd is a parent domain only", "mail.corp-example.com", "corp-example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			email := strings.Replace(testutil.Email("gauth"), "example.com", tc.domain, 1)
			expect(t, call(t, "POST", "/api/v1/auth/register", map[string]string{"email": email, "password": password, "name": "Local"}, ""), 202, "")
			expect(t, call(t, "POST", "/api/v1/auth/verify-email", map[string]string{"token": mailer.LastToken(email, "Verify")}, ""), 200, "")
			tok, _ := googleTokenHD(email, "Google Name", tc.hd)
			r := googleLogin(t, tok)
			u, err := container.Get().Repos.Users.GetByEmail(email)
			if err != nil {
				t.Fatal(err)
			}
			provs, _ := container.Get().Repos.Identities.ProvidersByUsers([]string{u.ID})
			ids := provs[u.ID]
			if tc.link {
				expect(t, r, 200, "")
				if len(ids) != 1 {
					t.Fatalf("identity should be linked: %d", len(ids))
				}
				return
			}
			expect(t, r, 409, "account_exists_sign_in_required")
			if !strings.Contains(r.str("error", "message"), "Sign in with your password") {
				t.Fatalf("unhelpful message: %s", r.Raw)
			}
			if len(ids) != 0 {
				t.Fatalf("identity must not be linked: %d", len(ids))
			}
			if !contains(auditActions(t, u.ID), services.AuditGoogleLinkDenied) {
				t.Fatal("missing audit entry for the denied link")
			}
			// the account is untouched and still usable with its password
			expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 200, "")
		})
	}
}

func TestGoogleUnauthoritativeDomainStillCreatesNewAccountsAndMatchesBySub(t *testing.T) {
	email := testutil.Email("gother") // example.com, no hd
	tok, sub := googleToken(email, "Fresh")
	first := googleLogin(t, tok)
	expect(t, first, 200, "")
	// once the identity exists, sign-in is by sub even though the domain is not authoritative
	googleFake.Add("again-"+sub, services.GoogleClaims{Subject: sub, Email: email, EmailVerified: true})
	second := googleLogin(t, "again-"+sub)
	expect(t, second, 200, "")
	if second.str("user", "id") != first.str("user", "id") {
		t.Fatalf("expected same account: %s", second.Raw)
	}
}

func TestGoogleLinkedLoginBySubIgnoresEmailChange(t *testing.T) {
	email := testutil.Email("gsub")
	tok, sub := googleToken(email, "Sub User")
	first := googleLogin(t, tok)
	expect(t, first, 200, "")
	id := first.str("user", "id")

	// same sub, different email at Google: still the same local account
	tok2 := googleFake.Add("tok2-"+sub, services.GoogleClaims{Subject: sub, Email: testutil.Email("gsub-new"), EmailVerified: true})
	second := googleLogin(t, tok2)
	expect(t, second, 200, "")
	if second.str("user", "id") != id {
		t.Fatalf("expected same account: %s", second.Raw)
	}
}

func TestGoogleTOTPChallenge(t *testing.T) {
	email := testutil.Email("gmfa")
	tok, _ := googleToken(email, "Mfa User")
	r := googleLogin(t, tok)
	expect(t, r, 200, "")
	access := r.str("tokens", "access_token")

	setup := call(t, "POST", "/api/v1/me/2fa/totp/setup", nil, access)
	expect(t, setup, 200, "")
	secret := setup.str("secret")
	clock := testutil.NewFakeClock()
	mfa := container.Get().MFA
	mfa.Now = clock.Now
	defer func() { mfa.Now = time.Now }()
	code, _ := totp.GenerateCode(secret, clock.Now())
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/enable", map[string]string{"code": code}, access), 200, "")

	// Google alone must now yield only a challenge
	ch := googleLogin(t, tok)
	expect(t, ch, 200, "")
	if ch.Body["mfa_required"] != true || ch.str("mfa_token") == "" || ch.Body["tokens"] != nil {
		t.Fatalf("expected mfa challenge: %s", ch.Raw)
	}
	mfaTok := ch.str("mfa_token")
	expect(t, call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": "000000"}, ""), 401, "invalid_mfa_code")
	clock.Advance(30 * time.Second)
	code, _ = totp.GenerateCode(secret, clock.Now())
	ok := call(t, "POST", "/api/v1/auth/login/2fa", map[string]string{"mfa_token": mfaTok, "code": code}, "")
	expect(t, ok, 200, "")
	access2 := ok.str("tokens", "access_token")

	// a password-less user disables TOTP with a code alone
	clock.Advance(30 * time.Second)
	code, _ = totp.GenerateCode(secret, clock.Now())
	expect(t, call(t, "POST", "/api/v1/me/2fa/totp/disable", map[string]string{"code": code}, access2), 200, "")
}

func TestGoogleRejections(t *testing.T) {
	// unknown / invalid token
	expect(t, googleLogin(t, "garbage"), 401, "invalid_token")
	expect(t, call(t, "POST", "/api/v1/auth/google", map[string]string{}, ""), 422, "validation_error")

	// unverified email is rejected
	googleFake.Add("unverified", services.GoogleClaims{Subject: "s-unv", Email: testutil.Email("unv"), EmailVerified: false})
	expect(t, googleLogin(t, "unverified"), 401, "invalid_token")

	// disabled accounts cannot sign in via Google
	email := testutil.Email("gdis")
	tok, _ := googleToken(email, "Disabled")
	r := googleLogin(t, tok)
	expect(t, r, 200, "")
	c := container.Get()
	u, err := c.Repos.Users.GetByID(r.str("user", "id"))
	if err != nil {
		t.Fatal(err)
	}
	u.IsActive = false
	if err := c.Repos.Users.Update(u, "IsActive"); err != nil {
		t.Fatal(err)
	}
	expect(t, googleLogin(t, tok), 403, "account_disabled")
}

func TestGoogleIdentitiesPurgedOnUserDeletion(t *testing.T) {
	tok, sub := googleToken(testutil.Email("gdel"), "Doomed")
	r := googleLogin(t, tok)
	expect(t, r, 200, "")
	admin := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": adminEmail, "password": adminPassword}, "")
	expect(t, call(t, "DELETE", "/api/v1/admin/users/"+r.str("user", "id"), nil, admin.str("tokens", "access_token")), 204, "")
	if _, err := container.Get().Repos.Identities.GetByProviderSubject("google", sub); err == nil {
		t.Fatal("identity should have been deleted with the user")
	}
}
