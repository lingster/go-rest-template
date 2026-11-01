// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultJWTSecret is the insecure development secret; prod refuses to start with it.
const DefaultJWTSecret = "dev-insecure-change-me-please-0123456789"

// Config holds all runtime configuration.
type Config struct {
	AppEnv   string
	HTTPPort int

	DBDriver    string
	DatabaseURL string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	RequireEmailVerification bool

	// GoogleClientIDs are the accepted OAuth client IDs (audiences) for Google ID tokens. The first entry
	// is the web client ID advertised to clients. Empty disables Google sign-in.
	GoogleClientIDs []string

	MailDriver   string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	MailFrom     string

	FrontendURL        string
	CORSAllowedOrigins []string

	WebAuthnRPID      string
	WebAuthnRPName    string
	WebAuthnRPOrigins []string

	AdminEmail    string
	AdminPassword string

	// RateLimitPerMinute is the max requests per client IP per minute on /auth/* (0 disables).
	RateLimitPerMinute int
	// BcryptCost is the bcrypt work factor.
	BcryptCost int
	// TrustProxy makes the client IP come from X-Forwarded-For (only enable behind a trusted reverse proxy).
	TrustProxy bool
}

// GoogleEnabled reports whether Google sign-in is configured.
func (c *Config) GoogleEnabled() bool { return len(c.GoogleClientIDs) > 0 }

// IsProd reports whether APP_ENV=prod.
func (c *Config) IsProd() bool { return c.AppEnv == "prod" }

// Load reads configuration from the environment, applying dev defaults.
func Load() (*Config, error) {
	c := &Config{
		AppEnv:                   env("APP_ENV", "dev"),
		HTTPPort:                 envInt("HTTP_PORT", 8080),
		DBDriver:                 env("DB_DRIVER", "sqlite"),
		DatabaseURL:              env("DATABASE_URL", "file:data/app.db"),
		JWTSecret:                env("JWT_SECRET", DefaultJWTSecret),
		AccessTokenTTL:           envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:          envDuration("REFRESH_TOKEN_TTL", 720*time.Hour),
		RequireEmailVerification: envBool("REQUIRE_EMAIL_VERIFICATION", true),
		GoogleClientIDs:          envList("GOOGLE_CLIENT_IDS", nil),
		MailDriver:               env("MAIL_DRIVER", "console"),
		SMTPHost:                 env("SMTP_HOST", "localhost"),
		SMTPPort:                 envInt("SMTP_PORT", 1025),
		SMTPUsername:             env("SMTP_USERNAME", ""),
		SMTPPassword:             env("SMTP_PASSWORD", ""),
		MailFrom:                 env("MAIL_FROM", "App <no-reply@example.com>"),
		FrontendURL:              strings.TrimRight(env("FRONTEND_URL", "http://localhost:5173"), "/"),
		CORSAllowedOrigins:       envList("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"}),
		WebAuthnRPID:             env("WEBAUTHN_RP_ID", "localhost"),
		WebAuthnRPName:           env("WEBAUTHN_RP_NAME", "Go REST Template"),
		WebAuthnRPOrigins:        envList("WEBAUTHN_RP_ORIGINS", []string{"http://localhost:5173"}),
		AdminEmail:               strings.ToLower(strings.TrimSpace(env("ADMIN_EMAIL", ""))),
		AdminPassword:            env("ADMIN_PASSWORD", ""),
		RateLimitPerMinute:       envInt("RATE_LIMIT_PER_MINUTE", 60),
		BcryptCost:               envInt("BCRYPT_COST", 12),
		TrustProxy:               envBool("TRUST_PROXY", false),
	}
	return c, c.Validate()
}

// Validate checks the configuration for unsafe or inconsistent values.
func (c *Config) Validate() error {
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return fmt.Errorf("DB_DRIVER must be sqlite or postgres, got %q", c.DBDriver)
	}
	if c.MailDriver != "console" && c.MailDriver != "smtp" {
		return fmt.Errorf("MAIL_DRIVER must be console or smtp, got %q", c.MailDriver)
	}
	if c.IsProd() && c.JWTSecret == DefaultJWTSecret {
		return errors.New("refusing to start in APP_ENV=prod with the default JWT_SECRET")
	}
	if c.IsProd() && len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters in prod")
	}
	return nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return def
}

func envList(key string, def []string) []string {
	raw := env(key, "")
	if raw == "" {
		return def
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
