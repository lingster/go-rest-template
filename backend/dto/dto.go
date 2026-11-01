// Package dto holds the request/response shapes of the JSON API (see docs/API_CONTRACT.md).
package dto

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// ---------- responses ----------

// User is the public representation of an account.
type User struct {
	ID            string   `json:"id" description:"User UUID" example:"6f1c1a0e-5a0e-4a3e-9f7a-3b1d2f6b7c11"`
	Email         string   `json:"email" example:"ada@example.com"`
	Name          string   `json:"name" example:"Ada"`
	Role          string   `json:"role" description:"user or admin" example:"user"`
	EmailVerified bool     `json:"email_verified"`
	IsActive      bool     `json:"is_active"`
	TOTPEnabled   bool     `json:"totp_enabled"`
	PasskeyCount  int      `json:"passkey_count"`
	HasPassword   bool     `json:"has_password" description:"false for accounts created via an identity provider until a password is set"`
	Identities    []string `json:"identities" description:"linked identity providers, e.g. [\"google\"]" example:"google"`
	CreatedAt     string   `json:"created_at" description:"RFC3339 UTC" example:"2026-01-01T12:00:00Z"`
	UpdatedAt     string   `json:"updated_at" description:"RFC3339 UTC" example:"2026-01-01T12:00:00Z"`
}

// Tokens is an access/refresh token pair.
type Tokens struct {
	AccessToken  string `json:"access_token" description:"HS256 JWT"`
	RefreshToken string `json:"refresh_token" description:"Opaque rotating refresh token"`
	TokenType    string `json:"token_type" example:"Bearer"`
	ExpiresIn    int    `json:"expires_in" description:"Access token lifetime in seconds" example:"900"`
}

// AuthResponse is returned by a successful login.
type AuthResponse struct {
	MFARequired bool   `json:"mfa_required" example:"false"`
	Tokens      Tokens `json:"tokens"`
	User        User   `json:"user"`
}

// MFAChallenge is returned by /auth/login when a second factor is required.
type MFAChallenge struct {
	MFARequired bool     `json:"mfa_required" example:"true"`
	MFAToken    string   `json:"mfa_token" description:"Short-lived (5 min) token to present to /auth/login/2fa"`
	Methods     []string `json:"methods" description:"totp, recovery_code and/or passkey"`
}

// Message is a generic informational response.
type Message struct {
	Message string `json:"message"`
}

// RegisterResponse is returned by /auth/register.
type RegisterResponse struct {
	User    User   `json:"user"`
	Message string `json:"message"`
}

// Passkey describes a registered WebAuthn credential.
type Passkey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name" example:"MacBook"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at" description:"RFC3339 or null"`
}

// CeremonyResponse carries WebAuthn options plus the server-side session key.
type CeremonyResponse struct {
	SessionID string      `json:"session_id"`
	Options   interface{} `json:"options" ignore:"true" description:"go-webauthn options object ({publicKey: {...}})"`
}

// TOTPSetupResponse is returned when starting TOTP enrolment.
type TOTPSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

// RecoveryCodesResponse lists freshly generated recovery codes.
type RecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// Stats are admin dashboard counters.
type Stats struct {
	UsersTotal    int64 `json:"users_total"`
	UsersActive   int64 `json:"users_active"`
	UsersVerified int64 `json:"users_verified"`
	Users2FA      int64 `json:"users_2fa"`
}

// UserPage is a paginated list of users.
type UserPage struct {
	Items   []User `json:"items"`
	Total   int64  `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// AuditLog is one audit entry.
type AuditLog struct {
	ID        string            `json:"id"`
	UserID    *string           `json:"user_id"`
	ActorID   *string           `json:"actor_id"`
	Action    string            `json:"action" example:"login.success"`
	IP        string            `json:"ip"`
	UserAgent string            `json:"user_agent"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt string            `json:"created_at"`
}

// AuditLogPage is a paginated list of audit entries.
type AuditLogPage struct {
	Items   []AuditLog `json:"items"`
	Total   int64      `json:"total"`
	Page    int        `json:"page"`
	PerPage int        `json:"per_page"`
}

// ErrorBody is the error envelope content.
type ErrorBody struct {
	Code    string            `json:"code" example:"validation_error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// ErrorResponse is the error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// HealthResponse is returned by /healthz.
type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Version string `json:"version,omitempty" example:"dev"`
}

// ---------- requests ----------

// Validator is implemented by request DTOs; it returns field -> reason.
type Validator interface {
	Validate() map[string]string
}

// RegisterRequest creates an account.
type RegisterRequest struct {
	Email    string `json:"email" required:"true" example:"ada@example.com"`
	Password string `json:"password" required:"true" description:"8-128 chars"`
	Name     string `json:"name" required:"true"`
}

// LoginRequest authenticates with a password.
type LoginRequest struct {
	Email    string `json:"email" required:"true"`
	Password string `json:"password" required:"true"`
}

// MFALoginRequest completes a login that needs a second factor.
type MFALoginRequest struct {
	MFAToken string `json:"mfa_token" required:"true"`
	Code     string `json:"code" required:"true" description:"6-digit TOTP code or a recovery code"`
}

// RefreshRequest exchanges / revokes a refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" required:"true"`
}

// TokenRequest carries a one-time token.
type TokenRequest struct {
	Token string `json:"token" required:"true"`
}

// EmailRequest carries only an email address.
type EmailRequest struct {
	Email string `json:"email" required:"true"`
}

// ResetPasswordRequest completes a password reset.
type ResetPasswordRequest struct {
	Token    string `json:"token" required:"true"`
	Password string `json:"password" required:"true"`
}

// PasskeyLoginBeginRequest starts a passkey login; email is optional (discoverable credentials).
type PasskeyLoginBeginRequest struct {
	Email string `json:"email" description:"omit for discoverable credential login"`
}

// PasskeyFinishRequest completes a passkey login ceremony.
type PasskeyFinishRequest struct {
	SessionID  string      `json:"session_id" required:"true"`
	Credential interface{} `json:"credential" required:"true" ignore:"true" description:"PublicKeyCredential JSON"`
}

// PasskeyRegisterFinishRequest completes passkey registration.
type PasskeyRegisterFinishRequest struct {
	SessionID  string      `json:"session_id" required:"true"`
	Name       string      `json:"name" example:"MacBook"`
	Credential interface{} `json:"credential" required:"true" ignore:"true" description:"PublicKeyCredential JSON"`
}

// UpdateProfileRequest updates the current user.
type UpdateProfileRequest struct {
	Name string `json:"name" required:"true"`
}

// GoogleLoginRequest carries a Google ID token (JWT) obtained client-side.
type GoogleLoginRequest struct {
	IDToken string `json:"id_token" required:"true" description:"Google ID token (JWT)"`
}

// GoogleProvider describes the Google sign-in configuration for clients.
type GoogleProvider struct {
	Enabled     bool   `json:"enabled"`
	WebClientID string `json:"web_client_id" description:"public OAuth web client id; empty when disabled"`
}

// ProvidersResponse lists the enabled external sign-in providers.
type ProvidersResponse struct {
	Google GoogleProvider `json:"google"`
}

// ChangePasswordRequest changes the current user's password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" required:"true"`
	NewPassword     string `json:"new_password" required:"true"`
}

// CodeRequest carries a TOTP or recovery code.
type CodeRequest struct {
	Code string `json:"code" required:"true"`
}

// DisableTOTPRequest disables 2FA.
type DisableTOTPRequest struct {
	Password string `json:"password" description:"required unless the account has no password (has_password=false)"`
	Code     string `json:"code" required:"true"`
}

// AdminUpdateUserRequest patches a user (all fields optional).
type AdminUpdateUserRequest struct {
	Name          *string `json:"name"`
	Role          *string `json:"role" description:"user or admin"`
	IsActive      *bool   `json:"is_active"`
	EmailVerified *bool   `json:"email_verified"`
}

// ---------- validation ----------

const (
	MinPasswordLen = 8
	MaxPasswordLen = 128
)

// NormalizeEmail lowercases and trims an email.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func checkEmail(d map[string]string, field, v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		d[field] = "is required"
		return
	}
	if a, err := mail.ParseAddress(v); err != nil || a.Address != v || len(v) > 255 {
		d[field] = "must be a valid email address"
	}
}

func checkPassword(d map[string]string, field, v string) {
	if n := utf8.RuneCountInString(v); n < MinPasswordLen || n > MaxPasswordLen {
		d[field] = "must be between 8 and 128 characters"
	}
}

func checkRequired(d map[string]string, field, v string) {
	if strings.TrimSpace(v) == "" {
		d[field] = "is required"
	}
}

func checkName(d map[string]string, field, v string) {
	v = strings.TrimSpace(v)
	if v == "" || utf8.RuneCountInString(v) > 100 {
		d[field] = "is required (max 100 characters)"
	}
}

func (r *RegisterRequest) Validate() map[string]string {
	d := map[string]string{}
	checkEmail(d, "email", r.Email)
	checkPassword(d, "password", r.Password)
	checkName(d, "name", r.Name)
	return d
}

func (r *LoginRequest) Validate() map[string]string {
	d := map[string]string{}
	checkEmail(d, "email", r.Email)
	checkRequired(d, "password", r.Password)
	return d
}

func (r *MFALoginRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "mfa_token", r.MFAToken)
	checkRequired(d, "code", r.Code)
	return d
}

func (r *RefreshRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "refresh_token", r.RefreshToken)
	return d
}

func (r *TokenRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "token", r.Token)
	return d
}

func (r *EmailRequest) Validate() map[string]string {
	d := map[string]string{}
	checkEmail(d, "email", r.Email)
	return d
}

func (r *ResetPasswordRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "token", r.Token)
	checkPassword(d, "password", r.Password)
	return d
}

func (r *PasskeyLoginBeginRequest) Validate() map[string]string {
	d := map[string]string{}
	if strings.TrimSpace(r.Email) != "" {
		checkEmail(d, "email", r.Email)
	}
	return d
}

func (r *PasskeyFinishRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "session_id", r.SessionID)
	if r.Credential == nil {
		d["credential"] = "is required"
	}
	return d
}

func (r *PasskeyRegisterFinishRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "session_id", r.SessionID)
	if r.Credential == nil {
		d["credential"] = "is required"
	}
	if utf8.RuneCountInString(r.Name) > 100 {
		d["name"] = "must be at most 100 characters"
	}
	return d
}

func (r *UpdateProfileRequest) Validate() map[string]string {
	d := map[string]string{}
	checkName(d, "name", r.Name)
	return d
}

func (r *ChangePasswordRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "current_password", r.CurrentPassword)
	checkPassword(d, "new_password", r.NewPassword)
	return d
}

// MaxIDTokenLen bounds the accepted Google ID token size.
const MaxIDTokenLen = 8192

func (r *GoogleLoginRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "id_token", r.IDToken)
	if len(r.IDToken) > MaxIDTokenLen {
		d["id_token"] = "is too long"
	}
	return d
}

func (r *CodeRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "code", r.Code)
	return d
}

func (r *DisableTOTPRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "code", r.Code)
	return d
}

func (r *AdminUpdateUserRequest) Validate() map[string]string {
	d := map[string]string{}
	if r.Name != nil {
		checkName(d, "name", *r.Name)
	}
	if r.Role != nil && *r.Role != "user" && *r.Role != "admin" {
		d["role"] = "must be user or admin"
	}
	return d
}
