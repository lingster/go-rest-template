package filters

import (
	"strings"

	"github.com/beego/beego/v2/server/web/context"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/httpx"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/ratelimit"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// CurrentUserKey is the ctx.Input data key under which the authenticated *models.User is stored.
const CurrentUserKey = "current_user"

// ClaimsKey is the ctx.Input data key under which the verified access token *services.Claims are stored.
const ClaimsKey = "access_claims"

// Authenticate is a Beego filter requiring a valid Bearer access token of an active user.
// The user is loaded from the DB so role/active changes apply immediately.
func Authenticate(ctx *context.Context) {
	c := container.Get()
	h := ctx.Input.Header("Authorization")
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		httpx.WriteError(ctx.ResponseWriter, apperr.Unauthorized("Missing bearer token"))
		return
	}
	claims, err := c.Tokens.ParseAccess(strings.TrimSpace(h[7:]))
	if err != nil {
		httpx.WriteError(ctx.ResponseWriter, err)
		return
	}
	u, err := c.Repos.Users.GetByID(claims.Subject)
	if err == repositories.ErrNotFound {
		httpx.WriteError(ctx.ResponseWriter, apperr.Unauthorized("User no longer exists"))
		return
	} else if err != nil {
		httpx.WriteError(ctx.ResponseWriter, err)
		return
	}
	if !u.IsActive {
		httpx.WriteError(ctx.ResponseWriter, apperr.AccountDisabled())
		return
	}
	if claims.Ver != u.TokenVersion { // signed out everywhere since this token was issued
		httpx.WriteError(ctx.ResponseWriter, apperr.Unauthorized("Session has been revoked"))
		return
	}
	ctx.Input.SetData(CurrentUserKey, u)
	ctx.Input.SetData(ClaimsKey, claims)
}

// requireRole must run after Authenticate; it rejects users whose role is not allowed with 403.
func requireRole(ctx *context.Context, allowed func(role string) bool, msg string) {
	u, ok := ctx.Input.GetData(CurrentUserKey).(*models.User)
	if !ok || !allowed(u.Role) {
		httpx.WriteError(ctx.ResponseWriter, apperr.Forbidden(msg))
	}
}

// RequireAdmin must run after Authenticate; it rejects non-admins with 403.
func RequireAdmin(ctx *context.Context) {
	requireRole(ctx, func(r string) bool { return r == models.RoleAdmin }, "Admin role required")
}

// RequireCoverStaff must run after Authenticate; it admits reception, cover managers and admins.
func RequireCoverStaff(ctx *context.Context) {
	requireRole(ctx, models.IsCoverStaff, "Cover desk role required")
}

// RequireCoverManager must run after Authenticate; it admits cover managers and admins.
func RequireCoverManager(ctx *context.Context) {
	requireRole(ctx, models.IsCoverManager, "Cover manager role required")
}

// AgencyKeyHeader carries an agency's API key.
const AgencyKeyHeader = "X-Agency-Key"

// CurrentAgencyKey is the ctx.Input data key under which the authenticated *models.Agency is stored.
const CurrentAgencyKey = "current_agency"

// AuthenticateAgency is a Beego filter requiring a valid X-Agency-Key of an active agency. Every failure
// gets the same 401, so a response never reveals whether a key exists or was rotated out.
func AuthenticateAgency(ctx *context.Context) {
	c := container.Get()
	client := ratelimit.ClientKey(httpx.ClientIP(ctx.Request, c.Cfg.TrustProxy))
	a, err := c.Agencies.Authenticate(client, ctx.Input.Header(AgencyKeyHeader))
	if err != nil {
		httpx.WriteError(ctx.ResponseWriter, err)
		return
	}
	ctx.Input.SetData(CurrentAgencyKey, a)
}
