package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeboehm/invito/internal/booking"
	"github.com/jeboehm/invito/internal/config"
	"github.com/jeboehm/invito/internal/db"
	"github.com/jeboehm/invito/internal/email"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func newTestWidgetHandler(t *testing.T, database *sql.DB) *WidgetHandler {
	t.Helper()
	cfg := &config.Config{
		BaseURL:    "http://localhost:8080",
		BookingTTL: 24 * time.Hour,
	}
	mailer := email.NewMailer(cfg)
	bookingSvc := booking.NewService(database, mailer, cfg.BaseURL, cfg.SessionSecret)
	pubH := NewPublicHandler(cfg, database, bookingSvc)
	return NewWidgetHandler(pubH)
}

func TestWidgetSlotPicker_UnknownUser(t *testing.T) {
	database := openTestDB(t)
	h := newTestWidgetHandler(t, database)

	req := httptest.NewRequest(http.MethodGet, "/widget/nobody/30min", nil)
	req.SetPathValue("username", "nobody")
	req.SetPathValue("slug", "30min")
	rec := httptest.NewRecorder()

	h.HandleWidgetSlotPicker(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user: got %d, want 404", rec.Code)
	}
}

func TestWidgetSlotPicker_InactiveEventType(t *testing.T) {
	database := openTestDB(t)
	h := newTestWidgetHandler(t, database)

	user, err := db.UpsertUser(database, "sub1", "host@example.com", "Host", "host")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	etID, err := db.CreateEventType(database, &db.EventType{
		UserID:            user.ID,
		Slug:              "inactive",
		Title:             "Inactive",
		DurationMinutes:   30,
		BookingWindowDays: 14,
	})
	if err != nil {
		t.Fatalf("create event type: %v", err)
	}
	// CreateEventType always starts active; toggle it off.
	if err := db.ToggleEventType(database, etID, user.ID); err != nil {
		t.Fatalf("toggle event type: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/widget/host/inactive", nil)
	req.SetPathValue("username", "host")
	req.SetPathValue("slug", "inactive")
	rec := httptest.NewRecorder()

	h.HandleWidgetSlotPicker(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("inactive event type: got %d, want 404", rec.Code)
	}
}

func TestWidgetSlotPicker_ReturnsHTMLWithoutSiteHeader(t *testing.T) {
	database := openTestDB(t)
	h := newTestWidgetHandler(t, database)

	user, err := db.UpsertUser(database, "sub2", "host2@example.com", "Host Two", "host2")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	_, err = db.CreateEventType(database, &db.EventType{
		UserID:            user.ID,
		Slug:              "30min",
		Title:             "30 Minute Meeting",
		DurationMinutes:   30,
		BookingWindowDays: 14,
		Active:            true,
	})
	if err != nil {
		t.Fatalf("create event type: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/widget/host2/30min", nil)
	req.SetPathValue("username", "host2")
	req.SetPathValue("slug", "30min")
	rec := httptest.NewRecorder()

	h.HandleWidgetSlotPicker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("valid request: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "site-header") {
		t.Error("widget page must not contain site-header navigation")
	}
}

func TestWidgetSlotPicker_HTMXReturnsPartial(t *testing.T) {
	database := openTestDB(t)
	h := newTestWidgetHandler(t, database)

	user, err := db.UpsertUser(database, "sub3", "host3@example.com", "Host Three", "host3")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	_, err = db.CreateEventType(database, &db.EventType{
		UserID:            user.ID,
		Slug:              "30min",
		Title:             "30 Minute Meeting",
		DurationMinutes:   30,
		BookingWindowDays: 14,
		Active:            true,
	})
	if err != nil {
		t.Fatalf("create event type: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/widget/host3/30min", nil)
	req.SetPathValue("username", "host3")
	req.SetPathValue("slug", "30min")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	h.HandleWidgetSlotPicker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTMX request: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// Partial must not contain the full HTML document structure
	if strings.Contains(body, "<!DOCTYPE") {
		t.Error("HTMX response must be a partial, not a full HTML document")
	}
}

func TestWidgetBookingSubmit_NoCsrfRequired(t *testing.T) {
	database := openTestDB(t)
	h := newTestWidgetHandler(t, database)

	user, err := db.UpsertUser(database, "sub4", "host4@example.com", "Host Four", "host4")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	_, err = db.CreateEventType(database, &db.EventType{
		UserID:            user.ID,
		Slug:              "30min",
		Title:             "30 Minute Meeting",
		DurationMinutes:   30,
		BookingWindowDays: 14,
		Active:            true,
	})
	if err != nil {
		t.Fatalf("create event type: %v", err)
	}

	// POST without any CSRF token — must not return 403
	form := "slot=invalid-slot"
	req := httptest.NewRequest(http.MethodPost, "/widget/host4/30min/book",
		strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("username", "host4")
	req.SetPathValue("slug", "30min")
	rec := httptest.NewRecorder()

	h.HandleWidgetBookingSubmit(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatal("widget booking submit must not require CSRF token (got 403)")
	}
	// Invalid slot → 400 is expected, just not 403
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid slot: got %d, want 400", rec.Code)
	}
}
