package calendar

import (
	"testing"
	"time"

	"github.com/jboehm/invito/internal/db"
)

func TestCalculateSlots_Basic(t *testing.T) {
	loc := time.UTC
	// Monday 2026-04-27
	date := time.Date(2026, 4, 27, 0, 0, 0, 0, loc)

	rules := []db.AvailabilityRule{
		{Weekday: 1, StartTime: "09:00", EndTime: "11:00", Active: true}, // Monday
	}

	slots := CalculateSlots(rules, nil, nil, date, 30*time.Minute, loc, 60)

	// Expect 09:00, 09:30, 10:00, 10:30 = 4 slots
	if len(slots) != 4 {
		t.Fatalf("expected 4 slots, got %d: %v", len(slots), slots)
	}
	if slots[0].Start.Hour() != 9 || slots[0].Start.Minute() != 0 {
		t.Errorf("first slot should be 09:00, got %v", slots[0].Start)
	}
}

func TestCalculateSlots_EventBlocking(t *testing.T) {
	loc := time.UTC
	date := time.Date(2026, 4, 27, 0, 0, 0, 0, loc)

	rules := []db.AvailabilityRule{
		{Weekday: 1, StartTime: "09:00", EndTime: "11:00", Active: true},
	}
	events := []db.CalendarEvent{
		{
			StartAt: time.Date(2026, 4, 27, 9, 30, 0, 0, loc),
			EndAt:   time.Date(2026, 4, 27, 10, 0, 0, 0, loc),
		},
	}

	slots := CalculateSlots(rules, events, nil, date, 30*time.Minute, loc, 60)

	// 09:30 slot is blocked → 3 slots remain
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}
	for _, s := range slots {
		if s.Start.Hour() == 9 && s.Start.Minute() == 30 {
			t.Error("09:30 slot should have been filtered")
		}
	}
}

// TestCalculateSlots_EventBlockingCrossTimezone verifies that CalDAV events stored as
// UTC correctly block slots that are calculated in a local (non-UTC) timezone.
// This is the runtime half of the floating-time bug: even after the event is stored
// correctly as UTC, the overlap check must compare absolute instants, not wall-clock values.
func TestCalculateSlots_EventBlockingCrossTimezone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("Europe/Berlin timezone not available:", err)
	}

	// Tuesday 2026-04-28 in Berlin (CEST = UTC+2)
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, berlin)

	rules := []db.AvailabilityRule{
		{Weekday: 2, StartTime: "09:00", EndTime: "11:00", Active: true}, // Tuesday
	}

	// CalDAV event at 09:00–10:00 Berlin time, stored in DB as UTC (07:00–08:00 UTC).
	events := []db.CalendarEvent{
		{
			StartAt: time.Date(2026, 4, 28, 7, 0, 0, 0, time.UTC), // 09:00 CEST
			EndAt:   time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC), // 10:00 CEST
		},
	}

	slots := CalculateSlots(rules, events, nil, date, 30*time.Minute, berlin, 60)

	// Slots in window: 09:00, 09:30, 10:00, 10:30 CEST.
	// 09:00–09:30 and 09:30–10:00 CEST overlap the event → blocked.
	// 10:00–10:30 and 10:30–11:00 CEST are free → 2 slots expected.
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d: %v", len(slots), slots)
	}
	for _, s := range slots {
		local := s.Start.In(berlin)
		if local.Hour() < 10 {
			t.Errorf("slot at %v should have been blocked by CalDAV event", local)
		}
	}
}

func TestCalculateSlots_WrongWeekday(t *testing.T) {
	loc := time.UTC
	// Tuesday 2026-04-28
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, loc)

	rules := []db.AvailabilityRule{
		{Weekday: 1, StartTime: "09:00", EndTime: "17:00", Active: true}, // Monday only
	}

	slots := CalculateSlots(rules, nil, nil, date, 30*time.Minute, loc, 60)
	if len(slots) != 0 {
		t.Fatalf("expected 0 slots for wrong weekday, got %d", len(slots))
	}
}
