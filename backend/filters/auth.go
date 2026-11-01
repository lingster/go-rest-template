package filters

import (
	"strings"

	"github.com/beego/beego/v2/server/web/context"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/httpx"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// CurrentUserKey is the ctx.Input data key under which the authenticated *models.User is stored.
const CurrentUserKey = "current_user"

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
	ctx.Input.SetData(CurrentUserKey, u)
}

// RequireAdmin must run after Authenticate; it rejects non-admins with 403.
func RequireAdmin(ctx *context.Context) {
	u, ok := ctx.Input.GetData(CurrentUserKey).(*models.User)
	if !ok || u.Role != models.RoleAdmin {
		httpx.WriteError(ctx.ResponseWriter, apperr.Forbidden("Admin role required"))
	}
}
