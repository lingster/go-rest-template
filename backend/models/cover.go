package models

import "time"

// Booking kinds.
const (
	BookingFull = "full"
	BookingAM   = "am"
	BookingPM   = "pm"
)

// Booking statuses.
const (
	BookingRequested = "requested"
	BookingConfirmed = "confirmed"
	BookingCancelled = "cancelled"
)

// Sign-in event types.
const (
	SignInTypeSignIn     = "signin"
	SignInTypeCorrection = "correction"
)

// DBS evidence recorded at sign-in.
const (
	DBSAgencyConfirmation = "agency_confirmation"
	DBSCertificateSeen    = "certificate_seen"
)

// Sign-in correction reasons (fixed list).
const (
	CorrectionWrongPerson      = "wrong_person"
	CorrectionNotOnSite        = "not_on_site"
	CorrectionChecksIncomplete = "checks_incomplete"
)

// CorrectionReasons lists the accepted correction reasons.
var CorrectionReasons = []string{CorrectionWrongPerson, CorrectionNotOnSite, CorrectionChecksIncomplete}

// Agency is a supply-teacher agency. It authenticates to /api/v1/agency with an API key of which only
// the SHA-256 hash is stored.
type Agency struct {
	ID         string    `orm:"column(id);pk;size(36)"`
	Name       string    `orm:"column(name);size(120);unique"`
	Phone      string    `orm:"column(phone);size(32)"`
	APIKeyHash string    `orm:"column(api_key_hash);size(64);unique"`
	IsActive   bool      `orm:"column(is_active)"`
	CreatedAt  time.Time `orm:"column(created_at);type(datetime)"`
}

func (*Agency) TableName() string { return "agencies" }

// SupplyTeacher is a person an agency can send.
type SupplyTeacher struct {
	ID       string `orm:"column(id);pk;size(36)"`
	AgencyID string `orm:"column(agency_id);size(36);index"`
	Name     string `orm:"column(name);size(120)"`
	Subject  string `orm:"column(subject);size(60)"`
	// DBSConfirmed: the agency attests the teacher holds an enhanced DBS check. When false reception must
	// see the certificate at sign-in.
	DBSConfirmed bool      `orm:"column(dbs_confirmed)"`
	CreatedAt    time.Time `orm:"column(created_at);type(datetime)"`
}

func (*SupplyTeacher) TableName() string { return "supply_teachers" }

// Booking is a request to an agency to cover an absent member of staff on one school day.
// Only its status/assignment fields change after creation (see repositories.BookingRepository).
//
// Version is the optimistic-concurrency token: every change to the booking, and every append to its
// sign-in log, increments it with a compare-and-set, so two operations that both read the same state
// can never both be applied.
type Booking struct {
	ID              string    `orm:"column(id);pk;size(36)"`
	AgencyID        string    `orm:"column(agency_id);size(36);index"`
	SupplyTeacherID string    `orm:"column(supply_teacher_id);size(36)"` // empty until confirmed
	Date            string    `orm:"column(date);size(10);index"`        // YYYY-MM-DD, school-local
	Kind            string    `orm:"column(kind);size(8)"`
	Covering        string    `orm:"column(covering);size(120)"` // absent staff display name
	Subject         string    `orm:"column(subject);size(60)"`
	Status          string    `orm:"column(status);size(16)"`
	ExpectedArrival time.Time `orm:"column(expected_arrival);type(datetime)"` // UTC
	Lanyard         string    `orm:"column(lanyard);size(16)"`
	Version         int64     `orm:"column(version)"`
	CreatedAt       time.Time `orm:"column(created_at);type(datetime)"`
	UpdatedAt       time.Time `orm:"column(updated_at);type(datetime)"`
}

func (*Booking) TableName() string { return "bookings" }

// BookingLesson is one period of a booking.
type BookingLesson struct {
	ID        string `orm:"column(id);pk;size(36)"`
	BookingID string `orm:"column(booking_id);size(36);index"`
	Period    string `orm:"column(period);size(8)"`
	ClassName string `orm:"column(class_name);size(50)"`
	Room      string `orm:"column(room);size(50)"`
	Free      bool   `orm:"column(free)"` // booked time with no lesson
}

func (*BookingLesson) TableName() string { return "booking_lessons" }

// EtaUpdate is an append-only arrival estimate reported by an agency. It holds an ETA only, never a location.
//
// Seq is the number of earlier updates for the booking and (booking_id, seq) is unique: two concurrent
// reports computed from the same earlier updates collide, so the per-booking limits are enforced atomically.
type EtaUpdate struct {
	ID         string    `orm:"column(id);pk;size(36)"`
	BookingID  string    `orm:"column(booking_id);size(36);index"`
	Seq        int       `orm:"column(seq)"`
	AgencyID   string    `orm:"column(agency_id);size(36)"`
	ReportedAt time.Time `orm:"column(reported_at);type(datetime)"` // server receive time
	ETA        time.Time `orm:"column(eta);type(datetime)"`
	Note       string    `orm:"column(note);size(120)"`
	CreatedAt  time.Time `orm:"column(created_at);type(datetime)"`
}

func (*EtaUpdate) TableName() string { return "eta_updates" }

// TableUnique makes (booking_id, seq) unique: the concurrency guard of the ETA limits.
func (*EtaUpdate) TableUnique() [][]string { return [][]string{{"BookingID", "Seq"}} }

// SignInEvent is an entry in the append-only reception sign-in log. A correction withdraws the
// sign-in it references (RefID); entries are never edited or removed.
//
// Per booking the events strictly alternate (signin, correction, signin, ...), so Seq is the number of
// earlier events for the booking. (booking_id, seq) is unique: two concurrent appends computed from the
// same state (a double sign-in, or two corrections of one sign-in) collide and only one is stored.
//
// SupplyTeacherID and TeacherName snapshot the teacher the booking had when the event was appended (a
// correction copies them from the sign-in it withdraws), so the log keeps showing who was signed in even
// after the booking is reassigned.
type SignInEvent struct {
	ID              string    `orm:"column(id);pk;size(36)"`
	BookingID       string    `orm:"column(booking_id);size(36);index"`
	Seq             int       `orm:"column(seq)"`
	Type            string    `orm:"column(type);size(16)"`
	RefID           string    `orm:"column(ref_id);size(36)"`
	SupplyTeacherID string    `orm:"column(supply_teacher_id);size(36)"`
	TeacherName     string    `orm:"column(teacher_name);size(255)"`
	At              time.Time `orm:"column(at);type(datetime);index"`
	ActorUserID     string    `orm:"column(actor_user_id);size(36)"`
	Lanyard         string    `orm:"column(lanyard);size(16)"`
	PhotoIDChecked  bool      `orm:"column(photo_id_checked)"`
	DBSEvidence     string    `orm:"column(dbs_evidence);size(32)"`
	Reason          string    `orm:"column(reason);size(32)"`
}

func (*SignInEvent) TableName() string { return "sign_in_events" }

// TableUnique makes (booking_id, seq) unique: the append-only log's concurrency guard.
func (*SignInEvent) TableUnique() [][]string { return [][]string{{"BookingID", "Seq"}} }
