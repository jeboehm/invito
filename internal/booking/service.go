package booking

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jboehm/invito/internal/calendar"
	"github.com/jboehm/invito/internal/db"
	"github.com/jboehm/invito/internal/email"
)

type Service struct {
	db      *sql.DB
	mailer  *email.Mailer
	baseURL string
	key     [32]byte
}

func NewService(database *sql.DB, mailer *email.Mailer, baseURL string, key [32]byte) *Service {
	return &Service{db: database, mailer: mailer, baseURL: baseURL, key: key}
}

func (s *Service) CreateBooking(ctx context.Context, b *db.Booking, eventType *db.EventType, user *db.User) error {
	if err := db.CreateBooking(s.db, b); err != nil {
		return err
	}

	text, html, err := email.RenderBookingCreated(email.BookingEmailData{
		GuestName:  b.GuestName,
		GuestEmail: b.GuestEmail,
		GuestNote:  b.GuestNote,
		HostName:   user.Name,
		EventTitle: eventType.Title,
		StartAt:    b.StartAt,
		ConfirmURL: fmt.Sprintf("%s/booking/%s/confirm", s.baseURL, b.Token),
		RejectURL:  fmt.Sprintf("%s/booking/%s/reject", s.baseURL, b.Token),
	})
	if err != nil {
		log.Printf("render booking-created email: %v", err)
		return nil
	}

	subject := fmt.Sprintf("New booking request: %s with %s", eventType.Title, b.GuestName)
	if err := s.mailer.Send(user.Email, subject, text, html); err != nil {
		log.Printf("send booking-created email: %v", err)
	}
	return nil
}

func (s *Service) ConfirmBooking(ctx context.Context, token string) (*db.Booking, error) {
	b, err := db.GetBookingByToken(s.db, token)
	if err != nil {
		return nil, err
	}
	if b.Status != "PENDING" {
		return b, nil // idempotent
	}

	if err := db.UpdateBookingStatus(s.db, b.ID, "CONFIRMED"); err != nil {
		return nil, err
	}
	b.Status = "CONFIRMED"

	// Fetch event type and user
	et, user, err := s.fetchEventTypeAndUser(b.EventTypeID)
	if err != nil {
		log.Printf("fetch event type for confirm: %v", err)
		return b, nil
	}

	// CalDAV write-back (non-fatal). Uses a detached context so the PUT request
	// is not cancelled when the HTTP response is written.
	go func() {
		cals, err := db.ListCalendars(s.db, user.ID)
		if err != nil || len(cals) == 0 {
			return
		}
		if err := calendar.WriteEvent(context.Background(), &cals[0], s.key, b, et, b.GuestName); err != nil {
			log.Printf("caldav write-back: %v", err)
		}
	}()

	// Send emails
	data := email.BookingEmailData{
		GuestName:    b.GuestName,
		GuestEmail:   b.GuestEmail,
		HostName:     user.Name,
		EventTitle:   et.Title,
		GuestMessage: et.GuestMessage,
		StartAt:      b.StartAt,
	}

	subject := fmt.Sprintf("Confirmed: %s on %s", et.Title, b.StartAt.Format("January 2, 2006"))
	if text, html, err := email.RenderBookingConfirmed(data); err == nil {
		_ = s.mailer.Send(b.GuestEmail, subject, text, html)
		_ = s.mailer.Send(user.Email, subject, text, html)
	}

	return b, nil
}

func (s *Service) RejectBooking(ctx context.Context, token string) (*db.Booking, error) {
	b, err := db.GetBookingByToken(s.db, token)
	if err != nil {
		return nil, err
	}
	if b.Status != "PENDING" {
		return b, nil
	}

	if err := db.UpdateBookingStatus(s.db, b.ID, "REJECTED"); err != nil {
		return nil, err
	}
	b.Status = "REJECTED"

	et, _, err := s.fetchEventTypeAndUser(b.EventTypeID)
	if err != nil {
		return b, nil
	}

	subject := fmt.Sprintf("Your booking request for %s was not accepted", et.Title)
	data := email.BookingEmailData{
		GuestName:    b.GuestName,
		EventTitle:   et.Title,
		GuestMessage: et.GuestMessage,
		StartAt:      b.StartAt,
	}
	if text, html, err := email.RenderBookingRejected(data); err == nil {
		_ = s.mailer.Send(b.GuestEmail, subject, text, html)
	}

	return b, nil
}

func (s *Service) StartGCLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cancelled, err := db.CancelExpiredBookings(s.db)
			if err != nil {
				log.Printf("gc: cancel expired bookings: %v", err)
				continue
			}
			for _, b := range cancelled {
				et, _, err := s.fetchEventTypeAndUser(b.EventTypeID)
				if err != nil {
					continue
				}
				subject := fmt.Sprintf("Your booking request for %s has expired", et.Title)
				data := email.BookingEmailData{
					GuestName:  b.GuestName,
					EventTitle: et.Title,
					StartAt:    b.StartAt,
				}
				if text, html, err := email.RenderBookingCancelled(data); err == nil {
					_ = s.mailer.Send(b.GuestEmail, subject, text, html)
				}
			}
		}
	}
}

func (s *Service) fetchEventTypeAndUser(eventTypeID int64) (*db.EventType, *db.User, error) {
	var et db.EventType
	err := s.db.QueryRow(`
		SELECT id, user_id, slug, title, description, guest_message, duration_minutes, color,
		       booking_window_days, active, created_at
		FROM event_types WHERE id = ?
	`, eventTypeID).Scan(&et.ID, &et.UserID, &et.Slug, &et.Title, &et.Description, &et.GuestMessage,
		&et.DurationMinutes, &et.Color, &et.BookingWindowDays, &et.Active, &et.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	user, err := db.GetUserByID(s.db, et.UserID)
	if err != nil {
		return &et, nil, err
	}
	return &et, user, nil
}
