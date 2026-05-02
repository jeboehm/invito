package booking_test

import (
	"context"
	"database/sql"
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

// newTestService creates a Service with a mailer that will fail to send (no SMTP).
// Email failures are logged but not returned by the service methods under test.
func newTestService(t *testing.T, database *sql.DB) *booking.Service {
	t.Helper()
	cfg := &config.Config{
		SMTPHost: "localhost",
		SMTPPort: 0,
		SMTPFrom: "test@example.com",
	}
	mailer := email.NewMailer(cfg)
	return booking.NewService(database, mailer, "http://localhost:8080", [32]byte{})
}

type testFixture struct {
	database  *sql.DB
	service   *booking.Service
	user      *db.User
	eventType *db.EventType
}

func setupFixture(t *testing.T) testFixture {
	t.Helper()
	database := openTestDB(t)
	svc := newTestService(t, database)

	user, err := db.UpsertUser(database, "sub1", "host@example.com", "Host User", "host")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	etID, err := db.CreateEventType(database, &db.EventType{
		UserID:            user.ID,
		Slug:              "30min",
		Title:             "30 Minute Meeting",
		DurationMinutes:   30,
		BookingWindowDays: 14,
	})
	if err != nil {
		t.Fatalf("create event type: %v", err)
	}

	et := &db.EventType{ID: etID, UserID: user.ID, Title: "30 Minute Meeting"}

	return testFixture{database: database, service: svc, user: user, eventType: et}
}

func newPendingBooking(t *testing.T, f testFixture) *db.Booking {
	t.Helper()
	start := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	b := &db.Booking{
		EventTypeID:   f.eventType.ID,
		GuestName:     "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestNote:     "test note",
		StartAt:       start,
		EndAt:         start.Add(30 * time.Minute),
		Token:         "test-token-" + t.Name(),
		ReservedUntil: start.Add(24 * time.Hour),
	}
	if err := db.CreateBooking(f.database, b); err != nil {
		t.Fatalf("create booking: %v", err)
	}
	return b
}

func TestCreateBooking(t *testing.T) {
	f := setupFixture(t)
	start := time.Now().Add(48 * time.Hour).Truncate(time.Minute)
	b := &db.Booking{
		EventTypeID:   f.eventType.ID,
		GuestName:     "Alice",
		GuestEmail:    "alice@example.com",
		StartAt:       start,
		EndAt:         start.Add(30 * time.Minute),
		Token:         "create-test-token",
		ReservedUntil: start.Add(24 * time.Hour),
	}
	err := f.service.CreateBooking(context.Background(), b, f.eventType, f.user)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	got, err := db.GetBookingByToken(f.database, "create-test-token")
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if got.GuestName != "Alice" {
		t.Errorf("GuestName: got %q, want Alice", got.GuestName)
	}
	if got.Status != "PENDING" {
		t.Errorf("Status: got %q, want PENDING", got.Status)
	}
}

func TestConfirmBooking_PENDING(t *testing.T) {
	f := setupFixture(t)
	b := newPendingBooking(t, f)

	got, err := f.service.ConfirmBooking(context.Background(), b.Token)
	if err != nil {
		t.Fatalf("ConfirmBooking: %v", err)
	}
	if got.Status != "CONFIRMED" {
		t.Errorf("returned status: got %q, want CONFIRMED", got.Status)
	}

	fromDB, err := db.GetBookingByToken(f.database, b.Token)
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if fromDB.Status != "CONFIRMED" {
		t.Errorf("db status: got %q, want CONFIRMED", fromDB.Status)
	}
}

func TestConfirmBooking_Idempotent(t *testing.T) {
	f := setupFixture(t)
	b := newPendingBooking(t, f)

	if _, err := f.service.ConfirmBooking(context.Background(), b.Token); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	got, err := f.service.ConfirmBooking(context.Background(), b.Token)
	if err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if got.Status != "CONFIRMED" {
		t.Errorf("status after second confirm: got %q, want CONFIRMED", got.Status)
	}
}

func TestRejectBooking_PENDING(t *testing.T) {
	f := setupFixture(t)
	b := newPendingBooking(t, f)

	got, err := f.service.RejectBooking(context.Background(), b.Token)
	if err != nil {
		t.Fatalf("RejectBooking: %v", err)
	}
	if got.Status != "REJECTED" {
		t.Errorf("returned status: got %q, want REJECTED", got.Status)
	}

	fromDB, err := db.GetBookingByToken(f.database, b.Token)
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if fromDB.Status != "REJECTED" {
		t.Errorf("db status: got %q, want REJECTED", fromDB.Status)
	}
}

func TestRejectBooking_Idempotent(t *testing.T) {
	f := setupFixture(t)
	b := newPendingBooking(t, f)

	if _, err := f.service.RejectBooking(context.Background(), b.Token); err != nil {
		t.Fatalf("first reject: %v", err)
	}
	got, err := f.service.RejectBooking(context.Background(), b.Token)
	if err != nil {
		t.Fatalf("second reject: %v", err)
	}
	if got.Status != "REJECTED" {
		t.Errorf("status after second reject: got %q, want REJECTED", got.Status)
	}
}

func TestConfirmBooking_UnknownToken(t *testing.T) {
	f := setupFixture(t)
	_, err := f.service.ConfirmBooking(context.Background(), "no-such-token")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestRejectBooking_UnknownToken(t *testing.T) {
	f := setupFixture(t)
	_, err := f.service.RejectBooking(context.Background(), "no-such-token")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}
