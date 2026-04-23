package db

import (
	"database/sql"
	"time"
)

type EventType struct {
	ID                int64
	UserID            int64
	Slug              string
	Title             string
	Description       string
	GuestMessage      string
	DurationMinutes   int
	Color             string
	BookingWindowDays int
	Active            bool
	CreatedAt         time.Time
}

func CreateEventType(db *sql.DB, et *EventType) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO event_types
			(user_id, slug, title, description, guest_message, duration_minutes, color, booking_window_days)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, et.UserID, et.Slug, et.Title, et.Description, et.GuestMessage,
		et.DurationMinutes, et.Color, et.BookingWindowDays)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListEventTypes(db *sql.DB, userID int64) ([]EventType, error) {
	rows, err := db.Query(`
		SELECT id, user_id, slug, title, description, guest_message, duration_minutes, color,
		       booking_window_days, active, created_at
		FROM event_types WHERE user_id = ? ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ets []EventType
	for rows.Next() {
		var et EventType
		if err := rows.Scan(&et.ID, &et.UserID, &et.Slug, &et.Title, &et.Description, &et.GuestMessage,
			&et.DurationMinutes, &et.Color, &et.BookingWindowDays, &et.Active, &et.CreatedAt); err != nil {
			return nil, err
		}
		ets = append(ets, et)
	}
	return ets, rows.Err()
}

func GetEventType(db *sql.DB, id, userID int64) (*EventType, error) {
	et := &EventType{}
	err := db.QueryRow(`
		SELECT id, user_id, slug, title, description, guest_message, duration_minutes, color,
		       booking_window_days, active, created_at
		FROM event_types WHERE id = ? AND user_id = ?
	`, id, userID).Scan(&et.ID, &et.UserID, &et.Slug, &et.Title, &et.Description, &et.GuestMessage,
		&et.DurationMinutes, &et.Color, &et.BookingWindowDays, &et.Active, &et.CreatedAt)
	if err != nil {
		return nil, err
	}
	return et, nil
}

func GetEventTypeBySlug(db *sql.DB, userID int64, slug string) (*EventType, error) {
	et := &EventType{}
	err := db.QueryRow(`
		SELECT id, user_id, slug, title, description, guest_message, duration_minutes, color,
		       booking_window_days, active, created_at
		FROM event_types WHERE user_id = ? AND slug = ?
	`, userID, slug).Scan(&et.ID, &et.UserID, &et.Slug, &et.Title, &et.Description, &et.GuestMessage,
		&et.DurationMinutes, &et.Color, &et.BookingWindowDays, &et.Active, &et.CreatedAt)
	if err != nil {
		return nil, err
	}
	return et, nil
}

func UpdateEventType(db *sql.DB, et *EventType) error {
	_, err := db.Exec(`
		UPDATE event_types SET
			title = ?, description = ?, guest_message = ?, duration_minutes = ?,
			color = ?, booking_window_days = ?
		WHERE id = ? AND user_id = ?
	`, et.Title, et.Description, et.GuestMessage, et.DurationMinutes,
		et.Color, et.BookingWindowDays, et.ID, et.UserID)
	return err
}

func ToggleEventType(db *sql.DB, id, userID int64) error {
	_, err := db.Exec(`
		UPDATE event_types SET active = NOT active WHERE id = ? AND user_id = ?
	`, id, userID)
	return err
}
