package main

import (
	"strings"
	"testing"

	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/lingster/go-rest-template/backend/testutil"
)

func registerBody(email, name string) map[string]string {
	return map[string]string{"email": email, "password": password, "name": name}
}

func TestRegisterResponseIsIdenticalForExistingEmail(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "enum")
	fresh := call(t, "POST", "/api/v1/auth/register", registerBody(testutil.Email("enum-new"), "New"), "")
	again := call(t, "POST", "/api/v1/auth/register", registerBody(strings.ToUpper(email), "Impostor"), "")
	expect(t, fresh, 202, "")
	expect(t, again, 202, "")
	if string(fresh.Raw) != string(again.Raw) {
		t.Fatalf("responses differ:\nnew:      %s\nexisting: %s", fresh.Raw, again.Raw)
	}
	if len(again.Body) != 1 || again.str("message") == "" {
		t.Fatalf("response must only carry a message: %s", again.Raw)
	}
}

func TestRegisterExistingEmailLeavesAccountUntouchedAndNotifiesOwner(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "enumown")
	before := mailer.Count(email)
	expect(t, call(t, "POST", "/api/v1/auth/register", map[string]string{"email": email, "password": "another password 9", "name": "Impostor"}, ""), 202, "")

	// the attacker's password and name were not applied; the real password still works
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "another password 9"}, ""), 401, "invalid_credentials")
	login := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	expect(t, login, 200, "")
	if login.str("user", "name") != "Test" {
		t.Fatalf("account was modified: %s", login.Raw)
	}

	// the owner got an "already registered" notice with sign-in and reset links, and no token
	if got := mailer.Count(email) - before; got != 1 {
		t.Fatalf("expected 1 mail to the owner, got %d", got)
	}
	msg := mailer.LastTo(email)
	for _, body := range []string{msg.Text, msg.HTML} {
		if !strings.Contains(body, "http://localhost:5173/login") || !strings.Contains(body, "http://localhost:5173/forgot-password") {
			t.Fatalf("mail lacks sign-in/reset links (%q): %s", msg.Subject, body)
		}
		if strings.Contains(body, "token=") {
			t.Fatalf("notice must not carry a token: %s", body)
		}
	}
	if !contains(auditActions(t, login.str("user", "id")), services.AuditRegisterExisting) {
		t.Fatal("missing audit entry for the repeated registration")
	}
}

func TestRegisterExistingUnverifiedAccountIsNotChanged(t *testing.T) {
	email := testutil.Email("enumunv")
	expect(t, call(t, "POST", "/api/v1/auth/register", registerBody(email, "First"), ""), 202, "")
	expect(t, call(t, "POST", "/api/v1/auth/register", map[string]string{"email": email, "password": "another password 9", "name": "Second"}, ""), 202, "")
	// still unverified, with the original password only
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": password}, ""), 403, "email_not_verified")
	expect(t, call(t, "POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "another password 9"}, ""), 401, "invalid_credentials")
}

func TestRegisterExistingNoticeIsThrottledPerAddress(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "enumflood")
	before := mailer.Count(email)
	for i := 0; i < services.MailLimit+3; i++ {
		expect(t, call(t, "POST", "/api/v1/auth/register", registerBody(email, "Spam"), ""), 202, "")
	}
	if got := mailer.Count(email) - before; got != services.MailLimit {
		t.Fatalf("expected %d notices, got %d", services.MailLimit, got)
	}
}

func TestRegisterExistingDisabledAccountSendsNothing(t *testing.T) {
	email, _, _ := newVerifiedUser(t, "enumdis")
	c := container.Get()
	u, err := c.Repos.Users.GetByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	u.IsActive = false
	if err := c.Repos.Users.Update(u, "IsActive"); err != nil {
		t.Fatal(err)
	}
	before := mailer.Count(email)
	expect(t, call(t, "POST", "/api/v1/auth/register", registerBody(email, "X"), ""), 202, "")
	if got := mailer.Count(email) - before; got != 0 {
		t.Fatalf("disabled account must not be mailed, got %d", got)
	}
}
