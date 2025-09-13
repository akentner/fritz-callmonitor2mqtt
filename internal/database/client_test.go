package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	client, err := NewClient(tempDir)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check that data directory was created
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Errorf("Data directory was not created")
	}

	// Check that database subdirectory was created
	dbDir := filepath.Join(tempDir, "database")
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		t.Errorf("Database directory was not created")
	}

	// Check database path
	expectedPath := filepath.Join(dbDir, "fritz-callmonitor.db")
	if client.GetDatabasePath() != expectedPath {
		t.Errorf("Expected database path %s, got %s", expectedPath, client.GetDatabasePath())
	}

	// Check data directory
	if client.GetDataDir() != tempDir {
		t.Errorf("Expected data directory %s, got %s", tempDir, client.GetDataDir())
	}
}

func TestClientConnectAndClose(t *testing.T) {
	tempDir := t.TempDir()

	client, err := NewClient(tempDir)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test connection
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Check that we can get the database connection
	db := client.DB()
	if db == nil {
		t.Error("Database connection is nil")
	}

	// Test ping
	if err := db.Ping(); err != nil {
		t.Errorf("Database ping failed: %v", err)
	}

	// Test close
	if err := client.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

func TestClientMigrations(t *testing.T) {
	tempDir := t.TempDir()

	client, err := NewClient(tempDir)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Test embedded migrations
	if err := client.RunEmbeddedMigrations(); err != nil {
		t.Errorf("Failed to run embedded migrations: %v", err)
	}

	// Check that migration table exists
	db := client.DB()
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query migrations table: %v", err)
	}

	if count == 0 {
		t.Error("No migrations were applied")
	}

	// Check that our tables were created
	tables := []string{"calls", "config"}
	for _, table := range tables {
		var tableExists int
		query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		err = db.QueryRow(query, table).Scan(&tableExists)
		if err != nil {
			t.Errorf("Failed to check if table %s exists: %v", table, err)
		}
		if tableExists == 0 {
			t.Errorf("Table %s was not created", table)
		}
	}
}

func TestGetMigrator(t *testing.T) {
	tempDir := t.TempDir()

	client, err := NewClient(tempDir)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	migrator := client.GetMigrator()
	if migrator == nil {
		t.Error("Migrator is nil")
	}
}
