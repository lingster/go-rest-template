package services

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

const (
	// RecoveryCodeCount is the number of recovery codes generated per batch.
	RecoveryCodeCount = 10
	recoveryAlphabet  = "abcdefghjkmnpqrstuvwxyz23456789"
	recoveryLen       = 10
)

// MFAService manages TOTP enrolment/verification and recovery codes.
type MFAService struct {
	users    repositories.UserRepository
	recovery repositories.RecoveryCodeRepository
	hasher   PasswordHasher
	box      *SecretBox
	audit    *AuditService
	issuer   string
	Now      func() time.Time
}

func NewMFAService(users repositories.UserRepository, recovery repositories.RecoveryCodeRepository,
	hasher PasswordHasher, box *SecretBox, audit *AuditService, issuer string) *MFAService {
	return &MFAService{users: users, recovery: recovery, hasher: hasher, box: box, audit: audit, issuer: issuer, Now: time.Now}
}

// Setup generates a pending TOTP secret (not active until Enable succeeds).
func (m *MFAService) Setup(u *models.User) (secret, otpauthURL string, err error) {
	if u.TOTPEnabled {
		return "", "", apperr.Conflict("Two-factor authentication is already enabled")
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: m.issuer, AccountName: u.Email})
	if err != nil {
		return "", "", err
	}
	sealed, err := m.box.Seal(key.Secret())
	if err != nil {
		return "", "", err
	}
	u.TOTPSecret = sealed
	u.UpdatedAt = m.Now().UTC()
	if err := m.users.Update(u, "TOTPSecret", "UpdatedAt"); err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func (m *MFAService) validateTOTP(u *models.User, code string) bool {
	if u.TOTPSecret == "" {
		return false
	}
	secret, err := m.box.Open(u.TOTPSecret)
	if err != nil {
		return false
	}
	ok, err := totp.ValidateCustom(code, secret, m.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && ok
}

// Enable activates TOTP after verifying a code, and returns fresh recovery codes.
func (m *MFAService) Enable(ctx context.Context, u *models.User, code string) ([]string, error) {
	if u.TOTPEnabled {
		return nil, apperr.Conflict("Two-factor authentication is already enabled")
	}
	if u.TOTPSecret == "" {
		return nil, apperr.Validation(map[string]string{"code": "call /me/2fa/totp/setup first"})
	}
	if !m.validateTOTP(u, normalizeTOTP(code)) {
		return nil, apperr.InvalidMFACode()
	}
	u.TOTPEnabled = true
	u.UpdatedAt = m.Now().UTC()
	if err := m.users.Update(u, "TOTPEnabled", "UpdatedAt"); err != nil {
		return nil, err
	}
	codes, err := m.generateRecoveryCodes(u.ID)
	if err != nil {
		return nil, err
	}
	m.audit.Log(ctx, AuditTOTPEnable, u.ID, u.ID, nil)
	return codes, nil
}

// Disable turns TOTP off after verifying the password and a TOTP/recovery code.
func (m *MFAService) Disable(ctx context.Context, u *models.User, password, code string) error {
	if !u.TOTPEnabled {
		return apperr.Conflict("Two-factor authentication is not enabled")
	}
	if !m.hasher.Verify(u.PasswordHash, password) {
		return apperr.InvalidCredentials()
	}
	if ok, err := m.VerifyCode(u, code); err != nil {
		return err
	} else if !ok {
		return apperr.InvalidMFACode()
	}
	u.TOTPEnabled, u.TOTPSecret = false, ""
	u.UpdatedAt = m.Now().UTC()
	if err := m.users.Update(u, "TOTPEnabled", "TOTPSecret", "UpdatedAt"); err != nil {
		return err
	}
	if err := m.recovery.DeleteByUser(u.ID); err != nil {
		return err
	}
	m.audit.Log(ctx, AuditTOTPDisable, u.ID, u.ID, nil)
	return nil
}

// RegenerateRecoveryCodes replaces all recovery codes after verifying a code.
func (m *MFAService) RegenerateRecoveryCodes(ctx context.Context, u *models.User, code string) ([]string, error) {
	if !u.TOTPEnabled {
		return nil, apperr.Conflict("Two-factor authentication is not enabled")
	}
	if ok, err := m.VerifyCode(u, code); err != nil {
		return nil, err
	} else if !ok {
		return nil, apperr.InvalidMFACode()
	}
	codes, err := m.generateRecoveryCodes(u.ID)
	if err != nil {
		return nil, err
	}
	m.audit.Log(ctx, AuditRecoveryRegen, u.ID, u.ID, nil)
	return codes, nil
}

// VerifyCode checks a 6-digit TOTP code or consumes a recovery code.
func (m *MFAService) VerifyCode(u *models.User, code string) (bool, error) {
	code = strings.TrimSpace(code)
	if t := normalizeTOTP(code); len(t) == 6 && isDigits(t) {
		return m.validateTOTP(u, t), nil
	}
	rc, err := m.recovery.FindUnused(u.ID, HashToken(NormalizeRecoveryCode(code)))
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return m.recovery.MarkUsed(rc.ID) // false if a concurrent request consumed it
}

func (m *MFAService) generateRecoveryCodes(userID string) ([]string, error) {
	plain := make([]string, RecoveryCodeCount)
	rows := make([]*models.RecoveryCode, RecoveryCodeCount)
	now := m.Now().UTC()
	for i := range plain {
		c, err := newRecoveryCode()
		if err != nil {
			return nil, err
		}
		plain[i] = c
		rows[i] = &models.RecoveryCode{ID: NewID(), UserID: userID, CodeHash: HashToken(NormalizeRecoveryCode(c)), CreatedAt: now}
	}
	return plain, m.recovery.ReplaceForUser(userID, rows)
}

// NormalizeRecoveryCode lowercases and strips separators.
func NormalizeRecoveryCode(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	return strings.NewReplacer("-", "", " ", "").Replace(c)
}

func normalizeTOTP(c string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(c))
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// newRecoveryCode returns e.g. "k3m9x-q7w2b".
func newRecoveryCode() (string, error) {
	var sb strings.Builder
	max := big.NewInt(int64(len(recoveryAlphabet)))
	for i := 0; i < recoveryLen; i++ {
		if i == recoveryLen/2 {
			sb.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(recoveryAlphabet[n.Int64()])
	}
	return sb.String(), nil
}
