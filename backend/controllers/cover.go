package controllers

import (
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/filters"
)

// CoverDeskController serves the cover desk to reception, cover managers and admins (filters enforce
// the role); the service projects every response for the caller's role.
type CoverDeskController struct{ BaseController }

// Today lists the day's expected supply-teacher arrivals with derived status
// @Title CoverToday
// @Description Reception receives dto.CoverTodayReception (no covering, subject, lessons, notes or attention items); cover managers and admins receive dto.CoverTodayManager.
// @Security BearerAuth
// @Param	date	query	string	false	"YYYY-MM-DD (default: today in SCHOOL_TIMEZONE)"
// @Success 200 {object} dto.CoverTodayManager
// @Failure 403 forbidden
// @Failure 422 validation_error
// @router /today [get]
func (c *CoverDeskController) Today() {
	c.Result(c.C.Bookings.Today(c.CurrentUser(), c.GetString("date")))
}

// Week lists Monday-to-Friday bookings per supply teacher
// @Title CoverWeek
// @Description Reception receives dto.CoverWeekReception (name, agency, kind, status); cover managers and admins receive dto.CoverWeekManager.
// @Security BearerAuth
// @Param	start	query	string	false	"Monday, YYYY-MM-DD (default: this week)"
// @Success 200 {object} dto.CoverWeekManager
// @Failure 403 forbidden
// @Failure 422 validation_error
// @router /week [get]
func (c *CoverDeskController) Week() {
	c.Result(c.C.Bookings.Week(c.CurrentUser(), c.GetString("start")))
}

// SignIn signs a supply teacher in at reception
// @Title CoverSignIn
// @Security BearerAuth
// @Param	id	path	string	true	"Booking id"
// @Param	body	body	dto.SignInRequest	true	"Checks made"
// @Success 201 {object} dto.SignInEvent
// @Failure 404 not_found
// @Failure 409 conflict: not confirmed, not today or already signed in
// @Failure 422 validation_error: photo ID not checked, or DBS certificate required
// @router /bookings/:id/sign-in [post]
func (c *CoverDeskController) SignIn() {
	var req dto.SignInRequest
	if !c.Decode(&req) {
		return
	}
	c.Created(c.C.SignIns.SignIn(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id"), &req))
}

// Correct withdraws a sign-in by appending a correction
// @Title CoverSignInCorrection
// @Security BearerAuth
// @Param	id	path	string	true	"Sign-in event id"
// @Param	body	body	dto.CorrectionRequest	true	"Reason"
// @Success 201 {object} dto.SignInEvent
// @Failure 404 not_found
// @Failure 409 conflict: already withdrawn
// @router /sign-ins/:id/corrections [post]
func (c *CoverDeskController) Correct() {
	var req dto.CorrectionRequest
	if !c.Decode(&req) {
		return
	}
	c.Created(c.C.SignIns.Correct(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id"), &req))
}

// SignIns returns the append-only sign-in log
// @Title CoverSignInLog
// @Security BearerAuth
// @Param	date	query	string	false	"YYYY-MM-DD booking date (default: today in SCHOOL_TIMEZONE)"
// @Success 200 {object} dto.SignInLog
// @Failure 422 validation_error
// @router /sign-ins [get]
func (c *CoverDeskController) SignIns() {
	c.Result(c.C.SignIns.Log(c.CurrentUser(), c.GetString("date")))
}

// CoverManagerController handles agencies, supply teachers and bookings (cover managers and admins).
type CoverManagerController struct{ BaseController }

// Prepare rejects reception (and anyone else who is not a cover manager or admin) with 403.
func (c *CoverManagerController) Prepare() {
	c.BaseController.Prepare()
	if filters.RequireCoverManager(c.Ctx); c.Ctx.ResponseWriter.Started {
		c.StopRun()
	}
}

// CreateAgency creates an agency and returns its API key once
// @Title CoverCreateAgency
// @Security BearerAuth
// @Param	body	body	dto.CreateAgencyRequest	true	"Agency"
// @Success 201 {object} dto.AgencyKeyResponse
// @Failure 403 forbidden
// @Failure 409 conflict: name taken
// @router /agencies [post]
func (c *CoverManagerController) CreateAgency() {
	var req dto.CreateAgencyRequest
	if !c.Decode(&req) {
		return
	}
	c.Created(c.C.Agencies.CreateAgency(c.ReqContext(), c.CurrentUser(), &req))
}

// ListAgencies lists agencies (never keys or key hashes)
// @Title CoverListAgencies
// @Security BearerAuth
// @Success 200 {object} []dto.Agency
// @Failure 403 forbidden
// @router /agencies [get]
func (c *CoverManagerController) ListAgencies() {
	c.Result(c.C.Agencies.ListAgencies())
}

// RotateKey replaces an agency's API key; the old key stops working at once
// @Title CoverRotateAgencyKey
// @Security BearerAuth
// @Param	id	path	string	true	"Agency id"
// @Success 200 {object} dto.AgencyKeyResponse
// @Failure 404 not_found
// @router /agencies/:id/rotate-key [post]
func (c *CoverManagerController) RotateKey() {
	c.Result(c.C.Agencies.RotateKey(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id")))
}

// CreateTeacher registers a supply teacher with an agency
// @Title CoverCreateTeacher
// @Security BearerAuth
// @Param	body	body	dto.CreateTeacherRequest	true	"Teacher"
// @Success 201 {object} dto.SupplyTeacher
// @Failure 422 validation_error
// @router /teachers [post]
func (c *CoverManagerController) CreateTeacher() {
	var req dto.CreateTeacherRequest
	if !c.Decode(&req) {
		return
	}
	c.Created(c.C.Agencies.CreateTeacher(c.CurrentUser(), &req))
}

// ListTeachers lists supply teachers
// @Title CoverListTeachers
// @Security BearerAuth
// @Param	agency_id	query	string	false	"Only this agency's teachers"
// @Success 200 {object} []dto.SupplyTeacher
// @router /teachers [get]
func (c *CoverManagerController) ListTeachers() {
	c.Result(c.C.Agencies.ListTeachers(c.GetString("agency_id")))
}

// CreateBooking books an agency for one school day
// @Title CoverCreateBooking
// @Security BearerAuth
// @Param	body	body	dto.CreateBookingRequest	true	"Booking with one lesson per period of its kind"
// @Success 201 {object} dto.Booking
// @Failure 403 forbidden
// @Failure 422 validation_error
// @router /bookings [post]
func (c *CoverManagerController) CreateBooking() {
	var req dto.CreateBookingRequest
	if !c.Decode(&req) {
		return
	}
	c.Created(c.C.Bookings.Create(c.ReqContext(), c.CurrentUser(), &req))
}

// UpdateBooking confirms, cancels or edits a booking
// @Title CoverUpdateBooking
// @Security BearerAuth
// @Param	id	path	string	true	"Booking id"
// @Param	body	body	dto.UpdateBookingRequest	true	"Changes"
// @Success 200 {object} dto.Booking
// @Failure 404 not_found
// @Failure 409 conflict: cancelled, signed in, or changed meanwhile
// @Failure 422 validation_error: e.g. teacher of another agency
// @router /bookings/:id [patch]
func (c *CoverManagerController) UpdateBooking() {
	var req dto.UpdateBookingRequest
	if !c.Decode(&req) {
		return
	}
	c.Result(c.C.Bookings.Update(c.ReqContext(), c.CurrentUser(), c.Ctx.Input.Param(":id"), &req))
}
