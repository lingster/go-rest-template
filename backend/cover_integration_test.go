package main

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beego/beego/v2/client/orm"

	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/services"
)

// coverDate is a Monday; every cover test runs with the cover desk's clock on it (school time).
const coverDate = "2026-11-23"

// setCoverClock fixes the cover desk's clock at the London wall-clock time hhmm on coverDate.
func setCoverClock(t *testing.T, hhmm string) {
	t.Helper()
	loc, _ := time.LoadLocation("Europe/London")
	v, err := time.ParseInLocation("2006-01-02 15:04", coverDate+" "+hhmm, loc)
	if err != nil {
		t.Fatal(err)
	}
	coverClock.Set(v)
	t.Cleanup(func() { coverClock.Set(time.Time{}) })
}

func adminToken(t *testing.T) string {
	t.Helper()
	r := call(t, "POST", "/api/v1/auth/login", map[string]string{"email": adminEmail, "password": adminPassword}, "")
	expect(t, r, 200, "")
	return r.str("tokens", "access_token")
}

// coverUser returns an access token for a new user with the role ("user" keeps the default).
func coverUser(t *testing.T, admin, role string) string {
	t.Helper()
	email, access, _ := newVerifiedUser(t, role)
	if role != models.RoleUser {
		list := call(t, "GET", "/api/v1/admin/users?q="+email, nil, admin)
		id := list.Body["items"].([]interface{})[0].(map[string]interface{})["id"].(string)
		expect(t, call(t, "PATCH", "/api/v1/admin/users/"+id, map[string]string{"role": role}, admin), 200, "")
	}
	return access
}

type coverFixture struct {
	admin, manager, reception, user string
}

func newCoverFixture(t *testing.T) coverFixture {
	t.Helper()
	admin := adminToken(t)
	return coverFixture{admin: admin, manager: coverUser(t, admin, models.RoleCoverManager),
		reception: coverUser(t, admin, models.RoleReception), user: coverUser(t, admin, models.RoleUser)}
}

// createAgency returns the agency id and its plaintext API key.
func createAgency(t *testing.T, manager string) (id, key string) {
	t.Helper()
	r := call(t, "POST", "/api/v1/cover/agencies", map[string]string{
		"name": "Agency " + services.NewID()[:8], "phone": "01632 960 214"}, manager)
	expect(t, r, 201, "")
	if !strings.HasPrefix(r.str("api_key"), "agk_") {
		t.Fatalf("no api key: %s", r.Raw)
	}
	return r.str("id"), r.str("api_key")
}

func createTeacher(t *testing.T, manager, agencyID, name string, dbs bool) string {
	t.Helper()
	r := call(t, "POST", "/api/v1/cover/teachers", map[string]interface{}{
		"agency_id": agencyID, "name": name, "subject": "Science", "dbs_confirmed": dbs}, manager)
	expect(t, r, 201, "")
	return r.str("id")
}

var amLessons = []map[string]interface{}{
	{"period": "P1", "class_name": "9X2", "room": "Lab 2"},
	{"period": "P2", "class_name": "8K1", "room": "Lab 2"},
	{"period": "P3", "free": true},
}

// createBooking books agencyID for an "am" session on date, expected at 08:15, optionally confirming teacherID.
func createBooking(t *testing.T, manager, agencyID, date, teacherID string) string {
	t.Helper()
	r := call(t, "POST", "/api/v1/cover/bookings", map[string]interface{}{
		"agency_id": agencyID, "date": date, "kind": "am", "covering": "Mr Okafor-Secret", "subject": "Science",
		"expected_arrival": "08:15", "lanyard": "S-01", "lessons": amLessons}, manager)
	expect(t, r, 201, "")
	if r.str("status") != "requested" || r.str("expected_arrival") != date+"T08:15:00Z" { // GMT in November
		t.Fatalf("bad booking: %s", r.Raw)
	}
	id := r.str("id")
	if teacherID != "" {
		c := call(t, "PATCH", "/api/v1/cover/bookings/"+id, map[string]string{"status": "confirmed", "supply_teacher_id": teacherID}, manager)
		expect(t, c, 200, "")
	}
	return id
}

// confirmedBooking sets up an agency, a teacher and a confirmed booking on coverDate.
func confirmedBooking(t *testing.T, manager string, dbs bool) (bookingID, agencyKey string) {
	t.Helper()
	agencyID, key := createAgency(t, manager)
	teacher := createTeacher(t, manager, agencyID, "Teacher "+services.NewID()[:6], dbs)
	return createBooking(t, manager, agencyID, coverDate, teacher), key
}

func agencyCall(t *testing.T, method, path string, body interface{}, key string) resp {
	t.Helper()
	h := map[string]string{}
	if key != "" {
		h["X-Agency-Key"] = key
	}
	return callWith(t, method, path, body, "", h)
}

// findByBooking returns the object in r.Body[list] whose booking_id (or id) is id.
func findByBooking(t *testing.T, r resp, list, id string) map[string]interface{} {
	t.Helper()
	items, _ := r.Body[list].([]interface{})
	for _, it := range items {
		m := it.(map[string]interface{})
		if m["booking_id"] == id || m["id"] == id {
			return m
		}
	}
	t.Fatalf("booking %s not in %s: %s", id, list, r.Raw)
	return nil
}

func TestCoverRoleGatingAndProjection(t *testing.T) {
	setCoverClock(t, "08:00")
	f := newCoverFixture(t)

	expect(t, call(t, "GET", "/api/v1/cover/today", nil, ""), 401, "unauthorized")
	expect(t, call(t, "GET", "/api/v1/cover/today", nil, f.user), 403, "forbidden")
	expect(t, call(t, "GET", "/api/v1/cover/sign-ins", nil, f.user), 403, "forbidden")

	agencyID, _ := createAgency(t, f.manager)
	bookingID := createBooking(t, f.manager, agencyID, coverDate, "")

	// Reception cannot manage cover, whatever the body.
	for _, c := range []struct{ method, path string }{
		{"POST", "/api/v1/cover/agencies"}, {"GET", "/api/v1/cover/agencies"},
		{"POST", "/api/v1/cover/agencies/" + agencyID + "/rotate-key"},
		{"POST", "/api/v1/cover/teachers"}, {"GET", "/api/v1/cover/teachers"},
		{"POST", "/api/v1/cover/bookings"}, {"PATCH", "/api/v1/cover/bookings/" + bookingID},
	} {
		expect(t, call(t, c.method, c.path, map[string]string{"name": "x", "phone": "1"}, f.reception), 403, "forbidden")
		expect(t, call(t, c.method, c.path, map[string]string{"name": "x", "phone": "1"}, f.user), 403, "forbidden")
	}
	// Admins have cover-manager rights.
	expect(t, call(t, "GET", "/api/v1/cover/agencies", nil, f.admin), 200, "")

	// Agency listings never carry keys or hashes.
	list := call(t, "GET", "/api/v1/cover/agencies", nil, f.manager)
	expect(t, list, 200, "")
	if strings.Contains(string(list.Raw), "api_key") || strings.Contains(string(list.Raw), "hash") {
		t.Fatalf("agency list leaks key material: %s", list.Raw)
	}

	// Reception's today view physically lacks manager fields (asserted on raw JSON keys).
	rec := call(t, "GET", "/api/v1/cover/today?date="+coverDate, nil, f.reception)
	expect(t, rec, 200, "")
	if _, ok := rec.Body["attention"]; ok {
		t.Fatalf("reception got attention items: %s", rec.Raw)
	}
	row := findByBooking(t, rec, "arrivals", bookingID)
	for _, k := range []string{"covering", "subject", "kind", "first_lesson", "dbs_confirmed", "note"} {
		if _, ok := row[k]; ok {
			t.Errorf("reception row has %q", k)
		}
	}
	if row["status"] != "unconfirmed" || row["agency_phone"] != "01632 960 214" {
		t.Errorf("reception row = %v", row)
	}
	for _, leak := range []string{"Mr Okafor-Secret", "9X2", "Lab 2"} {
		if strings.Contains(string(rec.Raw), leak) {
			t.Errorf("reception today leaks %q", leak)
		}
	}

	mgr := call(t, "GET", "/api/v1/cover/today", nil, f.manager) // default date = clock's school date
	expect(t, mgr, 200, "")
	mrow := findByBooking(t, mgr, "arrivals", bookingID)
	first, _ := mrow["first_lesson"].(map[string]interface{})
	if mrow["covering"] != "Mr Okafor-Secret" || first["period"] != "P1" || first["start"] != "08:55" || mgr.str("date") != coverDate {
		t.Errorf("manager row = %v", mrow)
	}
	attention, _ := mgr.Body["attention"].([]interface{})
	found := false
	for _, a := range attention {
		m := a.(map[string]interface{})
		found = found || (m["booking_id"] == bookingID && m["type"] == "unconfirmed" && m["severity"] == "bad")
	}
	if !found {
		t.Errorf("unconfirmed booking not flagged: %s", mgr.Raw)
	}

	// Week view: reception gets name/agency/kind/status only.
	wr := call(t, "GET", "/api/v1/cover/week?start="+coverDate, nil, f.reception)
	expect(t, wr, 200, "")
	if strings.Contains(string(wr.Raw), "covering") || strings.Contains(string(wr.Raw), "lessons") {
		t.Errorf("reception week leaks manager data: %s", wr.Raw)
	}
	wm := call(t, "GET", "/api/v1/cover/week", nil, f.manager)
	expect(t, wm, 200, "")
	if !strings.Contains(string(wm.Raw), "Mr Okafor-Secret") || wm.str("start") != coverDate {
		t.Errorf("manager week misses covering: %s", wm.Raw)
	}
	expect(t, call(t, "GET", "/api/v1/cover/week?start=2026-11-24", nil, f.manager), 422, "validation_error")
	expect(t, call(t, "GET", "/api/v1/cover/today?date=23/11/2026", nil, f.reception), 422, "validation_error")
}

func TestCoverBookingValidation(t *testing.T) {
	setCoverClock(t, "08:00")
	f := newCoverFixture(t)
	agencyID, _ := createAgency(t, f.manager)
	otherAgency, _ := createAgency(t, f.manager)
	foreign := createTeacher(t, f.manager, otherAgency, "Other", true)

	base := func() map[string]interface{} {
		return map[string]interface{}{"agency_id": agencyID, "date": coverDate, "kind": "pm", "covering": "Ms Ruiz",
			"subject": "PE", "expected_arrival": "12:30", "lessons": []map[string]interface{}{
				{"period": "P4", "class_name": "7R2", "room": "Hall"}, {"period": "P5", "free": true}}}
	}
	for name, mutate := range map[string]func(m map[string]interface{}){
		"kind/period mismatch": func(m map[string]interface{}) { m["kind"] = "am" },
		"bad date":             func(m map[string]interface{}) { m["date"] = "2026-13-01" },
		"bad time":             func(m map[string]interface{}) { m["expected_arrival"] = "8am" },
		"unknown agency":       func(m map[string]interface{}) { m["agency_id"] = "nope" },
		"missing class": func(m map[string]interface{}) {
			m["lessons"] = []map[string]interface{}{{"period": "P4"}, {"period": "P5", "free": true}}
		},
	} {
		body := base()
		mutate(body)
		if r := call(t, "POST", "/api/v1/cover/bookings", body, f.manager); r.Status != 422 {
			t.Errorf("%s: expected 422, got %d %s", name, r.Status, r.Raw)
		}
	}
	id := createBooking(t, f.manager, agencyID, coverDate, "")
	// The teacher must belong to the booking's agency.
	r := call(t, "PATCH", "/api/v1/cover/bookings/"+id, map[string]string{"status": "confirmed", "supply_teacher_id": foreign}, f.manager)
	expect(t, r, 422, "validation_error")
	expect(t, call(t, "PATCH", "/api/v1/cover/bookings/"+id, map[string]string{"status": "confirmed"}, f.manager), 422, "validation_error")
	// Cancelled bookings are final and disappear from today.
	expect(t, call(t, "PATCH", "/api/v1/cover/bookings/"+id, map[string]string{"status": "cancelled"}, f.manager), 200, "")
	expect(t, call(t, "PATCH", "/api/v1/cover/bookings/"+id, map[string]string{"covering": "Someone"}, f.manager), 409, "conflict")
	if strings.Contains(string(call(t, "GET", "/api/v1/cover/today", nil, f.manager).Raw), id) {
		t.Error("cancelled booking listed today")
	}
	expect(t, call(t, "PATCH", "/api/v1/cover/bookings/missing", map[string]string{"covering": "x"}, f.manager), 404, "not_found")
}

func TestAgencyKeyAuthentication(t *testing.T) {
	setCoverClock(t, "08:00")
	f := newCoverFixture(t)
	agencyID, key := createAgency(t, f.manager)

	unauthorized := func(key string) string {
		t.Helper()
		r := agencyCall(t, "GET", "/api/v1/agency/bookings", nil, key)
		expect(t, r, 401, "unauthorized")
		return r.str("error", "message")
	}
	msgs := []string{unauthorized(""), unauthorized("agk_wrong"), unauthorized(strings.Repeat("x", 500))}
	expect(t, agencyCall(t, "GET", "/api/v1/agency/bookings", nil, key), 200, "")
	// A user's bearer token is not an agency credential.
	expect(t, call(t, "GET", "/api/v1/agency/bookings", nil, f.admin), 401, "unauthorized")

	rot := call(t, "POST", "/api/v1/cover/agencies/"+agencyID+"/rotate-key", nil, f.manager)
	expect(t, rot, 200, "")
	newKey := rot.str("api_key")
	if newKey == "" || newKey == key {
		t.Fatalf("key not rotated: %s", rot.Raw)
	}
	msgs = append(msgs, unauthorized(key))
	expect(t, agencyCall(t, "GET", "/api/v1/agency/bookings", nil, newKey), 200, "")

	// Deactivated agencies are refused like unknown keys (no endpoint deactivates yet; flip the row).
	if _, err := orm.NewOrm().QueryTable(new(models.Agency)).Filter("id", agencyID).Update(orm.Params{"is_active": false}); err != nil {
		t.Fatal(err)
	}
	msgs = append(msgs, unauthorized(newKey))
	for _, m := range msgs {
		if m != msgs[0] {
			t.Fatalf("401 messages differ, revealing key state: %q", msgs)
		}
	}
}

func TestAgencyBookingsAndETA(t *testing.T) {
	setCoverClock(t, "08:00")
	f := newCoverFixture(t)
	bookingID, key := confirmedBooking(t, f.manager, true)
	otherBooking, otherKey := confirmedBooking(t, f.manager, true)

	// Agencies see only their own bookings, without class, room or covering data.
	r := agencyCall(t, "GET", "/api/v1/agency/bookings?date="+coverDate, nil, key)
	expect(t, r, 200, "")
	items, _ := r.Body["items"].([]interface{})
	if len(items) != 1 || items[0].(map[string]interface{})["id"] != bookingID {
		t.Fatalf("agency bookings = %s", r.Raw)
	}
	for _, leak := range []string{"covering", "lessons", "Mr Okafor-Secret", "9X2", "Lab 2", otherBooking} {
		if strings.Contains(string(r.Raw), leak) {
			t.Errorf("agency bookings leak %q: %s", leak, r.Raw)
		}
	}

	eta := func(id, key string, body map[string]string) resp {
		t.Helper()
		return agencyCall(t, "POST", "/api/v1/agency/bookings/"+id+"/eta", body, key)
	}
	// Another agency's booking looks exactly like a missing one.
	cross := eta(otherBooking, key, map[string]string{"eta": "08:10"})
	missing := eta("does-not-exist", key, map[string]string{"eta": "08:10"})
	expect(t, cross, 404, "not_found")
	if string(cross.Raw) != string(missing.Raw) {
		t.Errorf("cross-agency and missing differ: %s vs %s", cross.Raw, missing.Raw)
	}
	for name, body := range map[string]map[string]string{
		"before window":     {"eta": "05:30"},
		"after window":      {"eta": "16:30"},
		"15 min in past":    {"eta": "07:45"},
		"garbage":           {"eta": "soon"},
		"missing":           {},
		"other day RFC3339": {"eta": "2026-11-24T08:10:00Z"},
		"long note":         {"eta": "08:10", "note": strings.Repeat("a", 121)},
	} {
		if r := eta(bookingID, key, body); r.Status != 422 || r.errCode() != "validation_error" {
			t.Errorf("%s: expected 422, got %d %s", name, r.Status, r.Raw)
		}
	}

	ok := eta(bookingID, key, map[string]string{"eta": "2026-11-23T08:25:00Z", "note": "Set off\n‮evil"})
	expect(t, ok, 201, "")
	if ok.str("reported_at") != "2026-11-23T08:00:00Z" || ok.str("eta") != "2026-11-23T08:25:00Z" || ok.str("note") != "Set offevil" {
		t.Fatalf("eta response = %s", ok.Raw)
	}
	expect(t, eta(bookingID, key, map[string]string{"eta": "08:30"}), 429, "rate_limited")
	coverClock.Advance(31 * time.Second)
	expect(t, eta(bookingID, key, map[string]string{"eta": "08:30", "note": "Roadworks"}), 201, "")
	expect(t, eta(otherBooking, otherKey, map[string]string{"eta": "08:12"}), 201, "")

	today := call(t, "GET", "/api/v1/cover/today", nil, f.manager)
	row := findByBooking(t, today, "arrivals", bookingID)
	if row["status"] != "running_late" || row["eta"] != "2026-11-23T08:30:00Z" || row["note"] != "Roadworks" || row["eta_stale"] != false {
		t.Errorf("today row = %v", row)
	}
	if other := findByBooking(t, today, "arrivals", otherBooking); other["status"] != "on_the_way" {
		t.Errorf("other row = %v", other)
	}
}

// Unconfirmed and other-day bookings take no ETAs.
func TestAgencyETAStateRules(t *testing.T) {
	setCoverClock(t, "08:00")
	f := newCoverFixture(t)
	agencyID, key := createAgency(t, f.manager)
	teacher := createTeacher(t, f.manager, agencyID, "Tom", true)
	requested := createBooking(t, f.manager, agencyID, coverDate, "")
	tomorrow := createBooking(t, f.manager, agencyID, "2026-11-24", teacher)
	for _, id := range []string{requested, tomorrow} {
		r := agencyCall(t, "POST", "/api/v1/agency/bookings/"+id+"/eta", map[string]string{"eta": "08:10"}, key)
		expect(t, r, 409, "conflict")
	}
}

func TestCoverSignInFlow(t *testing.T) {
	setCoverClock(t, "08:10")
	f := newCoverFixture(t)
	noDBS, key := confirmedBooking(t, f.manager, false)
	signIn := func(id string, body map[string]interface{}) resp {
		t.Helper()
		return call(t, "POST", "/api/v1/cover/bookings/"+id+"/sign-in", body, f.reception)
	}
	details := func(r resp) map[string]interface{} {
		d, _ := r.Body["error"].(map[string]interface{})["details"].(map[string]interface{})
		return d
	}

	// The agency has not confirmed a DBS check: reception must see the certificate.
	row := findByBooking(t, call(t, "GET", "/api/v1/cover/today", nil, f.reception), "arrivals", noDBS)
	if row["dbs_certificate_required"] != true {
		t.Errorf("dbs flag = %v", row)
	}
	r := signIn(noDBS, map[string]interface{}{"photo_id_checked": true})
	expect(t, r, 422, "validation_error")
	if details(r)["dbs_certificate_seen"] == nil {
		t.Errorf("missing dbs detail: %s", r.Raw)
	}
	r = signIn(noDBS, map[string]interface{}{"photo_id_checked": false, "dbs_certificate_seen": true})
	expect(t, r, 422, "validation_error")
	if details(r)["photo_id_checked"] == nil {
		t.Errorf("missing photo detail: %s", r.Raw)
	}
	in := signIn(noDBS, map[string]interface{}{"photo_id_checked": true, "dbs_certificate_seen": true})
	expect(t, in, 201, "")
	if in.str("dbs_evidence") != "certificate_seen" || in.str("lanyard") != "S-01" || in.str("at") != coverDate+"T08:10:00Z" || in.str("type") != "signin" {
		t.Fatalf("sign-in = %s", in.Raw)
	}
	signInID := in.str("id")
	expect(t, signIn(noDBS, map[string]interface{}{"photo_id_checked": true, "dbs_certificate_seen": true}), 409, "conflict")
	expect(t, agencyCall(t, "POST", "/api/v1/agency/bookings/"+noDBS+"/eta", map[string]string{"eta": "08:20"}, key), 409, "conflict")
	if row := findByBooking(t, call(t, "GET", "/api/v1/cover/today", nil, f.reception), "arrivals", noDBS); row["status"] != "signed_in" {
		t.Errorf("status after sign-in = %v", row)
	}
	// A signed-in booking cannot be cancelled before the sign-in is corrected.
	expect(t, call(t, "PATCH", "/api/v1/cover/bookings/"+noDBS, map[string]string{"status": "cancelled"}, f.manager), 409, "conflict")

	// Correction: fixed reasons, only the current sign-in, appended (never removing the original).
	correct := func(id, reason string) resp {
		t.Helper()
		return call(t, "POST", "/api/v1/cover/sign-ins/"+id+"/corrections", map[string]string{"reason": reason}, f.reception)
	}
	expect(t, correct(signInID, "felt like it"), 422, "validation_error")
	expect(t, correct("missing", "wrong_person"), 404, "not_found")
	coverClock.Advance(time.Minute)
	c := correct(signInID, "wrong_person")
	expect(t, c, 201, "")
	if c.str("ref_id") != signInID || c.str("type") != "correction" || c.str("reason") != "wrong_person" {
		t.Fatalf("correction = %s", c.Raw)
	}
	expect(t, correct(signInID, "wrong_person"), 409, "conflict")
	expect(t, correct(c.str("id"), "wrong_person"), 404, "not_found") // a correction is not a sign-in

	coverClock.Advance(time.Minute)
	again := signIn(noDBS, map[string]interface{}{"photo_id_checked": true, "dbs_certificate_seen": true})
	expect(t, again, 201, "")
	expect(t, correct(signInID, "not_on_site"), 409, "conflict") // superseded

	// The log keeps every entry, newest first, for both roles.
	for _, tok := range []string{f.reception, f.manager} {
		log := call(t, "GET", "/api/v1/cover/sign-ins?date="+coverDate, nil, tok)
		expect(t, log, 200, "")
		var mine []string
		for _, it := range log.Body["items"].([]interface{}) {
			m := it.(map[string]interface{})
			if m["booking_id"] == noDBS {
				mine = append(mine, m["id"].(string))
			}
		}
		if want := []string{again.str("id"), c.str("id"), signInID}; strings.Join(mine, ",") != strings.Join(want, ",") {
			t.Errorf("log = %v, want %v", mine, want)
		}
	}
	al := call(t, "GET", "/api/v1/admin/audit-logs?per_page=100", nil, f.admin)
	for _, action := range []string{"cover.agency.create", "cover.booking.create", "cover.booking.confirm", "cover.signin", "cover.signin.correction"} {
		if !strings.Contains(string(al.Raw), `"`+action+`"`) {
			t.Errorf("audit log misses %s", action)
		}
	}
}

func TestCoverSignInRules(t *testing.T) {
	setCoverClock(t, "08:10")
	f := newCoverFixture(t)
	agencyID, _ := createAgency(t, f.manager)
	teacher := createTeacher(t, f.manager, agencyID, "Aisha", true)
	ok := map[string]interface{}{"photo_id_checked": true}
	requested := createBooking(t, f.manager, agencyID, coverDate, "")
	tomorrow := createBooking(t, f.manager, agencyID, "2026-11-24", teacher)
	expect(t, call(t, "POST", "/api/v1/cover/bookings/"+requested+"/sign-in", ok, f.reception), 409, "conflict")
	expect(t, call(t, "POST", "/api/v1/cover/bookings/"+tomorrow+"/sign-in", ok, f.reception), 409, "conflict")
	expect(t, call(t, "POST", "/api/v1/cover/bookings/missing/sign-in", ok, f.reception), 404, "not_found")
	expect(t, call(t, "POST", "/api/v1/cover/bookings/"+requested+"/sign-in", ok, f.user), 403, "forbidden")

	// An agency-confirmed DBS check needs no certificate.
	confirmed := createBooking(t, f.manager, agencyID, coverDate, teacher)
	in := call(t, "POST", "/api/v1/cover/bookings/"+confirmed+"/sign-in", ok, f.reception)
	expect(t, in, 201, "")
	if in.str("dbs_evidence") != "agency_confirmation" {
		t.Errorf("evidence = %s", in.Raw)
	}
}

func TestCoverConcurrentSignInsOnlyOneWins(t *testing.T) {
	setCoverClock(t, "08:10")
	f := newCoverFixture(t)
	bookingID, _ := confirmedBooking(t, f.manager, true)
	const n = 8
	var wg sync.WaitGroup
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i] = call(t, "POST", "/api/v1/cover/bookings/"+bookingID+"/sign-in",
				map[string]bool{"photo_id_checked": true}, f.reception).Status
		}(i)
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		switch s {
		case 201:
			created++
		case 409:
		default:
			t.Errorf("unexpected status %d", s)
		}
	}
	if created != 1 {
		t.Fatalf("%d concurrent sign-ins succeeded: %v", created, statuses)
	}
	log := call(t, "GET", "/api/v1/cover/sign-ins", nil, f.reception)
	count := 0
	for _, it := range log.Body["items"].([]interface{}) {
		if it.(map[string]interface{})["booking_id"] == bookingID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("log has %d entries for the booking", count)
	}
}

func TestAdminCanAssignCoverRoles(t *testing.T) {
	admin := adminToken(t)
	email, _, _ := newVerifiedUser(t, "roles")
	list := call(t, "GET", "/api/v1/admin/users?q="+email, nil, admin)
	id := list.Body["items"].([]interface{})[0].(map[string]interface{})["id"].(string)
	for _, role := range []string{"reception", "cover_manager", "user"} {
		r := call(t, "PATCH", "/api/v1/admin/users/"+id, map[string]string{"role": role}, admin)
		expect(t, r, 200, "")
		if r.str("role") != role {
			t.Fatalf("role = %s", r.Raw)
		}
	}
	bad := call(t, "PATCH", "/api/v1/admin/users/"+id, map[string]string{"role": "superuser"}, admin)
	expect(t, bad, 422, "validation_error")
	var body struct {
		Error struct{ Details map[string]string } `json:"error"`
	}
	_ = json.Unmarshal(bad.Raw, &body)
	if !strings.Contains(body.Error.Details["role"], "cover_manager") {
		t.Errorf("role detail = %v", body.Error.Details)
	}
}
