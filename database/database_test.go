package database

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesTables(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "test.db")

	db, err := Open(databasePath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	expectedTables := []string{
		"users",
		"accounts",
	}

	for _, expectedTable := range expectedTables {
		t.Run(expectedTable, func(t *testing.T) {
			var tableName string

			err := db.QueryRow(
				`
					SELECT name
					FROM sqlite_master
					WHERE type = 'table'
					  AND name = ?
				`,
				expectedTable,
			).Scan(&tableName)
			if err != nil {
				t.Fatalf(
					"failed to find %s table: %v",
					expectedTable,
					err,
				)
			}

			if tableName != expectedTable {
				t.Errorf(
					"expected table %q, got %q",
					expectedTable,
					tableName,
				)
			}
		})
	}
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "test.db")

	db, err := Open(databasePath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	var foreignKeysEnabled int

	if err := db.QueryRow(
		"PRAGMA foreign_keys",
	).Scan(&foreignKeysEnabled); err != nil {
		t.Fatalf("failed to read foreign-key setting: %v", err)
	}

	if foreignKeysEnabled != 1 {
		t.Errorf(
			"expected foreign keys to be enabled, got %d",
			foreignKeysEnabled,
		)
	}
}
