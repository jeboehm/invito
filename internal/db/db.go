package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// migrations are run after schema init; "duplicate column name" errors are ignored.
var migrations = []string{
	`ALTER TABLE event_types ADD COLUMN guest_message TEXT`,
	`ALTER TABLE event_types ADD COLUMN confirmed_message TEXT`,
	`ALTER TABLE event_types ADD COLUMN rejected_message TEXT`,
	`UPDATE event_types SET confirmed_message = guest_message, rejected_message = guest_message WHERE guest_message IS NOT NULL AND confirmed_message IS NULL`,
}

func Open(path string) (*sql.DB, error) {
	dsn := path + "?_foreign_keys=on"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			db.Close()
			return nil, fmt.Errorf("migration %q: %w", m, err)
		}
	}

	return db, nil
}
