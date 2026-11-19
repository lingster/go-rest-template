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

func TestSwaggerEnabledDefaultsByEnvironment(t *testing.T) {
	cases := []struct {
		env, override string
		want          bool
	}{
		{"dev", "", true}, {"test", "", true}, {"prod", "", false},
		{"prod", "true", true}, {"dev", "false", false},
		{"prod", "not-a-bool", false}, // unparsable values fall back to the environment default
	}
	for _, c := range cases {
		t.Setenv("APP_ENV", c.env)
		t.Setenv("SWAGGER_ENABLED", c.override)
		if c.env == "prod" {
			t.Setenv("JWT_SECRET", strings.Repeat("s", 40))
		}
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		if cfg.SwaggerEnabled != c.want {
			t.Errorf("APP_ENV=%s SWAGGER_ENABLED=%q: got %v, want %v", c.env, c.override, cfg.SwaggerEnabled, c.want)
		}
	}
}

func TestProdWarnsWhenSwaggerEnabled(t *testing.T) {
	c := prod()
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("swagger off must not warn: %v", w)
	}
	c.SwaggerEnabled = true
	w := c.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], "SWAGGER_ENABLED") {
		t.Fatalf("expected a swagger warning, got %v", w)
	}
	c.AppEnv = "dev"
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("dev must not warn: %v", w)
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
