package calendar

import (
	"testing"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/jboehm/invito/internal/db"
)

// veventComp builds a minimal VEVENT component with a floating (no Z, no TZID) DTSTART/DTEND.
func veventComp(uid, start, end string) *ical.Component {
	comp := ical.NewComponent(ical.CompEvent)
	comp.Props.SetText(ical.PropUID, uid)
	comp.Props.Set(&ical.Prop{Name: ical.PropDateTimeStart, Value: start})
	comp.Props.Set(&ical.Prop{Name: ical.PropDateTimeEnd, Value: end})
	return comp
}

// TestUpsertVEVENT_FloatingTimeUsesProvidedLocation verifies that a VEVENT with a
// floating DTSTART (no Z suffix, no TZID) is stored using the supplied location, not UTC.
// This catches the bug where a CalDAV appointment at 09:00 local time was stored as
// 09:00 UTC and therefore missed the overlap check for slots calculated in local time.
func TestUpsertVEVENT_FloatingTimeUsesProvidedLocation(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	user, err := db.UpsertUser(database, "sub1", "u@example.com", "U", "u")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	calID, err := db.CreateCalendar(database, &db.Calendar{
		UserID: user.ID, CalDAVURL: "http://x", Username: "u", Password: "p", DisplayName: "C",
	})
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

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
	events, err := db.ListCalendarEventsForUser(database, user.ID, from, to)
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
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	user, err := db.UpsertUser(database, "sub3", "w@example.com", "W", "w")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	calID, err := db.CreateCalendar(database, &db.Calendar{
		UserID: user.ID, CalDAVURL: "http://x", Username: "u", Password: "p", DisplayName: "C",
	})
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

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
	events, err := db.ListCalendarEventsForUser(database, user.ID, from, to)
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
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	user, err := db.UpsertUser(database, "sub2", "v@example.com", "V", "v")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	calID, err := db.CreateCalendar(database, &db.Calendar{
		UserID: user.ID, CalDAVURL: "http://x", Username: "u", Password: "p", DisplayName: "C",
	})
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

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
	events, err := db.ListCalendarEventsForUser(database, user.ID, from, to)
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
