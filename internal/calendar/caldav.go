package calendar

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
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

	homeSet, err := client.FindCalendarHomeSet(ctx, "")
	if err != nil {
		log.Printf("sync calendar %d: FindCalendarHomeSet: %v — using base URL as home set", cal.ID, err)
		homeSet = ""
	} else {
		log.Printf("sync calendar %d: home set = %q", cal.ID, homeSet)
	}

	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil {
		log.Printf("sync calendar %d: FindCalendars(%q): %v — falling back to direct URL sync", cal.ID, homeSet, err)
	} else {
		log.Printf("sync calendar %d: found %d calendar(s)", cal.ID, len(calendars))
	}

	if err != nil || len(calendars) == 0 {
		if err2 := syncPath(ctx, client, database, cal.ID, "", from, to); err2 != nil {
			return fmt.Errorf("sync failed (direct URL): %w", err2)
		}
		return nil
	}

	for _, c := range calendars {
		log.Printf("sync calendar %d: syncing path %q", cal.ID, c.Path)
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

	// cal.CalDAVURL may be a principal URL, which rejects PUT with 403; discover the actual collection first.
	client, err := newClient(cal.CalDAVURL, cal.Username, password)
	if err != nil {
		return err
	}

	collectionPath, err := discoverFirstCalendarPath(ctx, client)
	if err != nil {
		log.Printf("caldav write-back: discovery failed (%v), falling back to configured URL", err)
		collectionPath = cal.CalDAVURL
	}

	// collectionPath may be server-root-relative; resolve against the configured host.
	putURL, err := resolveCalendarObjectURL(cal.CalDAVURL, collectionPath, booking.Token+".ics")
	if err != nil {
		return err
	}

	icalStr := buildICAL(booking, eventType, guestName)
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

func discoverFirstCalendarPath(ctx context.Context, client *caldav.Client) (string, error) {
	homeSet, err := client.FindCalendarHomeSet(ctx, "")
	if err != nil {
		return "", fmt.Errorf("FindCalendarHomeSet: %w", err)
	}
	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil || len(calendars) == 0 {
		return "", fmt.Errorf("no calendars found (FindCalendars: %v)", err)
	}
	return calendars[0].Path, nil
}

// resolveCalendarObjectURL resolves calendarPath (which may be server-root-relative)
// against the scheme+host of baseURL, then appends filename.
func resolveCalendarObjectURL(baseURL, calendarPath, filename string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	ref, err := url.Parse(calendarPath)
	if err != nil {
		return "", fmt.Errorf("parse calendar path: %w", err)
	}
	resolved := base.ResolveReference(ref)
	resolved.Path = strings.TrimRight(resolved.Path, "/") + "/" + filename
	return resolved.String(), nil
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
				Name:     "VEVENT",
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
		log.Printf("sync calendar %d path %q: time-filtered query failed (%v), retrying without filter", calendarID, path, err)
		queryAll := &caldav.CalendarQuery{
			CompRequest: caldav.CalendarCompRequest{
				Name: "VCALENDAR",
				Comps: []caldav.CalendarCompRequest{{
					Name:     "VEVENT",
					AllProps: true,
				}},
			},
			CompFilter: caldav.CompFilter{Name: "VCALENDAR"},
		}
		objects, err = client.QueryCalendar(ctx, path, queryAll)
		if err != nil {
			return fmt.Errorf("query calendar: %w", err)
		}
	}

	log.Printf("sync calendar %d path %q: received %d object(s)", calendarID, path, len(objects))

	inserted := 0
	for _, obj := range objects {
		if obj.Data == nil {
			continue
		}
		for _, comp := range obj.Data.Children {
			if comp.Name != ical.CompEvent {
				continue
			}
			if err := upsertVEVENT(database, calendarID, comp, time.Local); err != nil {
				return err
			}
			inserted++
		}
	}

	log.Printf("sync calendar %d path %q: upserted %d event(s)", calendarID, path, inserted)
	return nil
}

func upsertVEVENT(database *sql.DB, calendarID int64, comp *ical.Component, loc *time.Location) error {
	uid, err := comp.Props.Text(ical.PropUID)
	if err != nil || uid == "" {
		log.Printf("sync: skipping event without UID (err: %v)", err)
		return nil
	}

	startAt, err := parseDateTimeWithFallback(comp.Props, ical.PropDateTimeStart, loc)
	if err != nil || startAt.IsZero() {
		log.Printf("sync: skipping event UID=%q: DTSTART parse error: %v", uid, err)
		return nil
	}

	endAt, err := parseDateTimeWithFallback(comp.Props, ical.PropDateTimeEnd, loc)
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

// parseDateTimeWithFallback calls Prop.DateTime with loc. If it fails because
// the TZID parameter refers to an unrecognised timezone (common with Windows /
// Outlook calendar data, e.g. "W. Europe Standard Time"), it retries by
// ignoring the TZID and treating the value as a local time in loc.
func parseDateTimeWithFallback(props ical.Props, name string, loc *time.Location) (time.Time, error) {
	prop := props.Get(name)
	if prop == nil {
		return time.Time{}, fmt.Errorf("property %s not found", name)
	}

	t, err := prop.DateTime(loc)
	if err == nil {
		return t, nil
	}

	// If a TZID is set but unrecognised, retry without it by parsing the raw value.
	if tzid := prop.Params.Get(ical.PropTimezoneID); tzid != "" {
		log.Printf("sync: TZID %q unrecognised for property %s, falling back to local time", tzid, name)
		v := prop.Value
		switch len(v) {
		case 8: // DATE: 20060102
			return time.ParseInLocation("20060102", v, loc)
		case 15: // DATE-TIME local: 20060102T150405
			return time.ParseInLocation("20060102T150405", v, loc)
		case 16: // DATE-TIME UTC: 20060102T150405Z
			return time.ParseInLocation("20060102T150405Z", v, time.UTC)
		}
	}

	return time.Time{}, err
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
	fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", buildDescription(booking, eventType))
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func buildDescription(booking *db.Booking, eventType *db.EventType) string {
	parts := []string{
		"Guest: " + icalEscape(booking.GuestName),
		"Email: " + icalEscape(booking.GuestEmail),
	}
	if booking.GuestNote != "" {
		parts = append(parts, icalEscape(booking.GuestNote))
	}
	if eventType.ConfirmedMessage != "" {
		parts = append(parts, icalEscape(eventType.ConfirmedMessage))
	}
	return strings.Join(parts, "\\n")
}

func icalEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	return s
}

func decryptPassword(key [32]byte, encPass string) (string, error) {
	plain, err := crypto.Decrypt(key, []byte(encPass))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
