package controllers

import (
	"github.com/lingster/go-rest-template/backend/dto"
)

// AgencyController is the API for supply agencies (X-Agency-Key enforced by filters.AuthenticateAgency).
type AgencyController struct{ BaseController }

// Bookings lists the calling agency's bookings
// @Title AgencyBookings
// @Security AgencyKey
// @Param	date	query	string	false	"YYYY-MM-DD (default: today in SCHOOL_TIMEZONE)"
// @Success 200 {object} dto.AgencyBookingList
// @Failure 401 unauthorized
// @Failure 429 rate_limited
// @router /bookings [get]
func (c *AgencyController) Bookings() {
	c.Result(c.C.AgencyPortal.Bookings(c.CurrentAgency(), c.GetString("date")))
}

// ReportETA reports a supply teacher's estimated arrival (an ETA only, never a location)
// @Title AgencyReportETA
// @Security AgencyKey
// @Param	id	path	string	true	"Booking id"
// @Param	body	body	dto.EtaRequest	true	"ETA"
// @Success 201 {object} dto.EtaUpdate
// @Failure 401 unauthorized
// @Failure 404 not_found: unknown booking or another agency's
// @Failure 409 conflict: not confirmed, not today or already signed in
// @Failure 422 validation_error: ETA outside 06:00-16:00 on the booking date or over 10 minutes in the past
// @Failure 429 rate_limited: more than one update per 30s or 100 per booking
// @router /bookings/:id/eta [post]
func (c *AgencyController) ReportETA() {
	var req dto.EtaRequest
	if !c.Decode(&req) {
		return
	}
	c.Created(c.C.AgencyPortal.ReportETA(c.ReqContext(), c.CurrentAgency(), c.Ctx.Input.Param(":id"), &req))
}
