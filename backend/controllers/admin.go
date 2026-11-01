package controllers

import (
	"github.com/lingster/go-rest-template/backend/dto"
)

// AdminController handles admin-only endpoints (role=admin enforced by filters).
type AdminController struct{ BaseController }

// Stats returns dashboard counters
// @Title Stats
// @Security BearerAuth
// @Success 200 {object} dto.Stats
// @Failure 403 forbidden
// @router /stats [get]
func (c *AdminController) Stats() {
	s, err := c.C.Admin.Stats()
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(s)
}

// ListUsers lists users
// @Title ListUsers
// @Security BearerAuth
// @Param	page	query	int	false	"Page (default 1)"
// @Param	per_page	query	int	false	"Page size (default 20, max 100)"
// @Param	q	query	string	false	"Search email or name"
// @Success 200 {object} dto.UserPage
// @router /users [get]
func (c *AdminController) ListUsers() {
	page, _ := c.GetInt("page", 1)
	per, _ := c.GetInt("per_page", 20)
	res, err := c.C.Admin.ListUsers(c.GetString("q"), page, per)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(res)
}

// GetUser returns a user
// @Title GetUser
// @Security BearerAuth
// @Param	id	path	string	true	"User id"
// @Success 200 {object} dto.User
// @Failure 404 not_found
// @router /users/:id [get]
func (c *AdminController) GetUser() {
	u, err := c.C.Admin.GetUser(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(u)
}

// UpdateUser patches a user
// @Title UpdateUser
// @Security BearerAuth
// @Param	id	path	string	true	"User id"
// @Param	body	body	dto.AdminUpdateUserRequest	true	"Fields to change"
// @Success 200 {object} dto.User
// @Failure 403 forbidden
// @router /users/:id [patch]
func (c *AdminController) UpdateUser() {
	var req dto.AdminUpdateUserRequest
	if !c.Decode(&req) {
		return
	}
	u, err := c.C.Admin.UpdateUser(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id"), &req)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(u)
}

// DeleteUser deletes a user
// @Title DeleteUser
// @Security BearerAuth
// @Param	id	path	string	true	"User id"
// @Success 204 no content
// @Failure 403 forbidden: cannot delete self
// @router /users/:id [delete]
func (c *AdminController) DeleteUser() {
	if err := c.C.Admin.DeleteUser(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id")); err != nil {
		c.Fail(err)
		return
	}
	c.NoContent()
}

// RevokeSessions revokes all refresh tokens of a user
// @Title RevokeSessions
// @Security BearerAuth
// @Param	id	path	string	true	"User id"
// @Success 204 no content
// @router /users/:id/revoke-sessions [post]
func (c *AdminController) RevokeSessions() {
	if err := c.C.Admin.RevokeSessions(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id")); err != nil {
		c.Fail(err)
		return
	}
	c.NoContent()
}

// AuditLogs lists audit entries
// @Title AuditLogs
// @Security BearerAuth
// @Param	page	query	int	false	"Page (default 1)"
// @Param	per_page	query	int	false	"Page size (default 20, max 100)"
// @Param	user_id	query	string	false	"Filter by subject user id"
// @Success 200 {object} dto.AuditLogPage
// @router /audit-logs [get]
func (c *AdminController) AuditLogs() {
	page, _ := c.GetInt("page", 1)
	per, _ := c.GetInt("per_page", 20)
	res, err := c.C.Admin.ListAuditLogs(c.GetString("user_id"), page, per)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(res)
}
