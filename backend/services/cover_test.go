package services_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
	"github.com/lingster/go-rest-template/backend/services"
	"github.com/lingster/go-rest-template/backend/testutil"
)

func london(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func schoolDay(t *testing.T) services.SchoolDay { return services.DefaultSchoolDay(london(t)) }

// at returns the instant of the London wall-clock time "YYYY-MM-DD HH:MM".
func at(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, london(t))
	if err != nil {
		t.Fatal(err)
	}
	return v.UTC()
}

func fullDayLessons(id string) []*models.BookingLesson {
	return []*models.BookingLesson{
		{BookingID: id, Period: "P2", ClassName: "8K1", Room: "E14"},
		{BookingID: id, Period: "P1", ClassName: "9X2", Room: "Lab 2", Free: true},
		{BookingID: id, Period: "P3", ClassName: "7R2", Room: "Lab 2"},
	}
}

// arrival builds a confirmed full-day booking on 2026-11-23 expected at 08:15.
func arrival(t *testing.T, id string) services.Arrival {
	t.Helper()
	return services.Arrival{
		Booking: &models.Booking{ID: id, AgencyID: "ag", SupplyTeacherID: "t-" + id, Date: "2026-11-23",
			Kind: models.BookingFull, Covering: "Mr Okafor", Subject: "Science", Status: models.BookingConfirmed,
			ExpectedArrival: at(t, "2026-11-23 08:15"), Lanyard: "S-01"},
		Agency:  &models.Agency{ID: "ag", Name: "Kestrel Teaching", Phone: "01632 960 214"},
		Teacher: &models.SupplyTeacher{ID: "t-" + id, Name: "Priya " + id, DBSConfirmed: true},
		Lessons: fullDayLessons(id),
	}
}

func eta(t *testing.T, reported, eta, note string) *models.EtaUpdate {
	return &models.EtaUpdate{ReportedAt: at(t, "2026-11-23 "+reported), ETA: at(t, "2026-11-23 "+eta), Note: note}
}

func TestTimeOfDay(t *testing.T) {
	for _, s := range []string{"00:00", "08:55", "23:59"} {
		v, ok := services.ParseTimeOfDay(s)
		if !ok || v.String() != s {
			t.Errorf("%s -> %v %v", s, v, ok)
		}
	}
	for _, s := range []string{"", "8:55", "24:00", "12:60", "12-30", "12:30:00"} {
		if _, ok := services.ParseTimeOfDay(s); ok {
			t.Errorf("%q must not parse", s)
		}
	}
}

func TestSchoolDayTimezoneAcrossDST(t *testing.T) {
	d := schoolDay(t)
	eight, _ := services.ParseTimeOfDay("08:00")
	cases := []struct{ date, want string }{
		{"2026-03-27", "2026-03-27T08:00:00Z"}, // GMT (Friday before the change)
		{"2026-03-29", "2026-03-29T07:00:00Z"}, // clocks went forward at 01:00 GMT that morning
		{"2026-03-30", "2026-03-30T07:00:00Z"}, // BST
		{"2026-10-23", "2026-10-23T07:00:00Z"}, // still BST
		{"2026-10-25", "2026-10-25T08:00:00Z"}, // clocks went back at 02:00 BST
		{"2026-10-26", "2026-10-26T08:00:00Z"},
	}
	for _, c := range cases {
		got, err := d.At(c.date, eight)
		if err != nil || got.Format(time.RFC3339) != c.want {
			t.Errorf("08:00 on %s = %v (%v), want %s", c.date, got, err, c.want)
		}
		if d.LocalClock(got) != "08:00" || d.LocalDate(got) != c.date {
			t.Errorf("round trip on %s: %s %s", c.date, d.LocalDate(got), d.LocalClock(got))
		}
	}
	// Late evening UTC is already tomorrow in London during BST.
	if got := d.LocalDate(time.Date(2026, 6, 30, 23, 30, 0, 0, time.UTC)); got != "2026-07-01" {
		t.Errorf("local date = %s", got)
	}
	if _, err := d.At("2026-02-30", eight); err == nil {
		t.Error("invalid date accepted")
	}
}

func TestMondayAndAddDays(t *testing.T) {
	for date, want := range map[string]string{
		"2026-11-23": "2026-11-23", "2026-11-27": "2026-11-23", "2026-11-29": "2026-11-23", "2026-03-29": "2026-03-23",
	} {
		if got, err := services.Monday(date); err != nil || got != want {
			t.Errorf("Monday(%s) = %s %v, want %s", date, got, err, want)
		}
	}
	if got := services.AddDays("2026-03-27", 4); got != "2026-03-31" { // across the DST change
		t.Errorf("AddDays = %s", got)
	}
}

func TestValidateLessons(t *testing.T) {
	d := schoolDay(t)
	l := func(periods ...string) []dto.LessonInput {
		out := make([]dto.LessonInput, len(periods))
		for i, p := range periods {
			out[i] = dto.LessonInput{Period: p, Free: true}
		}
		return out
	}
	if got := d.ValidateLessons(models.BookingPM, l("P5", "P4")); got != nil {
		t.Errorf("valid pm booking rejected: %v", got)
	}
	if got := d.ValidateLessons(models.BookingFull, l("P1", "P2", "P3", "P4", "P5")); got != nil {
		t.Errorf("valid full booking rejected: %v", got)
	}
	for name, in := range map[string][]dto.LessonInput{
		"missing period": l("P1", "P2"),
		"foreign period": l("P1", "P2", "P3", "P4"),
		"duplicate":      l("P1", "P1", "P2", "P3"),
		"unknown":        l("P1", "P2", "P9"),
	} {
		if got := d.ValidateLessons(models.BookingAM, in); got["lessons"] == "" {
			t.Errorf("%s: expected a lessons error", name)
		}
	}
}

func TestCurrentSignIn(t *testing.T) {
	s1 := &models.SignInEvent{ID: "s1", Type: models.SignInTypeSignIn}
	c1 := &models.SignInEvent{ID: "c1", Type: models.SignInTypeCorrection, RefID: "s1"}
	s2 := &models.SignInEvent{ID: "s2", Type: models.SignInTypeSignIn}
	if services.CurrentSignIn(nil) != nil {
		t.Error("empty log has no sign-in")
	}
	if services.CurrentSignIn([]*models.SignInEvent{s1}) != s1 {
		t.Error("s1 should be current")
	}
	if services.CurrentSignIn([]*models.SignInEvent{s1, c1}) != nil {
		t.Error("correction must withdraw s1")
	}
	if services.CurrentSignIn([]*models.SignInEvent{s1, c1, s2}) != s2 {
		t.Error("re-sign-in should be current")
	}
	stray := &models.SignInEvent{ID: "c9", Type: models.SignInTypeCorrection, RefID: "other"}
	if services.CurrentSignIn([]*models.SignInEvent{s1, stray}) != s1 {
		t.Error("a correction of another sign-in must not withdraw s1")
	}
}

func TestDeriveState(t *testing.T) {
	now := at(t, "2026-11-23 08:00")
	check := func(name string, a services.Arrival, now time.Time, status string, stale bool) {
		t.Helper()
		st := services.DeriveState(a, now)
		if st.Status != status || st.Stale != stale {
			t.Errorf("%s: got %s stale=%v, want %s stale=%v", name, st.Status, st.Stale, status, stale)
		}
	}
	a := arrival(t, "b")
	check("confirmed without ETA", a, now, services.StatusNoETA, false)

	a.ETAs = []*models.EtaUpdate{eta(t, "07:45", "08:19", "")}
	check("4 min late is on the way", a, now, services.StatusOnTheWay, false)
	a.ETAs = append(a.ETAs, eta(t, "07:50", "08:20", "Roadworks"))
	check("5 min late is running late", a, now, services.StatusRunningLate, false)
	a.ETAs = append(a.ETAs, eta(t, "07:55", "08:10", ""))
	check("latest ETA wins", a, now, services.StatusOnTheWay, false)
	check("exactly 20 min old is not stale", a, at(t, "2026-11-23 08:15"), services.StatusOnTheWay, false)
	check("over 20 min old is stale", a, at(t, "2026-11-23 08:16"), services.StatusOnTheWay, true)
	if n := services.LatestNote(a.ETAs); n != "Roadworks" {
		t.Errorf("latest note = %q", n)
	}

	a.SignIns = []*models.SignInEvent{{ID: "s1", Type: models.SignInTypeSignIn}}
	check("signed in beats ETA and staleness", a, at(t, "2026-11-23 09:00"), services.StatusSignedIn, false)
	a.SignIns = append(a.SignIns, &models.SignInEvent{ID: "c1", Type: models.SignInTypeCorrection, RefID: "s1"})
	check("corrected sign-in falls back to ETA", a, at(t, "2026-11-23 08:16"), services.StatusOnTheWay, true)

	u := arrival(t, "u")
	u.Booking.Status, u.Teacher, u.Booking.SupplyTeacherID = models.BookingRequested, nil, ""
	check("requested", u, now, services.StatusUnconfirmed, false)
	c := arrival(t, "c")
	c.Booking.Status = models.BookingCancelled
	c.SignIns = a.SignIns[:1]
	check("cancelled wins", c, now, services.StatusCancelled, false)
}

func TestAttentionItems(t *testing.T) {
	d := schoolDay(t)
	gap := arrival(t, "gap") // first taught lesson is P2 at 09:55 (P1 is free)
	gap.ETAs = []*models.EtaUpdate{eta(t, "08:30", "09:52", "")}
	ok := arrival(t, "ok")
	ok.ETAs = []*models.EtaUpdate{eta(t, "08:30", "09:50", "")} // exactly start-5: no gap
	unconf := arrival(t, "unconf")
	unconf.Booking.Status, unconf.Teacher = models.BookingRequested, nil
	stale := arrival(t, "stale")
	stale.ETAs = []*models.EtaUpdate{eta(t, "08:20", "08:20", "")}
	in := arrival(t, "in")
	in.ETAs = []*models.EtaUpdate{eta(t, "07:00", "10:30", "")}
	in.SignIns = []*models.SignInEvent{{ID: "s", Type: models.SignInTypeSignIn}}

	now := at(t, "2026-11-23 08:45")
	all := []services.Arrival{gap, ok, unconf, stale, in}
	states := make([]services.ArrivalState, len(all))
	for i, a := range all {
		states[i] = services.DeriveState(a, now)
	}
	items := d.AttentionItems(all, states, now)
	got := map[string]dto.CoverAttention{}
	for _, it := range items {
		got[it.BookingID+"/"+it.Type] = it
	}
	if len(items) != 3 || items[0].Type != services.AttentionUnconfirmed || items[0].Severity != services.SeverityBad {
		t.Fatalf("unexpected items (bad first): %+v", items)
	}
	if g := got["gap/"+services.AttentionCoverGap]; g.Message != "P2 Science needs cover until 09:57" {
		t.Errorf("cover gap = %+v", g)
	}
	if s := got["stale/"+services.AttentionETAStale]; !strings.Contains(s.Message, "25 min") {
		t.Errorf("stale = %+v", s)
	}
	if !strings.Contains(got["unconf/"+services.AttentionUnconfirmed].Detail, "P2 at 09:55") {
		t.Errorf("unconfirmed detail = %+v", got["unconf/unconfirmed"])
	}

	// Once the first lesson has ended, unconfirmed bookings and cover gaps are no longer actionable.
	late := at(t, "2026-11-23 11:00")
	for i, a := range all {
		states[i] = services.DeriveState(a, late)
	}
	for _, it := range d.AttentionItems(all, states, late) {
		if it.Type != services.AttentionETAStale {
			t.Errorf("unexpected item after first lesson: %+v", it)
		}
	}
}

func jsonKeys(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

var managerOnlyArrivalKeys = []string{"covering", "subject", "kind", "first_lesson", "dbs_confirmed", "note"}

func TestBuildTodayAndReceptionProjection(t *testing.T) {
	d := schoolDay(t)
	early, late, cancelled := arrival(t, "early"), arrival(t, "late"), arrival(t, "cancelled")
	early.Booking.ExpectedArrival = at(t, "2026-11-23 08:05")
	early.Teacher.DBSConfirmed = false
	early.ETAs = []*models.EtaUpdate{eta(t, "07:30", "08:06", "Set off")}
	late.ETAs = []*models.EtaUpdate{eta(t, "07:30", "08:40", "Train cancelled")}
	cancelled.Booking.Status = models.BookingCancelled
	now := at(t, "2026-11-23 07:40")

	v := d.BuildToday("2026-11-23", []services.Arrival{late, cancelled, early}, now)
	if len(v.Arrivals) != 2 || v.Arrivals[0].BookingID != "early" || v.Arrivals[1].BookingID != "late" {
		t.Fatalf("want early, late (cancelled dropped): %+v", v.Arrivals)
	}
	if v.Counts[services.StatusOnTheWay] != 1 || v.Counts[services.StatusRunningLate] != 1 || v.Counts[services.StatusSignedIn] != 0 {
		t.Errorf("counts = %v", v.Counts)
	}
	first := v.Arrivals[0]
	if first.FirstLesson == nil || first.FirstLesson.Period != "P2" || first.FirstLesson.Start != "09:55" ||
		first.Note != "Set off" || !first.DBSCertificateRequired || first.DBSConfirmed {
		t.Errorf("manager row = %+v", first)
	}
	if *first.ETA != "2026-11-23T08:06:00Z" || *first.ETAUpdatedAt != "2026-11-23T07:30:00Z" {
		t.Errorf("eta fields = %s %s", *first.ETA, *first.ETAUpdatedAt)
	}

	r := jsonKeys(t, services.TodayForReception(v))
	if _, ok := r["attention"]; ok {
		t.Error("reception view must not contain attention items")
	}
	row := r["arrivals"].([]interface{})[0].(map[string]interface{})
	for _, k := range managerOnlyArrivalKeys {
		if _, ok := row[k]; ok {
			t.Errorf("reception row leaks %q", k)
		}
	}
	for _, k := range []string{"booking_id", "teacher_name", "agency_name", "agency_phone", "expected_arrival", "status",
		"eta", "eta_updated_at", "eta_stale", "lanyard", "dbs_certificate_required"} {
		if _, ok := row[k]; !ok {
			t.Errorf("reception row misses %q", k)
		}
	}
	if len(row) != 11 {
		t.Errorf("reception row has unexpected keys: %v", row)
	}
}

func TestBuildWeekAndReceptionProjection(t *testing.T) {
	d := schoolDay(t)
	mon, wed := arrival(t, "mon"), arrival(t, "wed")
	wed.Booking.Date = "2026-11-25"
	wed.Booking.SupplyTeacherID, wed.Teacher = mon.Booking.SupplyTeacherID, mon.Teacher
	open := arrival(t, "open")
	open.Booking.SupplyTeacherID, open.Teacher, open.Booking.Status = "", nil, models.BookingRequested
	outside := arrival(t, "next-week")
	outside.Booking.Date = "2026-11-30"

	v := d.BuildWeek("2026-11-23", []services.Arrival{open, wed, mon, outside})
	if len(v.Rows) != 2 || v.Rows[0].TeacherName != "Priya mon" || v.Rows[1].TeacherName != "" {
		t.Fatalf("rows = %+v", v.Rows)
	}
	row := v.Rows[0]
	if len(row.Days) != 5 || row.Days[0].Date != "2026-11-23" || row.Days[4].Date != "2026-11-27" ||
		len(row.Days[0].Bookings) != 1 || len(row.Days[2].Bookings) != 1 || len(row.Days[1].Bookings) != 0 {
		t.Fatalf("days = %+v", row.Days)
	}
	if c := row.Days[0].Bookings[0]; c.Covering != "Mr Okafor" || len(c.Lessons) != 3 || c.Lessons[0].Period != "P1" {
		t.Errorf("manager cell = %+v", c)
	}
	raw, _ := json.Marshal(services.WeekForReception(v))
	for _, leak := range []string{"covering", "lessons", "Mr Okafor", "Lab 2", "8K1"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("reception week leaks %q: %s", leak, raw)
		}
	}
}

func TestETAValidation(t *testing.T) {
	d := schoolDay(t)
	now := at(t, "2026-11-23 08:00")
	parse := func(raw string) time.Time {
		t.Helper()
		v, err := d.ParseETA("2026-11-23", raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		return v
	}
	if !parse("08:30").Equal(at(t, "2026-11-23 08:30")) || !parse("2026-11-23T09:30:00+01:00").Equal(at(t, "2026-11-23 08:30")) {
		t.Error("HH:MM and RFC3339 must denote the same instant")
	}
	for _, bad := range []string{"8.30", "tomorrow", "2026-11-23 08:30"} {
		if _, err := d.ParseETA("2026-11-23", bad); err == nil {
			t.Errorf("%q must not parse", bad)
		}
	}
	for raw, ok := range map[string]bool{
		"06:00":                false, // before now-10min
		"07:50":                true,  // exactly 10 min in the past
		"07:49":                false,
		"16:00":                true,
		"16:01":                false, // after the arrival window
		"2026-11-24T08:30:00Z": false, // another day
	} {
		if got := d.CheckETA("2026-11-23", parse(raw), now); (got == nil) != ok {
			t.Errorf("CheckETA(%s) = %v, want ok=%v", raw, got, ok)
		}
	}
	if got := d.CheckETA("2026-11-23", parse("05:59"), at(t, "2026-11-23 05:00")); got == nil {
		t.Error("05:59 is before the arrival window")
	}
	// On the DST change day the window is still school-local.
	dst := at(t, "2026-03-29 06:05")
	if v, _ := d.ParseETA("2026-03-29", "06:00"); d.CheckETA("2026-03-29", v, dst) != nil || v.Format(time.RFC3339) != "2026-03-29T05:00:00Z" {
		t.Errorf("06:00 BST on 2026-03-29 = %v", v)
	}
}

func TestETARate(t *testing.T) {
	now := at(t, "2026-11-23 08:00")
	if services.CheckETARate(nil, now) != nil {
		t.Error("first update must pass")
	}
	prior := []*models.EtaUpdate{{ReportedAt: now.Add(-29 * time.Second)}}
	if code(t, services.CheckETARate(prior, now)) != "rate_limited" {
		t.Error("an update within 30s must be rate limited")
	}
	prior[0].ReportedAt = now.Add(-30 * time.Second)
	if services.CheckETARate(prior, now) != nil {
		t.Error("30s later is fine")
	}
	many := make([]*models.EtaUpdate, services.ETAMaxPerBooking)
	for i := range many {
		many[i] = &models.EtaUpdate{ReportedAt: now.Add(-time.Hour)}
	}
	if code(t, services.CheckETARate(many, now)) != "rate_limited" {
		t.Error("the per-booking cap must apply")
	}
}

func TestCleanText(t *testing.T) {
	if got := dto.CleanText("  Train\ncancelled‮\x00 \t"); got != "Traincancelled" {
		t.Errorf("CleanText = %q", got)
	}
}

// The sign-in log's concurrency guard lives in the database: a second event with the same
// (booking_id, seq), as two racing sign-ins would compute, is rejected whatever the timing.
func TestSignInLogRejectsConflictingAppend(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	booking := services.NewID()
	ev := func() *models.SignInEvent {
		return &models.SignInEvent{ID: services.NewID(), BookingID: booking, Seq: 0, Type: models.SignInTypeSignIn,
			At: time.Now().UTC(), PhotoIDChecked: true}
	}
	if err := c.Repos.SignIns.Create(ev()); err != nil {
		t.Fatal(err)
	}
	if err := c.Repos.SignIns.Create(ev()); !errors.Is(err, repositories.ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
	next := ev()
	next.Seq = 1
	if err := c.Repos.SignIns.Create(next); err != nil {
		t.Fatalf("next seq must append: %v", err)
	}
}

// Booking updates are a compare-and-set on the previous status and round-trip their timestamps.
func TestBookingUpdateIsConditional(t *testing.T) {
	c, _ := testutil.NewContainer(t)
	created := time.Date(2026, 11, 23, 7, 0, 0, 0, time.UTC)
	b := &models.Booking{ID: services.NewID(), AgencyID: "ag", Date: "2026-11-23", Kind: models.BookingAM,
		Status: models.BookingRequested, ExpectedArrival: created.Add(time.Hour), CreatedAt: created, UpdatedAt: created}
	if err := c.Repos.Bookings.Create(b, nil); err != nil {
		t.Fatal(err)
	}
	b.Status, b.SupplyTeacherID, b.UpdatedAt = models.BookingConfirmed, "teacher", created.Add(90*time.Second)
	if ok, err := c.Repos.Bookings.Update(b, models.BookingRequested); !ok || err != nil {
		t.Fatalf("update: %v %v", ok, err)
	}
	if ok, err := c.Repos.Bookings.Update(b, models.BookingRequested); ok || err != nil {
		t.Fatalf("stale status must not update: %v %v", ok, err)
	}
	got, err := c.Repos.Bookings.GetByID(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.BookingConfirmed || got.SupplyTeacherID != "teacher" ||
		!got.UpdatedAt.Equal(b.UpdatedAt) || !got.CreatedAt.Equal(created) || !got.ExpectedArrival.Equal(b.ExpectedArrival) {
		t.Fatalf("round trip: %+v", got)
	}
	list, err := c.Repos.Bookings.ListByDates("2026-11-23", "2026-11-23", "ag")
	if err != nil || len(list) == 0 {
		t.Fatalf("list: %v %d", err, len(list))
	}
}
