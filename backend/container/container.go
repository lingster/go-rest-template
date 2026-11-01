// Package container is the composition root: it wires repositories and services, and exposes them
// to Beego controllers/filters (which are instantiated by the framework and cannot take constructor args).
package container

import (
	"sync/atomic"

	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/mail"
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
}

// NewMailer builds the configured mail driver.
func NewMailer(cfg *config.Config) mail.Mailer {
	if cfg.MailDriver == "smtp" {
		return &mail.SMTPMailer{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword, From: cfg.MailFrom}
	}
	return mail.ConsoleMailer{}
}

// New wires everything. mailer may be nil (the configured driver is used).
func New(cfg *config.Config, repos *repositories.Set, mailer mail.Mailer) (*Container, error) {
	if mailer == nil {
		mailer = NewMailer(cfg)
	}
	box, err := services.NewSecretBox(cfg.JWTSecret)
	if err != nil {
		return nil, err
	}
	hasher := services.BcryptHasher{Cost: cfg.BcryptCost}
	audit := services.NewAuditService(repos.Audit)
	tokens := services.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, repos.RefreshTokens, audit)
	users := services.NewUserService(repos.Users, repos.Passkeys, audit)
	ott := services.NewOneTimeTokenService(repos.OneTime)
	mfa := services.NewMFAService(repos.Users, repos.Recovery, hasher, box, audit, cfg.WebAuthnRPName)
	passkeys, err := services.NewPasskeyService(cfg, repos.Passkeys, repos.Users, services.NewMemorySessionStore(), audit)
	if err != nil {
		return nil, err
	}
	auth := services.NewAuthService(cfg, repos.Users, repos.Passkeys, hasher, tokens, ott, mfa, passkeys, users, mailer, audit)
	admin := services.NewAdminService(repos, hasher, tokens, users, audit)
	return &Container{Cfg: cfg, Repos: repos, Audit: audit, Tokens: tokens, Users: users, MFA: mfa,
		Passkeys: passkeys, Auth: auth, Admin: admin, Mailer: mailer}, nil
}

// Version is the build version reported by /healthz (set from main.version).
var Version = "dev"

var current atomic.Pointer[Container]

// Set installs the process-wide container.
func Set(c *Container) { current.Store(c) }

// Get returns the process-wide container (nil before Set).
func Get() *Container { return current.Load() }
