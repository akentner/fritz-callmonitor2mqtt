package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // SQLite driver
)

// Client represents a database client with migration support
type Client struct {
	db           *sql.DB
	dataDir      string
	databasePath string
	migrator     *Migrator
}

// NewClient creates a new database client
func NewClient(dataDir string) (*Client, error) {
	// Ensure the data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}

	// Ensure the database subdirectory exists
	databaseDir := filepath.Join(dataDir, "database")
	if err := os.MkdirAll(databaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", databaseDir, err)
	}

	// Database file path
	databasePath := filepath.Join(databaseDir, "fritz-callmonitor.db")

	return &Client{
		dataDir:      dataDir,
		databasePath: databasePath,
	}, nil
}

// Connect opens a connection to the SQLite database
func (c *Client) Connect() error {
	var err error
	c.db, err = sql.Open("sqlite", c.databasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := c.db.Ping(); err != nil {
		c.db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := c.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		c.db.Close()
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := c.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		c.db.Close()
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Initialize migrator
	c.migrator = NewMigrator(c.db, "")

	return nil
}

// Close closes the database connection
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// DB returns the underlying database connection
func (c *Client) DB() *sql.DB {
	return c.db
}

// GetDatabasePath returns the path to the database file
func (c *Client) GetDatabasePath() string {
	return c.databasePath
}

// GetDataDir returns the data directory path
func (c *Client) GetDataDir() string {
	return c.dataDir
}

// GetMigrator returns the migrator instance
func (c *Client) GetMigrator() *Migrator {
	return c.migrator
}

// RunMigrations loads and runs migrations from embedded filesystem
func (c *Client) RunMigrations(fs embed.FS, migrationsPath string) error {
	if c.migrator == nil {
		return fmt.Errorf("migrator not initialized")
	}

	if err := c.migrator.LoadMigrationsFromFS(fs, migrationsPath); err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	if err := c.migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RunEmbeddedMigrations loads and runs the built-in migrations
func (c *Client) RunEmbeddedMigrations() error {
	if c.migrator == nil {
		return fmt.Errorf("migrator not initialized")
	}

	if err := c.migrator.LoadEmbeddedMigrations(); err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	if err := c.migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
