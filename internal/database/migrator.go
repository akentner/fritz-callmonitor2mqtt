package database

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Name        string
	UpSQL       string
	DownSQL     string
	Description string
}

// Migrator handles database migrations
type Migrator struct {
	db             *sql.DB
	migrationsPath string
	migrations     []Migration
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB, migrationsPath string) *Migrator {
	return &Migrator{
		db:             db,
		migrationsPath: migrationsPath,
		migrations:     []Migration{},
	}
}

// LoadEmbeddedMigrations loads the embedded migrations
func (m *Migrator) LoadEmbeddedMigrations() error {
	m.migrations = GetEmbeddedMigrations()

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	return nil
}

// LoadMigrationsFromFS loads migrations from an embedded filesystem
func (m *Migrator) LoadMigrationsFromFS(fs embed.FS, path string) error {
	entries, err := fs.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		migration, err := m.parseMigrationFile(fs, filepath.Join(path, filename))
		if err != nil {
			return fmt.Errorf("failed to parse migration %s: %w", filename, err)
		}

		m.migrations = append(m.migrations, migration)
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	return nil
}

// parseMigrationFile parses a migration file
func (m *Migrator) parseMigrationFile(fs embed.FS, path string) (Migration, error) {
	content, err := fs.ReadFile(path)
	if err != nil {
		return Migration{}, fmt.Errorf("failed to read migration file: %w", err)
	}

	filename := filepath.Base(path)
	// Parse filename: 001_create_users_table.sql
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 {
		return Migration{}, fmt.Errorf("invalid migration filename format: %s", filename)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return Migration{}, fmt.Errorf("invalid version number in filename: %s", filename)
	}

	name := strings.TrimSuffix(parts[1], ".sql")

	// Parse SQL content
	sqlContent := string(content)
	upSQL, downSQL, description := m.parseSQLContent(sqlContent)

	return Migration{
		Version:     version,
		Name:        name,
		UpSQL:       upSQL,
		DownSQL:     downSQL,
		Description: description,
	}, nil
}

// parseSQLContent parses SQL content and extracts UP, DOWN, and description sections
func (m *Migrator) parseSQLContent(content string) (upSQL, downSQL, description string) {
	lines := strings.Split(content, "\n")

	var currentSection string
	var upLines, downLines, descLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for section markers
		if strings.HasPrefix(trimmedLine, "-- Description:") {
			currentSection = "description"
			description = strings.TrimPrefix(trimmedLine, "-- Description:")
			description = strings.TrimSpace(description)
			continue
		} else if trimmedLine == "-- +migrate Up" {
			currentSection = "up"
			continue
		} else if trimmedLine == "-- +migrate Down" {
			currentSection = "down"
			continue
		}

		// Add lines to appropriate section
		switch currentSection {
		case "description":
			if strings.HasPrefix(trimmedLine, "--") {
				descLine := strings.TrimPrefix(trimmedLine, "--")
				descLine = strings.TrimSpace(descLine)
				if descLine != "" {
					descLines = append(descLines, descLine)
				}
			}
		case "up":
			upLines = append(upLines, line)
		case "down":
			downLines = append(downLines, line)
		}
	}

	upSQL = strings.TrimSpace(strings.Join(upLines, "\n"))
	downSQL = strings.TrimSpace(strings.Join(downLines, "\n"))

	if len(descLines) > 0 {
		description = strings.Join(descLines, " ")
	}

	return upSQL, downSQL, description
}

// InitSchema initializes the migration tracking table
func (m *Migrator) InitSchema() error {
	createSchemaSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		);
	`

	if _, err := m.db.Exec(createSchemaSQL); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	return nil
}

// GetCurrentVersion returns the current database schema version
func (m *Migrator) GetCurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current schema version: %w", err)
	}
	return version, nil
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
	if err := m.InitSchema(); err != nil {
		return err
	}

	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return err
	}

	// Find migrations to apply
	var pendingMigrations []Migration
	for _, migration := range m.migrations {
		if migration.Version > currentVersion {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	if len(pendingMigrations) == 0 {
		return nil // No pending migrations
	}

	// Apply migrations in transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, migration := range pendingMigrations {
		if err := m.applyMigration(tx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}

	return nil
}

// applyMigration applies a single migration
func (m *Migrator) applyMigration(tx *sql.Tx, migration Migration) error {
	// Execute the UP SQL
	if migration.UpSQL != "" {
		if _, err := tx.Exec(migration.UpSQL); err != nil {
			return fmt.Errorf("failed to execute migration SQL: %w", err)
		}
	}

	// Record the migration
	insertSQL := `
		INSERT INTO schema_migrations (version, name, description, applied_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`
	if _, err := tx.Exec(insertSQL, migration.Version, migration.Name, migration.Description); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

// GetAppliedMigrations returns all applied migrations
func (m *Migrator) GetAppliedMigrations() ([]Migration, error) {
	query := `
		SELECT version, name, description
		FROM schema_migrations
		ORDER BY version
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var migration Migration
		err := rows.Scan(&migration.Version, &migration.Name, &migration.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration row: %w", err)
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}
