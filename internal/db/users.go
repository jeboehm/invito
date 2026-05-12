package db

import (
	"database/sql"
	"time"
)

type User struct {
	ID        int64
	OIDCSub   string
	Email     string
	Name      string
	Username  string
	Timezone  string // IANA timezone name, e.g. "Europe/Berlin"; default "UTC"
	CreatedAt time.Time
}

func (u *User) Location() *time.Location {
	if u.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func UpsertUser(db *sql.DB, sub, email, name, username string) (*User, error) {
	_, err := db.Exec(`
		INSERT INTO users (oidc_sub, email, name, username)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(oidc_sub) DO NOTHING
	`, sub, email, name, username)
	if err != nil {
		return nil, err
	}
	return GetUserBySub(db, sub)
}

func UpdateUserProfile(db *sql.DB, id int64, name, username, timezone, email string) error {
	_, err := db.Exec(`
		UPDATE users SET name = ?, username = ?, timezone = ?, email = ? WHERE id = ?
	`, name, username, timezone, email, id)
	return err
}

func GetUserBySub(db *sql.DB, sub string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, oidc_sub, email, name, username, timezone, created_at
		FROM users WHERE oidc_sub = ?
	`, sub).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.Name, &u.Username, &u.Timezone, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, oidc_sub, email, name, username, timezone, created_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.Name, &u.Username, &u.Timezone, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, oidc_sub, email, name, username, timezone, created_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.Name, &u.Username, &u.Timezone, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
