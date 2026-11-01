// Package repositories defines persistence interfaces (consumed by services) and
// their Beego ORM implementations.
package repositories

import (
	"errors"

	"github.com/lingster/go-rest-template/backend/models"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicate is returned when a unique constraint is violated.
var ErrDuplicate = errors.New("duplicate record")

// UserStats are aggregate counters for the admin dashboard.
type UserStats struct {
	Total, Active, Verified, TwoFA int64
}

// UserRepository persists users.
type UserRepository interface {
	Create(u *models.User) error
	GetByID(id string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	// Update writes the named columns (all columns when none given).
	Update(u *models.User, columns ...string) error
	Delete(id string) error
	// AdvanceTOTPStep atomically sets totp_last_step=step iff it is greater than the stored value.
	AdvanceTOTPStep(id string, step int64) (bool, error)
	List(query string, offset, limit int) ([]*models.User, int64, error)
	Stats() (UserStats, error)
}

// RefreshTokenRepository persists hashed refresh tokens.
type RefreshTokenRepository interface {
	Create(t *models.RefreshToken) error
	GetByHash(hash string) (*models.RefreshToken, error)
	// Revoke marks the token revoked; returns false if it already was (atomic compare-and-set).
	Revoke(id string) (bool, error)
	RevokeFamily(familyID string) error
	RevokeAllForUser(userID string) error
	DeleteByUser(userID string) error
}

// OneTimeTokenRepository persists hashed single-use tokens.
type OneTimeTokenRepository interface {
	Create(t *models.OneTimeToken) error
	GetByHash(hash string) (*models.OneTimeToken, error)
	// MarkUsed flags the token used; returns false if it already was (atomic).
	MarkUsed(id string) (bool, error)
	// InvalidateOutstanding marks all unused tokens of the purpose for the user as used.
	InvalidateOutstanding(userID, purpose string) error
	DeleteByUser(userID string) error
}

// RecoveryCodeRepository persists hashed recovery codes.
type RecoveryCodeRepository interface {
	// ReplaceForUser removes all existing codes and stores the new ones.
	ReplaceForUser(userID string, codes []*models.RecoveryCode) error
	FindUnused(userID, hash string) (*models.RecoveryCode, error)
	MarkUsed(id string) (bool, error)
	CountUnused(userID string) (int64, error)
	DeleteByUser(userID string) error
}

// WebAuthnCredentialRepository persists passkeys.
type WebAuthnCredentialRepository interface {
	Create(c *models.WebAuthnCredential) error
	ListByUser(userID string) ([]*models.WebAuthnCredential, error)
	GetByCredentialID(credentialID string) (*models.WebAuthnCredential, error)
	GetForUser(id, userID string) (*models.WebAuthnCredential, error)
	Update(c *models.WebAuthnCredential, columns ...string) error
	Delete(id string) error
	CountByUser(userID string) (int64, error)
	CountByUsers(userIDs []string) (map[string]int64, error)
	DeleteByUser(userID string) error
}

// AuditLogRepository persists audit entries.
type AuditLogRepository interface {
	Create(l *models.AuditLog) error
	List(userID string, offset, limit int) ([]*models.AuditLog, int64, error)
}

// Set bundles all repositories (composition root convenience).
type Set struct {
	Users         UserRepository
	RefreshTokens RefreshTokenRepository
	OneTime       OneTimeTokenRepository
	Recovery      RecoveryCodeRepository
	Passkeys      WebAuthnCredentialRepository
	Audit         AuditLogRepository
}

// NewORMSet builds the Beego ORM backed repositories.
func NewORMSet() *Set {
	return &Set{
		Users:         &ormUserRepo{},
		RefreshTokens: &ormRefreshRepo{},
		OneTime:       &ormOneTimeRepo{},
		Recovery:      &ormRecoveryRepo{},
		Passkeys:      &ormPasskeyRepo{},
		Audit:         &ormAuditRepo{},
	}
}
