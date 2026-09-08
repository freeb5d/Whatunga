package db

import (
	"database/sql"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestSeedDefaultAdmin_CreatesAdminUser(t *testing.T) {
	conn := openTestDB(t)

	if err := Authenticate(conn, DefaultAdminUsername, DefaultAdminPassword); err != nil {
		t.Fatalf("Authenticate with default credentials: %v", err)
	}
}

func TestAuthenticate_RejectsWrongPassword(t *testing.T) {
	conn := openTestDB(t)

	err := Authenticate(conn, DefaultAdminUsername, "totally-wrong")
	if err != ErrInvalidCredentials {
		t.Fatalf("got err %v, want ErrInvalidCredentials", err)
	}
}

func TestChangePassword_UpdatesAndOldPasswordStopsWorking(t *testing.T) {
	conn := openTestDB(t)

	if err := ChangePassword(conn, DefaultAdminUsername, DefaultAdminPassword, "a-new-strong-password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if err := Authenticate(conn, DefaultAdminUsername, "a-new-strong-password"); err != nil {
		t.Fatalf("Authenticate with new password: %v", err)
	}

	if err := Authenticate(conn, DefaultAdminUsername, DefaultAdminPassword); err != ErrInvalidCredentials {
		t.Fatalf("old password should no longer work, got err %v", err)
	}
}

func TestChangePassword_RejectsWrongCurrentPassword(t *testing.T) {
	conn := openTestDB(t)

	err := ChangePassword(conn, DefaultAdminUsername, "wrong-current", "new-password-123")
	if err != ErrIncorrectCurrentPassword {
		t.Fatalf("got err %v, want ErrIncorrectCurrentPassword", err)
	}
}

func TestSettings_GetSetDefault(t *testing.T) {
	conn := openTestDB(t)

	if got := GetSetting(conn, "language", "en"); got != "en" {
		t.Errorf("GetSetting default = %q, want %q", got, "en")
	}

	if err := SetSetting(conn, "language", "mi"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	if got := GetSetting(conn, "language", "en"); got != "mi" {
		t.Errorf("GetSetting after update = %q, want %q", got, "mi")
	}
}
