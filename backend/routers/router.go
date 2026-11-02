// @APIVersion 1.0.0
// @Title Go REST Template API
// @Description JSON API for the SaaS starter template: auth (JWT + rotating refresh tokens), TOTP 2FA, WebAuthn passkeys, profile and admin endpoints. Errors use the envelope {"error":{"code","message","details"}}. Health check: GET /healthz.
// @Contact ling@aidraw.ai
// @License MIT
// @SecurityDefinition BearerAuth apiKey Authorization header "Enter: Bearer <access_token>"
package routers

import (
	"github.com/beego/beego/v2/server/web"

	"github.com/lingster/go-rest-template/backend/controllers"
	"github.com/lingster/go-rest-template/backend/filters"
)

func init() {
	ns := web.NewNamespace("/api/v1",
		web.NSNamespace("/auth",
			web.NSInclude(&controllers.AuthController{}),
		),
		web.NSNamespace("/me",
			web.NSBefore(filters.Authenticate),
			web.NSInclude(&controllers.MeController{}),
		),
		web.NSNamespace("/admin",
			web.NSBefore(filters.Authenticate, filters.RequireAdmin),
			web.NSInclude(&controllers.AdminController{}),
		),
	)
	web.AddNamespace(ns)

	// Public landing page and crawler files.
	web.Router("/", &controllers.LandingController{}, "get:Get")
	web.Router("/robots.txt", &controllers.LandingController{}, "get:Robots")
	web.Router("/sitemap.xml", &controllers.LandingController{}, "get:Sitemap")
	web.Router("/static/landing/theme.css", &controllers.LandingController{}, "get:ThemeCSS")

	web.Router("/healthz", &controllers.HealthController{}, "get:Get")

	// Server-rendered admin UI (static assets are served by the app handler).
	web.Router("/admin-ui", &controllers.AdminUIController{}, "get:Login")
	web.Router("/admin-ui/dashboard", &controllers.AdminUIController{}, "get:Dashboard")
	web.Router("/admin-ui/users", &controllers.AdminUIController{}, "get:Users")
	web.Router("/admin-ui/audit", &controllers.AdminUIController{}, "get:Audit")
}
