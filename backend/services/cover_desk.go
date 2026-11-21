package services

import (
	"errors"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// Cover-desk audit actions.
const (
	AuditAgencyCreate     = "cover.agency.create"
	AuditAgencyKeyRotate  = "cover.agency.rotate_key"
	AuditBookingCreate    = "cover.booking.create"
	AuditBookingConfirm   = "cover.booking.confirm"
	AuditBookingCancel    = "cover.booking.cancel"
	AuditBookingUpdate    = "cover.booking.update"
	AuditSignIn           = "cover.signin"
	AuditSignInCorrection = "cover.signin.correction"
)

// CoverDeps is what every cover-desk service needs. Now is the clock (injected so tests are deterministic).
type CoverDeps struct {
	Repos *repositories.Set
	Day   SchoolDay
	Now   func() time.Time
	Audit *AuditService
}

// today returns the current school-local date.
func (c CoverDeps) today() string { return c.Day.LocalDate(c.Now()) }

// dateOrToday validates an optional YYYY-MM-DD query parameter, defaulting to today.
func (c CoverDeps) dateOrToday(field, date string) (string, error) {
	if date == "" {
		return c.today(), nil
	}
	if !dto.IsDate(date) {
		return "", apperr.Validation(map[string]string{field: "must be YYYY-MM-DD"})
	}
	return date, nil
}

// booking loads a booking, mapping a missing row to not_found.
func (c CoverDeps) booking(id string) (*models.Booking, error) {
	b, err := c.Repos.Bookings.GetByID(id)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, apperr.NotFound("Booking not found")
	}
	return b, err
}

// arrivals assembles agencies, teachers, lessons, ETA updates and sign-in events for bookings with one
// query per table.
func (c CoverDeps) arrivals(bookings []*models.Booking) ([]Arrival, error) {
	ids := make([]string, len(bookings))
	var agencyIDs, teacherIDs []string
	for i, b := range bookings {
		ids[i] = b.ID
		agencyIDs = append(agencyIDs, b.AgencyID)
		if b.SupplyTeacherID != "" {
			teacherIDs = append(teacherIDs, b.SupplyTeacherID)
		}
	}
	agencies, err := c.Repos.Agencies.GetMany(agencyIDs)
	if err != nil {
		return nil, err
	}
	teachers, err := c.Repos.Teachers.GetMany(teacherIDs)
	if err != nil {
		return nil, err
	}
	lessons, err := c.Repos.Bookings.LessonsByBookings(ids)
	if err != nil {
		return nil, err
	}
	etas, err := c.Repos.EtaUpdates.ListByBookings(ids)
	if err != nil {
		return nil, err
	}
	events, err := c.Repos.SignIns.ListByBookings(ids)
	if err != nil {
		return nil, err
	}
	out := make([]Arrival, len(bookings))
	for i, b := range bookings {
		ag := agencies[b.AgencyID]
		if ag == nil {
			ag = &models.Agency{ID: b.AgencyID}
		}
		out[i] = Arrival{Booking: b, Agency: ag, Teacher: teachers[b.SupplyTeacherID], Lessons: lessons[b.ID],
			ETAs: etas[b.ID], SignIns: events[b.ID]}
	}
	return out, nil
}

// arrival assembles one booking's arrival.
func (c CoverDeps) arrival(b *models.Booking) (Arrival, error) {
	list, err := c.arrivals([]*models.Booking{b})
	if err != nil {
		return Arrival{}, err
	}
	return list[0], nil
}

// requireCoverManager is the service-side check behind the controller guard (defence in depth).
func requireCoverManager(actor *models.User) error {
	if actor == nil || !models.IsCoverManager(actor.Role) {
		return apperr.Forbidden("Cover manager role required")
	}
	return nil
}

// requireCoverStaff allows reception, cover managers and admins.
func requireCoverStaff(actor *models.User) error {
	if actor == nil || !models.IsCoverStaff(actor.Role) {
		return apperr.Forbidden("Cover desk role required")
	}
	return nil
}
