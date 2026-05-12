package calendar

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/jeboehm/invito/internal/db"
)

// veventComp builds a minimal VEVENT component with a floating (no Z, no TZID) DTSTART/DTEND.
func veventComp(uid, start, end string) *ical.Component {
	comp := ical.NewComponent(ical.CompEvent)
	comp.Props.SetText(ical.PropUID, uid)
	comp.Props.Set(&ical.Prop{Name: ical.PropDateTimeStart, Value: start})
	comp.Props.Set(&ical.Prop{Name: ical.PropDateTimeEnd, Value: end})
	return comp
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func setupTestCalendar(t *testing.T, database *sql.DB, sub, email string) (userID, calID int64) {
	t.Helper()
	user, err := db.UpsertUser(database, sub, email, "U", "u")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	calID, err = db.CreateCalendar(database, &db.Calendar{
		UserID: user.ID, CalDAVURL: "http://x", Username: "u", Password: "p", DisplayName: "C",
	})
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	return user.ID, calID
}

// TestUpsertVEVENT_FloatingTimeUsesProvidedLocation verifies that a VEVENT with a
// floating DTSTART (no Z suffix, no TZID) is stored using the supplied location, not UTC.
// This catches the bug where a CalDAV appointment at 09:00 local time was stored as
// 09:00 UTC and therefore missed the overlap check for slots calculated in local time.
func TestUpsertVEVENT_FloatingTimeUsesProvidedLocation(t *testing.T) {
	database := openTestDB(t)
	userID, calID := setupTestCalendar(t, database, "sub1", "u@example.com")

	// Use a fixed UTC+2 location to make the test deterministic regardless of the
	// machine's local timezone.
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("Europe/Berlin timezone not available:", err)
	}

	// Floating DTSTART at 09:00 — must be interpreted as 09:00 Berlin (= 07:00 UTC in CEST).
	comp := veventComp("uid-float-1", "20260428T090000", "20260428T100000")
	if err := upsertVEVENT(database, calID, comp, berlin); err != nil {
		t.Fatalf("upsertVEVENT: %v", err)
	}

	from := time.Date(2026, 4, 28, 0, 0, 0, 0, berlin)
	to := from.Add(24 * time.Hour)
	events, err := db.ListCalendarEventsForUser(database, userID, from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// 09:00 Berlin (CEST = UTC+2) → 07:00 UTC
	gotUTC := events[0].StartAt.UTC()
	wantUTC := time.Date(2026, 4, 28, 7, 0, 0, 0, time.UTC)
	if !gotUTC.Equal(wantUTC) {
		t.Errorf("startAt UTC: got %v, want %v (floating time was not interpreted as Berlin local)",
			gotUTC, wantUTC)
	}
}

// TestUpsertVEVENT_UnknownTZIDFallsBackToLocal verifies that an event whose TZID is not
// in the system timezone database (e.g. Windows-style "W. Europe Standard Time") is
// stored using the supplied fallback location rather than being silently dropped.
func TestUpsertVEVENT_UnknownTZIDFallsBackToLocal(t *testing.T) {
	database := openTestDB(t)
	userID, calID := setupTestCalendar(t, database, "sub3", "w@example.com")

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("Europe/Berlin timezone not available:", err)
	}

	// DTSTART with a Windows TZID that Go's timezone DB doesn't know.
	// The fallback must treat the value as Berlin local time.
	comp := ical.NewComponent(ical.CompEvent)
	comp.Props.SetText(ical.PropUID, "uid-win-tz-1")
	p := &ical.Prop{Name: ical.PropDateTimeStart, Value: "20260428T090000", Params: ical.Params{}}
	p.Params.Set(ical.PropTimezoneID, "W. Europe Standard Time") // unknown to time.LoadLocation
	comp.Props.Set(p)
	endP := &ical.Prop{Name: ical.PropDateTimeEnd, Value: "20260428T100000", Params: ical.Params{}}
	endP.Params.Set(ical.PropTimezoneID, "W. Europe Standard Time")
	comp.Props.Set(endP)

	if err := upsertVEVENT(database, calID, comp, berlin); err != nil {
		t.Fatalf("upsertVEVENT: %v", err)
	}

	from := time.Date(2026, 4, 28, 0, 0, 0, 0, berlin)
	to := from.Add(24 * time.Hour)
	events, err := db.ListCalendarEventsForUser(database, userID, from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (unknown TZID must not silently drop event), got %d", len(events))
	}

	// With the Berlin fallback, 09:00 → 07:00 UTC (CEST).
	gotUTC := events[0].StartAt.UTC()
	wantUTC := time.Date(2026, 4, 28, 7, 0, 0, 0, time.UTC)
	if !gotUTC.Equal(wantUTC) {
		t.Errorf("startAt UTC: got %v, want %v", gotUTC, wantUTC)
	}
}

// TestUpsertVEVENT_ExplicitUTCSuffix verifies that a DTSTART ending in Z is always
// stored as UTC regardless of the supplied default location.
func TestUpsertVEVENT_ExplicitUTCSuffix(t *testing.T) {
	database := openTestDB(t)
	userID, calID := setupTestCalendar(t, database, "sub2", "v@example.com")

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("Europe/Berlin timezone not available:", err)
	}

	// Explicit UTC suffix — must remain 09:00 UTC even when berlin is passed as default.
	comp := veventComp("uid-utc-1", "20260428T090000Z", "20260428T100000Z")
	if err := upsertVEVENT(database, calID, comp, berlin); err != nil {
		t.Fatalf("upsertVEVENT: %v", err)
	}

	from := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	events, err := db.ListCalendarEventsForUser(database, userID, from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	gotUTC := events[0].StartAt.UTC()
	wantUTC := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	if !gotUTC.Equal(wantUTC) {
		t.Errorf("startAt UTC: got %v, want %v", gotUTC, wantUTC)
	}
}

// TestUpsertVEVENT_RecurringEventStoresAllInstances verifies that multiple occurrences
// of a recurring event (same UID, different RECURRENCE-ID) are each stored as a
// separate row rather than overwriting each other.
func TestUpsertVEVENT_RecurringEventStoresAllInstances(t *testing.T) {
	database := openTestDB(t)
	userID, calID := setupTestCalendar(t, database, "sub-recur", "r@example.com")

	sharedUID := "recurring-uid-1"
	makeInstance := func(start, end, recurrenceID string) *ical.Component {
		comp := ical.NewComponent(ical.CompEvent)
		comp.Props.SetText(ical.PropUID, sharedUID)
		comp.Props.Set(&ical.Prop{Name: ical.PropDateTimeStart, Value: start})
		comp.Props.Set(&ical.Prop{Name: ical.PropDateTimeEnd, Value: end})
		comp.Props.Set(&ical.Prop{Name: ical.PropRecurrenceID, Value: recurrenceID})
		return comp
	}

	instance1 := makeInstance("20260601T090000Z", "20260601T100000Z", "20260601T090000Z")
	instance2 := makeInstance("20260608T090000Z", "20260608T100000Z", "20260608T090000Z")

	if err := upsertVEVENT(database, calID, instance1, time.UTC); err != nil {
		t.Fatalf("upsertVEVENT instance1: %v", err)
	}
	if err := upsertVEVENT(database, calID, instance2, time.UTC); err != nil {
		t.Fatalf("upsertVEVENT instance2: %v", err)
	}

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	events, err := db.ListCalendarEventsForUser(database, userID, from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (one per recurring instance), got %d", len(events))
	}
}

// TestUpsertVEVENT_RRULEExpandedLocally verifies that a master VEVENT with RRULE
// (no RECURRENCE-ID, i.e. server did not expand it) is expanded locally and all
// occurrences within the sync window are stored as separate rows.
func TestUpsertVEVENT_RRULEExpandedLocally(t *testing.T) {
	database := openTestDB(t)
	userID, calID := setupTestCalendar(t, database, "sub-rrule", "rrule@example.com")

	master := ical.NewComponent(ical.CompEvent)
	master.Props.SetText(ical.PropUID, "blocked-recurring-uid")
	master.Props.Set(&ical.Prop{Name: ical.PropDateTimeStart, Value: "20260513T090000Z"})
	master.Props.Set(&ical.Prop{Name: ical.PropDateTimeEnd, Value: "20260513T180000Z"})
	master.Props.Set(&ical.Prop{Name: ical.PropRecurrenceRule, Value: "FREQ=DAILY;COUNT=6"})

	from := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)

	if err := expandAndUpsertRRULE(database, calID, master, time.UTC, from, to); err != nil {
		t.Fatalf("expandAndUpsertRRULE: %v", err)
	}

	events, err := db.ListCalendarEventsForUser(database, userID, from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	// All 6 occurrences (May 13–18) fall in the window.
	if len(events) != 6 {
		t.Fatalf("expected 6 events (RRULE expanded), got %d", len(events))
	}
	// Each occurrence should be 9 hours long.
	for _, e := range events {
		if dur := e.EndAt.Sub(e.StartAt); dur != 9*time.Hour {
			t.Errorf("expected 9h duration, got %v for event starting %v", dur, e.StartAt)
		}
	}
}

// TestUpsertVEVENT_CommaExdateExpanded verifies that a master VEVENT whose EXDATE
// carries comma-separated values (valid per RFC 5545 §3.8.5.1) is still expanded
// correctly and the excluded dates are not stored.
func TestUpsertVEVENT_CommaExdateExpanded(t *testing.T) {
	database := openTestDB(t)
	userID, calID := setupTestCalendar(t, database, "sub-exdate", "exdate@example.com")

	// Weekly event Mon 09:00 UTC, 4 occurrences: Jun 1, 8, 15, 22.
	// EXDATE excludes Jun 8 and Jun 15 as a comma-separated value on one line.
	master := ical.NewComponent(ical.CompEvent)
	master.Props.SetText(ical.PropUID, "al-weekly-test-uid")
	master.Props.Set(&ical.Prop{Name: ical.PropDateTimeStart, Value: "20260601T090000Z"})
	master.Props.Set(&ical.Prop{Name: ical.PropDateTimeEnd, Value: "20260601T100000Z"})
	master.Props.Set(&ical.Prop{Name: ical.PropRecurrenceRule, Value: "FREQ=WEEKLY;COUNT=4"})
	// Comma-separated EXDATE — the form that breaks go-ical without our fix.
	master.Props.Set(&ical.Prop{Name: ical.PropExceptionDates, Value: "20260608T090000Z,20260615T090000Z"})

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	if err := expandAndUpsertRRULE(database, calID, master, time.UTC, from, to); err != nil {
		t.Fatalf("expandAndUpsertRRULE: %v", err)
	}

	events, err := db.ListCalendarEventsForUser(database, userID, from, to)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	// Only Jun 1 and Jun 22 survive; Jun 8 and Jun 15 are excluded.
	if len(events) != 2 {
		t.Fatalf("expected 2 events (excluded dates should be absent), got %d", len(events))
	}
	for _, e := range events {
		if e.StartAt.Day() == 8 || e.StartAt.Day() == 15 {
			t.Errorf("excluded date %v should not be stored", e.StartAt)
		}
	}
}

func TestBuildICAL_Description(t *testing.T) {
	booking := &db.Booking{
		Token:      "tok123",
		GuestName:  "Alice Smith",
		GuestEmail: "alice@example.com",
		GuestNote:  "Please bring coffee.",
		StartAt:    time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		EndAt:      time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC),
	}
	et := &db.EventType{
		Title:            "Intro Call",
		DurationMinutes:  60,
		ConfirmedMessage: "Looking forward to it!",
	}

	ics := buildICAL(booking, et, booking.GuestName)

	for _, want := range []string{
		"DESCRIPTION:",
		"Guest: Alice Smith",
		"Email: alice@example.com",
		"Please bring coffee.",
		"Looking forward to it!",
	} {
		if !strings.Contains(ics, want) {
			t.Errorf("iCal output missing %q\ngot:\n%s", want, ics)
		}
	}
}

func TestBuildICAL_DescriptionWithoutOptionalFields(t *testing.T) {
	booking := &db.Booking{
		Token:      "tok456",
		GuestName:  "Bob",
		GuestEmail: "bob@example.com",
		StartAt:    time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		EndAt:      time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	}
	et := &db.EventType{Title: "Chat", DurationMinutes: 60}

	ics := buildICAL(booking, et, booking.GuestName)

	if !strings.Contains(ics, "Guest: Bob") {
		t.Errorf("DESCRIPTION missing guest name\ngot:\n%s", ics)
	}
	if !strings.Contains(ics, "Email: bob@example.com") {
		t.Errorf("DESCRIPTION missing guest email\ngot:\n%s", ics)
	}
}

func TestICALEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"a,b", "a\\,b"},
		{"a;b", "a\\;b"},
		{"a\\b", "a\\\\b"},
		{"line1\nline2", "line1\\nline2"},
	}
	for _, c := range cases {
		if got := icalEscape(c.in); got != c.want {
			t.Errorf("icalEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
