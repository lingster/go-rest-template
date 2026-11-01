// Package models contains the Beego ORM entities.
package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

// Roles.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// One-time token purposes.
const (
	PurposeVerifyEmail   = "verify"
	PurposeResetPassword = "reset"
)

// User is an account.
type User struct {
	ID            string    `orm:"column(id);pk;size(36)"`
	Email         string    `orm:"column(email);size(255);unique"`
	Name          string    `orm:"column(name);size(255)"`
	PasswordHash  string    `orm:"column(password_hash);size(255)"`
	Role          string    `orm:"column(role);size(32);default(user)"`
	EmailVerified bool      `orm:"column(email_verified)"`
	IsActive      bool      `orm:"column(is_active)"`
	TOTPSecret    string    `orm:"column(totp_secret);size(512)"` // AES-GCM encrypted, base64
	TOTPEnabled   bool      `orm:"column(totp_enabled)"`
	TOTPLastStep  int64     `orm:"column(totp_last_step)"` // last accepted 30s time-step (replay protection)
	CreatedAt     time.Time `orm:"column(created_at);type(datetime)"`
	UpdatedAt     time.Time `orm:"column(updated_at);type(datetime)"`
}

func (*User) TableName() string { return "users" }

// HasPassword reports whether the account can sign in with a password. Accounts created through an
// external identity provider (e.g. Google) have an empty hash, which never verifies.
func (u *User) HasPassword() bool { return u.PasswordHash != "" }

// Identity providers.
const ProviderGoogle = "google"

// UserIdentity links a user to an account at an external identity provider.
type UserIdentity struct {
	ID        string    `orm:"column(id);pk;size(36)"`
	UserID    string    `orm:"column(user_id);size(36);index"`
	Provider  string    `orm:"column(provider);size(32)"`
	Subject   string    `orm:"column(subject);size(255)"` // provider-stable account id (Google "sub")
	Email     string    `orm:"column(email);size(255)"`   // email at link time (informational)
	CreatedAt time.Time `orm:"column(created_at);type(datetime)"`
}

func (*UserIdentity) TableName() string { return "user_identities" }

// TableUnique makes (provider, subject) unique.
func (*UserIdentity) TableUnique() [][]string { return [][]string{{"Provider", "Subject"}} }

// RefreshToken is a hashed, rotating refresh token belonging to a family.
type RefreshToken struct {
	ID        string    `orm:"column(id);pk;size(36)"`
	UserID    string    `orm:"column(user_id);size(36);index"`
	FamilyID  string    `orm:"column(family_id);size(36);index"`
	TokenHash string    `orm:"column(token_hash);size(64);unique"`
	Revoked   bool      `orm:"column(revoked);index"`
	ExpiresAt time.Time `orm:"column(expires_at);type(datetime)"`
	CreatedAt time.Time `orm:"column(created_at);type(datetime)"`
}

func (*RefreshToken) TableName() string { return "refresh_tokens" }

// OneTimeToken is a hashed single-use token for email verification or password reset.
type OneTimeToken struct {
	ID        string    `orm:"column(id);pk;size(36)"`
	UserID    string    `orm:"column(user_id);size(36);index"`
	Purpose   string    `orm:"column(purpose);size(16)"`
	TokenHash string    `orm:"column(token_hash);size(64);unique"`
	Used      bool      `orm:"column(used)"`
	ExpiresAt time.Time `orm:"column(expires_at);type(datetime)"`
	CreatedAt time.Time `orm:"column(created_at);type(datetime)"`
}

func (*OneTimeToken) TableName() string { return "one_time_tokens" }

// RecoveryCode is a hashed single-use 2FA recovery code.
type RecoveryCode struct {
	ID        string    `orm:"column(id);pk;size(36)"`
	UserID    string    `orm:"column(user_id);size(36);index"`
	CodeHash  string    `orm:"column(code_hash);size(64)"`
	Used      bool      `orm:"column(used)"`
	CreatedAt time.Time `orm:"column(created_at);type(datetime)"`
}

func (*RecoveryCode) TableName() string { return "recovery_codes" }

// WebAuthnCredential is a registered passkey. CredentialJSON holds the serialized
// go-webauthn Credential (public key, sign counter, flags...).
type WebAuthnCredential struct {
	ID             string     `orm:"column(id);pk;size(36)"`
	UserID         string     `orm:"column(user_id);size(36);index"`
	Name           string     `orm:"column(name);size(255)"`
	CredentialID   string     `orm:"column(credential_id);size(512);unique"` // base64url
	CredentialJSON string     `orm:"column(credential_json);type(text)"`
	CreatedAt      time.Time  `orm:"column(created_at);type(datetime)"`
	LastUsedAt     *time.Time `orm:"column(last_used_at);null;type(datetime)"`
}

func (*WebAuthnCredential) TableName() string { return "webauthn_credentials" }

// AuditLog records a security-relevant event.
type AuditLog struct {
	ID        string    `orm:"column(id);pk;size(36)"`
	UserID    string    `orm:"column(user_id);size(36);index"` // subject, may be empty
	ActorID   string    `orm:"column(actor_id);size(36)"`      // who performed it, may be empty
	Action    string    `orm:"column(action);size(64);index"`
	IP        string    `orm:"column(ip);size(64)"`
	UserAgent string    `orm:"column(user_agent);size(512)"`
	Metadata  string    `orm:"column(metadata);type(text)"` // JSON object
	CreatedAt time.Time `orm:"column(created_at);type(datetime);index"`
}

func (*AuditLog) TableName() string { return "audit_logs" }

// Register registers all models with the Beego ORM.
func Register() {
	orm.RegisterModel(new(User), new(RefreshToken), new(OneTimeToken), new(RecoveryCode),
		new(WebAuthnCredential), new(AuditLog), new(UserIdentity))
}
