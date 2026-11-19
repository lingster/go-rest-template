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
	"github.com/lingster/go-rest-template/backend/ratelimit"
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
	recovery repositories.RecoveryCodeRepository
	idents   repositories.IdentityRepository
	google   GoogleTokenVerifier // nil when Google sign-in is disabled
	hasher   PasswordHasher
	tokens   *TokenService
	ott      *OneTimeTokenService
	mfa      *MFAService
	passkey  *PasskeyService
	userSvc  *UserService
	mailer   mail.Mailer
	audit    *AuditService
	// MailLimiter caps emails sent per address by unauthenticated endpoints (nil disables).
	MailLimiter ratelimit.Limiter
	// LoginThrottle limits failed password logins per email+client and per email (nil disables).
	LoginThrottle *LoginThrottle
	// dummyHash equalises timing for unknown emails.
	dummyHash string
}

func NewAuthService(cfg *config.Config, repos *repositories.Set, google GoogleTokenVerifier,
	hasher PasswordHasher, tokens *TokenService, ott *OneTimeTokenService, mfa *MFAService, passkey *PasskeyService,
	userSvc *UserService, mailer mail.Mailer, audit *AuditService, mailLimiter ratelimit.Limiter) *AuthService {
	dummy, _ := hasher.Hash("dummy-password-for-timing")
	return &AuthService{cfg: cfg, users: repos.Users, passkeys: repos.Passkeys, recovery: repos.Recovery,
		idents: repos.Identities, google: google, hasher: hasher, tokens: tokens, ott: ott, mfa: mfa,
		passkey: passkey, userSvc: userSvc, mailer: mailer, audit: audit, MailLimiter: mailLimiter, dummyHash: dummy}
}

// mailAllowed reports whether another email may be sent to the address (anti inbox-flooding).
func (s *AuthService) mailAllowed(email string) bool {
	return s.MailLimiter == nil || s.MailLimiter.Allow(email)
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

// registerAcceptedMessage is the one response to every registration attempt, so it cannot reveal
// whether the address already has an account.
const registerAcceptedMessage = "Check your email to finish setting up your account."

// Register creates an account and sends the verification email. For an address that already has an
// account nothing is changed: its owner is told by email instead. The response is identical either
// way, and the password is hashed in both cases so timing does not differ either.
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
	switch err := s.users.Create(u); {
	case errors.Is(err, repositories.ErrDuplicate):
		s.notifyExistingAccount(ctx, u.Email)
	case err != nil:
		return nil, err
	default:
		if err := s.sendVerification(ctx, u); err != nil {
			return nil, err
		}
		s.audit.Log(ctx, AuditRegister, u.ID, u.ID, nil)
	}
	return &dto.RegisterResponse{Message: registerAcceptedMessage}, nil
}

// notifyExistingAccount emails the owner of an address that someone tried to register again. It never
// fails or reveals anything to the caller: lookup errors, disabled accounts and throttling are silent.
func (s *AuthService) notifyExistingAccount(ctx context.Context, email string) {
	u, err := s.users.GetByEmail(email)
	if err != nil {
		log.Printf("register: existing account lookup: %v", err)
		return
	}
	if !u.IsActive || !s.mailAllowed(u.Email) {
		return
	}
	s.sendMail(ctx, mail.AccountExists(u.Email, u.Name, s.cfg.FrontendURL+"/login", s.cfg.FrontendURL+"/forgot-password"))
	s.audit.Log(ctx, AuditRegisterExisting, u.ID, "", nil)
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

// Login verifies email/password. Returns an MFA challenge when TOTP is enabled. Failed password checks
// (unknown emails included) are throttled per email+client and per email; passkey and Google sign-in are not.
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*LoginResult, error) {
	email, ip := dto.NormalizeEmail(req.Email), MetaFrom(ctx).IP
	if err := s.LoginThrottle.Check(email, ip); err != nil {
		return nil, err
	}
	u, err := s.verifyPassword(ctx, email, req.Password)
	if err != nil {
		if apperr.As(err).Code == apperr.CodeInvalidCredentials {
			s.LoginThrottle.Fail(email, ip)
		}
		return nil, err
	}
	s.LoginThrottle.Succeed(email, ip)
	if err := s.checkLoginAllowed(u); err != nil {
		_ = s.fail(ctx, u.ID, apperr.As(err).Code)
		return nil, err
	}
	return s.completeOrChallenge(ctx, u, "password")
}

// verifyPassword returns the user whose password matches, or invalid_credentials. Unknown emails and
// password-less accounts run a dummy hash so all failures look and take the same.
func (s *AuthService) verifyPassword(ctx context.Context, email, password string) (*models.User, error) {
	u, err := s.users.GetByEmail(email)
	if errors.Is(err, repositories.ErrNotFound) {
		s.hasher.Verify(s.dummyHash, password) // timing equalisation
		_ = s.fail(ctx, "", "unknown_email")
		return nil, apperr.InvalidCredentials()
	}
	if err != nil {
		return nil, err
	}
	if !u.HasPassword() {
		s.hasher.Verify(s.dummyHash, password) // same failure and timing as a wrong password
		_ = s.fail(ctx, u.ID, "no_password")
		return nil, apperr.InvalidCredentials()
	}
	if !s.hasher.Verify(u.PasswordHash, password) {
		_ = s.fail(ctx, u.ID, "bad_password")
		return nil, apperr.InvalidCredentials()
	}
	return u, nil
}

// completeOrChallenge finishes a first-factor login: a session, or an MFA challenge when TOTP is
// enabled. Every first factor (password, Google) goes through here so none can bypass 2FA.
func (s *AuthService) completeOrChallenge(ctx context.Context, u *models.User, method string) (*LoginResult, error) {
	if !u.TOTPEnabled {
		return s.complete(ctx, u, method)
	}
	tok, err := s.tokens.IssueMFAToken(u)
	if err != nil {
		return nil, err
	}
	methods := []string{"totp", "recovery_code"}
	if n, _ := s.passkeys.CountByUser(u.ID); n > 0 {
		methods = append(methods, "passkey")
	}
	return &LoginResult{Challenge: &dto.MFAChallenge{MFARequired: true, MFAToken: tok, Methods: methods}}, nil
}

// LoginMFA completes a login with a TOTP or recovery code.
func (s *AuthService) LoginMFA(ctx context.Context, req *dto.MFALoginRequest) (*LoginResult, error) {
	claims, err := s.tokens.ParseMFAToken(req.MFAToken)
	if err != nil {
		return nil, err
	}
	u, err := s.userSvc.Get(claims.Subject)
	if err != nil || claims.Ver != u.TokenVersion { // a revoked session also revokes pending challenges
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
	tokens, err := s.tokens.Issue(u, nil)
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
		_ = s.tokens.RevokeAll(u)
		return nil, apperr.AccountDisabled()
	}
	if err := s.tokens.CheckVersion(rt, u); err != nil {
		return nil, err
	}
	return s.tokens.Issue(u, rt)
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
	if err != nil || u.EmailVerified || !u.IsActive || !s.mailAllowed(u.Email) {
		return
	}
	if err := s.sendVerification(ctx, u); err != nil {
		log.Printf("resend verification: %v", err)
	}
}

// ForgotPassword emails a reset link if the account exists; silent otherwise.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) {
	u, err := s.users.GetByEmail(dto.NormalizeEmail(email))
	if err != nil || !u.IsActive || !s.mailAllowed(u.Email) {
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
	if err := s.tokens.RevokeAll(u); err != nil {
		return err
	}
	s.audit.Log(ctx, AuditPasswordReset, u.ID, u.ID, nil)
	return nil
}

// ChangePassword changes the password of an authenticated user.
func (s *AuthService) ChangePassword(ctx context.Context, u *models.User, current, next string) error {
	if !u.HasPassword() {
		// Accounts created via Google have no password; they set one through the e-mail reset flow
		// (proof of mailbox control) rather than being able to set it with just an access token.
		return apperr.PasswordNotSet()
	}
	if err := s.mfa.VerifyPassword(u, current); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	u.PasswordHash, u.UpdatedAt = hash, time.Now().UTC()
	if err := s.users.Update(u, "PasswordHash", "UpdatedAt"); err != nil {
		return err
	}
	if err := s.tokens.RevokeAll(u); err != nil {
		return err
	}
	s.sendMail(ctx, mail.PasswordChanged(u.Email, u.Name))
	s.audit.Log(ctx, AuditPasswordChange, u.ID, u.ID, nil)
	return nil
}
