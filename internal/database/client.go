package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // SQLite driver

	"fritz-callmonitor2mqtt/pkg/types"
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

// Call represents a persisted call record
type Call struct {
	CallID           uuid.UUID         `db:"call_id"`
	Line             int               `db:"line"`
	Status           types.CallStatus  `db:"status"`
	FinishState      *types.CallStatus `db:"finish_state"`
	Caller           *string           `db:"caller"`
	Called           *string           `db:"called"`
	CallerMSN        *string           `db:"caller_msn"`
	CalledMSN        *string           `db:"called_msn"`
	Trunk            *string           `db:"trunk"`
	StartTimestamp   *time.Time        `db:"start_timestamp"`
	ConnectTimestamp *time.Time        `db:"connect_timestamp"`
	EndTimestamp     *time.Time        `db:"end_timestamp"`
	Duration         *int              `db:"duration"`
	CreatedAt        time.Time         `db:"created_at"`
	UpdatedAt        time.Time         `db:"updated_at"`
}

// InsertCall inserts a new call record when transitioning from idle to ringing/calling
func (c *Client) InsertCall(callID uuid.UUID, line int, status types.CallStatus, event *types.CallEvent) error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Convert UUID to string for storage
	callIDString := callID.String()

	var caller, called, callerMSN, calledMSN, trunk *string
	var startTimestamp *time.Time

	if event != nil {
		if event.Caller != "" {
			caller = &event.Caller
		}
		if event.Called != "" {
			called = &event.Called
		}
		if event.CallerMSN != "" {
			callerMSN = &event.CallerMSN
		}
		if event.CalledMSN != "" {
			calledMSN = &event.CalledMSN
		}
		if event.Trunk != "" {
			trunk = &event.Trunk
		}
		if !event.Timestamp.IsZero() {
			startTimestamp = &event.Timestamp
		}
	}

	query := `INSERT INTO calls (
		call_id, line, status, caller, called, caller_msn, called_msn, trunk, 
		start_timestamp, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err := c.db.Exec(query, callIDString, line, string(status), caller, called,
		callerMSN, calledMSN, trunk, startTimestamp)
	if err != nil {
		return fmt.Errorf("failed to insert call: %w", err)
	}

	return nil
}

// UpdateCall updates an existing call record for status transitions
func (c *Client) UpdateCall(callID uuid.UUID, status types.CallStatus, finishState *types.CallStatus, event *types.CallEvent) error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Convert UUID to string for lookup
	callIDString := callID.String()

	// Build dynamic update query based on status
	var setClauses []string
	var args []interface{}

	// Always update status and updated_at
	setClauses = append(setClauses, "status = ?", "updated_at = CURRENT_TIMESTAMP")
	args = append(args, string(status))

	// Update finish_state if provided
	if finishState != nil {
		setClauses = append(setClauses, "finish_state = ?")
		args = append(args, string(*finishState))
	}

	// Handle status-specific updates
	if event != nil {
		switch status {
		case types.CallStatusTalking:
			// Set connect timestamp
			setClauses = append(setClauses, "connect_timestamp = ?")
			args = append(args, event.Timestamp)
		case types.CallStatusFinished, types.CallStatusMissedCall, types.CallStatusNotReached:
			// Set end timestamp and calculate duration
			setClauses = append(setClauses, "end_timestamp = ?")
			args = append(args, event.Timestamp)

			if event.Duration > 0 {
				setClauses = append(setClauses, "duration = ?")
				args = append(args, event.Duration)
			}
		}
	}

	// Construct and execute query
	setClause := ""
	for i, clause := range setClauses {
		if i > 0 {
			setClause += ", "
		}
		setClause += clause
	}
	query := fmt.Sprintf("UPDATE calls SET %s WHERE call_id = ?", setClause)

	args = append(args, callIDString)

	result, err := c.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update call: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("call not found: %s", callID.String())
	}

	return nil
}

// GetCall retrieves a call record by UUID
func (c *Client) GetCall(callID uuid.UUID) (*Call, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	callIDString := callID.String()

	query := `SELECT call_id, line, status, finish_state, caller, called, caller_msn, 
		called_msn, trunk, start_timestamp, connect_timestamp, end_timestamp, 
		duration, created_at, updated_at FROM calls WHERE call_id = ?`

	row := c.db.QueryRow(query, callIDString)

	var call Call
	var callIDStr string
	var finishStateStr *string

	err := row.Scan(&callIDStr, &call.Line, (*string)(&call.Status), &finishStateStr,
		&call.Caller, &call.Called, &call.CallerMSN, &call.CalledMSN, &call.Trunk,
		&call.StartTimestamp, &call.ConnectTimestamp, &call.EndTimestamp,
		&call.Duration, &call.CreatedAt, &call.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("call not found: %s", callID.String())
		}
		return nil, fmt.Errorf("failed to scan call: %w", err)
	}

	// Convert string back to UUID
	parsedUUID, err := uuid.Parse(callIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse UUID: %w", err)
	}
	call.CallID = parsedUUID

	// Convert finish state string to CallStatus
	if finishStateStr != nil {
		finishState := types.CallStatus(*finishStateStr)
		call.FinishState = &finishState
	}

	return &call, nil
}

// GetCallsByLine retrieves all calls for a specific line, ordered by start timestamp
func (c *Client) GetCallsByLine(line int, limit int) ([]Call, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT call_id, line, status, finish_state, caller, called, caller_msn, 
		called_msn, trunk, start_timestamp, connect_timestamp, end_timestamp, 
		duration, created_at, updated_at FROM calls 
		WHERE line = ? ORDER BY start_timestamp DESC LIMIT ?`

	rows, err := c.db.Query(query, line, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls: %w", err)
	}
	defer rows.Close()

	var calls []Call
	for rows.Next() {
		var call Call
		var callIDStr string
		var finishStateStr *string

		err := rows.Scan(&callIDStr, &call.Line, (*string)(&call.Status), &finishStateStr,
			&call.Caller, &call.Called, &call.CallerMSN, &call.CalledMSN, &call.Trunk,
			&call.StartTimestamp, &call.ConnectTimestamp, &call.EndTimestamp,
			&call.Duration, &call.CreatedAt, &call.UpdatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}

		// Convert string back to UUID
		parsedUUID, err := uuid.Parse(callIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse UUID: %w", err)
		}
		call.CallID = parsedUUID

		// Convert finish state string to CallStatus
		if finishStateStr != nil {
			finishState := types.CallStatus(*finishStateStr)
			call.FinishState = &finishState
		}

		calls = append(calls, call)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}

	return calls, nil
}
