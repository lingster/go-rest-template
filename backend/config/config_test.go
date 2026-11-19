package config_test

import (
	"strings"
	"testing"

	"github.com/lingster/go-rest-template/backend/config"
)

func prod() *config.Config {
	return &config.Config{AppEnv: "prod", DBDriver: "postgres", MailDriver: "smtp", SMTPRequireTLS: true,
		JWTSecret: strings.Repeat("s", 40)}
}

func TestProdRejectsWeakAdminPassword(t *testing.T) {
	for _, pw := range []string{config.DefaultAdminPassword, "short-pw"} {
		c := prod()
		c.AdminPassword = pw
		if err := c.Validate(); err == nil {
			t.Errorf("ADMIN_PASSWORD %q must be rejected in prod", pw)
		}
	}
	c := prod()
	c.AdminPassword = "a-long-unique-admin-password"
	if err := c.Validate(); err != nil {
		t.Errorf("strong password rejected: %v", err)
	}
	c.AdminPassword = ""
	if err := c.Validate(); err != nil {
		t.Errorf("no seeding must be fine: %v", err)
	}
}

func TestProdWarnings(t *testing.T) {
	c := prod()
	c.MailDriver = "console"
	c.CORSAllowedOrigins = []string{"*"}
	c.WebAuthnRPOrigins = []string{"http://app.example.com"}
	if w := c.Warnings(); len(w) != 3 {
		t.Fatalf("expected 3 warnings, got %v", w)
	}
	c.AppEnv = "dev"
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("dev must not warn: %v", w)
	}
}
