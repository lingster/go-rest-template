package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/lingster/go-rest-template/backend/testutil"
)

func newUser(t *testing.T, c *container.Container) *models.User {
	t.Helper()
	now := time.Now().UTC()
	u := &models.User{ID: services.NewID(), Email: testutil.Email("svc"), Name: "T", PasswordHash: "x",
		Role: models.RoleUser, IsActive: true, EmailVerified: true, CreatedAt: now, UpdatedAt: now}
	if err := c.Repos.Users.Create(u); err != nil {
		t.Fatal(err)
	}
	return u
}

func code(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	return apperr.As(err).Code
}

func TestPasswordHasher(t *testing.T) {
	h := services.BcryptHasher{Cost: 4}
	long := strings.Repeat("a", 100) // > bcrypt's 72 byte limit
	hash, err := h.Hash(long)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Verify(hash, long) {
		t.Error("expected password to verify")
	}
	if h.Verify(hash, long[:99]+"b") || h.Verify(hash, "wrong") {
		t.Error("wrong passwords must not verify")
	}
	if hash == long {
		t.Error("hash must differ from password")
	}
}

func TestSecretBox(t *testing.T) {
	box, _ := services.NewSecretBox("k")
	sealed, _ := box.Seal("JBSWY3DPEHPK3PXP")
	if sealed == "JBSWY3DPEHPK3PXP" {
		t.Fatal("not encrypted")
	}
	plain, err := box.Open(sealed)
	if err != nil || plain != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("roundtrip failed: %q %v", plain, err)
	}
	other, _ := services.NewSecretBox("other")
	if _, err := other.Open(sealed); err == nil {
		t.Error("different key must fail")
	}
}

func TestAccessTokenValidation(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)
	tok, _ := c.Tokens.IssueAccess(u)
	claims, err := c.Tokens.ParseAccess(tok)
	if err != nil || claims.Subject != u.ID || claims.Role != "user" || claims.Typ != "access" {
		t.Fatalf("bad claims %+v %v", claims, err)
	}
	// an MFA token must not be usable as an access token
	mfa, _ := c.Tokens.IssueMFAToken(u.ID)
	if _, err := c.Tokens.ParseAccess(mfa); code(t, err) != apperr.CodeInvalidToken {
		t.Errorf("mfa token accepted as access token: %v", err)
	}
	if _, err := c.Tokens.ParseAccess(tok + "x"); code(t, err) != apperr.CodeInvalidToken {
		t.Errorf("tampered token: %v", err)
	}
	// expiry
	c.Tokens.Now = func() time.Time { return time.Now().Add(time.Hour) }
	defer func() { c.Tokens.Now = time.Now }()
	if _, err := c.Tokens.ParseAccess(tok); code(t, err) != apperr.CodeTokenExpired {
		t.Errorf("expected token_expired, got %v", err)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)

	first, err := c.Tokens.Issue(u, "")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := c.Tokens.Consume(first.RefreshToken)
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}
	second, err := c.Tokens.Issue(u, rt.FamilyID)
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	// Replaying the old token is theft: rejected and the whole family is revoked.
	if _, err := c.Tokens.Consume(first.RefreshToken); code(t, err) != apperr.CodeInvalidToken {
		t.Fatalf("reuse must be rejected, got %v", err)
	}
	if _, err := c.Tokens.Consume(second.RefreshToken); code(t, err) != apperr.CodeInvalidToken {
		t.Fatalf("family must be revoked after reuse, got %v", err)
	}
	// Other families are unaffected.
	other, _ := c.Tokens.Issue(u, "")
	if _, err := c.Tokens.Consume(other.RefreshToken); err != nil {
		t.Errorf("unrelated family affected: %v", err)
	}
	if _, err := c.Tokens.Consume("does-not-exist"); code(t, err) != apperr.CodeInvalidToken {
		t.Errorf("unknown token: %v", err)
	}
}

func TestRefreshExpiryAndRevokeAll(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)
	tk, _ := c.Tokens.Issue(u, "")
	c.Tokens.Now = func() time.Time { return time.Now().Add(31 * 24 * time.Hour) }
	_, err := c.Tokens.Consume(tk.RefreshToken)
	c.Tokens.Now = time.Now
	if code(t, err) != apperr.CodeTokenExpired {
		t.Errorf("expected token_expired, got %v", err)
	}
	tk2, _ := c.Tokens.Issue(u, "")
	if err := c.Tokens.RevokeAll(u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tokens.Consume(tk2.RefreshToken); err == nil {
		t.Error("revoked token accepted")
	}
}

func TestTOTPAndRecoveryCodes(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	ctx := context.Background()
	hash, _ := services.BcryptHasher{Cost: 4}.Hash("password123")
	u := newUser(t, c)
	u.PasswordHash = hash
	_ = c.Repos.Users.Update(u, "PasswordHash")
	clock := testutil.NewFakeClock()
	c.MFA.Now = clock.Now

	secret, url, err := c.MFA.Setup(u)
	if err != nil || secret == "" || !strings.HasPrefix(url, "otpauth://totp/") {
		t.Fatalf("setup: %q %q %v", secret, url, err)
	}
	if u.TOTPSecret == secret {
		t.Error("secret must be stored encrypted")
	}
	if _, err := c.MFA.Enable(ctx, u, "000000"); code(t, err) != apperr.CodeInvalidMFACode {
		t.Fatalf("wrong code accepted: %v", err)
	}
	prev, _ := totp.GenerateCode(secret, clock.Now())
	clock.Advance(30 * time.Second)
	good, _ := totp.GenerateCode(secret, clock.Now())
	codes, err := c.MFA.Enable(ctx, u, prev)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if len(codes) != 10 || !u.TOTPEnabled {
		t.Fatalf("expected 10 codes and enabled, got %d", len(codes))
	}

	// Replay protection: the enabling step cannot be reused, a newer step works once.
	if ok, _ := c.MFA.VerifyCode(u, prev); ok {
		t.Error("TOTP step replayed after enable")
	}
	if ok, _ := c.MFA.VerifyCode(u, good); !ok {
		t.Error("valid TOTP rejected")
	}
	if ok, _ := c.MFA.VerifyCode(u, good); ok {
		t.Error("TOTP code accepted twice")
	}
	fresh, _ := c.Repos.Users.GetByID(u.ID)
	if fresh.TOTPLastStep != u.TOTPLastStep || u.TOTPLastStep == 0 {
		t.Errorf("last step not persisted: db=%d mem=%d", fresh.TOTPLastStep, u.TOTPLastStep)
	}
	if ok, _ := c.MFA.VerifyCode(u, "123456"); ok && good != "123456" {
		t.Error("invalid TOTP accepted")
	}

	// Recovery codes are single use, case/dash-insensitive.
	rc := codes[0]
	if ok, _ := c.MFA.VerifyCode(u, strings.ToUpper(rc)); !ok {
		t.Error("recovery code rejected")
	}
	if ok, _ := c.MFA.VerifyCode(u, rc); ok {
		t.Error("recovery code reused")
	}
	if n, _ := c.Repos.Recovery.CountUnused(u.ID); n != 9 {
		t.Errorf("expected 9 unused codes, got %d", n)
	}

	// Regeneration invalidates old codes.
	newCodes, err := c.MFA.RegenerateRecoveryCodes(ctx, u, codes[1])
	if err != nil || len(newCodes) != 10 {
		t.Fatalf("regenerate: %v", err)
	}
	if ok, _ := c.MFA.VerifyCode(u, codes[2]); ok {
		t.Error("old code valid after regeneration")
	}

	// Disable needs password + code.
	if err := c.MFA.Disable(ctx, u, "bad", good); code(t, err) != apperr.CodeInvalidCredentials {
		t.Errorf("disable with bad password: %v", err)
	}
	if err := c.MFA.Disable(ctx, u, "password123", newCodes[0]); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if u.TOTPEnabled || u.TOTPSecret != "" {
		t.Error("totp not cleared")
	}
	if n, _ := c.Repos.Recovery.CountUnused(u.ID); n != 0 {
		t.Error("recovery codes not deleted")
	}
}

func TestOneTimeTokens(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)
	ott := services.NewOneTimeTokenService(c.Repos.OneTime)
	raw, err := ott.Issue(u.ID, models.PurposeResetPassword, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ott.Consume(raw, models.PurposeVerifyEmail); err == nil {
		t.Error("wrong purpose accepted")
	}
	if id, err := ott.Consume(raw, models.PurposeResetPassword); err != nil || id != u.ID {
		t.Fatalf("consume: %v", err)
	}
	if _, err := ott.Consume(raw, models.PurposeResetPassword); err == nil {
		t.Error("token reused")
	}
	exp, _ := ott.Issue(u.ID, models.PurposeVerifyEmail, -time.Minute)
	if _, err := ott.Consume(exp, models.PurposeVerifyEmail); err == nil {
		t.Error("expired token accepted")
	}
}
