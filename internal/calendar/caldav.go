package calendar

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/jboehm/invito/internal/crypto"
	"github.com/jboehm/invito/internal/db"
)

func VerifyCredentials(ctx context.Context, rawURL, username, password string) error {
	client, err := newClient(rawURL, username, password)
	if err != nil {
		return err
	}
	_, err = client.FindCalendars(ctx, "")
	if err != nil {
		// Try PROPFIND on the URL directly — some servers expose a single calendar
		_, err2 := client.FindCalendarHomeSet(ctx, "")
		if err2 != nil {
			return fmt.Errorf("could not connect: %w", err)
		}
	}
	return nil
}

func SyncCalendar(ctx context.Context, cal *db.Calendar, key [32]byte, database *sql.DB, from, to time.Time) error {
	password, err := decryptPassword(key, cal.Password)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	client, err := newClient(cal.CalDAVURL, cal.Username, password)
	if err != nil {
		return err
	}

	// Try to find calendar home set, then enumerate calendars
	homeSet, err := client.FindCalendarHomeSet(ctx, "")
	if err != nil {
		// Fallback: treat the URL itself as the calendar path
		homeSet = ""
	}

	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil {
		// If FindCalendars fails, try treating the URL itself as one calendar
		if err2 := syncPath(ctx, client, database, cal.ID, "", from, to); err2 != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
		return nil
	}

	for _, c := range calendars {
		if err := syncPath(ctx, client, database, cal.ID, c.Path, from, to); err != nil {
			return fmt.Errorf("sync %s: %w", c.Path, err)
		}
	}

	_ = db.PurgeOldCalendarEvents(database, cal.ID, from)
	return nil
}

func WriteEvent(ctx context.Context, cal *db.Calendar, key [32]byte, booking *db.Booking, eventType *db.EventType, guestName string) error {
	password, err := decryptPassword(key, cal.Password)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	icalStr := buildICAL(booking, eventType, guestName)
	putURL := strings.TrimRight(cal.CalDAVURL, "/") + "/" + booking.Token + ".ics"

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, strings.NewReader(icalStr))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cal.Username, password)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("CalDAV PUT returned %d", resp.StatusCode)
	}
	return nil
}

func newClient(rawURL, username, password string) (*caldav.Client, error) {
	httpClient := webdav.HTTPClientWithBasicAuth(http.DefaultClient, username, password)
	return caldav.NewClient(httpClient, rawURL)
}

func syncPath(ctx context.Context, client *caldav.Client, database *sql.DB, calendarID int64, path string, from, to time.Time) error {
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR",
			Comps: []caldav.CalendarCompRequest{{
				Name:    "VEVENT",
				AllProps: true,
			}},
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name:  "VEVENT",
				Start: from,
				End:   to,
			}},
		},
	}

	objects, err := client.QueryCalendar(ctx, path, query)
	if err != nil {
		// Fallback: fetch all without time filter
		queryAll := &caldav.CalendarQuery{
			CompRequest: caldav.CalendarCompRequest{
				Name: "VCALENDAR",
				Comps: []caldav.CalendarCompRequest{{
					Name:    "VEVENT",
					AllProps: true,
				}},
			},
			CompFilter: caldav.CompFilter{Name: "VCALENDAR"},
		}
		objects, err = client.QueryCalendar(ctx, path, queryAll)
		if err != nil {
			return err
		}
	}

	for _, obj := range objects {
		if obj.Data == nil {
			continue
		}
		for _, comp := range obj.Data.Children {
			if comp.Name != ical.CompEvent {
				continue
			}
			if err := upsertVEVENT(database, calendarID, comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertVEVENT(database *sql.DB, calendarID int64, comp *ical.Component) error {
	uid, err := comp.Props.Text(ical.PropUID)
	if err != nil || uid == "" {
		return nil
	}

	startAt, err := comp.Props.DateTime(ical.PropDateTimeStart, time.UTC)
	if err != nil || startAt.IsZero() {
		return nil
	}

	endAt, err := comp.Props.DateTime(ical.PropDateTimeEnd, time.UTC)
	if err != nil || endAt.IsZero() {
		endAt = startAt.Add(time.Hour)
	}

	summary, _ := comp.Props.Text(ical.PropSummary)

	return db.UpsertCalendarEvent(database, &db.CalendarEvent{
		CalendarID: calendarID,
		UID:        uid,
		StartAt:    startAt,
		EndAt:      endAt,
		Summary:    summary,
	})
}

func buildICAL(booking *db.Booking, eventType *db.EventType, guestName string) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Invito//Invito//EN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s@invito\r\n", booking.Token)
	fmt.Fprintf(&b, "DTSTART:%s\r\n", booking.StartAt.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(&b, "DTEND:%s\r\n", booking.EndAt.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(&b, "SUMMARY:%s with %s\r\n", eventType.Title, guestName)
	if booking.GuestNote != "" {
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", strings.ReplaceAll(booking.GuestNote, "\n", "\\n"))
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func decryptPassword(key [32]byte, encPass string) (string, error) {
	plain, err := crypto.Decrypt(key, []byte(encPass))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
