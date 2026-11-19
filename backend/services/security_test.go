package services_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/lingster/go-rest-template/backend/testutil"
)

func TestRefreshRejectsRotationThatRacedRevokeAll(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	ctx := context.Background()
	u := newUser(t, c)

	first, err := c.Tokens.Issue(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := c.Tokens.Consume(first.RefreshToken) // the rotation consumed the token...
	if err != nil {
		t.Fatal(err)
	}
	stale := *u // ...and loaded the user before the revoke-all ran
	if err := c.Tokens.RevokeAll(u); err != nil {
		t.Fatal(err)
	}
	raced, err := c.Tokens.Issue(&stale, rt) // so it issues a new row that RevokeAll never saw
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Auth.Refresh(ctx, raced.RefreshToken); code(t, err) != apperr.CodeInvalidToken {
		t.Fatalf("a refresh token issued across a revoke-all must be rejected, got %v", err)
	}
}

func TestRefreshRejectsTokenFromOlderSessionVersionAndRevokesFamily(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	ctx := context.Background()
	u := newUser(t, c)

	old, _ := c.Tokens.Issue(u, nil)
	if _, err := c.Repos.Users.BumpTokenVersion(u.ID); err != nil { // version moved, row not revoked
		t.Fatal(err)
	}
	if _, err := c.Auth.Refresh(ctx, old.RefreshToken); code(t, err) != apperr.CodeInvalidToken {
		t.Fatalf("stale-version token accepted: %v", err)
	}

	// A session started after the bump refreshes normally and carries the current version forward.
	fresh, _ := c.Repos.Users.GetByID(u.ID)
	tk, _ := c.Tokens.Issue(fresh, nil)
	next, err := c.Auth.Refresh(ctx, tk.RefreshToken)
	if err != nil {
		t.Fatalf("current-version refresh failed: %v", err)
	}
	if _, err := c.Auth.Refresh(ctx, next.RefreshToken); err != nil {
		t.Fatalf("rotated token must keep the version: %v", err)
	}
}

func TestMFATokenIsBoundToSessionVersion(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)
	tok, err := c.Tokens.IssueMFAToken(u)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := c.Tokens.ParseMFAToken(tok)
	if err != nil || claims.Subject != u.ID || claims.Ver != u.TokenVersion {
		t.Fatalf("bad claims %+v %v", claims, err)
	}
	if err := c.Tokens.RevokeAll(u); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Auth.LoginMFA(context.Background(), &dto.MFALoginRequest{MFAToken: tok, Code: "000000"}); code(t, err) != apperr.CodeInvalidToken {
		t.Fatalf("challenge from before the revoke accepted: %v", err)
	}
}

func ctxFrom(ip string) context.Context {
	return services.WithMeta(context.Background(), services.RequestMeta{IP: ip})
}

func TestLoginThrottleAccountBucketBoundsDistributedGuessing(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)
	hash, _ := services.BcryptHasher{Cost: 4}.Hash("right password")
	u.PasswordHash = hash
	if err := c.Repos.Users.Update(u, "PasswordHash"); err != nil {
		t.Fatal(err)
	}
	login := func(ip, pw string) error {
		_, err := c.Auth.Login(ctxFrom(ip), &dto.LoginRequest{Email: u.Email, Password: pw})
		return err
	}
	// Every guess comes from a different client, so no per-client bucket fills up.
	for i := 0; i < services.LoginAccountFailLimit; i++ {
		if err := login(fmt.Sprintf("10.1.%d.%d", i/200, i%200+1), "wrong"); code(t, err) != apperr.CodeInvalidCredentials {
			t.Fatalf("guess %d: %v", i, err)
		}
	}
	if err := login("203.0.113.250", "right password"); code(t, err) != apperr.CodeRateLimited {
		t.Fatalf("account-wide ceiling must hold for any client, got %v", err)
	}
}

func TestLoginSuccessDoesNotConsumeBudget(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	u := newUser(t, c)
	hash, _ := services.BcryptHasher{Cost: 4}.Hash("right password")
	u.PasswordHash = hash
	if err := c.Repos.Users.Update(u, "PasswordHash"); err != nil {
		t.Fatal(err)
	}
	ctx := ctxFrom("198.51.100.7")
	for i := 0; i < 3*services.LoginClientFailLimit; i++ {
		if _, err := c.Auth.Login(ctx, &dto.LoginRequest{Email: u.Email, Password: "right password"}); err != nil {
			t.Fatalf("login %d: %v", i, err)
		}
	}
}
