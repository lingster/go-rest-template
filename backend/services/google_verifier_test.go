package services_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/lingster/go-rest-template/backend/testutil"
)

const testAud = "web-client.apps.googleusercontent.com"

// fakeJWKS serves a mutable key set and counts downloads.
type fakeJWKS struct {
	mu           sync.Mutex
	keys         map[string]*rsa.PublicKey
	hits         int
	cacheControl string
	srv          *httptest.Server
}

func newFakeJWKS(t *testing.T) *fakeJWKS {
	t.Helper()
	f := &fakeJWKS{keys: map[string]*rsa.PublicKey{}, cacheControl: "public, max-age=3600"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.hits++
		type jwk struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		var set struct {
			Keys []jwk `json:"keys"`
		}
		for kid, k := range f.keys {
			set.Keys = append(set.Keys, jwk{Kid: kid, Kty: "RSA", Alg: "RS256",
				N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
				E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())})
		}
		w.Header().Set("Cache-Control", f.cacheControl)
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeJWKS) addKey(t *testing.T, kid string) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.keys[kid] = &k.PublicKey
	f.mu.Unlock()
	return k
}

func (f *fakeJWKS) hitCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits
}

func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func goodClaims(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testAud, "sub": "1234567890",
		"email": "ada@example.com", "email_verified": true, "name": "Ada",
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

func TestGoogleVerifier(t *testing.T) {
	jwks := newFakeJWKS(t)
	key := jwks.addKey(t, "k1")
	now := time.Now()
	v := services.NewJWKSGoogleVerifier([]string{testAud, "ios-client"}, jwks.srv.URL, nil)
	v.Now = func() time.Time { return now }
	ctx := context.Background()

	mutate := func(f func(c jwt.MapClaims)) string {
		c := goodClaims(now)
		f(c)
		return sign(t, key, "k1", c)
	}

	t.Run("valid", func(t *testing.T) {
		c, err := v.Verify(ctx, sign(t, key, "k1", goodClaims(now)))
		if err != nil {
			t.Fatal(err)
		}
		if c.Subject != "1234567890" || c.Email != "ada@example.com" || !c.EmailVerified || c.Name != "Ada" {
			t.Fatalf("bad claims: %+v", c)
		}
	})
	t.Run("issuer without scheme and second audience", func(t *testing.T) {
		tok := mutate(func(c jwt.MapClaims) { c["iss"] = "accounts.google.com"; c["aud"] = "ios-client" })
		if _, err := v.Verify(ctx, tok); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("string email_verified accepted", func(t *testing.T) {
		tok := mutate(func(c jwt.MapClaims) { c["email_verified"] = "true" })
		if _, err := v.Verify(ctx, tok); err != nil {
			t.Fatal(err)
		}
	})

	bad := map[string]string{
		"wrong audience":   mutate(func(c jwt.MapClaims) { c["aud"] = "someone-elses-client" }),
		"wrong issuer":     mutate(func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" }),
		"expired":          mutate(func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Hour).Unix() }),
		"missing exp":      mutate(func(c jwt.MapClaims) { delete(c, "exp") }),
		"issued in future": mutate(func(c jwt.MapClaims) { c["iat"] = now.Add(time.Hour).Unix() }),
		"unverified email": mutate(func(c jwt.MapClaims) { c["email_verified"] = false }),
		"missing email":    mutate(func(c jwt.MapClaims) { delete(c, "email") }),
		"missing subject":  mutate(func(c jwt.MapClaims) { delete(c, "sub") }),
		"garbage":          "not.a.jwt",
	}
	for name, tok := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(ctx, tok); !errors.Is(err, services.ErrInvalidGoogleToken) {
				t.Fatalf("expected ErrInvalidGoogleToken, got %v", err)
			}
		})
	}

	t.Run("small clock skew is tolerated", func(t *testing.T) {
		tok := mutate(func(c jwt.MapClaims) { c["exp"] = now.Add(-10 * time.Second).Unix() })
		if _, err := v.Verify(ctx, tok); err != nil {
			t.Fatalf("leeway not applied: %v", err)
		}
	})
	t.Run("signature from a different key", func(t *testing.T) {
		other, _ := rsa.GenerateKey(rand.Reader, 2048)
		if _, err := v.Verify(ctx, sign(t, other, "k1", goodClaims(now))); !errors.Is(err, services.ErrInvalidGoogleToken) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("alg none and HS256 are refused", func(t *testing.T) {
		none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, goodClaims(now)).SignedString(jwt.UnsafeAllowNoneSignatureType)
		hs := jwt.NewWithClaims(jwt.SigningMethodHS256, goodClaims(now))
		hs.Header["kid"] = "k1"
		hsTok, _ := hs.SignedString([]byte("secret"))
		for _, tok := range []string{none, hsTok} {
			if _, err := v.Verify(ctx, tok); !errors.Is(err, services.ErrInvalidGoogleToken) {
				t.Fatalf("got %v", err)
			}
		}
	})
}

func TestGoogleVerifierKeyCache(t *testing.T) {
	jwks := newFakeJWKS(t)
	k1 := jwks.addKey(t, "k1")
	now := time.Now()
	v := services.NewJWKSGoogleVerifier([]string{testAud}, jwks.srv.URL, nil)
	clock := &now
	v.Now = func() time.Time { return *clock }
	v.MinRefetch = 0
	ctx := context.Background()

	if _, err := v.Verify(ctx, sign(t, k1, "k1", goodClaims(now))); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, sign(t, k1, "k1", goodClaims(now))); err != nil {
		t.Fatal(err)
	}
	if jwks.hitCount() != 1 {
		t.Fatalf("keys should be cached, got %d downloads", jwks.hitCount())
	}

	// Google rotates keys: an unknown kid triggers a refresh and then verifies.
	k2 := jwks.addKey(t, "k2")
	if _, err := v.Verify(ctx, sign(t, k2, "k2", goodClaims(now))); err != nil {
		t.Fatalf("unknown kid should refresh keys: %v", err)
	}
	if jwks.hitCount() != 2 {
		t.Fatalf("expected a refresh, got %d downloads", jwks.hitCount())
	}

	// a kid that does not exist even after refresh is rejected
	k3, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := v.Verify(ctx, sign(t, k3, "ghost", goodClaims(now))); !errors.Is(err, services.ErrInvalidGoogleToken) {
		t.Fatalf("got %v", err)
	}

	// the cache honours Cache-Control max-age
	before := jwks.hitCount()
	later := now.Add(2 * time.Hour)
	clock = &later
	claims := goodClaims(later)
	if _, err := v.Verify(ctx, sign(t, k1, "k1", claims)); err != nil {
		t.Fatal(err)
	}
	if jwks.hitCount() != before+1 {
		t.Fatalf("expired cache should refresh: %d -> %d", before, jwks.hitCount())
	}
}

func TestGoogleVerifierThrottlesUnknownKidRefresh(t *testing.T) {
	jwks := newFakeJWKS(t)
	k1 := jwks.addKey(t, "k1")
	now := time.Now()
	v := services.NewJWKSGoogleVerifier([]string{testAud}, jwks.srv.URL, nil)
	v.Now = func() time.Time { return now }
	ctx := context.Background()
	_, _ = v.Verify(ctx, sign(t, k1, "k1", goodClaims(now)))
	for i := 0; i < 5; i++ {
		_, _ = v.Verify(ctx, sign(t, k1, "junk", goodClaims(now)))
	}
	if jwks.hitCount() != 1 {
		t.Fatalf("junk kids must not hammer the JWKS endpoint, got %d downloads", jwks.hitCount())
	}
}

func TestGoogleServiceDisabledWithoutClientIDs(t *testing.T) {
	c, _ := testutil.NewContainer(t) // test config has no GOOGLE_CLIENT_IDS
	if c.Auth.Providers().Google.Enabled {
		t.Fatal("providers must report google disabled")
	}
	_, err := c.Auth.LoginGoogle(context.Background(), "anything")
	if e := apperr.As(err); e.Status != http.StatusNotFound || e.Code != "google_signin_disabled" {
		t.Fatalf("got %v", err)
	}
}

func TestGoogleConfigParsing(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_IDS", " web.apps.googleusercontent.com , ios.apps.googleusercontent.com,,")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GoogleClientIDs) != 2 || cfg.GoogleClientIDs[0] != "web.apps.googleusercontent.com" || !cfg.GoogleEnabled() {
		t.Fatalf("got %v", cfg.GoogleClientIDs)
	}
}
