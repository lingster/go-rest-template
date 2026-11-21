package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
)

// Derived arrival statuses. Whether a teacher has "set off" is not knowable server-side (agencies send
// ETAs only, never locations), so a confirmed booking with an ETA is on_the_way or running_late.
const (
	StatusSignedIn    = "signed_in"
	StatusUnconfirmed = "unconfirmed"
	StatusCancelled   = "cancelled"
	StatusNoETA       = "no_eta"
	StatusOnTheWay    = "on_the_way"
	StatusRunningLate = "running_late"
)

// todayStatuses are the statuses counted on the today view (cancelled bookings are not listed).
var todayStatuses = []string{StatusSignedIn, StatusUnconfirmed, StatusNoETA, StatusOnTheWay, StatusRunningLate}

// Cover-desk thresholds.
const (
	// LateThreshold: an ETA this far past the expected arrival is running_late.
	LateThreshold = 5 * time.Minute
	// ETAStaleAfter: no ETA update for longer than this while travelling is flagged.
	ETAStaleAfter = 20 * time.Minute
	// CoverGapMargin: a teacher needs this long between arriving and teaching.
	CoverGapMargin = 5 * time.Minute
)

// Attention severities and types.
const (
	SeverityBad  = "bad"
	SeverityWarn = "warn"

	AttentionUnconfirmed = "unconfirmed"
	AttentionCoverGap    = "cover_gap"
	AttentionETAStale    = "eta_stale"
)

// Arrival is everything known about one booking, assembled by the services and consumed by the pure
// derivation and projection functions below.
type Arrival struct {
	Booking *models.Booking
	Agency  *models.Agency        // never nil
	Teacher *models.SupplyTeacher // nil until a teacher is assigned
	Lessons []*models.BookingLesson
	ETAs    []*models.EtaUpdate   // oldest first
	SignIns []*models.SignInEvent // seq order
}

// ArrivalState is the derived state of an arrival at a moment.
type ArrivalState struct {
	Status string
	ETA    *models.EtaUpdate   // latest accepted ETA, if any
	SignIn *models.SignInEvent // current (not withdrawn) sign-in, if any
	Stale  bool
}

// CurrentSignIn returns the sign-in that has not been withdrawn by a later correction, or nil.
func CurrentSignIn(events []*models.SignInEvent) *models.SignInEvent {
	var cur *models.SignInEvent
	for _, e := range events {
		switch {
		case e.Type == models.SignInTypeSignIn:
			cur = e
		case e.Type == models.SignInTypeCorrection && cur != nil && e.RefID == cur.ID:
			cur = nil
		}
	}
	return cur
}

// LatestETA returns the most recently received ETA update, or nil.
func LatestETA(updates []*models.EtaUpdate) *models.EtaUpdate {
	var latest *models.EtaUpdate
	for _, u := range updates {
		if latest == nil || !u.ReportedAt.Before(latest.ReportedAt) {
			latest = u
		}
	}
	return latest
}

// LatestNote returns the newest non-empty agency note.
func LatestNote(updates []*models.EtaUpdate) string {
	note := ""
	for _, u := range updates {
		if u.Note != "" {
			note = u.Note
		}
	}
	return note
}

// DeriveState mirrors the cover desk's status rules: cancelled, then signed in, then unconfirmed, then
// by the latest ETA (none, or late by LateThreshold or more).
func DeriveState(a Arrival, now time.Time) ArrivalState {
	b := a.Booking
	if b.Status == models.BookingCancelled {
		return ArrivalState{Status: StatusCancelled}
	}
	if s := CurrentSignIn(a.SignIns); s != nil {
		return ArrivalState{Status: StatusSignedIn, SignIn: s}
	}
	if b.Status != models.BookingConfirmed {
		return ArrivalState{Status: StatusUnconfirmed}
	}
	eta := LatestETA(a.ETAs)
	if eta == nil {
		return ArrivalState{Status: StatusNoETA}
	}
	st := ArrivalState{Status: StatusOnTheWay, ETA: eta, Stale: now.Sub(eta.ReportedAt) > ETAStaleAfter}
	if eta.ETA.Sub(b.ExpectedArrival) >= LateThreshold {
		st.Status = StatusRunningLate
	}
	return st
}

// firstLesson returns the first taught (non-free) lesson in timetable order, falling back to the first
// lesson, with its period; ok is false when the booking has no lessons.
func (d SchoolDay) firstLesson(lessons []*models.BookingLesson) (l *models.BookingLesson, p Period, ok bool) {
	sorted := append([]*models.BookingLesson(nil), lessons...)
	sort.SliceStable(sorted, func(i, j int) bool { return d.periodIndex(sorted[i].Period) < d.periodIndex(sorted[j].Period) })
	for _, x := range sorted {
		if !x.Free {
			l = x
			break
		}
	}
	if l == nil && len(sorted) > 0 {
		l = sorted[0]
	}
	if l == nil {
		return nil, Period{}, false
	}
	p, ok = d.Period(l.Period)
	return l, p, ok
}

func teacherName(t *models.SupplyTeacher) string {
	if t == nil {
		return ""
	}
	return t.Name
}

func timePtr(t time.Time) *string {
	s := formatTime(t)
	return &s
}

// projectArrival builds the cover manager's row; the reception row is its embedded CoverArrival.
func (d SchoolDay) projectArrival(a Arrival, st ArrivalState) dto.CoverManagerArrival {
	b := a.Booking
	row := dto.CoverManagerArrival{
		CoverArrival: dto.CoverArrival{
			BookingID: b.ID, TeacherName: teacherName(a.Teacher), AgencyName: a.Agency.Name, AgencyPhone: a.Agency.Phone,
			ExpectedArrival: formatTime(b.ExpectedArrival), Status: st.Status, ETAStale: st.Stale, Lanyard: b.Lanyard,
			DBSCertificateRequired: a.Teacher != nil && !a.Teacher.DBSConfirmed,
		},
		Covering: b.Covering, Subject: b.Subject, Kind: b.Kind, Note: LatestNote(a.ETAs),
		DBSConfirmed: a.Teacher != nil && a.Teacher.DBSConfirmed,
	}
	if st.ETA != nil {
		row.ETA, row.ETAUpdatedAt = timePtr(st.ETA.ETA), timePtr(st.ETA.ReportedAt)
	}
	if l, p, ok := d.firstLesson(a.Lessons); ok {
		row.FirstLesson = &dto.FirstLesson{Period: l.Period, Start: p.Start.String(), ClassName: l.ClassName, Room: l.Room}
	}
	return row
}

// AttentionItems lists what a cover manager should act on: unconfirmed bookings and cover gaps (an ETA
// later than CoverGapMargin before the first lesson) while the first lesson has not ended, and stale
// ETAs. Problems come before warnings.
func (d SchoolDay) AttentionItems(arrivals []Arrival, states []ArrivalState, now time.Time) []dto.CoverAttention {
	items := []dto.CoverAttention{}
	for i, a := range arrivals {
		st, b := states[i], a.Booking
		if st.Status == StatusSignedIn || st.Status == StatusCancelled {
			continue
		}
		name := teacherName(a.Teacher)
		l, p, hasLesson := d.firstLesson(a.Lessons)
		if hasLesson && now.Before(d.mustAt(b.Date, p.End)) {
			start := d.mustAt(b.Date, p.Start)
			switch {
			case st.Status == StatusUnconfirmed:
				items = append(items, dto.CoverAttention{Severity: SeverityBad, Type: AttentionUnconfirmed, BookingID: b.ID,
					Message: fmt.Sprintf("No confirmed teacher for %s's classes", b.Covering),
					Detail: fmt.Sprintf("%s at %s: %s, %s in %s. Call %s on %s or arrange internal cover.",
						l.Period, p.Start, b.Subject, l.ClassName, l.Room, a.Agency.Name, a.Agency.Phone)})
			case st.ETA != nil && st.ETA.ETA.After(start.Add(-CoverGapMargin)):
				items = append(items, dto.CoverAttention{Severity: SeverityWarn, Type: AttentionCoverGap, BookingID: b.ID,
					Message: fmt.Sprintf("%s %s needs cover until %s", l.Period, b.Subject, d.LocalClock(st.ETA.ETA.Add(CoverGapMargin))),
					Detail: fmt.Sprintf("%s arrives around %s, after %s starts in %s at %s.",
						name, d.LocalClock(st.ETA.ETA), l.ClassName, l.Room, p.Start)})
			}
		}
		if st.Stale {
			items = append(items, dto.CoverAttention{Severity: SeverityWarn, Type: AttentionETAStale, BookingID: b.ID,
				Message: fmt.Sprintf("No ETA update for %s in %d min", name, int(now.Sub(st.ETA.ReportedAt).Minutes())),
				Detail:  fmt.Sprintf("Last ETA %s. Call %s on %s.", d.LocalClock(st.ETA.ETA), a.Agency.Name, a.Agency.Phone)})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Severity == SeverityBad && items[j].Severity != SeverityBad
	})
	return items
}

// BuildToday derives the full (cover manager) today view of the date's arrivals: cancelled bookings
// are dropped and the rest sorted by expected arrival.
func (d SchoolDay) BuildToday(date string, arrivals []Arrival, now time.Time) dto.CoverTodayManager {
	kept := make([]Arrival, 0, len(arrivals))
	for _, a := range arrivals {
		if a.Booking.Status != models.BookingCancelled {
			kept = append(kept, a)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		bi, bj := kept[i].Booking, kept[j].Booking
		if !bi.ExpectedArrival.Equal(bj.ExpectedArrival) {
			return bi.ExpectedArrival.Before(bj.ExpectedArrival)
		}
		return bi.ID < bj.ID
	})
	out := dto.CoverTodayManager{Date: date, Timezone: d.Loc.String(), Arrivals: make([]dto.CoverManagerArrival, 0, len(kept)),
		Counts: map[string]int{}}
	for _, s := range todayStatuses {
		out.Counts[s] = 0
	}
	states := make([]ArrivalState, len(kept))
	for i, a := range kept {
		states[i] = DeriveState(a, now)
		out.Arrivals = append(out.Arrivals, d.projectArrival(a, states[i]))
		out.Counts[states[i].Status]++
	}
	out.Attention = d.AttentionItems(kept, states, now)
	return out
}

// TodayForReception projects the today view down to what reception may see. The result type has no
// field for covering, subject, lessons, notes or attention items, so they cannot leak.
func TodayForReception(v dto.CoverTodayManager) dto.CoverTodayReception {
	out := dto.CoverTodayReception{Date: v.Date, Timezone: v.Timezone, Counts: v.Counts,
		Arrivals: make([]dto.CoverArrival, len(v.Arrivals))}
	for i, a := range v.Arrivals {
		out.Arrivals[i] = a.CoverArrival
	}
	return out
}

// presentLessons converts lessons to their API shape in timetable order.
func (d SchoolDay) presentLessons(lessons []*models.BookingLesson) []dto.Lesson {
	sorted := append([]*models.BookingLesson(nil), lessons...)
	sort.SliceStable(sorted, func(i, j int) bool { return d.periodIndex(sorted[i].Period) < d.periodIndex(sorted[j].Period) })
	out := make([]dto.Lesson, 0, len(sorted))
	for _, l := range sorted {
		p, _ := d.Period(l.Period)
		out = append(out, dto.Lesson{Period: l.Period, Start: p.Start.String(), End: p.End.String(),
			ClassName: l.ClassName, Room: l.Room, Free: l.Free})
	}
	return out
}

// WeekDays is the number of school days in the week view.
const WeekDays = 5

// BuildWeek groups the Monday-to-Friday bookings starting at start into one row per supply teacher
// (each unassigned booking gets its own row), sorted by teacher then agency name.
func (d SchoolDay) BuildWeek(start string, arrivals []Arrival) dto.CoverWeekManager {
	type rowKey struct{ teacher, booking string }
	rows := map[rowKey]*dto.CoverWeekManagerRow{}
	var order []rowKey
	for _, a := range arrivals {
		b := a.Booking
		day := -1
		for i := 0; i < WeekDays; i++ {
			if AddDays(start, i) == b.Date {
				day = i
			}
		}
		if day < 0 {
			continue
		}
		key := rowKey{teacher: b.SupplyTeacherID}
		if key.teacher == "" {
			key.booking = b.ID
		}
		row, ok := rows[key]
		if !ok {
			row = &dto.CoverWeekManagerRow{TeacherName: teacherName(a.Teacher), AgencyName: a.Agency.Name,
				Days: make([]dto.CoverWeekManagerDay, WeekDays)}
			for i := range row.Days {
				row.Days[i] = dto.CoverWeekManagerDay{Date: AddDays(start, i), Bookings: []dto.CoverWeekManagerCell{}}
			}
			rows[key] = row
			order = append(order, key)
		}
		row.Days[day].Bookings = append(row.Days[day].Bookings, dto.CoverWeekManagerCell{
			CoverWeekCell: dto.CoverWeekCell{BookingID: b.ID, Kind: b.Kind, Status: b.Status},
			Covering:      b.Covering, Lessons: d.presentLessons(a.Lessons),
		})
	}
	out := dto.CoverWeekManager{Start: start, Rows: make([]dto.CoverWeekManagerRow, 0, len(order))}
	for _, k := range order {
		out.Rows = append(out.Rows, *rows[k])
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		ri, rj := out.Rows[i], out.Rows[j]
		if (ri.TeacherName == "") != (rj.TeacherName == "") {
			return rj.TeacherName == "" // unassigned last
		}
		if ri.TeacherName != rj.TeacherName {
			return ri.TeacherName < rj.TeacherName
		}
		return ri.AgencyName < rj.AgencyName
	})
	return out
}

// WeekForReception projects the week view down to name, agency, kind and status.
func WeekForReception(v dto.CoverWeekManager) dto.CoverWeekReception {
	out := dto.CoverWeekReception{Start: v.Start, Rows: make([]dto.CoverWeekRow, len(v.Rows))}
	for i, r := range v.Rows {
		row := dto.CoverWeekRow{TeacherName: r.TeacherName, AgencyName: r.AgencyName, Days: make([]dto.CoverWeekDay, len(r.Days))}
		for j, day := range r.Days {
			cells := make([]dto.CoverWeekCell, len(day.Bookings))
			for k, c := range day.Bookings {
				cells[k] = c.CoverWeekCell
			}
			row.Days[j] = dto.CoverWeekDay{Date: day.Date, Bookings: cells}
		}
		out.Rows[i] = row
	}
	return out
}
