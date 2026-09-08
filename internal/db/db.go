// Package db owns Whatunga's small SQLite database: the single admin
// user (with a bcrypt password hash) and a flat key/value settings
// table (currently just the UI language preference).
//
// It uses modernc.org/sqlite, a pure-Go SQLite driver, specifically so
// the whole tool can still be built as a single static binary without
// requiring cgo or a system SQLite library.
package db

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

// DefaultAdminUsername and DefaultAdminPassword are used to seed the
// very first login on a fresh database. The password should be
// changed immediately from the web UI's Account page — the login
// page and README both call this out.
const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// Open opens (creating if necessary) the SQLite database at path,
// applies the schema, and seeds a default admin user if the users
// table is empty.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: opening %s: %w", path, err)
	}

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: applying schema: %w", err)
	}

	if err := seedDefaultAdmin(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func seedDefaultAdmin(conn *sql.DB) error {
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("db: counting users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("db: hashing default admin password: %w", err)
	}

	_, err = conn.Exec(
		`INSERT INTO users (username, password_hash) VALUES (?, ?)`,
		DefaultAdminUsername, string(hash),
	)
	if err != nil {
		return fmt.Errorf("db: seeding default admin: %w", err)
	}

	return nil
}

// ErrInvalidCredentials is returned by Authenticate when the username
// doesn't exist or the password doesn't match — deliberately the same
// error for both cases, so callers can't use response differences to
// enumerate valid usernames.
var ErrInvalidCredentials = errors.New("invalid username or password")

// Authenticate checks a username/password pair against the stored
// bcrypt hash.
func Authenticate(conn *sql.DB, username, password string) error {
	var hash string
	err := conn.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("db: looking up user: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}

	return nil
}

// ErrIncorrectCurrentPassword is returned by ChangePassword when the
// caller's supplied "current password" doesn't match.
var ErrIncorrectCurrentPassword = errors.New("current password is incorrect")

// ChangePassword verifies currentPassword against the stored hash and,
// if it matches, replaces it with a bcrypt hash of newPassword.
func ChangePassword(conn *sql.DB, username, currentPassword, newPassword string) error {
	if err := Authenticate(conn, username, currentPassword); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return ErrIncorrectCurrentPassword
		}
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("db: hashing new password: %w", err)
	}

	_, err = conn.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, string(hash), username)
	if err != nil {
		return fmt.Errorf("db: updating password: %w", err)
	}

	return nil
}

// GetSetting reads a value from the settings table, returning
// fallback if the key isn't set yet.
func GetSetting(conn *sql.DB, key, fallback string) string {
	var value string
	err := conn.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return fallback
	}
	return value
}

// SetSetting upserts a key/value pair in the settings table.
func SetSetting(conn *sql.DB, key, value string) error {
	_, err := conn.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("db: setting %s: %w", key, err)
	}
	return nil
}
