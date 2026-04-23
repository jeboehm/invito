package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

func CreateSession(db *sql.DB, userID int64, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)
	expiresAt := time.Now().Add(ttl)

	_, err := db.Exec(`
		INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)
	`, id, userID, expiresAt.UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func GetSession(db *sql.DB, id string) (*Session, error) {
	s := &Session{}
	err := db.QueryRow(`
		SELECT id, user_id, expires_at FROM sessions
		WHERE id = ? AND expires_at > CURRENT_TIMESTAMP
	`, id).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func DeleteSession(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func PruneExpiredSessions(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}
