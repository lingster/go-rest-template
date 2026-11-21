package services

import (
	"context"
	"errors"
	"strings"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// BookingService books agencies, manages bookings and serves the role-projected today and week views.
type BookingService struct{ CoverDeps }

func NewBookingService(deps CoverDeps) *BookingService { return &BookingService{CoverDeps: deps} }

func (s *BookingService) present(b *models.Booking, lessons []*models.BookingLesson) *dto.Booking {
	return &dto.Booking{ID: b.ID, AgencyID: b.AgencyID, SupplyTeacherID: nilIfEmpty(b.SupplyTeacherID), Date: b.Date,
		Kind: b.Kind, Covering: b.Covering, Subject: b.Subject, Status: b.Status,
		ExpectedArrival: formatTime(b.ExpectedArrival), Lanyard: b.Lanyard, Lessons: s.Day.presentLessons(lessons),
		CreatedAt: formatTime(b.CreatedAt), UpdatedAt: formatTime(b.UpdatedAt)}
}

func newLessons(bookingID string, in []dto.LessonInput) []*models.BookingLesson {
	out := make([]*models.BookingLesson, len(in))
	for i, l := range in {
		out[i] = &models.BookingLesson{ID: NewID(), BookingID: bookingID, Period: l.Period, Free: l.Free}
		if !l.Free {
			out[i].ClassName, out[i].Room = strings.TrimSpace(l.ClassName), strings.TrimSpace(l.Room)
		}
	}
	return out
}

// Create books an agency for one day. The expected arrival is HH:MM in the school's zone on that date.
func (s *BookingService) Create(ctx context.Context, actor *models.User, req *dto.CreateBookingRequest) (*dto.Booking, error) {
	if err := requireCoverManager(actor); err != nil {
		return nil, err
	}
	if d := s.Day.ValidateLessons(req.Kind, req.Lessons); d != nil {
		return nil, apperr.Validation(d)
	}
	ag, err := s.Repos.Agencies.GetByID(req.AgencyID)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && !ag.IsActive) {
		return nil, apperr.Validation(map[string]string{"agency_id": "unknown or inactive agency"})
	} else if err != nil {
		return nil, err
	}
	tod, _ := ParseTimeOfDay(req.ExpectedArrival) // format checked by Validate
	expected, err := s.Day.At(req.Date, tod)
	if err != nil {
		return nil, apperr.Validation(map[string]string{"date": "must be YYYY-MM-DD"})
	}
	now := s.Now().UTC()
	b := &models.Booking{ID: NewID(), AgencyID: ag.ID, Date: req.Date, Kind: req.Kind,
		Covering: strings.TrimSpace(req.Covering), Subject: strings.TrimSpace(req.Subject), Status: models.BookingRequested,
		ExpectedArrival: expected, Lanyard: strings.TrimSpace(req.Lanyard), CreatedAt: now, UpdatedAt: now}
	lessons := newLessons(b.ID, req.Lessons)
	if err := s.Repos.Bookings.Create(b, lessons); err != nil {
		return nil, err
	}
	s.Audit.Log(ctx, AuditBookingCreate, "", actor.ID, map[string]string{"booking_id": b.ID, "agency_id": ag.ID, "date": b.Date})
	return s.present(b, lessons), nil
}

// Update confirms (assigning a teacher of the booking's agency), cancels, or edits covering, lanyard and
// lessons. Cancelled bookings are final, and a signed-in booking cannot be reassigned or cancelled until
// the sign-in is corrected.
func (s *BookingService) Update(ctx context.Context, actor *models.User, id string, req *dto.UpdateBookingRequest) (*dto.Booking, error) {
	if err := requireCoverManager(actor); err != nil {
		return nil, err
	}
	b, err := s.booking(id)
	if err != nil {
		return nil, err
	}
	if b.Status == models.BookingCancelled {
		return nil, apperr.Conflict("Booking is cancelled")
	}
	if req.Lessons != nil {
		if d := s.Day.ValidateLessons(b.Kind, req.Lessons); d != nil {
			return nil, apperr.Validation(d)
		}
	}
	a, err := s.arrival(b)
	if err != nil {
		return nil, err
	}
	expectedVersion, action := b.Version, AuditBookingUpdate
	if req.Status != nil {
		if CurrentSignIn(a.SignIns) != nil {
			return nil, apperr.Conflict("The teacher is signed in; correct the sign-in first")
		}
		switch *req.Status {
		case models.BookingConfirmed:
			if err := s.assignTeacher(b, *req.SupplyTeacherID); err != nil {
				return nil, err
			}
			action = AuditBookingConfirm
		case models.BookingCancelled:
			b.Status, action = models.BookingCancelled, AuditBookingCancel
		}
	}
	if req.Covering != nil {
		b.Covering = strings.TrimSpace(*req.Covering)
	}
	if req.Lanyard != nil {
		b.Lanyard = strings.TrimSpace(*req.Lanyard)
	}
	b.UpdatedAt = s.Now().UTC()
	lessons := a.Lessons
	var replacement []*models.BookingLesson
	if req.Lessons != nil {
		replacement = newLessons(b.ID, req.Lessons)
		lessons = replacement
	}
	// One transaction: version compare-and-set, booking columns and lessons. A concurrent sign-in or
	// booking change bumps the version first and this one is refused.
	if err := s.Repos.Bookings.UpdateIf(b, expectedVersion, replacement); err != nil {
		return nil, mapBookingConflict(err)
	}
	s.Audit.Log(ctx, action, "", actor.ID, map[string]string{"booking_id": b.ID, "status": b.Status,
		"supply_teacher_id": b.SupplyTeacherID})
	return s.present(b, lessons), nil
}

// assignTeacher confirms b with a teacher who must belong to the booking's agency.
func (s *BookingService) assignTeacher(b *models.Booking, teacherID string) error {
	t, err := s.Repos.Teachers.GetByID(teacherID)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && t.AgencyID != b.AgencyID) {
		return apperr.Validation(map[string]string{"supply_teacher_id": "must be a teacher of the booking's agency"})
	} else if err != nil {
		return err
	}
	b.SupplyTeacherID, b.Status = t.ID, models.BookingConfirmed
	return nil
}

// Today returns the date's arrivals (default: today in the school's zone) projected for the actor's
// role: cover managers and admins get dto.CoverTodayManager, reception dto.CoverTodayReception.
func (s *BookingService) Today(actor *models.User, date string) (interface{}, error) {
	if err := requireCoverStaff(actor); err != nil {
		return nil, err
	}
	date, err := s.dateOrToday("date", date)
	if err != nil {
		return nil, err
	}
	bookings, err := s.Repos.Bookings.ListByDates(date, date, "")
	if err != nil {
		return nil, err
	}
	arrivals, err := s.arrivals(bookings)
	if err != nil {
		return nil, err
	}
	view := s.Day.BuildToday(date, arrivals, s.Now())
	if models.IsCoverManager(actor.Role) {
		return view, nil
	}
	return TodayForReception(view), nil
}

// Week returns Monday-to-Friday bookings from start (a Monday; default this week) projected for the
// actor's role: dto.CoverWeekManager or dto.CoverWeekReception.
func (s *BookingService) Week(actor *models.User, start string) (interface{}, error) {
	if err := requireCoverStaff(actor); err != nil {
		return nil, err
	}
	if start == "" {
		start, _ = Monday(s.today())
	} else if monday, err := Monday(start); err != nil || monday != start {
		return nil, apperr.Validation(map[string]string{"start": "must be a Monday (YYYY-MM-DD)"})
	}
	bookings, err := s.Repos.Bookings.ListByDates(start, AddDays(start, WeekDays-1), "")
	if err != nil {
		return nil, err
	}
	arrivals, err := s.arrivals(bookings)
	if err != nil {
		return nil, err
	}
	view := s.Day.BuildWeek(start, arrivals)
	if models.IsCoverManager(actor.Role) {
		return view, nil
	}
	return WeekForReception(view), nil
}
