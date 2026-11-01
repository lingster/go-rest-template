// Package testutil provides helpers shared by unit and integration tests.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/database"
	"github.com/lingster/go-rest-template/backend/mail"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// Config returns a fast, test-friendly configuration.
func Config() *config.Config {
	return &config.Config{
		AppEnv: "test", HTTPPort: 0, DBDriver: "sqlite", JWTSecret: "test-secret-test-secret-test-secret-0123",
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 720 * time.Hour, RequireEmailVerification: true,
		MailDriver: "console", FrontendURL: "http://localhost:5173",
		CORSAllowedOrigins: []string{"http://localhost:5173"},
		WebAuthnRPID:       "localhost", WebAuthnRPName: "Test", WebAuthnRPOrigins: []string{"http://localhost:5173"},
		RateLimitPerMinute: 100000, BcryptCost: 4,
	}
}

var (
	dbOnce sync.Once
	dbErr  error
	dbDir  string
)

// InitDB creates the process-wide temp SQLite database (once) and returns its DSN.
func InitDB(t testing.TB) string {
	t.Helper()
	dbOnce.Do(func() {
		dbDir, dbErr = os.MkdirTemp("", "gorest-test-*")
		if dbErr == nil {
			dbErr = database.Init("sqlite", "file:"+filepath.Join(dbDir, "test.db"))
		}
	})
	if dbErr != nil {
		t.Fatalf("init test db: %v", dbErr)
	}
	return dbDir
}

// NewContainer wires a container on the shared test DB with a capturing mailer.
func NewContainer(t testing.TB) (*container.Container, *MemMailer) {
	t.Helper()
	InitDB(t)
	m := &MemMailer{}
	c, err := container.New(Config(), repositories.NewORMSet(), m)
	if err != nil {
		t.Fatal(err)
	}
	return c, m
}

var counter atomic.Int64

// Email returns a unique email address.
func Email(prefix string) string { return fmt.Sprintf("%s-%d@example.com", prefix, counter.Add(1)) }

// MemMailer records sent messages.
type MemMailer struct {
	mu   sync.Mutex
	Sent []mail.Message
}

func (m *MemMailer) Send(_ context.Context, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, msg)
	return nil
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_\-%]+)`)

// LastToken returns the token from the most recent email to the address whose subject contains subjectPart.
func (m *MemMailer) LastToken(to, subjectPart string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.Sent) - 1; i >= 0; i-- {
		s := m.Sent[i]
		if s.To == to && regexp.MustCompile(subjectPart).MatchString(s.Subject) {
			if g := tokenRe.FindStringSubmatch(s.Text); g != nil {
				return g[1]
			}
		}
	}
	return ""
}

// Count returns the number of messages sent to the address.
func (m *MemMailer) Count(to string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.Sent {
		if s.To == to {
			n++
		}
	}
	return n
}
