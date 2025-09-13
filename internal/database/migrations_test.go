package database

import (
	"testing"
)

func TestGetEmbeddedMigrations(t *testing.T) {
	migrations := GetEmbeddedMigrations()

	if len(migrations) == 0 {
		t.Error("No embedded migrations found")
	}

	// Check first migration
	if len(migrations) > 0 {
		migration := migrations[0]

		if migration.Version != 1 {
			t.Errorf("Expected version 1, got %d", migration.Version)
		}

		if migration.Name != "initial_schema" {
			t.Errorf("Expected name 'initial_schema', got '%s'", migration.Name)
		}

		if migration.UpSQL == "" {
			t.Error("UpSQL is empty")
		}

		if migration.DownSQL == "" {
			t.Error("DownSQL is empty")
		}

		if migration.Description == "" {
			t.Error("Description is empty")
		}
	}
}

func TestMigrationSQLContent(t *testing.T) {
	migrations := GetEmbeddedMigrations()

	if len(migrations) == 0 {
		t.Fatal("No migrations found")
	}

	migration := migrations[0]

	// Check that UP SQL contains expected tables
	expectedTables := []string{"calls", "config"}
	for _, table := range expectedTables {
		if !containsSubstring(migration.UpSQL, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Errorf("UpSQL does not contain CREATE TABLE for %s", table)
		}
	}

	// Check that DOWN SQL contains expected drops
	for _, table := range expectedTables {
		if !containsSubstring(migration.DownSQL, "DROP TABLE IF EXISTS "+table) {
			t.Errorf("DownSQL does not contain DROP TABLE for %s", table)
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
