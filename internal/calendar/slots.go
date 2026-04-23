package calendar

import (
	"sort"
	"time"

	"github.com/jboehm/invito/internal/db"
)

type Slot struct {
	Start time.Time
	End   time.Time
}

// CalculateSlots returns available booking slots for the given date, duration, and location.
// It filters out slots that conflict with existing calendar events or bookings.
func CalculateSlots(
	rules []db.AvailabilityRule,
	events []db.CalendarEvent,
	bookings []db.Booking,
	date time.Time,
	duration time.Duration,
	loc *time.Location,
	bookingWindowDays int,
) []Slot {
	now := time.Now()
	windowEnd := now.Add(time.Duration(bookingWindowDays) * 24 * time.Hour)

	// Find rules matching the weekday of date
	weekday := int(date.Weekday())
	var matchingRules []db.AvailabilityRule
	for _, r := range rules {
		if r.Weekday == weekday && r.Active {
			matchingRules = append(matchingRules, r)
		}
	}

	year, month, day := date.In(loc).Date()

	var candidates []Slot
	for _, rule := range matchingRules {
		startH, startM := parseHHMM(rule.StartTime)
		endH, endM := parseHHMM(rule.EndTime)

		ruleStart := time.Date(year, month, day, startH, startM, 0, 0, loc)
		ruleEnd := time.Date(year, month, day, endH, endM, 0, 0, loc)

		for slotStart := ruleStart; ; slotStart = slotStart.Add(duration) {
			slotEnd := slotStart.Add(duration)
			if slotEnd.After(ruleEnd) {
				break
			}
			candidates = append(candidates, Slot{Start: slotStart, End: slotEnd})
		}
	}

	// Filter slots
	var available []Slot
	for _, slot := range candidates {
		if !slot.Start.After(now) {
			continue
		}
		if slot.Start.After(windowEnd) {
			continue
		}
		if overlapsAnyEvent(slot, events) {
			continue
		}
		if overlapsAnyBooking(slot, bookings) {
			continue
		}
		available = append(available, slot)
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].Start.Before(available[j].Start)
	})

	return available
}

func overlapsAnyEvent(slot Slot, events []db.CalendarEvent) bool {
	for _, e := range events {
		if slot.Start.Before(e.EndAt) && slot.End.After(e.StartAt) {
			return true
		}
	}
	return false
}

func overlapsAnyBooking(slot Slot, bookings []db.Booking) bool {
	for _, b := range bookings {
		if b.Status != "PENDING" && b.Status != "CONFIRMED" {
			continue
		}
		if slot.Start.Before(b.EndAt) && slot.End.After(b.StartAt) {
			return true
		}
	}
	return false
}

func parseHHMM(s string) (int, int) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h, m
}
