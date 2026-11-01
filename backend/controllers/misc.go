package controllers

import (
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/httpx"
)

// HealthController serves /healthz.
type HealthController struct{ BaseController }

// Get reports liveness.
func (c *HealthController) Get() {
	c.OK(dto.HealthResponse{Status: "ok", Version: container.Version})
}

// AdminUIController renders the server-side admin web UI shell. All data is loaded by
// static JS from the JSON API using a bearer token kept in sessionStorage.
type AdminUIController struct{ BaseController }

func (c *AdminUIController) page(name, tpl string) {
	c.Data["Page"] = name
	c.Data["Nonce"] = httpx.NonceFrom(c.Ctx.Request.Context())
	c.TplName = tpl
	c.Ctx.Output.Header("Cache-Control", "no-store")
	if err := c.Render(); err != nil {
		c.Fail(err)
	}
}

// Login renders the login page.
func (c *AdminUIController) Login() { c.page("login", "admin/login.tpl") }

// Dashboard renders the stats page.
func (c *AdminUIController) Dashboard() { c.page("dashboard", "admin/dashboard.tpl") }

// Users renders the user management page.
func (c *AdminUIController) Users() { c.page("users", "admin/users.tpl") }

// Audit renders the audit log page.
func (c *AdminUIController) Audit() { c.page("audit", "admin/audit.tpl") }
