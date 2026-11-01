package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// Audit actions.
const (
	AuditRegister         = "register"
	AuditLoginSuccess     = "login.success"
	AuditLoginFailure     = "login.failure"
	AuditLogout           = "logout"
	AuditPasswordForgot   = "password.forgot"
	AuditPasswordReset    = "password.reset"
	AuditPasswordChange   = "password.change"
	AuditEmailVerified    = "email.verified"
	AuditTOTPEnable       = "2fa.enable"
	AuditTOTPDisable      = "2fa.disable"
	AuditRecoveryRegen    = "2fa.recovery_codes"
	AuditPasskeyAdd       = "passkey.add"
	AuditPasskeyRemove    = "passkey.remove"
	AuditAdminUserUpdate  = "admin.user.update"
	AuditAdminUserDelete  = "admin.user.delete"
	AuditAdminRevokeSess  = "admin.user.revoke_sessions"
	AuditTokenReuse       = "refresh.reuse_detected"
	AuditProfileUpdate    = "profile.update"
	AuditAdminSeedCreated = "admin.seeded"
	AuditGoogleLogin      = "auth.google_login"
	AuditIdentityLinked   = "auth.identity_linked"
)

// RequestMeta is per-request context recorded in audit entries.
type RequestMeta struct{ IP, UserAgent string }

type metaKey struct{}

// WithMeta attaches request metadata to a context.
func WithMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// MetaFrom extracts request metadata (zero value if absent).
func MetaFrom(ctx context.Context) RequestMeta {
	m, _ := ctx.Value(metaKey{}).(RequestMeta)
	return m
}

// AuditService writes and lists audit entries.
type AuditService struct {
	repo repositories.AuditLogRepository
}

func NewAuditService(repo repositories.AuditLogRepository) *AuditService {
	return &AuditService{repo: repo}
}

// Log records an event. Failures are logged but never block the caller.
func (a *AuditService) Log(ctx context.Context, action, userID, actorID string, metadata map[string]string) {
	m := MetaFrom(ctx)
	raw, _ := json.Marshal(metadata)
	if metadata == nil {
		raw = []byte("{}")
	}
	entry := &models.AuditLog{
		ID: NewID(), UserID: userID, ActorID: actorID, Action: action,
		IP: m.IP, UserAgent: truncate(m.UserAgent, 500), Metadata: string(raw), CreatedAt: time.Now().UTC(),
	}
	if err := a.repo.Create(entry); err != nil {
		log.Printf("audit: failed to write %q: %v", action, err)
	}
}

// List returns a page of entries, optionally filtered by subject user.
func (a *AuditService) List(userID string, page, perPage int) (*dto.AuditLogPage, error) {
	page, perPage = NormalizePaging(page, perPage)
	logs, total, err := a.repo.List(userID, (page-1)*perPage, perPage)
	if err != nil {
		return nil, err
	}
	items := make([]dto.AuditLog, 0, len(logs))
	for _, l := range logs {
		meta := map[string]string{}
		_ = json.Unmarshal([]byte(l.Metadata), &meta)
		items = append(items, dto.AuditLog{
			ID: l.ID, UserID: nilIfEmpty(l.UserID), ActorID: nilIfEmpty(l.ActorID), Action: l.Action,
			IP: l.IP, UserAgent: l.UserAgent, Metadata: meta, CreatedAt: formatTime(l.CreatedAt),
		})
	}
	return &dto.AuditLogPage{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}

// NormalizePaging clamps page (>=1) and perPage (default 20, max 100).
func NormalizePaging(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
