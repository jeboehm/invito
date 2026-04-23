package db

import (
	"database/sql"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertAndGetUser(t *testing.T) {
	db := openTestDB(t)

	u, err := UpsertUser(db, "sub1", "a@example.com", "Alice", "alice")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if u.Username != "alice" {
		t.Fatalf("got username %q", u.Username)
	}

	// Update email on second upsert
	u2, err := UpsertUser(db, "sub1", "b@example.com", "Alice B", "alice")
	if err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	if u2.Email != "b@example.com" {
		t.Fatalf("email not updated: %q", u2.Email)
	}
	if u2.ID != u.ID {
		t.Fatal("id changed on upsert")
	}
}

func TestSession(t *testing.T) {
	db := openTestDB(t)

	u, _ := UpsertUser(db, "sub1", "a@example.com", "Alice", "alice")

	id, err := CreateSession(db, u.ID, time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	s, err := GetSession(db, id)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if s.UserID != u.ID {
		t.Fatalf("wrong user id: %d", s.UserID)
	}

	if err := DeleteSession(db, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := GetSession(db, id); err == nil {
		t.Fatal("expected error for deleted session")
	}
}

func TestExpiredSession(t *testing.T) {
	db := openTestDB(t)
	u, _ := UpsertUser(db, "sub2", "b@example.com", "Bob", "bob")

	id, _ := CreateSession(db, u.ID, -time.Second) // already expired

	if _, err := GetSession(db, id); err == nil {
		t.Fatal("expected error for expired session")
	}
}
