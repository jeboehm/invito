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
	CreatedAt time.Time
}

func UpsertUser(db *sql.DB, sub, email, name, username string) (*User, error) {
	_, err := db.Exec(`
		INSERT INTO users (oidc_sub, email, name, username)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(oidc_sub) DO UPDATE SET
			email = excluded.email
	`, sub, email, name, username)
	if err != nil {
		return nil, err
	}
	return GetUserBySub(db, sub)
}

func UpdateUserProfile(db *sql.DB, id int64, name, username string) error {
	_, err := db.Exec(`
		UPDATE users SET name = ?, username = ? WHERE id = ?
	`, name, username, id)
	return err
}

func GetUserBySub(db *sql.DB, sub string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, oidc_sub, email, name, username, created_at
		FROM users WHERE oidc_sub = ?
	`, sub).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.Name, &u.Username, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, oidc_sub, email, name, username, created_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.Name, &u.Username, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, oidc_sub, email, name, username, created_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.OIDCSub, &u.Email, &u.Name, &u.Username, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
