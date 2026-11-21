package services

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
)

// TimeOfDay is a school-local wall-clock time in minutes since midnight.
type TimeOfDay int

// ParseTimeOfDay parses "HH:MM" (24h).
func ParseTimeOfDay(s string) (TimeOfDay, bool) {
	if !dto.IsClock(s) {
		return 0, false
	}
	return TimeOfDay(int(s[0]-'0')*600 + int(s[1]-'0')*60 + int(s[3]-'0')*10 + int(s[4]-'0')), true
}

func mustTimeOfDay(s string) TimeOfDay {
	t, ok := ParseTimeOfDay(s)
	if !ok {
		panic("invalid time of day " + s)
	}
	return t
}

// String formats as "HH:MM".
func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60) }

// Period is one timetabled lesson slot.
type Period struct {
	ID         string
	Start, End TimeOfDay
}

// SchoolDay is the school's timetable and time zone: the single place that knows period times, which
// periods each booking kind covers and the window in which supply teachers can arrive. It is built from
// defaults today and can be loaded from configuration later without touching its users.
type SchoolDay struct {
	Loc     *time.Location
	Periods []Period            // in timetable order
	Kinds   map[string][]string // booking kind -> period ids
	// ArrivalFrom/ArrivalTo bound plausible ETAs (school-local, on the booking date).
	ArrivalFrom, ArrivalTo TimeOfDay
}

// DefaultSchoolDay is the standard five-period day in loc.
func DefaultSchoolDay(loc *time.Location) SchoolDay {
	p := func(id, start, end string) Period {
		return Period{ID: id, Start: mustTimeOfDay(start), End: mustTimeOfDay(end)}
	}
	return SchoolDay{
		Loc: loc,
		Periods: []Period{
			p("P1", "08:55", "09:55"), p("P2", "09:55", "10:55"), p("P3", "11:15", "12:15"),
			p("P4", "13:00", "14:00"), p("P5", "14:00", "15:00"),
		},
		Kinds: map[string][]string{
			models.BookingFull: {"P1", "P2", "P3", "P4", "P5"},
			models.BookingAM:   {"P1", "P2", "P3"},
			models.BookingPM:   {"P4", "P5"},
		},
		ArrivalFrom: mustTimeOfDay("06:00"),
		ArrivalTo:   mustTimeOfDay("16:00"),
	}
}

// Period returns the period with the given id.
func (d SchoolDay) Period(id string) (Period, bool) {
	for _, p := range d.Periods {
		if p.ID == id {
			return p, true
		}
	}
	return Period{}, false
}

// periodIndex orders periods by the timetable (unknown ids last).
func (d SchoolDay) periodIndex(id string) int {
	for i, p := range d.Periods {
		if p.ID == id {
			return i
		}
	}
	return len(d.Periods)
}

// At returns the instant of the school-local wall-clock time t on date (YYYY-MM-DD), in UTC. The
// conversion uses the zone's rules for that date, so it is correct on both sides of a DST change.
func (d SchoolDay) At(date string, t TimeOfDay) (time.Time, error) {
	day, err := time.Parse(dto.DateLayout, date)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), int(t)/60, int(t)%60, 0, 0, d.Loc).UTC(), nil
}

// mustAt is At for dates that were already validated.
func (d SchoolDay) mustAt(date string, t TimeOfDay) time.Time {
	v, err := d.At(date, t)
	if err != nil {
		panic(err)
	}
	return v
}

// LocalDate returns the school-local date of instant t.
func (d SchoolDay) LocalDate(t time.Time) string { return t.In(d.Loc).Format(dto.DateLayout) }

// LocalClock returns the school-local "HH:MM" of instant t.
func (d SchoolDay) LocalClock(t time.Time) string { return t.In(d.Loc).Format("15:04") }

// Monday returns the Monday of the school-local week containing date.
func Monday(date string) (string, error) {
	day, err := time.Parse(dto.DateLayout, date)
	if err != nil {
		return "", err
	}
	offset := (int(day.Weekday()) + 6) % 7 // Monday=0 ... Sunday=6
	return day.AddDate(0, 0, -offset).Format(dto.DateLayout), nil
}

// AddDays shifts a YYYY-MM-DD date by n calendar days (civil arithmetic, unaffected by DST).
func AddDays(date string, n int) string {
	day, err := time.Parse(dto.DateLayout, date)
	if err != nil {
		return date
	}
	return day.AddDate(0, 0, n).Format(dto.DateLayout)
}

// ValidateLessons checks that lessons list each period of the booking kind exactly once.
func (d SchoolDay) ValidateLessons(kind string, lessons []dto.LessonInput) map[string]string {
	want := d.Kinds[kind]
	seen := map[string]bool{}
	for _, l := range lessons {
		if !slices.Contains(want, l.Period) {
			return map[string]string{"lessons": fmt.Sprintf("period %q is not part of a %s booking (%s)",
				l.Period, kind, strings.Join(want, ", "))}
		}
		if seen[l.Period] {
			return map[string]string{"lessons": fmt.Sprintf("period %s is listed twice", l.Period)}
		}
		seen[l.Period] = true
	}
	if len(seen) != len(want) {
		return map[string]string{"lessons": fmt.Sprintf("a %s booking needs each of %s (use free: true for periods without a class)",
			kind, strings.Join(want, ", "))}
	}
	return nil
}
