// Package container is the composition root: it wires repositories and services, and exposes them
// to Beego controllers/filters (which are instantiated by the framework and cannot take constructor args).
package container

import (
	"sync/atomic"
	"time"

	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/mail"
	"github.com/lingster/go-rest-template/backend/ratelimit"
	"github.com/lingster/go-rest-template/backend/repositories"
	"github.com/lingster/go-rest-template/backend/services"
)

// Container holds the wired application services.
type Container struct {
	Cfg      *config.Config
	Repos    *repositories.Set
	Audit    *services.AuditService
	Tokens   *services.TokenService
	Users    *services.UserService
	MFA      *services.MFAService
	Passkeys *services.PasskeyService
	Auth     *services.AuthService
	Admin    *services.AdminService
	Mailer   mail.Mailer

	// Cover desk.
	Agencies     *services.AgencyService
	Bookings     *services.BookingService
	SignIns      *services.SignInService
	AgencyPortal *services.AgencyPortalService
}

// Option customises container wiring (tests inject fakes).
type Option func(*options)

type options struct {
	google services.GoogleTokenVerifier
	now    func() time.Time
}

// WithGoogleVerifier replaces the Google ID token verifier (used even when GOOGLE_CLIENT_IDS is empty).
func WithGoogleVerifier(v services.GoogleTokenVerifier) Option {
	return func(o *options) { o.google = v }
}

// WithClock replaces the cover desk's clock (nil keeps time.Now).
func WithClock(now func() time.Time) Option {
	return func(o *options) {
		if now != nil {
			o.now = now
		}
	}
}

// Agency API key budgets: invalid keys per client before it is blocked, and the window for both limits.
const (
	agencyKeyFailures      = 20
	agencyKeyFailureWindow = 15 * time.Minute
)

// newCover wires the cover-desk services.
func newCover(cfg *config.Config, repos *repositories.Set, audit *services.AuditService, now func() time.Time) (
	*services.AgencyService, *services.BookingService, *services.SignInService, *services.AgencyPortalService, error) {
	loc, err := cfg.SchoolLocation()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	deps := services.CoverDeps{Repos: repos, Day: services.DefaultSchoolDay(loc), Now: now, Audit: audit}
	var perAgency ratelimit.Limiter
	if cfg.AgencyRateLimitPerMinute > 0 {
		perAgency = ratelimit.NewFixedWindow(cfg.AgencyRateLimitPerMinute, time.Minute)
	}
	agencies := services.NewAgencyService(deps,
		ratelimit.NewFixedWindow(agencyKeyFailures, agencyKeyFailureWindow), perAgency)
	return agencies, services.NewBookingService(deps), services.NewSignInService(deps),
		services.NewAgencyPortalService(deps), nil
}

// Async mail delivery: a few workers are plenty for transactional mail; the queue absorbs bursts.
const (
	mailWorkers   = 4
	mailQueueSize = 1000
)

// NewMailer builds the configured mail driver, delivering asynchronously (see mail.AsyncMailer).
func NewMailer(cfg *config.Config) mail.Mailer {
	var m mail.Mailer = mail.ConsoleMailer{}
	if cfg.MailDriver == "smtp" {
		m = &mail.SMTPMailer{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword, From: cfg.MailFrom, RequireTLS: cfg.SMTPRequireTLS}
	}
	return mail.NewAsync(m, mailWorkers, mailQueueSize)
}

// New wires everything. mailer may be nil (the configured driver is used).
func New(cfg *config.Config, repos *repositories.Set, mailer mail.Mailer, opts ...Option) (*Container, error) {
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	if o.google == nil && cfg.GoogleEnabled() {
		o.google = services.NewJWKSGoogleVerifier(cfg.GoogleClientIDs, "", nil)
	}
	if mailer == nil {
		mailer = NewMailer(cfg)
	}
	box, err := services.NewSecretBox(cfg.JWTSecret)
	if err != nil {
		return nil, err
	}
	hasher := services.BcryptHasher{Cost: cfg.BcryptCost}
	audit := services.NewAuditService(repos.Audit)
	tokens := services.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, repos.RefreshTokens, repos.Users, audit)
	users := services.NewUserService(repos.Users, repos.Passkeys, repos.Identities, audit)
	ott := services.NewOneTimeTokenService(repos.OneTime)
	mfa := services.NewMFAService(repos.Users, repos.Recovery, hasher, box, audit, cfg.WebAuthnRPName, services.NewFactorLimiter())
	passkeys, err := services.NewPasskeyService(cfg, repos.Passkeys, repos.Users, services.NewMemorySessionStore(), audit)
	if err != nil {
		return nil, err
	}
	auth := services.NewAuthService(cfg, repos, o.google, hasher, tokens, ott, mfa, passkeys, users, mailer, audit,
		services.NewMailLimiter())
	auth.LoginThrottle = services.NewLoginThrottle()
	admin := services.NewAdminService(repos, hasher, tokens, users, audit)
	agencies, bookings, signIns, portal, err := newCover(cfg, repos, audit, o.now)
	if err != nil {
		return nil, err
	}
	return &Container{Cfg: cfg, Repos: repos, Audit: audit, Tokens: tokens, Users: users, MFA: mfa,
		Passkeys: passkeys, Auth: auth, Admin: admin, Mailer: mailer,
		Agencies: agencies, Bookings: bookings, SignIns: signIns, AgencyPortal: portal}, nil
}

// Version is the build version reported by /healthz (set from main.version).
var Version = "dev"

var current atomic.Pointer[Container]

// Set installs the process-wide container.
func Set(c *Container) { current.Store(c) }

// Get returns the process-wide container (nil before Set).
func Get() *Container { return current.Load() }
