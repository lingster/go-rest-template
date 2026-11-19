package services

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// JWT "typ" claim values.
const (
	TypAccess = "access"
	TypMFA    = "mfa"
)

// MFATokenTTL is the lifetime of the MFA challenge token.
const MFATokenTTL = 5 * time.Minute

// Claims are our JWT claims.
type Claims struct {
	Role string `json:"role,omitempty"`
	Typ  string `json:"typ"`
	// Ver must match users.token_version, so revoking sessions also kills outstanding access tokens.
	Ver int64 `json:"ver,omitempty"`
	// AuthTime (unix seconds) is when the user last presented a credential, used for step-up checks.
	AuthTime int64 `json:"auth_time,omitempty"`
	jwt.RegisteredClaims
}

// TokenService issues/validates JWTs and manages rotating refresh tokens.
type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	refresh    repositories.RefreshTokenRepository
	users      repositories.UserRepository
	audit      *AuditService
	Now        func() time.Time
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration,
	refresh repositories.RefreshTokenRepository, users repositories.UserRepository, audit *AuditService) *TokenService {
	return &TokenService{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL,
		refresh: refresh, users: users, audit: audit, Now: time.Now}
}

// AuthenticatedWithin reports whether the user proved a credential no longer than maxAge before now.
// A non-positive maxAge disables the check.
func (c *Claims) AuthenticatedWithin(maxAge time.Duration, now time.Time) bool {
	return maxAge <= 0 || (c.AuthTime > 0 && now.Sub(time.Unix(c.AuthTime, 0)) <= maxAge)
}

func (s *TokenService) sign(c Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}

func (s *TokenService) claims(sub, typ, role string, ttl time.Duration) Claims {
	now := s.Now().UTC()
	return Claims{Role: role, Typ: typ, RegisteredClaims: jwt.RegisteredClaims{
		Subject: sub, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl)), ID: NewID(),
	}}
}

// IssueAccess creates a signed access token for the user; authTime is when they last proved a credential.
func (s *TokenService) IssueAccess(u *models.User, authTime int64) (string, error) {
	c := s.claims(u.ID, TypAccess, u.Role, s.accessTTL)
	c.Ver, c.AuthTime = u.TokenVersion, authTime
	return s.sign(c)
}

// IssueMFAToken creates the short-lived token handed out after a correct password. It carries the
// user's session version, so revoking sessions (password reset, admin revoke) also kills challenges.
func (s *TokenService) IssueMFAToken(u *models.User) (string, error) {
	c := s.claims(u.ID, TypMFA, "", MFATokenTTL)
	c.Ver = u.TokenVersion
	return s.sign(c)
}

func (s *TokenService) parse(tok, wantTyp string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(tok, c, func(*jwt.Token) (interface{}, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithTimeFunc(s.Now))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperr.TokenExpired()
		}
		return nil, apperr.InvalidToken("Invalid token")
	}
	if c.Typ != wantTyp || c.Subject == "" {
		return nil, apperr.InvalidToken("Invalid token type")
	}
	return c, nil
}

// ParseAccess validates an access token.
func (s *TokenService) ParseAccess(tok string) (*Claims, error) { return s.parse(tok, TypAccess) }

// ParseMFAToken validates an MFA challenge token and returns its claims (Subject is the user id, Ver
// the session version it was issued for).
func (s *TokenService) ParseMFAToken(tok string) (*Claims, error) { return s.parse(tok, TypMFA) }

// Issue creates a token pair. prev nil starts a new family (fresh login: the credential was proven now);
// otherwise the pair continues prev's family and keeps its original authentication time.
func (s *TokenService) Issue(u *models.User, prev *models.RefreshToken) (*dto.Tokens, error) {
	now := s.Now().UTC()
	familyID, authTime := NewID(), now.Unix()
	if prev != nil {
		familyID, authTime = prev.FamilyID, prev.AuthTime
	}
	access, err := s.IssueAccess(u, authTime)
	if err != nil {
		return nil, err
	}
	raw := RandomToken(32)
	rt := &models.RefreshToken{
		ID: NewID(), UserID: u.ID, FamilyID: familyID, TokenHash: HashToken(raw), AuthTime: authTime,
		TokenVersion: u.TokenVersion, ExpiresAt: now.Add(s.refreshTTL), CreatedAt: now,
	}
	if err := s.refresh.Create(rt); err != nil {
		return nil, err
	}
	return &dto.Tokens{AccessToken: access, RefreshToken: raw, TokenType: "Bearer", ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

// Consume validates a presented refresh token and revokes it (rotation).
// Presenting an already-revoked token is treated as theft: the whole family is revoked.
func (s *TokenService) Consume(raw string) (*models.RefreshToken, error) {
	rt, err := s.refresh.GetByHash(HashToken(raw))
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, apperr.InvalidToken("Invalid refresh token")
	}
	if err != nil {
		return nil, err
	}
	if rt.Revoked {
		s.revokeFamilyOnReuse(rt)
		return nil, apperr.InvalidToken("Refresh token reuse detected; session revoked")
	}
	if !rt.ExpiresAt.After(s.Now().UTC()) {
		return nil, apperr.TokenExpired()
	}
	won, err := s.refresh.Revoke(rt.ID)
	if err != nil {
		return nil, err
	}
	if !won { // concurrent rotation of the same token: loser is a reuse
		s.revokeFamilyOnReuse(rt)
		return nil, apperr.InvalidToken("Refresh token reuse detected; session revoked")
	}
	return rt, nil
}

// CheckVersion rejects a consumed refresh token whose session version is not the user's current one
// (u must be read after Consume). Such a token comes from a rotation that raced RevokeAll, so its
// family is revoked rather than letting a later refresh restore the revoked session.
func (s *TokenService) CheckVersion(rt *models.RefreshToken, u *models.User) error {
	if rt.TokenVersion == u.TokenVersion {
		return nil
	}
	_ = s.refresh.RevokeFamily(rt.FamilyID)
	return apperr.InvalidToken("Session revoked")
}

func (s *TokenService) revokeFamilyOnReuse(rt *models.RefreshToken) {
	_ = s.refresh.RevokeFamily(rt.FamilyID)
	if s.audit != nil {
		s.audit.Log(context.Background(), AuditTokenReuse, rt.UserID, "", map[string]string{"family_id": rt.FamilyID})
	}
}

// Revoke revokes a single refresh token (logout). Unknown tokens are ignored; returns the owner if found.
func (s *TokenService) Revoke(raw string) (userID string, err error) {
	rt, err := s.refresh.GetByHash(HashToken(raw))
	if errors.Is(err, repositories.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	_, err = s.refresh.Revoke(rt.ID)
	return rt.UserID, err
}

// RevokeAll signs the user out everywhere: every refresh token is revoked and the token version is
// bumped so outstanding access tokens stop working at once. u.TokenVersion is updated in place, so
// tokens issued from u afterwards (e.g. completing the current login) are valid.
func (s *TokenService) RevokeAll(u *models.User) error {
	if err := s.refresh.RevokeAllForUser(u.ID); err != nil {
		return err
	}
	v, err := s.users.BumpTokenVersion(u.ID)
	if err != nil {
		return err
	}
	u.TokenVersion = v
	return nil
}
