package db

import (
	"database/sql"
	"time"
)

type Calendar struct {
	ID          int64
	UserID      int64
	CalDAVURL   string
	Username    string
	Password    string
	DisplayName string
	Color       string
	SyncEnabled bool
	LastSynced  *time.Time
	CreatedAt   time.Time
}

type CalendarEvent struct {
	ID         int64
	CalendarID int64
	UID        string
	StartAt    time.Time
	EndAt      time.Time
	Summary    string
	SyncedAt   time.Time
}

func CreateCalendar(db *sql.DB, c *Calendar) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO calendars (user_id, caldav_url, username, password, display_name, color)
		VALUES (?, ?, ?, ?, ?, ?)
	`, c.UserID, c.CalDAVURL, c.Username, c.Password, c.DisplayName, c.Color)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListCalendars(db *sql.DB, userID int64) ([]Calendar, error) {
	rows, err := db.Query(`
		SELECT id, user_id, caldav_url, username, password, display_name, color,
		       sync_enabled, last_synced, created_at
		FROM calendars WHERE user_id = ? ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cals []Calendar
	for rows.Next() {
		var c Calendar
		if err := rows.Scan(&c.ID, &c.UserID, &c.CalDAVURL, &c.Username, &c.Password,
			&c.DisplayName, &c.Color, &c.SyncEnabled, &c.LastSynced, &c.CreatedAt); err != nil {
			return nil, err
		}
		cals = append(cals, c)
	}
	return cals, rows.Err()
}

func GetCalendar(db *sql.DB, id, userID int64) (*Calendar, error) {
	c := &Calendar{}
	err := db.QueryRow(`
		SELECT id, user_id, caldav_url, username, password, display_name, color,
		       sync_enabled, last_synced, created_at
		FROM calendars WHERE id = ? AND user_id = ?
	`, id, userID).Scan(&c.ID, &c.UserID, &c.CalDAVURL, &c.Username, &c.Password,
		&c.DisplayName, &c.Color, &c.SyncEnabled, &c.LastSynced, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func ListAllSyncEnabledCalendars(db *sql.DB) ([]Calendar, error) {
	rows, err := db.Query(`
		SELECT id, user_id, caldav_url, username, password, display_name, color,
		       sync_enabled, last_synced, created_at
		FROM calendars WHERE sync_enabled = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cals []Calendar
	for rows.Next() {
		var c Calendar
		if err := rows.Scan(&c.ID, &c.UserID, &c.CalDAVURL, &c.Username, &c.Password,
			&c.DisplayName, &c.Color, &c.SyncEnabled, &c.LastSynced, &c.CreatedAt); err != nil {
			return nil, err
		}
		cals = append(cals, c)
	}
	return cals, rows.Err()
}

func DeleteCalendar(db *sql.DB, id, userID int64) error {
	_, err := db.Exec(`DELETE FROM calendars WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func UpdateLastSynced(db *sql.DB, id int64, t time.Time) error {
	_, err := db.Exec(`UPDATE calendars SET last_synced = ? WHERE id = ?`, t.UTC(), id)
	return err
}

func UpsertCalendarEvent(db *sql.DB, e *CalendarEvent) error {
	_, err := db.Exec(`
		INSERT INTO calendar_events (calendar_id, uid, start_at, end_at, summary, synced_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(calendar_id, uid) DO UPDATE SET
			start_at  = excluded.start_at,
			end_at    = excluded.end_at,
			summary   = excluded.summary,
			synced_at = excluded.synced_at
	`, e.CalendarID, e.UID, e.StartAt.UTC(), e.EndAt.UTC(), e.Summary)
	return err
}

func ListCalendarEventsForUser(db *sql.DB, userID int64, from, to time.Time) ([]CalendarEvent, error) {
	rows, err := db.Query(`
		SELECT ce.id, ce.calendar_id, ce.uid, ce.start_at, ce.end_at, ce.summary, ce.synced_at
		FROM calendar_events ce
		JOIN calendars c ON c.id = ce.calendar_id
		WHERE c.user_id = ?
		  AND ce.start_at < ?
		  AND ce.end_at > ?
	`, userID, to.UTC(), from.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []CalendarEvent
	for rows.Next() {
		var e CalendarEvent
		if err := rows.Scan(&e.ID, &e.CalendarID, &e.UID, &e.StartAt, &e.EndAt, &e.Summary, &e.SyncedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func PurgeOldCalendarEvents(db *sql.DB, calendarID int64, before time.Time) error {
	_, err := db.Exec(`
		DELETE FROM calendar_events WHERE calendar_id = ? AND end_at < ?
	`, calendarID, before.UTC())
	return err
}
