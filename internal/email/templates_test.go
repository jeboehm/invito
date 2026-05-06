package email_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jeboehm/invito/internal/email"
)

var testData = email.BookingEmailData{
	GuestName:  "Jane Doe",
	GuestEmail: "jane@example.com",
	GuestNote:  "Looking forward to it",
	HostName:   "John Host",
	EventTitle: "30 Minute Call",
	StartAt:    time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC),
	ConfirmURL: "https://example.com/booking/abc/confirm",
	RejectURL:  "https://example.com/booking/abc/reject",
}

func assertRender(t *testing.T, label, text, html string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
	if text == "" {
		t.Errorf("%s: empty text output", label)
	}
	if html == "" {
		t.Errorf("%s: empty html output", label)
	}
	if !strings.Contains(text, testData.EventTitle) {
		t.Errorf("%s: text missing EventTitle", label)
	}
	if !strings.Contains(html, testData.EventTitle) {
		t.Errorf("%s: html missing EventTitle", label)
	}
}

func TestRenderBookingCreated(t *testing.T) {
	text, html, err := email.RenderBookingCreated(testData)
	assertRender(t, "RenderBookingCreated", text, html, err)
	if !strings.Contains(text, testData.GuestName) {
		t.Errorf("text missing GuestName")
	}
	if !strings.Contains(html, testData.GuestName) {
		t.Errorf("html missing GuestName")
	}
}

func TestRenderBookingConfirmed(t *testing.T) {
	text, html, err := email.RenderBookingConfirmed(testData)
	assertRender(t, "RenderBookingConfirmed", text, html, err)
}

func TestRenderBookingRejected(t *testing.T) {
	text, html, err := email.RenderBookingRejected(testData)
	assertRender(t, "RenderBookingRejected", text, html, err)
}

func TestRenderBookingCancelled(t *testing.T) {
	text, html, err := email.RenderBookingCancelled(testData)
	assertRender(t, "RenderBookingCancelled", text, html, err)
}

func TestRenderBookingCreated_TimezoneApplied(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	data := testData
	data.HostLocation = loc
	// StartAt is 14:00 UTC → 10:00 EDT (UTC-4)
	text, _, err := email.RenderBookingCreated(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(text, "10:00") {
		t.Errorf("expected local time 10:00 in output, got: %s", text)
	}
	if strings.Contains(text, "14:00") {
		t.Errorf("output should not contain UTC time 14:00 when timezone is set")
	}
}
