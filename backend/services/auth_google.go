package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

const maxNameLen = 100

// Providers reports which external sign-in providers are enabled (public configuration).
func (s *AuthService) Providers() dto.ProvidersResponse {
	if !s.googleEnabled() {
		return dto.ProvidersResponse{}
	}
	return dto.ProvidersResponse{Google: dto.GoogleProvider{Enabled: true, WebClientID: s.cfg.GoogleClientIDs[0]}}
}

func (s *AuthService) googleEnabled() bool { return s.google != nil && s.cfg.GoogleEnabled() }

// LoginGoogle signs a user in with a Google ID token. Accounts are matched by Google "sub", then by
// email when Google is authoritative for it (linking the identity; otherwise the user must sign in
// with their password first), otherwise a new password-less account is created. TOTP users
// still receive the MFA challenge: Google is a first factor only.
func (s *AuthService) LoginGoogle(ctx context.Context, idToken string) (*LoginResult, error) {
	if !s.googleEnabled() {
		return nil, apperr.GoogleDisabled()
	}
	claims, err := s.google.Verify(ctx, idToken)
	if err != nil || claims == nil || !claims.EmailVerified || strings.TrimSpace(claims.Email) == "" {
		_ = s.fail(ctx, "", "google_invalid_token")
		return nil, apperr.InvalidToken("Invalid Google sign-in token")
	}
	u, created, err := s.resolveGoogleUser(ctx, claims)
	if err != nil {
		return nil, err
	}
	if err := s.checkLoginAllowed(u); err != nil {
		_ = s.fail(ctx, u.ID, apperr.As(err).Code)
		return nil, err
	}
	s.audit.Log(ctx, AuditGoogleLogin, u.ID, u.ID, map[string]string{"new_user": boolStr(created)})
	return s.completeOrChallenge(ctx, u, "google")
}

// resolveGoogleUser maps verified Google claims to a local user (see LoginGoogle).
func (s *AuthService) resolveGoogleUser(ctx context.Context, c *GoogleClaims) (*models.User, bool, error) {
	for attempt := 0; attempt < 2; attempt++ { // second pass only after losing a creation/link race
		ident, err := s.idents.GetByProviderSubject(models.ProviderGoogle, c.Subject)
		if err == nil {
			u, err := s.userSvc.Get(ident.UserID)
			return u, false, err
		}
		if !errors.Is(err, repositories.ErrNotFound) {
			return nil, false, err
		}
		email := dto.NormalizeEmail(c.Email)
		u, err := s.users.GetByEmail(email)
		switch {
		case err == nil:
			if !googleAuthoritative(c, u.Email) {
				s.audit.Log(ctx, AuditGoogleLinkDenied, u.ID, "", map[string]string{"provider": models.ProviderGoogle})
				return nil, false, apperr.AccountExistsSignIn()
			}
			linked, err := s.linkGoogle(ctx, u, c)
			if errors.Is(err, repositories.ErrDuplicate) {
				continue
			}
			return linked, false, err
		case errors.Is(err, repositories.ErrNotFound):
			created, err := s.createGoogleUser(ctx, email, c)
			if errors.Is(err, repositories.ErrDuplicate) {
				continue
			}
			return created, true, err
		default:
			return nil, false, err
		}
	}
	return nil, false, apperr.Conflict("Could not complete Google sign-in, please retry")
}

// googleConsumerDomains are the mail domains Google itself operates, so Google's word on those
// addresses is authoritative.
var googleConsumerDomains = map[string]bool{"gmail.com": true, "googlemail.com": true}

// googleAuthoritative reports whether Google can vouch for control of the address: it is a Google-run
// mailbox, or the token's hosted domain (Workspace "hd") is the address's own domain. For any other
// domain "email_verified" only says Google once confirmed the address, which says nothing about who
// controls the mailbox today (a lapsed domain can be re-registered), so it is not used to take over
// an existing account.
func googleAuthoritative(c *GoogleClaims, email string) bool {
	i := strings.LastIndex(email, "@")
	if i < 0 {
		return false
	}
	domain := strings.ToLower(email[i+1:])
	return googleConsumerDomains[domain] || (c.HostedDomain != "" && strings.EqualFold(c.HostedDomain, domain))
}

// linkGoogle attaches the Google identity to an existing account with the same (Google-verified) email.
func (s *AuthService) linkGoogle(ctx context.Context, u *models.User, c *GoogleClaims) (*models.User, error) {
	if !u.IsActive {
		return nil, apperr.AccountDisabled()
	}
	if !u.EmailVerified {
		// Pre-hijack defence: whoever registered this address never proved control of it, so any
		// credentials they set (password, TOTP, passkeys, sessions) are discarded now that the real
		// owner has proven it through Google.
		if err := s.resetUnverified(u); err != nil {
			return nil, err
		}
		s.audit.Log(ctx, AuditUnverifiedReset, u.ID, "", map[string]string{"provider": models.ProviderGoogle})
	}
	if err := s.createIdentity(u.ID, c); err != nil {
		return nil, err
	}
	u.EmailVerified, u.UpdatedAt = true, time.Now().UTC()
	if err := s.users.Update(u, "EmailVerified", "UpdatedAt"); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, AuditIdentityLinked, u.ID, u.ID, map[string]string{"provider": models.ProviderGoogle})
	return u, nil
}

// resetUnverified wipes every credential and session of an account whose email was never verified.
func (s *AuthService) resetUnverified(u *models.User) error {
	u.PasswordHash, u.TOTPEnabled, u.TOTPSecret, u.TOTPLastStep = "", false, "", 0
	if err := s.users.Update(u, "PasswordHash", "TOTPEnabled", "TOTPSecret", "TOTPLastStep"); err != nil {
		return err
	}
	for _, del := range []func(string) error{s.recovery.DeleteByUser, s.passkeys.DeleteByUser} {
		if err := del(u.ID); err != nil {
			return err
		}
	}
	return s.tokens.RevokeAll(u)
}

func (s *AuthService) createGoogleUser(ctx context.Context, email string, c *GoogleClaims) (*models.User, error) {
	now := time.Now().UTC()
	u := &models.User{
		ID: NewID(), Email: email, Name: displayName(c.Name, email), PasswordHash: "", Role: models.RoleUser,
		EmailVerified: true, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.users.Create(u); err != nil {
		return nil, err
	}
	if err := s.createIdentity(u.ID, c); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, AuditRegister, u.ID, u.ID, map[string]string{"provider": models.ProviderGoogle})
	return u, nil
}

func (s *AuthService) createIdentity(userID string, c *GoogleClaims) error {
	return s.idents.Create(&models.UserIdentity{
		ID: NewID(), UserID: userID, Provider: models.ProviderGoogle, Subject: c.Subject,
		Email: dto.NormalizeEmail(c.Email), CreatedAt: time.Now().UTC(),
	})
}

// displayName uses the provided name, falling back to the email's local part; capped to the API limit.
func displayName(name, email string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		name = string([]rune(name)[:maxNameLen])
	}
	return name
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
