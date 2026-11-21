// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultJWTSecret is the insecure development secret; prod refuses to start with it.
const DefaultJWTSecret = "dev-insecure-change-me-please-0123456789"

// DefaultAdminPassword is the placeholder from .env.example / docker-compose; prod refuses to seed it.
const DefaultAdminPassword = "change-me-please"

// DefaultSchoolTimezone is the default SCHOOL_TIMEZONE.
const DefaultSchoolTimezone = "Europe/London"

// MinProdAdminPasswordLen is the minimum seeded admin password length in prod.
const MinProdAdminPasswordLen = 12

// Config holds all runtime configuration.
type Config struct {
	AppEnv   string
	HTTPPort int

	DBDriver    string
	DatabaseURL string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	// ReauthMaxAge is how recent the last sign-in must be to add a passkey or set up TOTP
	// (a refreshed session keeps its original sign-in time).
	ReauthMaxAge time.Duration

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
	// SMTPRequireTLS refuses to send mail over a connection that could not be upgraded with STARTTLS
	// (SMTP_REQUIRE_TLS, default true when APP_ENV=prod).
	SMTPRequireTLS bool

	FrontendURL        string
	CORSAllowedOrigins []string

	// CookieSecure marks the webapp's refresh cookie Secure and names it __Secure-rt
	// (COOKIE_SECURE, default true when APP_ENV=prod; plain-HTTP dev needs false).
	CookieSecure bool
	// CookieDomain is the refresh cookie Domain (COOKIE_DOMAIN). Empty (default) is host-only, which is
	// right whenever browsers call the API on a single host; set a parent domain (example.com) only if the
	// API is reached on several sibling hosts.
	CookieDomain string

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

	// SwaggerEnabled serves the API spec and Swagger UI at /swagger/ (SWAGGER_ENABLED, default off in prod).
	SwaggerEnabled bool

	// SchoolTimezone is the IANA zone the school day is in (SCHOOL_TIMEZONE, default Europe/London). Booking
	// dates, period times and ETA windows are school-local; timestamps are stored and returned in UTC.
	SchoolTimezone string
	// AgencyRateLimitPerMinute is the max requests per agency API key per minute on /agency/* (0 disables).
	AgencyRateLimitPerMinute int

	// LandingStoreLinksLive shows the App Store / Google Play buttons on the landing page
	// (LANDING_STORE_LINKS_LIVE); off until the apps are public, when "Coming soon" is shown instead.
	LandingStoreLinksLive bool
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
		ReauthMaxAge:             envDuration("REAUTH_MAX_AGE", 10*time.Minute),
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
		CookieDomain:             env("COOKIE_DOMAIN", ""),
		WebAuthnRPID:             env("WEBAUTHN_RP_ID", "localhost"),
		WebAuthnRPName:           env("WEBAUTHN_RP_NAME", "Go REST Template"),
		WebAuthnRPOrigins:        envList("WEBAUTHN_RP_ORIGINS", []string{"http://localhost:5173"}),
		AdminEmail:               strings.ToLower(strings.TrimSpace(env("ADMIN_EMAIL", ""))),
		AdminPassword:            env("ADMIN_PASSWORD", ""),
		RateLimitPerMinute:       envInt("RATE_LIMIT_PER_MINUTE", 60),
		BcryptCost:               envInt("BCRYPT_COST", 12),
		TrustProxy:               envBool("TRUST_PROXY", false),
		LandingStoreLinksLive:    envBool("LANDING_STORE_LINKS_LIVE", false),
		SchoolTimezone:           env("SCHOOL_TIMEZONE", DefaultSchoolTimezone),
		AgencyRateLimitPerMinute: envInt("AGENCY_RATE_LIMIT_PER_MINUTE", 120),
	}
	c.SMTPRequireTLS = envBool("SMTP_REQUIRE_TLS", c.IsProd())
	c.SwaggerEnabled = envBool("SWAGGER_ENABLED", !c.IsProd())
	c.CookieSecure = envBool("COOKIE_SECURE", c.IsProd())
	return c, c.Validate()
}

// Validate checks the configuration for unsafe or inconsistent values.
func (c *Config) Validate() error {
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return fmt.Errorf("DB_DRIVER must be sqlite or postgres, got %q", c.DBDriver)
	}
	if _, err := c.SchoolLocation(); err != nil {
		return err
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
	if c.IsProd() && c.AdminPassword != "" &&
		(c.AdminPassword == DefaultAdminPassword || len(c.AdminPassword) < MinProdAdminPasswordLen) {
		return fmt.Errorf("ADMIN_PASSWORD must not be the example value and must be at least %d characters in prod",
			MinProdAdminPasswordLen)
	}
	// The CORS allow-list doubles as the CSRF origin check for the webapp's refresh cookie, and native
	// clients do not use CORS at all, so prod must name its browser origins.
	if c.IsProd() && slices.Contains(c.CORSAllowedOrigins, "*") {
		return errors.New("CORS_ALLOWED_ORIGINS must list explicit origins in prod, not *")
	}
	return nil
}

// SchoolLocation loads SCHOOL_TIMEZONE. An empty value is rejected rather than silently meaning UTC.
func (c *Config) SchoolLocation() (*time.Location, error) {
	if strings.TrimSpace(c.SchoolTimezone) == "" {
		return nil, errors.New("SCHOOL_TIMEZONE must be set (e.g. Europe/London)")
	}
	loc, err := time.LoadLocation(c.SchoolTimezone)
	if err != nil {
		return nil, fmt.Errorf("SCHOOL_TIMEZONE %q is not a known time zone: %w", c.SchoolTimezone, err)
	}
	return loc, nil
}

// Warnings lists risky but legal settings, logged at startup.
func (c *Config) Warnings() []string {
	var w []string
	if !c.IsProd() {
		return w
	}
	if c.MailDriver == "console" {
		w = append(w, "MAIL_DRIVER=console in prod writes password-reset and verification links to the logs")
	}
	if !c.SMTPRequireTLS && c.MailDriver == "smtp" {
		w = append(w, "SMTP_REQUIRE_TLS=false in prod: reset links may be sent unencrypted")
	}
	if !c.CookieSecure {
		w = append(w, "COOKIE_SECURE=false in prod: the webapp's refresh cookie may be sent over plain HTTP")
	}
	for _, o := range c.WebAuthnRPOrigins {
		if strings.HasPrefix(o, "http://") {
			w = append(w, "WEBAUTHN_RP_ORIGINS contains a non-HTTPS origin in prod: "+o)
		}
	}
	if c.SwaggerEnabled {
		w = append(w, "SWAGGER_ENABLED=true in prod publishes the API surface at /swagger/")
	}
	if c.AdminPassword != "" {
		w = append(w, "ADMIN_PASSWORD is set: remove it from the environment once the admin exists")
	}
	return w
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
