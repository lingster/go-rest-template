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
	jwt.RegisteredClaims
}

// TokenService issues/validates JWTs and manages rotating refresh tokens.
type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	refresh    repositories.RefreshTokenRepository
	audit      *AuditService
	Now        func() time.Time
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration,
	refresh repositories.RefreshTokenRepository, audit *AuditService) *TokenService {
	return &TokenService{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL,
		refresh: refresh, audit: audit, Now: time.Now}
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

// IssueAccess creates a signed access token for the user.
func (s *TokenService) IssueAccess(u *models.User) (string, error) {
	return s.sign(s.claims(u.ID, TypAccess, u.Role, s.accessTTL))
}

// IssueMFAToken creates the short-lived token handed out after a correct password.
func (s *TokenService) IssueMFAToken(userID string) (string, error) {
	return s.sign(s.claims(userID, TypMFA, "", MFATokenTTL))
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

// ParseMFAToken validates an MFA challenge token and returns the user id.
func (s *TokenService) ParseMFAToken(tok string) (string, error) {
	c, err := s.parse(tok, TypMFA)
	if err != nil {
		return "", err
	}
	return c.Subject, nil
}

// Issue creates a token pair; familyID "" starts a new family (fresh login).
func (s *TokenService) Issue(u *models.User, familyID string) (*dto.Tokens, error) {
	access, err := s.IssueAccess(u)
	if err != nil {
		return nil, err
	}
	if familyID == "" {
		familyID = NewID()
	}
	raw := RandomToken(32)
	now := s.Now().UTC()
	rt := &models.RefreshToken{
		ID: NewID(), UserID: u.ID, FamilyID: familyID, TokenHash: HashToken(raw),
		ExpiresAt: now.Add(s.refreshTTL), CreatedAt: now,
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

// RevokeAll revokes every refresh token of the user.
func (s *TokenService) RevokeAll(userID string) error { return s.refresh.RevokeAllForUser(userID) }
