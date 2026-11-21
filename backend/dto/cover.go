package dto

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lingster/go-rest-template/backend/models"
)

// ---------- cover desk: responses ----------

// Agency is a supply agency. API keys and their hashes are never returned here.
type Agency struct {
	ID        string `json:"id"`
	Name      string `json:"name" example:"Kestrel Teaching"`
	Phone     string `json:"phone" example:"01632 960 214"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at" description:"RFC3339 UTC"`
}

// AgencyKeyResponse is returned once when an agency is created or its key rotated.
type AgencyKeyResponse struct {
	Agency
	APIKey string `json:"api_key" description:"Plaintext agency API key (send as X-Agency-Key). Shown only once; only its SHA-256 hash is stored"`
}

// SupplyTeacher is a person an agency can send.
type SupplyTeacher struct {
	ID           string `json:"id"`
	AgencyID     string `json:"agency_id"`
	Name         string `json:"name" example:"Priya Natarajan"`
	Subject      string `json:"subject" example:"Science"`
	DBSConfirmed bool   `json:"dbs_confirmed" description:"the agency attests an enhanced DBS check"`
	CreatedAt    string `json:"created_at" description:"RFC3339 UTC"`
}

// Lesson is one period of a booking.
type Lesson struct {
	Period    string `json:"period" example:"P1"`
	Start     string `json:"start" description:"HH:MM school-local" example:"08:55"`
	End       string `json:"end" description:"HH:MM school-local" example:"09:55"`
	ClassName string `json:"class_name" example:"9X2"`
	Room      string `json:"room" example:"Lab 2"`
	Free      bool   `json:"free" description:"booked time with no lesson"`
}

// Booking is the cover manager's view of a booking.
type Booking struct {
	ID              string   `json:"id"`
	AgencyID        string   `json:"agency_id"`
	SupplyTeacherID *string  `json:"supply_teacher_id" description:"null until confirmed"`
	Date            string   `json:"date" description:"YYYY-MM-DD school-local" example:"2026-11-23"`
	Kind            string   `json:"kind" description:"full, am or pm" example:"full"`
	Covering        string   `json:"covering" description:"absent member of staff" example:"Mr Okafor"`
	Subject         string   `json:"subject" example:"Science"`
	Status          string   `json:"status" description:"requested, confirmed or cancelled" example:"requested"`
	ExpectedArrival string   `json:"expected_arrival" description:"RFC3339 UTC"`
	Lanyard         string   `json:"lanyard" example:"S-01"`
	Lessons         []Lesson `json:"lessons"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// CoverArrival is one expected arrival as reception sees it (no class, room or covering data).
type CoverArrival struct {
	BookingID              string  `json:"booking_id"`
	TeacherName            string  `json:"teacher_name" description:"empty until the booking is confirmed"`
	AgencyName             string  `json:"agency_name"`
	AgencyPhone            string  `json:"agency_phone"`
	ExpectedArrival        string  `json:"expected_arrival" description:"RFC3339 UTC"`
	Status                 string  `json:"status" description:"signed_in, unconfirmed, no_eta, on_the_way or running_late" example:"on_the_way"`
	ETA                    *string `json:"eta" description:"latest agency ETA, RFC3339 UTC, or null"`
	ETAUpdatedAt           *string `json:"eta_updated_at" description:"when the latest ETA was received (server time), or null"`
	ETAStale               bool    `json:"eta_stale" description:"no ETA update for over 20 minutes while travelling"`
	Lanyard                string  `json:"lanyard"`
	DBSCertificateRequired bool    `json:"dbs_certificate_required" description:"reception must see a DBS certificate at sign-in"`
}

// FirstLesson is the first taught period of a booking.
type FirstLesson struct {
	Period    string `json:"period" example:"P1"`
	Start     string `json:"start" description:"HH:MM school-local" example:"08:55"`
	ClassName string `json:"class_name"`
	Room      string `json:"room"`
}

// CoverManagerArrival adds cover planning data for cover managers.
type CoverManagerArrival struct {
	CoverArrival
	Covering     string       `json:"covering"`
	Subject      string       `json:"subject"`
	Kind         string       `json:"kind"`
	FirstLesson  *FirstLesson `json:"first_lesson"`
	DBSConfirmed bool         `json:"dbs_confirmed"`
	Note         string       `json:"note" description:"latest agency note"`
}

// CoverAttention is something a cover manager should act on.
type CoverAttention struct {
	Severity  string `json:"severity" description:"bad or warn" example:"warn"`
	Type      string `json:"type" description:"unconfirmed, cover_gap or eta_stale" example:"cover_gap"`
	BookingID string `json:"booking_id"`
	Message   string `json:"message" example:"P1 Science needs cover until 09:07"`
	Detail    string `json:"detail"`
}

// CoverTodayReception is GET /cover/today for reception.
type CoverTodayReception struct {
	Date     string         `json:"date" example:"2026-11-23"`
	Timezone string         `json:"timezone" example:"Europe/London"`
	Arrivals []CoverArrival `json:"arrivals"`
	Counts   map[string]int `json:"counts" description:"arrivals per status"`
}

// CoverTodayManager is GET /cover/today for cover managers and admins.
type CoverTodayManager struct {
	Date      string                `json:"date" example:"2026-11-23"`
	Timezone  string                `json:"timezone" example:"Europe/London"`
	Arrivals  []CoverManagerArrival `json:"arrivals"`
	Counts    map[string]int        `json:"counts" description:"arrivals per status"`
	Attention []CoverAttention      `json:"attention"`
}

// CoverWeekCell is one booking in the week grid as reception sees it.
type CoverWeekCell struct {
	BookingID string `json:"booking_id"`
	Kind      string `json:"kind"`
	Status    string `json:"status" description:"requested, confirmed or cancelled"`
}

// CoverWeekManagerCell adds cover planning data.
type CoverWeekManagerCell struct {
	CoverWeekCell
	Covering string   `json:"covering"`
	Lessons  []Lesson `json:"lessons"`
}

// CoverWeekDay is one school day of a row.
type CoverWeekDay struct {
	Date     string          `json:"date"`
	Bookings []CoverWeekCell `json:"bookings"`
}

// CoverWeekManagerDay is one school day of a row (manager view).
type CoverWeekManagerDay struct {
	Date     string                 `json:"date"`
	Bookings []CoverWeekManagerCell `json:"bookings"`
}

// CoverWeekRow is one supply teacher's week (unconfirmed bookings get a row each, teacher_name empty).
type CoverWeekRow struct {
	TeacherName string         `json:"teacher_name"`
	AgencyName  string         `json:"agency_name"`
	Days        []CoverWeekDay `json:"days" description:"Monday to Friday"`
}

// CoverWeekManagerRow is CoverWeekRow for cover managers.
type CoverWeekManagerRow struct {
	TeacherName string                `json:"teacher_name"`
	AgencyName  string                `json:"agency_name"`
	Days        []CoverWeekManagerDay `json:"days" description:"Monday to Friday"`
}

// CoverWeekReception is GET /cover/week for reception.
type CoverWeekReception struct {
	Start string         `json:"start" description:"Monday, YYYY-MM-DD"`
	Rows  []CoverWeekRow `json:"rows"`
}

// CoverWeekManager is GET /cover/week for cover managers and admins.
type CoverWeekManager struct {
	Start string                `json:"start" description:"Monday, YYYY-MM-DD"`
	Rows  []CoverWeekManagerRow `json:"rows"`
}

// SignInEvent is one entry of the append-only sign-in log.
type SignInEvent struct {
	ID             string `json:"id"`
	BookingID      string `json:"booking_id"`
	TeacherName    string `json:"teacher_name"`
	Type           string `json:"type" description:"signin or correction"`
	RefID          string `json:"ref_id,omitempty" description:"correction: the sign-in it withdraws"`
	At             string `json:"at" description:"RFC3339 UTC, server time"`
	ActorUserID    string `json:"actor_user_id"`
	Lanyard        string `json:"lanyard,omitempty"`
	PhotoIDChecked bool   `json:"photo_id_checked"`
	DBSEvidence    string `json:"dbs_evidence,omitempty" description:"agency_confirmation or certificate_seen"`
	Reason         string `json:"reason,omitempty" description:"correction: wrong_person, not_on_site or checks_incomplete"`
}

// SignInLog is GET /cover/sign-ins.
type SignInLog struct {
	Date  string        `json:"date"`
	Items []SignInEvent `json:"items" description:"newest first"`
}

// AgencyBooking is what an agency sees of its own booking (no class, room or covering data).
type AgencyBooking struct {
	ID              string `json:"id"`
	Date            string `json:"date"`
	Kind            string `json:"kind"`
	Status          string `json:"status"`
	TeacherName     string `json:"teacher_name"`
	ExpectedArrival string `json:"expected_arrival" description:"RFC3339 UTC"`
}

// AgencyBookingList is GET /agency/bookings.
type AgencyBookingList struct {
	Date  string          `json:"date"`
	Items []AgencyBooking `json:"items"`
}

// EtaUpdate is an accepted ETA report.
type EtaUpdate struct {
	ID         string `json:"id"`
	BookingID  string `json:"booking_id"`
	ETA        string `json:"eta" description:"RFC3339 UTC"`
	ReportedAt string `json:"reported_at" description:"server receive time, RFC3339 UTC"`
	Note       string `json:"note"`
}

// ---------- cover desk: requests ----------

// CreateAgencyRequest creates an agency.
type CreateAgencyRequest struct {
	Name  string `json:"name" required:"true"`
	Phone string `json:"phone" required:"true"`
}

// CreateTeacherRequest registers a supply teacher with an agency.
type CreateTeacherRequest struct {
	AgencyID     string `json:"agency_id" required:"true"`
	Name         string `json:"name" required:"true"`
	Subject      string `json:"subject"`
	DBSConfirmed bool   `json:"dbs_confirmed" description:"the agency attests an enhanced DBS check"`
}

// LessonInput is one period of a booking.
type LessonInput struct {
	Period    string `json:"period" required:"true" example:"P1"`
	ClassName string `json:"class_name" description:"required unless free"`
	Room      string `json:"room" description:"required unless free"`
	Free      bool   `json:"free" description:"booked time with no lesson"`
}

// CreateBookingRequest books an agency for one school day.
type CreateBookingRequest struct {
	AgencyID        string        `json:"agency_id" required:"true"`
	Date            string        `json:"date" required:"true" description:"YYYY-MM-DD school-local"`
	Kind            string        `json:"kind" required:"true" description:"full (P1-P5), am (P1-P3) or pm (P4-P5)"`
	Covering        string        `json:"covering" required:"true" description:"absent member of staff"`
	Subject         string        `json:"subject" required:"true"`
	ExpectedArrival string        `json:"expected_arrival" required:"true" description:"HH:MM school-local on the booking date" example:"08:15"`
	Lanyard         string        `json:"lanyard" description:"short visitor lanyard label"`
	Lessons         []LessonInput `json:"lessons" required:"true" description:"each period of the kind exactly once"`
}

// UpdateBookingRequest confirms, cancels or edits a booking (all fields optional).
type UpdateBookingRequest struct {
	Status          *string       `json:"status" description:"confirmed (needs supply_teacher_id) or cancelled"`
	SupplyTeacherID *string       `json:"supply_teacher_id" description:"teacher of the booking's agency; only with status=confirmed"`
	Covering        *string       `json:"covering"`
	Lanyard         *string       `json:"lanyard"`
	Lessons         []LessonInput `json:"lessons" description:"replaces all lessons"`
}

// SignInRequest records reception's checks at sign-in.
type SignInRequest struct {
	PhotoIDChecked     bool `json:"photo_id_checked" required:"true" description:"must be true"`
	DBSCertificateSeen bool `json:"dbs_certificate_seen" description:"required when the agency has not confirmed a DBS check"`
}

// CorrectionRequest withdraws a sign-in.
type CorrectionRequest struct {
	Reason string `json:"reason" required:"true" description:"wrong_person, not_on_site or checks_incomplete"`
}

// EtaRequest is an agency's arrival estimate.
type EtaRequest struct {
	ETA  string `json:"eta" required:"true" description:"HH:MM (school-local, booking date) or RFC3339" example:"08:12"`
	Note string `json:"note" description:"at most 120 characters; control characters are removed"`
}

// ---------- cover desk: validation ----------

// MaxNoteLen bounds an ETA note.
const MaxNoteLen = 120

// DateLayout is the booking date format.
const DateLayout = "2006-01-02"

var clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// IsClock reports whether s is a 24h "HH:MM" time.
func IsClock(s string) bool { return clockRe.MatchString(s) }

// IsDate reports whether s is a valid YYYY-MM-DD date.
func IsDate(s string) bool {
	_, err := time.Parse(DateLayout, s)
	return err == nil
}

// CleanText trims s and removes control characters (newlines included) from untrusted free text.
func CleanText(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		// Control characters (incl. newlines) and format characters (bidi overrides, zero-width) are dropped.
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, s))
}

func checkText(d map[string]string, field, v string, required bool, max int) {
	v = strings.TrimSpace(v)
	switch {
	case required && v == "":
		d[field] = "is required"
	case utf8.RuneCountInString(v) > max:
		d[field] = "is too long"
	}
}

func checkLessons(d map[string]string, lessons []LessonInput) {
	for _, l := range lessons {
		if l.Free {
			continue
		}
		if strings.TrimSpace(l.ClassName) == "" || strings.TrimSpace(l.Room) == "" {
			d["lessons"] = "class_name and room are required unless free"
		}
		checkText(d, "lessons", l.ClassName, false, 50)
		checkText(d, "lessons", l.Room, false, 50)
	}
}

func (r *CreateAgencyRequest) Validate() map[string]string {
	d := map[string]string{}
	checkText(d, "name", r.Name, true, 120)
	checkText(d, "phone", r.Phone, true, 32)
	return d
}

func (r *CreateTeacherRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "agency_id", r.AgencyID)
	checkText(d, "name", r.Name, true, 120)
	checkText(d, "subject", r.Subject, false, 60)
	return d
}

func (r *CreateBookingRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "agency_id", r.AgencyID)
	if !IsDate(r.Date) {
		d["date"] = "must be YYYY-MM-DD"
	}
	if r.Kind != models.BookingFull && r.Kind != models.BookingAM && r.Kind != models.BookingPM {
		d["kind"] = "must be full, am or pm"
	}
	checkText(d, "covering", r.Covering, true, 120)
	checkText(d, "subject", r.Subject, true, 60)
	if !IsClock(r.ExpectedArrival) {
		d["expected_arrival"] = "must be HH:MM"
	}
	checkText(d, "lanyard", r.Lanyard, false, 16)
	if len(r.Lessons) == 0 {
		d["lessons"] = "is required"
	}
	checkLessons(d, r.Lessons)
	return d
}

func (r *UpdateBookingRequest) Validate() map[string]string {
	d := map[string]string{}
	if r.Status == nil && r.SupplyTeacherID == nil && r.Covering == nil && r.Lanyard == nil && r.Lessons == nil {
		d["body"] = "nothing to update"
	}
	if r.Status != nil && *r.Status != models.BookingConfirmed && *r.Status != models.BookingCancelled {
		d["status"] = "must be confirmed or cancelled"
	}
	confirming := r.Status != nil && *r.Status == models.BookingConfirmed
	switch {
	case confirming && (r.SupplyTeacherID == nil || strings.TrimSpace(*r.SupplyTeacherID) == ""):
		d["supply_teacher_id"] = "is required to confirm"
	case !confirming && r.SupplyTeacherID != nil:
		d["supply_teacher_id"] = "is only accepted with status=confirmed"
	}
	if r.Covering != nil {
		checkText(d, "covering", *r.Covering, true, 120)
	}
	if r.Lanyard != nil {
		checkText(d, "lanyard", *r.Lanyard, false, 16)
	}
	if r.Lessons != nil && len(r.Lessons) == 0 {
		d["lessons"] = "must not be empty"
	}
	checkLessons(d, r.Lessons)
	return d
}

func (r *SignInRequest) Validate() map[string]string {
	d := map[string]string{}
	if !r.PhotoIDChecked {
		d["photo_id_checked"] = "photo ID must be checked against the booking before sign-in"
	}
	return d
}

func (r *CorrectionRequest) Validate() map[string]string {
	d := map[string]string{}
	if !slices.Contains(models.CorrectionReasons, r.Reason) {
		d["reason"] = "must be one of " + strings.Join(models.CorrectionReasons, ", ")
	}
	return d
}

func (r *EtaRequest) Validate() map[string]string {
	d := map[string]string{}
	checkRequired(d, "eta", r.ETA)
	if len(r.ETA) > 64 {
		d["eta"] = "is too long"
	}
	if utf8.RuneCountInString(CleanText(r.Note)) > MaxNoteLen {
		d["note"] = "must be at most 120 characters"
	}
	return d
}
