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
	// BumpTokenVersion atomically increments token_version and returns the new value.
	BumpTokenVersion(id string) (int64, error)
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

// IdentityRepository persists links between users and external identity providers.
type IdentityRepository interface {
	// Create returns ErrDuplicate when (provider, subject) is already linked.
	Create(i *models.UserIdentity) error
	GetByProviderSubject(provider, subject string) (*models.UserIdentity, error)
	// ProvidersByUsers returns the sorted, de-duplicated provider names per user id.
	ProvidersByUsers(userIDs []string) (map[string][]string, error)
	DeleteByUser(userID string) error
}

// AuditLogRepository persists audit entries.
type AuditLogRepository interface {
	Create(l *models.AuditLog) error
	List(userID string, offset, limit int) ([]*models.AuditLog, int64, error)
}

// AgencyRepository persists supply agencies.
type AgencyRepository interface {
	// Create returns ErrDuplicate when the name (or key hash) is taken.
	Create(a *models.Agency) error
	GetByID(id string) (*models.Agency, error)
	GetByKeyHash(hash string) (*models.Agency, error)
	List() ([]*models.Agency, error)
	// GetMany returns the agencies with the given ids, keyed by id.
	GetMany(ids []string) (map[string]*models.Agency, error)
	// SetKeyHash replaces the API key hash (key rotation); the old key stops working at once.
	SetKeyHash(id, hash string) error
}

// SupplyTeacherRepository persists supply teachers.
type SupplyTeacherRepository interface {
	Create(t *models.SupplyTeacher) error
	GetByID(id string) (*models.SupplyTeacher, error)
	// List returns all teachers, or only one agency's when agencyID is not empty.
	List(agencyID string) ([]*models.SupplyTeacher, error)
	GetMany(ids []string) (map[string]*models.SupplyTeacher, error)
}

// BookingRepository persists bookings. Agency, date, kind, subject and expected arrival are fixed at
// creation; Update writes only the status/assignment fields.
type BookingRepository interface {
	// Create stores the booking and its lessons atomically.
	Create(b *models.Booking, lessons []*models.BookingLesson) error
	GetByID(id string) (*models.Booking, error)
	// ListByDates returns the bookings dated from..to inclusive (YYYY-MM-DD), optionally for one agency.
	ListByDates(from, to, agencyID string) ([]*models.Booking, error)
	// Update writes status, supply_teacher_id, covering, lanyard and updated_at iff the stored status is
	// still fromStatus (optimistic concurrency); false means it changed meanwhile.
	Update(b *models.Booking, fromStatus string) (bool, error)
	// ReplaceLessons swaps the booking's lessons atomically.
	ReplaceLessons(bookingID string, lessons []*models.BookingLesson) error
	// LessonsByBookings returns lessons keyed by booking id.
	LessonsByBookings(ids []string) (map[string][]*models.BookingLesson, error)
}

// EtaUpdateRepository is append-only: there is deliberately no update or delete.
type EtaUpdateRepository interface {
	Create(u *models.EtaUpdate) error
	// ListByBookings returns updates keyed by booking id, oldest first.
	ListByBookings(ids []string) (map[string][]*models.EtaUpdate, error)
}

// SignInEventRepository is the append-only safeguarding log: there is deliberately no update or delete.
type SignInEventRepository interface {
	// Create returns ErrDuplicate when (booking_id, seq) is taken: a concurrent conflicting event won.
	Create(e *models.SignInEvent) error
	GetByID(id string) (*models.SignInEvent, error)
	// ListByBookings returns events keyed by booking id, in seq order.
	ListByBookings(ids []string) (map[string][]*models.SignInEvent, error)
}

// Set bundles all repositories (composition root convenience).
type Set struct {
	Users         UserRepository
	RefreshTokens RefreshTokenRepository
	OneTime       OneTimeTokenRepository
	Recovery      RecoveryCodeRepository
	Passkeys      WebAuthnCredentialRepository
	Audit         AuditLogRepository
	Identities    IdentityRepository
	Agencies      AgencyRepository
	Teachers      SupplyTeacherRepository
	Bookings      BookingRepository
	EtaUpdates    EtaUpdateRepository
	SignIns       SignInEventRepository
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
		Identities:    &ormIdentityRepo{},
		Agencies:      &ormAgencyRepo{},
		Teachers:      &ormTeacherRepo{},
		Bookings:      &ormBookingRepo{},
		EtaUpdates:    &ormEtaRepo{},
		SignIns:       &ormSignInRepo{},
	}
}
