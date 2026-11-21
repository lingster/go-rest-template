package repositories

import (
	"context"
	"fmt"

	"github.com/beego/beego/v2/client/orm"

	"github.com/lingster/go-rest-template/backend/models"
)

// ---- shared helpers (DRY) ----

// getMany loads rows whose id is in ids into dest (a pointer to a slice).
func getMany(model interface{}, ids []string, dest interface{}) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := orm.NewOrm().QueryTable(model).Filter("id__in", ids).All(dest)
	return err
}

// ---- agencies ----

type ormAgencyRepo struct{}

func (*ormAgencyRepo) Create(a *models.Agency) error { return insert(a) }

func (*ormAgencyRepo) GetByID(id string) (*models.Agency, error) {
	a := &models.Agency{}
	return a, one(orm.NewOrm().QueryTable(a).Filter("id", id), a)
}

func (*ormAgencyRepo) GetByKeyHash(hash string) (*models.Agency, error) {
	a := &models.Agency{}
	return a, one(orm.NewOrm().QueryTable(a).Filter("api_key_hash", hash), a)
}

func (*ormAgencyRepo) List() ([]*models.Agency, error) {
	var out []*models.Agency
	_, err := orm.NewOrm().QueryTable(new(models.Agency)).OrderBy("name").All(&out)
	return out, err
}

func (*ormAgencyRepo) GetMany(ids []string) (map[string]*models.Agency, error) {
	var rows []*models.Agency
	if err := getMany(new(models.Agency), ids, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]*models.Agency, len(rows))
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, nil
}

func (*ormAgencyRepo) SetKeyHash(id, hash string) error {
	n, err := orm.NewOrm().QueryTable(new(models.Agency)).Filter("id", id).Update(orm.Params{"api_key_hash": hash})
	if err == nil && n == 0 {
		return ErrNotFound
	}
	return err
}

// ---- supply teachers ----

type ormTeacherRepo struct{}

func (*ormTeacherRepo) Create(t *models.SupplyTeacher) error { return insert(t) }

func (*ormTeacherRepo) GetByID(id string) (*models.SupplyTeacher, error) {
	t := &models.SupplyTeacher{}
	return t, one(orm.NewOrm().QueryTable(t).Filter("id", id), t)
}

func (*ormTeacherRepo) List(agencyID string) ([]*models.SupplyTeacher, error) {
	qs := orm.NewOrm().QueryTable(new(models.SupplyTeacher))
	if agencyID != "" {
		qs = qs.Filter("agency_id", agencyID)
	}
	var out []*models.SupplyTeacher
	_, err := qs.OrderBy("name", "id").All(&out)
	return out, err
}

func (*ormTeacherRepo) GetMany(ids []string) (map[string]*models.SupplyTeacher, error) {
	var rows []*models.SupplyTeacher
	if err := getMany(new(models.SupplyTeacher), ids, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]*models.SupplyTeacher, len(rows))
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, nil
}

// ---- bookings ----

type ormBookingRepo struct{}

func (*ormBookingRepo) Create(b *models.Booking, lessons []*models.BookingLesson) error {
	return orm.NewOrm().DoTx(func(_ context.Context, tx orm.TxOrmer) error {
		if err := insertWith(tx, b); err != nil {
			return err
		}
		return insertLessons(tx, lessons)
	})
}

func insertLessons(tx orm.TxOrmer, lessons []*models.BookingLesson) error {
	for _, l := range lessons { // at most one per period: a handful of rows
		if err := insertWith(tx, l); err != nil {
			return err
		}
	}
	return nil
}

func (*ormBookingRepo) GetByID(id string) (*models.Booking, error) {
	b := &models.Booking{}
	return b, one(orm.NewOrm().QueryTable(b).Filter("id", id), b)
}

func (*ormBookingRepo) ListByDates(from, to, agencyID string) ([]*models.Booking, error) {
	qs := orm.NewOrm().QueryTable(new(models.Booking)).Filter("date__gte", from).Filter("date__lte", to)
	if agencyID != "" {
		qs = qs.Filter("agency_id", agencyID)
	}
	var out []*models.Booking
	_, err := qs.OrderBy("date", "expected_arrival", "id").All(&out)
	return out, err
}

// bookingColumns are the booking columns UpdateIf may write, with their values.
var bookingColumns = map[string]func(*models.Booking) interface{}{
	"status":            func(b *models.Booking) interface{} { return b.Status },
	"supply_teacher_id": func(b *models.Booking) interface{} { return b.SupplyTeacherID },
	"covering":          func(b *models.Booking) interface{} { return b.Covering },
	"lanyard":           func(b *models.Booking) interface{} { return b.Lanyard },
	"updated_at":        func(b *models.Booking) interface{} { return b.UpdatedAt },
}

var defaultBookingColumns = []string{"status", "supply_teacher_id", "covering", "lanyard", "updated_at"}

// casBooking is the one compare-and-set every booking mutation and sign-in append goes through: it
// increments the booking's version (and writes set) iff the booking still matches check, else it returns
// ErrVersionConflict. It must run inside the transaction that does the guarded writes.
func casBooking(tx orm.TxOrmer, id string, check BookingCheck, set orm.Params) error {
	qs := tx.QueryTable(new(models.Booking)).Filter("id", id).Filter("version", check.Version)
	if check.Status != "" {
		qs = qs.Filter("status", check.Status)
	}
	if check.SupplyTeacherID != "" {
		qs = qs.Filter("supply_teacher_id", check.SupplyTeacherID)
	}
	if set == nil {
		set = orm.Params{}
	}
	set["version"] = orm.ColValue(orm.ColAdd, 1)
	n, err := qs.Update(set)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (*ormBookingRepo) UpdateIf(b *models.Booking, expectedVersion int64, lessons []*models.BookingLesson, cols ...string) error {
	if len(cols) == 0 {
		cols = defaultBookingColumns
	}
	set := make(orm.Params, len(cols)+1)
	for _, c := range cols {
		value, ok := bookingColumns[c]
		if !ok {
			return fmt.Errorf("booking column %q cannot be updated", c)
		}
		set[c] = value(b)
	}
	err := orm.NewOrm().DoTx(func(_ context.Context, tx orm.TxOrmer) error {
		if err := casBooking(tx, b.ID, BookingCheck{Version: expectedVersion}, set); err != nil {
			return err
		}
		if lessons == nil {
			return nil
		}
		if _, err := tx.QueryTable(new(models.BookingLesson)).Filter("booking_id", b.ID).Delete(); err != nil {
			return err
		}
		return insertLessons(tx, lessons)
	})
	if err == nil {
		b.Version = expectedVersion + 1
	}
	return err
}

func (*ormBookingRepo) LessonsByBookings(ids []string) (map[string][]*models.BookingLesson, error) {
	out := map[string][]*models.BookingLesson{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []*models.BookingLesson
	if _, err := orm.NewOrm().QueryTable(new(models.BookingLesson)).Filter("booking_id__in", ids).
		OrderBy("period").All(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.BookingID] = append(out[r.BookingID], r)
	}
	return out, nil
}

// ---- ETA updates (append-only) ----

type ormEtaRepo struct{}

func (*ormEtaRepo) Create(u *models.EtaUpdate) error { return insert(u) }

func (*ormEtaRepo) ListByBookings(ids []string) (map[string][]*models.EtaUpdate, error) {
	out := map[string][]*models.EtaUpdate{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []*models.EtaUpdate
	if _, err := orm.NewOrm().QueryTable(new(models.EtaUpdate)).Filter("booking_id__in", ids).
		OrderBy("seq").All(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.BookingID] = append(out[r.BookingID], r)
	}
	return out, nil
}

// ---- sign-in log (append-only) ----

type ormSignInRepo struct{}

func (*ormSignInRepo) AppendIf(e *models.SignInEvent, check BookingCheck) error {
	return orm.NewOrm().DoTx(func(_ context.Context, tx orm.TxOrmer) error {
		if err := casBooking(tx, e.BookingID, check, nil); err != nil {
			return err
		}
		return insertWith(tx, e) // a unique-index loss returns ErrDuplicate and rolls the version bump back
	})
}

func (*ormSignInRepo) GetByID(id string) (*models.SignInEvent, error) {
	e := &models.SignInEvent{}
	return e, one(orm.NewOrm().QueryTable(e).Filter("id", id), e)
}

func (*ormSignInRepo) ListByBookings(ids []string) (map[string][]*models.SignInEvent, error) {
	out := map[string][]*models.SignInEvent{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []*models.SignInEvent
	if _, err := orm.NewOrm().QueryTable(new(models.SignInEvent)).Filter("booking_id__in", ids).
		OrderBy("seq").All(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.BookingID] = append(out[r.BookingID], r)
	}
	return out, nil
}
