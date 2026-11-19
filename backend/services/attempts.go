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

// checkAttempt records one verification attempt of kind for the user and returns 429 rate_limited
// once the account is over its budget. A nil limiter disables the check.
func checkAttempt(l ratelimit.Limiter, kind, userID string) error {
	if l != nil && !l.Allow(kind+":"+userID) {
		return apperr.RateLimited()
	}
	return nil
}
