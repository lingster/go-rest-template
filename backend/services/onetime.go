package services

import (
	"errors"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// One-time token lifetimes.
const (
	VerifyTokenTTL = 24 * time.Hour
	ResetTokenTTL  = time.Hour
)

// OneTimeTokenService issues and consumes single-use email verification / reset tokens.
type OneTimeTokenService struct {
	repo repositories.OneTimeTokenRepository
	Now  func() time.Time
}

func NewOneTimeTokenService(repo repositories.OneTimeTokenRepository) *OneTimeTokenService {
	return &OneTimeTokenService{repo: repo, Now: time.Now}
}

// Issue creates a token (invalidating older outstanding ones of the same purpose) and returns the raw value.
func (s *OneTimeTokenService) Issue(userID, purpose string, ttl time.Duration) (string, error) {
	if err := s.repo.InvalidateOutstanding(userID, purpose); err != nil {
		return "", err
	}
	raw := RandomToken(32)
	now := s.Now().UTC()
	err := s.repo.Create(&models.OneTimeToken{
		ID: NewID(), UserID: userID, Purpose: purpose, TokenHash: HashToken(raw),
		ExpiresAt: now.Add(ttl), CreatedAt: now,
	})
	return raw, err
}

// Consume validates the token for the purpose, marks it used and returns the owner id.
func (s *OneTimeTokenService) Consume(raw, purpose string) (string, error) {
	t, err := s.repo.GetByHash(HashToken(raw))
	if errors.Is(err, repositories.ErrNotFound) {
		return "", apperr.New(400, apperr.CodeInvalidToken, "Invalid or already used token")
	}
	if err != nil {
		return "", err
	}
	if t.Purpose != purpose || t.Used {
		return "", apperr.New(400, apperr.CodeInvalidToken, "Invalid or already used token")
	}
	if !t.ExpiresAt.After(s.Now().UTC()) {
		return "", apperr.New(400, apperr.CodeInvalidToken, "Token has expired")
	}
	won, err := s.repo.MarkUsed(t.ID)
	if err != nil {
		return "", err
	}
	if !won {
		return "", apperr.New(400, apperr.CodeInvalidToken, "Invalid or already used token")
	}
	return t.UserID, nil
}
