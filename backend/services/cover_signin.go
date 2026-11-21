package services

import (
	"context"
	"errors"
	"sort"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// SignInService keeps the append-only reception sign-in log.
type SignInService struct{ CoverDeps }

func NewSignInService(deps CoverDeps) *SignInService { return &SignInService{CoverDeps: deps} }

// presentSignIn shows an event from its own stored teacher snapshot, never the booking's current teacher.
func presentSignIn(e *models.SignInEvent) dto.SignInEvent {
	return dto.SignInEvent{ID: e.ID, BookingID: e.BookingID, SupplyTeacherID: e.SupplyTeacherID, TeacherName: e.TeacherName,
		Type: e.Type, RefID: e.RefID, At: formatTime(e.At), ActorUserID: e.ActorUserID, Lanyard: e.Lanyard,
		PhotoIDChecked: e.PhotoIDChecked, DBSEvidence: e.DBSEvidence, Reason: e.Reason}
}

// appendEvent stores e as the next event of its booking. In one transaction the booking's version is
// compared-and-set against check (so a concurrent booking change or other append makes this one fail) and
// the event inserted; the (booking_id, seq) unique index is a second guard. Either lost race is a 409.
func (s *SignInService) appendEvent(e *models.SignInEvent, prior []*models.SignInEvent, check repositories.BookingCheck) error {
	e.ID, e.Seq, e.At = NewID(), len(prior), s.Now().UTC()
	err := s.Repos.SignIns.AppendIf(e, check)
	if errors.Is(err, repositories.ErrDuplicate) {
		return mapBookingConflict(repositories.ErrVersionConflict)
	}
	return mapBookingConflict(err)
}

// SignIn records that reception checked photo ID (and, unless the agency confirmed an enhanced DBS check,
// the DBS certificate) and issued the lanyard. Only confirmed bookings dated today can be signed in, once.
func (s *SignInService) SignIn(ctx context.Context, actor *models.User, bookingID string, req *dto.SignInRequest) (*dto.SignInEvent, error) {
	if err := requireCoverStaff(actor); err != nil {
		return nil, err
	}
	b, err := s.booking(bookingID)
	if err != nil {
		return nil, err
	}
	switch {
	case b.Status != models.BookingConfirmed:
		return nil, apperr.Conflict("Only confirmed bookings can be signed in")
	case b.Date != s.today():
		return nil, apperr.Conflict("Sign-in is only possible on the booking date")
	}
	a, err := s.arrival(b)
	if err != nil {
		return nil, err
	}
	if CurrentSignIn(a.SignIns) != nil {
		return nil, apperr.Conflict("Already signed in")
	}
	evidence := models.DBSAgencyConfirmation
	if req.DBSCertificateSeen {
		evidence = models.DBSCertificateSeen
	} else if a.Teacher == nil || !a.Teacher.DBSConfirmed {
		return nil, apperr.Validation(map[string]string{
			"dbs_certificate_seen": "the agency has not confirmed a DBS check: see the DBS certificate before sign-in"})
	}
	e := &models.SignInEvent{BookingID: b.ID, Type: models.SignInTypeSignIn, ActorUserID: actor.ID,
		SupplyTeacherID: b.SupplyTeacherID, TeacherName: teacherName(a.Teacher),
		Lanyard: b.Lanyard, PhotoIDChecked: req.PhotoIDChecked, DBSEvidence: evidence}
	check := repositories.BookingCheck{Version: b.Version, Status: models.BookingConfirmed, SupplyTeacherID: b.SupplyTeacherID}
	if err := s.appendEvent(e, a.SignIns, check); err != nil {
		return nil, err
	}
	s.Audit.Log(ctx, AuditSignIn, "", actor.ID, map[string]string{"booking_id": b.ID, "sign_in_id": e.ID, "dbs_evidence": evidence})
	v := presentSignIn(e)
	return &v, nil
}

// Correct withdraws the current sign-in of a booking by appending a correction; the original entry stays.
func (s *SignInService) Correct(ctx context.Context, actor *models.User, signInID string, req *dto.CorrectionRequest) (*dto.SignInEvent, error) {
	if err := requireCoverStaff(actor); err != nil {
		return nil, err
	}
	ref, err := s.Repos.SignIns.GetByID(signInID)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && ref.Type != models.SignInTypeSignIn) {
		return nil, apperr.NotFound("Sign-in not found")
	} else if err != nil {
		return nil, err
	}
	b, err := s.booking(ref.BookingID)
	if err != nil {
		return nil, err
	}
	a, err := s.arrival(b)
	if err != nil {
		return nil, err
	}
	if cur := CurrentSignIn(a.SignIns); cur == nil || cur.ID != ref.ID {
		return nil, apperr.Conflict("This sign-in has already been withdrawn")
	}
	// The correction names the teacher who was signed in, copied from the sign-in it withdraws.
	e := &models.SignInEvent{BookingID: b.ID, Type: models.SignInTypeCorrection, RefID: ref.ID, ActorUserID: actor.ID,
		SupplyTeacherID: ref.SupplyTeacherID, TeacherName: ref.TeacherName, Reason: req.Reason}
	if err := s.appendEvent(e, a.SignIns, repositories.BookingCheck{Version: b.Version}); err != nil {
		return nil, err
	}
	s.Audit.Log(ctx, AuditSignInCorrection, "", actor.ID, map[string]string{"booking_id": b.ID, "sign_in_id": ref.ID,
		"correction_id": e.ID, "reason": req.Reason})
	v := presentSignIn(e)
	return &v, nil
}

// Log returns every sign-in and correction for the date's bookings (default today), newest first.
func (s *SignInService) Log(actor *models.User, date string) (*dto.SignInLog, error) {
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
	var events []*models.SignInEvent
	for _, a := range arrivals {
		events = append(events, a.SignIns...)
	}
	sort.SliceStable(events, func(i, j int) bool {
		ei, ej := events[i], events[j]
		if !ei.At.Equal(ej.At) {
			return ei.At.After(ej.At)
		}
		if ei.BookingID != ej.BookingID {
			return ei.BookingID < ej.BookingID
		}
		return ei.Seq > ej.Seq
	})
	out := &dto.SignInLog{Date: date, Items: make([]dto.SignInEvent, len(events))}
	for i, e := range events {
		out.Items[i] = presentSignIn(e)
	}
	return out, nil
}
