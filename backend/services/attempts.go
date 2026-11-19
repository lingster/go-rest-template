package services

import (
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/ratelimit"
)

// Per-account limits that hold regardless of how many IPs an attacker uses: the per-IP limiter alone
// lets a botnet (or one IPv6 block) brute-force a 6-digit TOTP code in minutes.
const (
	// FactorAttemptLimit is the number of second-factor or current-password verifications allowed
	// per account and kind per FactorAttemptWindow.
	FactorAttemptLimit  = 10
	FactorAttemptWindow = 15 * time.Minute
	// MailLimit caps emails triggered by unauthenticated endpoints per address per MailWindow
	// (stops using forgot-password / resend-verification to flood an inbox).
	MailLimit  = 5
	MailWindow = time.Hour
	// Password login counts failed password checks only. Per (email, client): a guessing client is
	// stopped quickly while the real user signing in from elsewhere is unaffected, so nobody can lock
	// another user out. Per email alone: a high ceiling that bounds distributed guessing.
	LoginClientFailLimit   = 10
	LoginClientFailWindow  = 15 * time.Minute
	LoginAccountFailLimit  = 100
	LoginAccountFailWindow = time.Hour
)

// Attempt kinds (limiter key prefixes).
const (
	attemptFactor   = "factor"
	attemptPassword = "password"
)

// NewFactorLimiter builds the per-account verification limiter.
func NewFactorLimiter() ratelimit.Limiter {
	return ratelimit.NewFixedWindow(FactorAttemptLimit, FactorAttemptWindow)
}

// NewMailLimiter builds the per-address outbound mail limiter.
func NewMailLimiter() ratelimit.Limiter { return ratelimit.NewFixedWindow(MailLimit, MailWindow) }

// LoginThrottle limits failed password logins; only failures consume budget. A nil throttle disables it.
// Checking and recording are separate steps, so a burst of concurrent requests can overshoot a bucket by
// the number of requests in flight; the per-IP middleware limit bounds that.
type LoginThrottle struct {
	perClient  ratelimit.FailureCounter // keyed by email + client
	perAccount ratelimit.FailureCounter // keyed by email only
}

// NewLoginThrottle builds the two failure buckets.
func NewLoginThrottle() *LoginThrottle {
	return &LoginThrottle{
		perClient:  ratelimit.NewFixedWindow(LoginClientFailLimit, LoginClientFailWindow),
		perAccount: ratelimit.NewFixedWindow(LoginAccountFailLimit, LoginAccountFailWindow),
	}
}

// loginClientKey scopes an email to one client (IPv6 clients by /64, see ratelimit.ClientKey). The client
// part never contains "|", so distinct (email, client) pairs cannot collide.
func loginClientKey(email, ip string) string { return email + "|" + ratelimit.ClientKey(ip) }

// Check returns 429 rate_limited when either bucket is exhausted, without consuming budget. It runs before
// the password is verified, for unknown emails too, so the response never reveals whether an account exists.
func (t *LoginThrottle) Check(email, ip string) error {
	if t != nil && (t.perClient.Blocked(loginClientKey(email, ip)) || t.perAccount.Blocked(email)) {
		return apperr.RateLimited()
	}
	return nil
}

// Fail records a failed password check.
func (t *LoginThrottle) Fail(email, ip string) {
	if t != nil {
		t.perClient.Fail(loginClientKey(email, ip))
		t.perAccount.Fail(email)
	}
}

// Succeed clears the client's bucket after a correct password. The account-wide bucket is left alone:
// it bounds distributed guessing and must not be resettable by whoever happens to know the password.
func (t *LoginThrottle) Succeed(email, ip string) {
	if t != nil {
		t.perClient.Reset(loginClientKey(email, ip))
	}
}

// checkAttempt records one verification attempt of kind for the user and returns 429 rate_limited
// once the account is over its budget. A nil limiter disables the check.
func checkAttempt(l ratelimit.Limiter, kind, userID string) error {
	if l != nil && !l.Allow(kind+":"+userID) {
		return apperr.RateLimited()
	}
	return nil
}
