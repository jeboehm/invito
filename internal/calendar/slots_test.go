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
