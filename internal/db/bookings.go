package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Booking struct {
	ID            int64
	EventTypeID   int64
	GuestName     string
	GuestEmail    string
	GuestNote     string
	StartAt       time.Time
	EndAt         time.Time
	Status        string
	Token         string
	ReservedUntil time.Time
	CreatedAt     time.Time
}

type BookingWithEventType struct {
	Booking
	EventType EventType
	User      User
}

func CreateBooking(database *sql.DB, b *Booking) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Re-check for conflicting calendar events
	var count int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM calendar_events ce
		JOIN calendars c ON c.id = ce.calendar_id
		JOIN event_types et ON et.id = ?
		WHERE c.user_id = et.user_id
		  AND ce.start_at < ? AND ce.end_at > ?
	`, b.EventTypeID, b.EndAt.UTC(), b.StartAt.UTC()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("slot conflicts with an existing calendar event")
	}

	// Re-check for conflicting bookings
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM bookings b2
		JOIN event_types et ON et.id = b2.event_type_id
		JOIN event_types et2 ON et2.id = ?
		WHERE et.user_id = et2.user_id
		  AND b2.status IN ('PENDING','CONFIRMED')
		  AND b2.start_at < ? AND b2.end_at > ?
	`, b.EventTypeID, b.EndAt.UTC(), b.StartAt.UTC()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("slot is no longer available")
	}

	_, err = tx.Exec(`
		INSERT INTO bookings
			(event_type_id, guest_name, guest_email, guest_note,
			 start_at, end_at, token, reserved_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, b.EventTypeID, b.GuestName, b.GuestEmail, b.GuestNote,
		b.StartAt.UTC(), b.EndAt.UTC(), b.Token, b.ReservedUntil.UTC())
	if err != nil {
		return err
	}

	return tx.Commit()
}

func GetBookingByToken(database *sql.DB, token string) (*Booking, error) {
	b := &Booking{}
	err := database.QueryRow(`
		SELECT id, event_type_id, guest_name, guest_email, guest_note,
		       start_at, end_at, status, token, reserved_until, created_at
		FROM bookings WHERE token = ?
	`, token).Scan(&b.ID, &b.EventTypeID, &b.GuestName, &b.GuestEmail, &b.GuestNote,
		&b.StartAt, &b.EndAt, &b.Status, &b.Token, &b.ReservedUntil, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ListPendingBookingsInRange(database *sql.DB, userID int64, from, to time.Time) ([]Booking, error) {
	rows, err := database.Query(`
		SELECT b.id, b.event_type_id, b.guest_name, b.guest_email, b.guest_note,
		       b.start_at, b.end_at, b.status, b.token, b.reserved_until, b.created_at
		FROM bookings b
		JOIN event_types et ON et.id = b.event_type_id
		WHERE et.user_id = ?
		  AND b.status IN ('PENDING','CONFIRMED')
		  AND b.start_at < ? AND b.end_at > ?
	`, userID, to.UTC(), from.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func UpdateBookingStatus(database *sql.DB, id int64, status string) error {
	_, err := database.Exec(`UPDATE bookings SET status = ? WHERE id = ?`, status, id)
	return err
}

func CancelExpiredBookings(database *sql.DB) ([]Booking, error) {
	rows, err := database.Query(`
		SELECT id, event_type_id, guest_name, guest_email, guest_note,
		       start_at, end_at, status, token, reserved_until, created_at
		FROM bookings
		WHERE status = 'PENDING' AND reserved_until <= CURRENT_TIMESTAMP
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings, err := scanBookings(rows)
	if err != nil {
		return nil, err
	}

	if len(bookings) > 0 {
		ids := make([]string, len(bookings))
		args := make([]any, len(bookings))
		for i, b := range bookings {
			ids[i] = "?"
			args[i] = b.ID
		}
		_, err = database.Exec(
			`UPDATE bookings SET status = 'CANCELLED' WHERE id IN (`+strings.Join(ids, ",")+`)`,
			args...,
		)
		if err != nil {
			return nil, err
		}
	}

	return bookings, nil
}

func CountPendingForUser(database *sql.DB, userID int64) (int, error) {
	var count int
	err := database.QueryRow(`
		SELECT COUNT(*) FROM bookings b
		JOIN event_types et ON et.id = b.event_type_id
		WHERE et.user_id = ? AND b.status = 'PENDING'
	`, userID).Scan(&count)
	return count, err
}

func ListUpcomingConfirmedForUser(database *sql.DB, userID int64, limit int) ([]BookingWithEventType, error) {
	rows, err := database.Query(`
		SELECT b.id, b.event_type_id, b.guest_name, b.guest_email, b.guest_note,
		       b.start_at, b.end_at, b.status, b.token, b.reserved_until, b.created_at,
		       et.id, et.user_id, et.slug, et.title, et.description,
		       et.duration_minutes, et.color, et.booking_window_days, et.active, et.created_at
		FROM bookings b
		JOIN event_types et ON et.id = b.event_type_id
		WHERE et.user_id = ? AND b.status = 'CONFIRMED' AND b.start_at > CURRENT_TIMESTAMP
		ORDER BY b.start_at ASC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookingsWithET(rows)
}

func ListBookingsForUser(database *sql.DB, userID int64, statusFilter string, limit int) ([]BookingWithEventType, error) {
	query := `
		SELECT b.id, b.event_type_id, b.guest_name, b.guest_email, b.guest_note,
		       b.start_at, b.end_at, b.status, b.token, b.reserved_until, b.created_at,
		       et.id, et.user_id, et.slug, et.title, et.description,
		       et.duration_minutes, et.color, et.booking_window_days, et.active, et.created_at
		FROM bookings b
		JOIN event_types et ON et.id = b.event_type_id
		WHERE et.user_id = ?`
	args := []any{userID}

	if statusFilter != "" {
		query += ` AND b.status = ?`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY b.start_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookingsWithET(rows)
}

func scanBookings(rows *sql.Rows) ([]Booking, error) {
	var bs []Booking
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.EventTypeID, &b.GuestName, &b.GuestEmail, &b.GuestNote,
			&b.StartAt, &b.EndAt, &b.Status, &b.Token, &b.ReservedUntil, &b.CreatedAt); err != nil {
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, rows.Err()
}

func scanBookingsWithET(rows *sql.Rows) ([]BookingWithEventType, error) {
	var result []BookingWithEventType
	for rows.Next() {
		var bwe BookingWithEventType
		b := &bwe.Booking
		et := &bwe.EventType
		if err := rows.Scan(
			&b.ID, &b.EventTypeID, &b.GuestName, &b.GuestEmail, &b.GuestNote,
			&b.StartAt, &b.EndAt, &b.Status, &b.Token, &b.ReservedUntil, &b.CreatedAt,
			&et.ID, &et.UserID, &et.Slug, &et.Title, &et.Description,
			&et.DurationMinutes, &et.Color, &et.BookingWindowDays, &et.Active, &et.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, bwe)
	}
	return result, rows.Err()
}
