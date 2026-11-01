package services

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/mail"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// LoginResult is either a full session or an MFA challenge.
type LoginResult struct {
	Auth      *dto.AuthResponse
	Challenge *dto.MFAChallenge
}

// AuthService orchestrates registration, login, token refresh and password flows.
type AuthService struct {
	cfg      *config.Config
	users    repositories.UserRepository
	passkeys repositories.WebAuthnCredentialRepository
	hasher   PasswordHasher
	tokens   *TokenService
	ott      *OneTimeTokenService
	mfa      *MFAService
	passkey  *PasskeyService
	userSvc  *UserService
	mailer   mail.Mailer
	audit    *AuditService
	// dummyHash equalises timing for unknown emails.
	dummyHash string
}

func NewAuthService(cfg *config.Config, users repositories.UserRepository, passkeys repositories.WebAuthnCredentialRepository,
	hasher PasswordHasher, tokens *TokenService, ott *OneTimeTokenService, mfa *MFAService, passkey *PasskeyService,
	userSvc *UserService, mailer mail.Mailer, audit *AuditService) *AuthService {
	dummy, _ := hasher.Hash("dummy-password-for-timing")
	return &AuthService{cfg: cfg, users: users, passkeys: passkeys, hasher: hasher, tokens: tokens, ott: ott, mfa: mfa,
		passkey: passkey, userSvc: userSvc, mailer: mailer, audit: audit, dummyHash: dummy}
}

func (s *AuthService) link(path, token string) string {
	return s.cfg.FrontendURL + path + "?token=" + url.QueryEscape(token)
}

// sendMail never fails the caller's flow (important for no-enumeration endpoints).
func (s *AuthService) sendMail(ctx context.Context, m mail.Message) {
	if err := s.mailer.Send(ctx, m); err != nil {
		log.Printf("mail: sending to %s failed: %v", m.To, err)
	}
}

func (s *AuthService) sendVerification(ctx context.Context, u *models.User) error {
	tok, err := s.ott.Issue(u.ID, models.PurposeVerifyEmail, VerifyTokenTTL)
	if err != nil {
		return err
	}
	s.sendMail(ctx, mail.VerifyEmail(u.Email, u.Name, s.link("/verify-email", tok)))
	return nil
}

// Register creates an account and sends the verification email.
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &models.User{
		ID: NewID(), Email: dto.NormalizeEmail(req.Email), Name: strings.TrimSpace(req.Name), PasswordHash: hash,
		Role: models.RoleUser, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.users.Create(u); errors.Is(err, repositories.ErrDuplicate) {
		return nil, apperr.Conflict("An account with this email already exists")
	} else if err != nil {
		return nil, err
	}
	if err := s.sendVerification(ctx, u); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, AuditRegister, u.ID, u.ID, nil)
	view, err := s.userSvc.Present(u)
	if err != nil {
		return nil, err
	}
	msg := "Account created."
	if s.cfg.RequireEmailVerification {
		msg = "Account created. Please check your email to verify your address."
	}
	return &dto.RegisterResponse{User: view, Message: msg}, nil
}

// checkLoginAllowed enforces account state after credentials were proven.
func (s *AuthService) checkLoginAllowed(u *models.User) error {
	if !u.IsActive {
		return apperr.AccountDisabled()
	}
	if s.cfg.RequireEmailVerification && !u.EmailVerified {
		return apperr.EmailNotVerified()
	}
	return nil
}

func (s *AuthService) fail(ctx context.Context, userID, reason string) error {
	s.audit.Log(ctx, AuditLoginFailure, userID, "", map[string]string{"reason": reason})
	return nil
}

// Login verifies email/password. Returns an MFA challenge when TOTP is enabled.
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*LoginResult, error) {
	email := dto.NormalizeEmail(req.Email)
	u, err := s.users.GetByEmail(email)
	if errors.Is(err, repositories.ErrNotFound) {
		s.hasher.Verify(s.dummyHash, req.Password) // timing equalisation
		_ = s.fail(ctx, "", "unknown_email")
		return nil, apperr.InvalidCredentials()
	}
	if err != nil {
		return nil, err
	}
	if !s.hasher.Verify(u.PasswordHash, req.Password) {
		_ = s.fail(ctx, u.ID, "bad_password")
		return nil, apperr.InvalidCredentials()
	}
	if err := s.checkLoginAllowed(u); err != nil {
		_ = s.fail(ctx, u.ID, apperr.As(err).Code)
		return nil, err
	}
	if u.TOTPEnabled {
		tok, err := s.tokens.IssueMFAToken(u.ID)
		if err != nil {
			return nil, err
		}
		methods := []string{"totp", "recovery_code"}
		if n, _ := s.passkeys.CountByUser(u.ID); n > 0 {
			methods = append(methods, "passkey")
		}
		return &LoginResult{Challenge: &dto.MFAChallenge{MFARequired: true, MFAToken: tok, Methods: methods}}, nil
	}
	return s.complete(ctx, u, "password")
}

// LoginMFA completes a login with a TOTP or recovery code.
func (s *AuthService) LoginMFA(ctx context.Context, req *dto.MFALoginRequest) (*LoginResult, error) {
	uid, err := s.tokens.ParseMFAToken(req.MFAToken)
	if err != nil {
		return nil, err
	}
	u, err := s.userSvc.Get(uid)
	if err != nil {
		return nil, apperr.InvalidToken("Invalid token")
	}
	if err := s.checkLoginAllowed(u); err != nil {
		return nil, err
	}
	ok, err := s.mfa.VerifyCode(u, req.Code)
	if err != nil {
		return nil, err
	}
	if !ok {
		_ = s.fail(ctx, u.ID, "bad_mfa_code")
		return nil, apperr.InvalidMFACode()
	}
	return s.complete(ctx, u, "password+2fa")
}

// LoginPasskey completes a passkey login; a passkey satisfies MFA on its own.
func (s *AuthService) LoginPasskey(ctx context.Context, sessionID string, credJSON []byte) (*LoginResult, error) {
	u, err := s.passkey.FinishLogin(sessionID, credJSON)
	if err != nil {
		if apperr.As(err).Code == apperr.CodeInvalidCredentials {
			_ = s.fail(ctx, "", "bad_passkey")
		}
		return nil, err
	}
	if err := s.checkLoginAllowed(u); err != nil {
		_ = s.fail(ctx, u.ID, apperr.As(err).Code)
		return nil, err
	}
	return s.complete(ctx, u, "passkey")
}

func (s *AuthService) complete(ctx context.Context, u *models.User, method string) (*LoginResult, error) {
	tokens, err := s.tokens.Issue(u, "")
	if err != nil {
		return nil, err
	}
	view, err := s.userSvc.Present(u)
	if err != nil {
		return nil, err
	}
	s.audit.Log(ctx, AuditLoginSuccess, u.ID, u.ID, map[string]string{"method": method})
	return &LoginResult{Auth: &dto.AuthResponse{MFARequired: false, Tokens: *tokens, User: view}}, nil
}

// Refresh rotates a refresh token.
func (s *AuthService) Refresh(ctx context.Context, raw string) (*dto.Tokens, error) {
	rt, err := s.tokens.Consume(raw)
	if err != nil {
		return nil, err
	}
	u, err := s.userSvc.Get(rt.UserID)
	if err != nil {
		return nil, apperr.InvalidToken("Invalid refresh token")
	}
	if !u.IsActive {
		_ = s.tokens.RevokeAll(u.ID)
		return nil, apperr.AccountDisabled()
	}
	return s.tokens.Issue(u, rt.FamilyID)
}

// Logout revokes the presented refresh token (idempotent).
func (s *AuthService) Logout(ctx context.Context, raw string) error {
	uid, err := s.tokens.Revoke(raw)
	if err != nil {
		return err
	}
	if uid != "" {
		s.audit.Log(ctx, AuditLogout, uid, uid, nil)
	}
	return nil
}

// VerifyEmail consumes a verification token.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	uid, err := s.ott.Consume(token, models.PurposeVerifyEmail)
	if err != nil {
		return err
	}
	u, err := s.userSvc.Get(uid)
	if err != nil {
		return apperr.New(400, apperr.CodeInvalidToken, "Invalid or already used token")
	}
	u.EmailVerified, u.UpdatedAt = true, time.Now().UTC()
	if err := s.users.Update(u, "EmailVerified", "UpdatedAt"); err != nil {
		return err
	}
	s.audit.Log(ctx, AuditEmailVerified, u.ID, u.ID, nil)
	return nil
}

// ResendVerification re-sends the verification mail; silent for unknown/verified accounts.
func (s *AuthService) ResendVerification(ctx context.Context, email string) {
	u, err := s.users.GetByEmail(dto.NormalizeEmail(email))
	if err != nil || u.EmailVerified || !u.IsActive {
		return
	}
	if err := s.sendVerification(ctx, u); err != nil {
		log.Printf("resend verification: %v", err)
	}
}

// ForgotPassword emails a reset link if the account exists; silent otherwise.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) {
	u, err := s.users.GetByEmail(dto.NormalizeEmail(email))
	if err != nil || !u.IsActive {
		return
	}
	tok, err := s.ott.Issue(u.ID, models.PurposeResetPassword, ResetTokenTTL)
	if err != nil {
		log.Printf("forgot password: %v", err)
		return
	}
	s.sendMail(ctx, mail.ResetPassword(u.Email, u.Name, s.link("/reset-password", tok)))
	s.audit.Log(ctx, AuditPasswordForgot, u.ID, "", nil)
}

// ResetPassword sets a new password using a reset token and revokes all sessions.
func (s *AuthService) ResetPassword(ctx context.Context, token, password string) error {
	uid, err := s.ott.Consume(token, models.PurposeResetPassword)
	if err != nil {
		return err
	}
	u, err := s.userSvc.Get(uid)
	if err != nil {
		return apperr.New(400, apperr.CodeInvalidToken, "Invalid or already used token")
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	// Proving control of the mailbox also verifies the address.
	u.PasswordHash, u.EmailVerified, u.UpdatedAt = hash, true, time.Now().UTC()
	if err := s.users.Update(u, "PasswordHash", "EmailVerified", "UpdatedAt"); err != nil {
		return err
	}
	if err := s.tokens.RevokeAll(u.ID); err != nil {
		return err
	}
	s.audit.Log(ctx, AuditPasswordReset, u.ID, u.ID, nil)
	return nil
}

// ChangePassword changes the password of an authenticated user.
func (s *AuthService) ChangePassword(ctx context.Context, u *models.User, current, next string) error {
	if !s.hasher.Verify(u.PasswordHash, current) {
		return apperr.InvalidCredentials()
	}
	hash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	u.PasswordHash, u.UpdatedAt = hash, time.Now().UTC()
	if err := s.users.Update(u, "PasswordHash", "UpdatedAt"); err != nil {
		return err
	}
	if err := s.tokens.RevokeAll(u.ID); err != nil {
		return err
	}
	s.sendMail(ctx, mail.PasswordChanged(u.Email, u.Name))
	s.audit.Log(ctx, AuditPasswordChange, u.ID, u.ID, nil)
	return nil
}
