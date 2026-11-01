package services

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GoogleJWKSURL is Google's public key set for ID token signatures.
const GoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

const (
	googleLeeway        = 30 * time.Second
	defaultKeyTTL       = time.Hour
	minKeyTTL           = time.Minute
	maxKeyTTL           = 24 * time.Hour
	defaultMinRefetch   = time.Minute // floor between forced refreshes caused by unknown kids
	maxJWKSResponseSize = 1 << 20
)

// ErrInvalidGoogleToken is returned for any token that fails verification (signature, claims, expiry...).
var ErrInvalidGoogleToken = errors.New("invalid google id token")

// GoogleClaims are the verified identity claims of a Google ID token.
type GoogleClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// GoogleTokenVerifier verifies Google ID tokens. It is an interface so tests can inject a fake.
type GoogleTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*GoogleClaims, error)
}

// JWKSGoogleVerifier verifies Google ID tokens offline against Google's cached JWKS.
type JWKSGoogleVerifier struct {
	audiences map[string]bool
	jwksURL   string
	client    *http.Client
	// Now is the clock (overridable in tests).
	Now func() time.Time
	// MinRefetch limits how often an unknown kid may force a JWKS download.
	MinRefetch time.Duration

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	fetchedAt time.Time
}

// NewJWKSGoogleVerifier accepts tokens issued for any of the given client IDs. jwksURL and client
// default to Google's endpoint and a 5s-timeout HTTP client when empty/nil.
func NewJWKSGoogleVerifier(audiences []string, jwksURL string, client *http.Client) *JWKSGoogleVerifier {
	aud := make(map[string]bool, len(audiences))
	for _, a := range audiences {
		aud[a] = true
	}
	if jwksURL == "" {
		jwksURL = GoogleJWKSURL
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &JWKSGoogleVerifier{audiences: aud, jwksURL: jwksURL, client: client, Now: time.Now, MinRefetch: defaultMinRefetch}
}

type googleIDClaims struct {
	jwt.RegisteredClaims
	Email         string   `json:"email"`
	EmailVerified flexBool `json:"email_verified"`
	Name          string   `json:"name"`
}

// flexBool accepts JSON true/false as well as the strings "true"/"false" (Google has emitted both).
type flexBool bool

func (b *flexBool) UnmarshalJSON(raw []byte) error {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	switch t := v.(type) {
	case bool:
		*b = flexBool(t)
	case string:
		*b = t == "true"
	}
	return nil
}

var googleIssuers = map[string]bool{"accounts.google.com": true, "https://accounts.google.com": true}

// Verify checks signature (RS256 only), issuer, audience, exp/iat (with leeway) and returns the claims.
func (v *JWKSGoogleVerifier) Verify(ctx context.Context, idToken string) (*GoogleClaims, error) {
	claims := &googleIDClaims{}
	_, err := jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		return v.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithLeeway(googleLeeway),
		jwt.WithTimeFunc(v.Now),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidGoogleToken, err)
	}
	if !googleIssuers[claims.Issuer] {
		return nil, fmt.Errorf("%w: unexpected issuer", ErrInvalidGoogleToken)
	}
	if !v.audienceOK(claims.Audience) {
		return nil, fmt.Errorf("%w: unexpected audience", ErrInvalidGoogleToken)
	}
	if !bool(claims.EmailVerified) || claims.Email == "" {
		return nil, fmt.Errorf("%w: email not verified", ErrInvalidGoogleToken)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject", ErrInvalidGoogleToken)
	}
	return &GoogleClaims{Subject: claims.Subject, Email: claims.Email, EmailVerified: bool(claims.EmailVerified), Name: claims.Name}, nil
}

func (v *JWKSGoogleVerifier) audienceOK(aud jwt.ClaimStrings) bool {
	for _, a := range aud {
		if v.audiences[a] {
			return true
		}
	}
	return false
}

// key returns the public key for kid, refreshing the cache when it is stale or the kid is unknown.
func (v *JWKSGoogleVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		return nil, errors.New("missing kid")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.Now()
	stale := v.keys == nil || !now.Before(v.expiresAt)
	if k, ok := v.keys[kid]; ok && !stale {
		return k, nil
	}
	// Refresh when stale, or when the kid is unknown and the last fetch is old enough (DoS guard).
	if stale || now.Sub(v.fetchedAt) >= v.MinRefetch {
		if err := v.refreshLocked(ctx, now); err != nil && v.keys == nil {
			return nil, err
		}
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown kid %q", kid)
}

func (v *JWKSGoogleVerifier) refreshLocked(ctx context.Context, now time.Time) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJWKSResponseSize)).Decode(&set); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		if pk, err := rsaKey(k.N, k.E); err == nil {
			keys[k.Kid] = pk
		}
	}
	if len(keys) == 0 {
		return errors.New("jwks contains no usable RSA keys")
	}
	v.keys, v.fetchedAt, v.expiresAt = keys, now, now.Add(keyTTL(resp.Header.Get("Cache-Control")))
	return nil
}

var maxAgeRe = regexp.MustCompile(`(?i)max-age=(\d+)`)

// keyTTL derives the cache lifetime from Cache-Control max-age, clamped to a sane range.
func keyTTL(cacheControl string) time.Duration {
	ttl := defaultKeyTTL
	if m := maxAgeRe.FindStringSubmatch(cacheControl); m != nil {
		if secs, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			ttl = time.Duration(secs) * time.Second
		}
	}
	return min(max(ttl, minKeyTTL), maxKeyTTL)
}

func rsaKey(n, e string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	exp := new(big.Int).SetBytes(eb)
	if !exp.IsInt64() || exp.Int64() < 3 || len(nb) < 128 {
		return nil, errors.New("invalid rsa key")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp.Int64())}, nil
}
