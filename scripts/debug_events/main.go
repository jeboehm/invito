// Debug script: triggers a live CalDAV sync then dumps all calendar events.
//
// Usage: source dev/local.env && go run ./scripts/debug_events
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	ical "github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/jeboehm/invito/internal/calendar"
	"github.com/jeboehm/invito/internal/crypto"
	"github.com/jeboehm/invito/internal/db"
)

func main() {
	dbPath := os.Getenv("INVITO_DB_PATH")
	if dbPath == "" {
		dbPath = "./invito-dev.db"
	}
	secretHex := os.Getenv("INVITO_SESSION_SECRET")

	var key [32]byte
	if secretHex != "" {
		b, err := hex.DecodeString(secretHex)
		if err != nil || len(b) != 32 {
			log.Fatalf("INVITO_SESSION_SECRET: must be 64 hex chars (err: %v)", err)
		}
		copy(key[:], b)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(90 * 24 * time.Hour)

	// --- Trigger a fresh sync for every calendar ---
	cals, err := db.ListAllSyncEnabledCalendars(database)
	if err != nil {
		log.Fatalf("list calendars: %v", err)
	}

	fmt.Printf("Syncing %d calendar(s)...\n", len(cals))
	for i := range cals {
		cal := &cals[i]
		if err := calendar.SyncCalendar(ctx, cal, key, database, from, to); err != nil {
			fmt.Printf("  calendar %d (%s): sync error: %v\n", cal.ID, cal.DisplayName, err)
		} else {
			fmt.Printf("  calendar %d (%s): OK\n", cal.ID, cal.DisplayName)
		}
	}
	fmt.Println()

	// --- DB dump after sync ---
	fmt.Println("=== CALENDAR EVENTS IN DATABASE ===")

	rows, err := database.Query(`
		SELECT u.email, c.display_name, ce.uid, ce.start_at, ce.end_at, ce.summary, ce.synced_at
		FROM calendar_events ce
		JOIN calendars c ON c.id = ce.calendar_id
		JOIN users u ON u.id = c.user_id
		ORDER BY u.email, c.display_name, ce.start_at
	`)
	if err != nil {
		log.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var userEmail, calName, uid, summary, syncedAt string
		var startAt, endAt time.Time
		if err := rows.Scan(&userEmail, &calName, &uid, &startAt, &endAt, &summary, &syncedAt); err != nil {
			log.Fatalf("scan: %v", err)
		}
		fmt.Printf("  [%s / %s] %s → %s  uid=%q  summary=%q\n",
			userEmail, calName,
			startAt.Format("2006-01-02 15:04"), endAt.Format("15:04"),
			uid, summary)
		count++
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}
	if count == 0 {
		fmt.Println("  (no events in database)")
	}
	fmt.Printf("  total: %d event(s)\n\n", count)

	// --- Live CalDAV query (raw, for comparison) ---
	fmt.Println("=== LIVE CALDAV QUERY (raw events from server) ===")

	for i := range cals {
		cal := &cals[i]
		fmt.Printf("\n--- Calendar ID=%d  URL=%s ---\n", cal.ID, cal.CalDAVURL)

		password, err := decryptPassword(key, cal.Password)
		if err != nil {
			fmt.Printf("  ERROR: decrypt password: %v\n", err)
			continue
		}

		httpClient := webdav.HTTPClientWithBasicAuth(http.DefaultClient, cal.Username, password)
		client, err := caldav.NewClient(httpClient, cal.CalDAVURL)
		if err != nil {
			fmt.Printf("  ERROR: new client: %v\n", err)
			continue
		}

		homeSet, _ := client.FindCalendarHomeSet(ctx, "")
		calendars, err := client.FindCalendars(ctx, homeSet)
		if err != nil || len(calendars) == 0 {
			fmt.Printf("  FindCalendars failed or empty (%v), querying base URL directly\n", err)
			queryAndPrint(ctx, client, "", from, to)
			continue
		}
		for _, c := range calendars {
			fmt.Printf("  [path: %s]\n", c.Path)
			queryAndPrint(ctx, client, c.Path, from, to)
		}
	}
}

func queryAndPrint(ctx context.Context, client *caldav.Client, path string, from, to time.Time) {
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR",
			Comps: []caldav.CalendarCompRequest{{Name: "VEVENT", AllProps: true}},
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{Name: "VEVENT", Start: from, End: to}},
		},
	}

	objects, err := client.QueryCalendar(ctx, path, query)
	if err != nil {
		fmt.Printf("    time-range query failed (%v), retrying without filter\n", err)
		queryAll := &caldav.CalendarQuery{
			CompRequest: caldav.CalendarCompRequest{
				Name:  "VCALENDAR",
				Comps: []caldav.CalendarCompRequest{{Name: "VEVENT", AllProps: true}},
			},
			CompFilter: caldav.CompFilter{Name: "VCALENDAR"},
		}
		objects, err = client.QueryCalendar(ctx, path, queryAll)
		if err != nil {
			fmt.Printf("    ERROR: %v\n", err)
			return
		}
	}

	fmt.Printf("    %d CalDAV object(s) returned\n", len(objects))
	for _, obj := range objects {
		if obj.Data == nil {
			continue
		}
		for _, comp := range obj.Data.Children {
			if comp.Name != ical.CompEvent {
				continue
			}
			uid, _ := comp.Props.Text(ical.PropUID)
			summary, _ := comp.Props.Text(ical.PropSummary)
			dtstart := propVal(comp.Props, ical.PropDateTimeStart)
			dtend := propVal(comp.Props, ical.PropDateTimeEnd)
			rrule := propVal(comp.Props, ical.PropRecurrenceRule)
			recurrID := propVal(comp.Props, ical.PropRecurrenceID)

			fmt.Printf("    VEVENT summary=%q dtstart=%q dtend=%q", summary, dtstart, dtend)
			if rrule != "" {
				fmt.Printf(" RRULE=%q", rrule)
			}
			if recurrID != "" {
				fmt.Printf(" RECURRENCE-ID=%q", recurrID)
			}
			fmt.Printf(" uid=%q\n", uid)
		}
	}
}

func propVal(props ical.Props, name string) string {
	p := props.Get(name)
	if p == nil {
		return ""
	}
	return p.Value
}

func decryptPassword(key [32]byte, encPass string) (string, error) {
	plain, err := crypto.Decrypt(key, []byte(encPass))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
