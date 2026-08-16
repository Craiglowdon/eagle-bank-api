package database

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesUsersTable(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "test.db")

	db, err := Open(databasePath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	var tableName string

	err = db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name = 'users'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("failed to find users table: %v", err)
	}

	if tableName != "users" {
		t.Errorf("expected users table, got %q", tableName)
	}
}
