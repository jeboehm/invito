package calendar

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/jeboehm/invito/internal/db"
)

func StartSyncLoop(ctx context.Context, database *sql.DB, key [32]byte, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSync(ctx, database, key)
		}
	}
}

func runSync(ctx context.Context, database *sql.DB, key [32]byte) {
	cals, err := db.ListAllSyncEnabledCalendars(database)
	if err != nil {
		log.Printf("sync: list calendars: %v", err)
		return
	}

	now := time.Now()
	from := now.Add(-time.Hour)

	for _, cal := range cals {
		// Determine booking window from associated event types (use 60 days as safe default)
		to := now.Add(60 * 24 * time.Hour)

		calCopy := cal
		if err := SyncCalendar(ctx, &calCopy, key, database, from, to); err != nil {
			log.Printf("sync calendar %d (%s): %v", cal.ID, cal.DisplayName, err)
			continue
		}
		db.UpdateLastSynced(database, cal.ID, now)
	}
}
