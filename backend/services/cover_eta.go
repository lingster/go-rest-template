package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// ETA rules for agency reports.
const (
	// ETAMaxPast: an ETA may be at most this far before the server's clock (late reports of an arrival).
	ETAMaxPast = 10 * time.Minute
	// ETAMinInterval and ETAMaxPerBooking cap how often one booking's ETA can be reported.
	ETAMinInterval   = 30 * time.Second
	ETAMaxPerBooking = 100
)

// ParseETA reads an agency ETA: "HH:MM" in the school's zone on the booking date, or an RFC3339 instant.
func (d SchoolDay) ParseETA(date, raw string) (time.Time, error) {
	if tod, ok := ParseTimeOfDay(raw); ok {
		return d.At(date, tod)
	}
	t, err := time.Parse(time.RFC3339, raw)
	return t.UTC(), err
}

// CheckETA validates a parsed ETA for a booking on date: it must fall inside the school's arrival window
// on that date and must not be more than ETAMaxPast before now. It returns field details or nil.
func (d SchoolDay) CheckETA(date string, eta, now time.Time) map[string]string {
	from, to := d.mustAt(date, d.ArrivalFrom), d.mustAt(date, d.ArrivalTo)
	if eta.Before(from) || eta.After(to) {
		return map[string]string{"eta": fmt.Sprintf("must be between %s and %s school time on %s", d.ArrivalFrom, d.ArrivalTo, date)}
	}
	if eta.Before(now.Add(-ETAMaxPast)) {
		return map[string]string{"eta": fmt.Sprintf("must not be more than %d minutes in the past", int(ETAMaxPast.Minutes()))}
	}
	return nil
}

// CheckETARate enforces the per-booking cap given the booking's earlier updates: at most one per
// ETAMinInterval and ETAMaxPerBooking in total (a booking spans one day).
func CheckETARate(prior []*models.EtaUpdate, now time.Time) error {
	if len(prior) >= ETAMaxPerBooking {
		return apperr.RateLimited()
	}
	if last := LatestETA(prior); last != nil && now.Sub(last.ReportedAt) < ETAMinInterval {
		return apperr.RateLimited()
	}
	return nil
}

// AgencyPortalService is what an authenticated agency can do: read its own bookings and report ETAs.
type AgencyPortalService struct{ CoverDeps }

func NewAgencyPortalService(deps CoverDeps) *AgencyPortalService {
	return &AgencyPortalService{CoverDeps: deps}
}

// Bookings returns the agency's bookings on date (default today) with only what the agency needs.
func (s *AgencyPortalService) Bookings(agency *models.Agency, date string) (*dto.AgencyBookingList, error) {
	date, err := s.dateOrToday("date", date)
	if err != nil {
		return nil, err
	}
	bookings, err := s.Repos.Bookings.ListByDates(date, date, agency.ID)
	if err != nil {
		return nil, err
	}
	var teacherIDs []string
	for _, b := range bookings {
		teacherIDs = append(teacherIDs, b.SupplyTeacherID)
	}
	teachers, err := s.Repos.Teachers.GetMany(teacherIDs)
	if err != nil {
		return nil, err
	}
	out := &dto.AgencyBookingList{Date: date, Items: make([]dto.AgencyBooking, len(bookings))}
	for i, b := range bookings {
		out.Items[i] = dto.AgencyBooking{ID: b.ID, Date: b.Date, Kind: b.Kind, Status: b.Status,
			TeacherName: teacherName(teachers[b.SupplyTeacherID]), ExpectedArrival: formatTime(b.ExpectedArrival)}
	}
	return out, nil
}

// ReportETA appends an ETA update for one of the agency's bookings. Another agency's booking is
// reported as not found so booking ids cannot be probed. The receive time is the server's clock.
func (s *AgencyPortalService) ReportETA(_ context.Context, agency *models.Agency, bookingID string, req *dto.EtaRequest) (*dto.EtaUpdate, error) {
	b, err := s.Repos.Bookings.GetByID(bookingID)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && b.AgencyID != agency.ID) {
		return nil, apperr.NotFound("Booking not found")
	} else if err != nil {
		return nil, err
	}
	now := s.Now().UTC()
	switch {
	case b.Status != models.BookingConfirmed:
		return nil, apperr.Conflict("ETAs can only be reported for confirmed bookings")
	case b.Date != s.Day.LocalDate(now):
		return nil, apperr.Conflict("ETAs can only be reported on the booking date")
	}
	eta, err := s.Day.ParseETA(b.Date, req.ETA)
	if err != nil {
		return nil, apperr.Validation(map[string]string{"eta": "must be HH:MM or an RFC3339 time"})
	}
	if d := s.Day.CheckETA(b.Date, eta, now); d != nil {
		return nil, apperr.Validation(d)
	}
	a, err := s.arrival(b)
	if err != nil {
		return nil, err
	}
	if CurrentSignIn(a.SignIns) != nil {
		return nil, apperr.Conflict("The teacher has already signed in")
	}
	if err := CheckETARate(a.ETAs, now); err != nil {
		return nil, err // not audited: a chatty integration would flood the log
	}
	u := &models.EtaUpdate{ID: NewID(), BookingID: b.ID, AgencyID: agency.ID, ReportedAt: now, ETA: eta,
		Note: dto.CleanText(req.Note), CreatedAt: now}
	if err := s.Repos.EtaUpdates.Create(u); err != nil {
		return nil, err
	}
	return &dto.EtaUpdate{ID: u.ID, BookingID: b.ID, ETA: formatTime(u.ETA), ReportedAt: formatTime(u.ReportedAt), Note: u.Note}, nil
}
