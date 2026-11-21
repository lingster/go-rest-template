// @APIVersion 1.0.0
// @Title Go REST Template API
// @Description JSON API for the SaaS starter template: auth (JWT + rotating refresh tokens), TOTP 2FA, WebAuthn passkeys, profile and admin endpoints, and the school cover desk (supply bookings, agency ETAs, reception sign-in log). Errors use the envelope {"error":{"code","message","details"}}. Health check: GET /healthz.
// @Contact ling@aidraw.ai
// @License MIT
// @SecurityDefinition BearerAuth apiKey Authorization header "Enter: Bearer <access_token>"
// @SecurityDefinition AgencyKey apiKey X-Agency-Key header "Supply agency API key (cover desk)"
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
		// Cover desk: reception + cover managers; CoverManagerController further requires a cover manager.
		web.NSNamespace("/cover",
			web.NSBefore(filters.Authenticate, filters.RequireCoverStaff),
			web.NSInclude(&controllers.CoverDeskController{}, &controllers.CoverManagerController{}),
		),
		// Supply agencies (API key).
		web.NSNamespace("/agency",
			web.NSBefore(filters.AuthenticateAgency),
			web.NSInclude(&controllers.AgencyController{}),
		),
	)
	web.AddNamespace(ns)

	// Public landing page and crawler files.
	web.Router("/", &controllers.LandingController{}, "get,head:Get")
	web.Router("/robots.txt", &controllers.LandingController{}, "get,head:Robots")
	web.Router("/sitemap.xml", &controllers.LandingController{}, "get,head:Sitemap")
	web.Router("/static/landing/theme.css", &controllers.LandingController{}, "get,head:ThemeCSS")

	web.Router("/healthz", &controllers.HealthController{}, "get,head:Get")

	// Server-rendered admin UI (static assets are served by the app handler).
	web.Router("/admin-ui", &controllers.AdminUIController{}, "get:Login")
	web.Router("/admin-ui/dashboard", &controllers.AdminUIController{}, "get:Dashboard")
	web.Router("/admin-ui/users", &controllers.AdminUIController{}, "get:Users")
	web.Router("/admin-ui/audit", &controllers.AdminUIController{}, "get:Audit")
}
