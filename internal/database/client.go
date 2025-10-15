package database

import (
	"database/sql"
	"embed"
	"encoding/json"
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
		_ = c.db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := c.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = c.db.Close()
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := c.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = c.db.Close()
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
	CallID           uuid.UUID            `db:"call_id"`
	Line             int                  `db:"line"`
	Status           types.CallStatus     `db:"status"`
	FinishState      *types.CallStatus    `db:"finish_state"`
	Caller           *string              `db:"caller"`
	Called           *string              `db:"called"`
	CallerMSN        *string              `db:"caller_msn"`
	CalledMSN        *string              `db:"called_msn"`
	CallerExtension  *types.ExtensionInfo `db:"caller_extension_json"`
	CalledExtension  *types.ExtensionInfo `db:"called_extension_json"`
	Trunk            *string              `db:"trunk"`
	StartTimestamp   *time.Time           `db:"start_timestamp"`
	ConnectTimestamp *time.Time           `db:"connect_timestamp"`
	EndTimestamp     *time.Time           `db:"end_timestamp"`
	Duration         *int                 `db:"duration"`
	CreatedAt        time.Time            `db:"created_at"`
	UpdatedAt        time.Time            `db:"updated_at"`
}

// InsertCall inserts a new call record when transitioning from idle to ringing/calling
func (c *Client) InsertCall(callID uuid.UUID, line int, status types.CallStatus, event *types.CallEvent) error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Convert UUID to string for storage
	callIDString := callID.String()

	var caller, called, callerMSN, calledMSN, trunk *string
	var callerExtensionJSON, calledExtensionJSON *string
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

		// Serialize extension information to JSON
		if event.CallerExtension != nil {
			if jsonData, err := json.Marshal(event.CallerExtension); err == nil {
				jsonStr := string(jsonData)
				callerExtensionJSON = &jsonStr
			}
		}
		if event.CalledExtension != nil {
			if jsonData, err := json.Marshal(event.CalledExtension); err == nil {
				jsonStr := string(jsonData)
				calledExtensionJSON = &jsonStr
			}
		}
	}

	query := `INSERT INTO calls (
		call_id, line, status, caller, called, caller_msn, called_msn, 
		caller_extension_json, called_extension_json, trunk, 
		start_timestamp, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err := c.db.Exec(query, callIDString, line, string(status), caller, called,
		callerMSN, calledMSN, callerExtensionJSON, calledExtensionJSON, trunk, startTimestamp)
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
		// Handle extension updates when available (typically on CONNECT events)
		var callerExtensionJSON *string
		var calledExtensionJSON *string

		if event.CallerExtension != nil {
			if jsonData, err := json.Marshal(event.CallerExtension); err == nil {
				jsonStr := string(jsonData)
				callerExtensionJSON = &jsonStr
			}
		}
		if event.CalledExtension != nil {
			if jsonData, err := json.Marshal(event.CalledExtension); err == nil {
				jsonStr := string(jsonData)
				calledExtensionJSON = &jsonStr
			}
		}

		// Update extensions if they exist
		if callerExtensionJSON != nil {
			setClauses = append(setClauses, "caller_extension_json = ?")
			args = append(args, *callerExtensionJSON)
		}
		if calledExtensionJSON != nil {
			setClauses = append(setClauses, "called_extension_json = ?")
			args = append(args, *calledExtensionJSON)
		}

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
		called_msn, caller_extension_json, called_extension_json, trunk, start_timestamp, connect_timestamp, end_timestamp, 
		duration, created_at, updated_at FROM calls WHERE call_id = ?`

	row := c.db.QueryRow(query, callIDString)

	var call Call
	var callIDStr string
	var finishStateStr *string
	var callerExtensionJSON, calledExtensionJSON *string

	err := row.Scan(&callIDStr, &call.Line, (*string)(&call.Status), &finishStateStr,
		&call.Caller, &call.Called, &call.CallerMSN, &call.CalledMSN,
		&callerExtensionJSON, &calledExtensionJSON, &call.Trunk,
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

	// Deserialize extension JSON data
	if callerExtensionJSON != nil && *callerExtensionJSON != "" {
		var callerExt types.ExtensionInfo
		if err := json.Unmarshal([]byte(*callerExtensionJSON), &callerExt); err == nil {
			call.CallerExtension = &callerExt
		}
	}
	if calledExtensionJSON != nil && *calledExtensionJSON != "" {
		var calledExt types.ExtensionInfo
		if err := json.Unmarshal([]byte(*calledExtensionJSON), &calledExt); err == nil {
			call.CalledExtension = &calledExt
		}
	}

	return &call, nil
}

// GetCallsByLine retrieves all calls for a specific line, ordered by start timestamp
func (c *Client) GetCallsByLine(line int, limit int) ([]Call, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT call_id, line, status, finish_state, caller, called, caller_msn, 
		called_msn, caller_extension_json, called_extension_json, trunk, start_timestamp, connect_timestamp, end_timestamp, 
		duration, created_at, updated_at FROM calls 
		WHERE line = ? ORDER BY start_timestamp DESC LIMIT ?`

	rows, err := c.db.Query(query, line, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var calls []Call
	for rows.Next() {
		var call Call
		var callIDStr string
		var finishStateStr *string
		var callerExtensionJSON, calledExtensionJSON *string

		err := rows.Scan(&callIDStr, &call.Line, (*string)(&call.Status), &finishStateStr,
			&call.Caller, &call.Called, &call.CallerMSN, &call.CalledMSN,
			&callerExtensionJSON, &calledExtensionJSON, &call.Trunk,
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

		// Deserialize extension JSON data
		if callerExtensionJSON != nil && *callerExtensionJSON != "" {
			var callerExt types.ExtensionInfo
			if err := json.Unmarshal([]byte(*callerExtensionJSON), &callerExt); err == nil {
				call.CallerExtension = &callerExt
			}
		}
		if calledExtensionJSON != nil && *calledExtensionJSON != "" {
			var calledExt types.ExtensionInfo
			if err := json.Unmarshal([]byte(*calledExtensionJSON), &calledExt); err == nil {
				call.CalledExtension = &calledExt
			}
		}

		calls = append(calls, call)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}

	return calls, nil
}

// GetCallsByMSN retrieves all completed calls for a specific MSN, ordered by end timestamp DESC
func (c *Client) GetCallsByMSN(msn string, limit int) ([]Call, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	// Only get completed calls that involve this MSN
	// Completed calls have end_timestamp AND finish_state (finished, missedCall, notReached)
	query := `SELECT call_id, line, status, finish_state, caller, called, caller_msn, 
		called_msn, caller_extension_json, called_extension_json, trunk, start_timestamp, connect_timestamp, end_timestamp, 
		duration, created_at, updated_at FROM calls 
		WHERE (caller_msn = ? OR called_msn = ?) AND end_timestamp IS NOT NULL AND finish_state IS NOT NULL
		ORDER BY end_timestamp DESC LIMIT ?`

	rows, err := c.db.Query(query, msn, msn, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls by MSN: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var calls []Call
	for rows.Next() {
		var call Call
		var callIDStr string
		var finishStateStr *string
		var callerExtensionJSON, calledExtensionJSON *string

		err := rows.Scan(&callIDStr, &call.Line, (*string)(&call.Status), &finishStateStr,
			&call.Caller, &call.Called, &call.CallerMSN, &call.CalledMSN,
			&callerExtensionJSON, &calledExtensionJSON, &call.Trunk,
			&call.StartTimestamp, &call.ConnectTimestamp, &call.EndTimestamp,
			&call.Duration, &call.CreatedAt, &call.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}

		// Parse call ID
		callID, err := uuid.Parse(callIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse call ID: %w", err)
		}
		call.CallID = callID

		// Parse finish state if present
		if finishStateStr != nil {
			finishState := types.CallStatus(*finishStateStr)
			call.FinishState = &finishState
		}

		// Deserialize extension JSON data
		if callerExtensionJSON != nil && *callerExtensionJSON != "" {
			var callerExt types.ExtensionInfo
			if err := json.Unmarshal([]byte(*callerExtensionJSON), &callerExt); err == nil {
				call.CallerExtension = &callerExt
			}
		}
		if calledExtensionJSON != nil && *calledExtensionJSON != "" {
			var calledExt types.ExtensionInfo
			if err := json.Unmarshal([]byte(*calledExtensionJSON), &calledExt); err == nil {
				call.CalledExtension = &calledExt
			}
		}

		calls = append(calls, call)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls by MSN: %w", err)
	}

	return calls, nil
}

// PhoneNumber represents a phone number with an associated name
type PhoneNumber struct {
	PhoneNumber string    `db:"phone_number"`
	Name        *string   `db:"name"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// InsertPhoneNumber inserts or updates a phone number with its associated name
func (c *Client) InsertPhoneNumber(phoneNumber, name string) error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	query := `INSERT INTO phone_numbers (phone_number, name, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(phone_number) 
		DO UPDATE SET name = excluded.name, updated_at = CURRENT_TIMESTAMP`

	var namePtr *string
	if name != "" {
		namePtr = &name
	}

	_, err := c.db.Exec(query, phoneNumber, namePtr)
	if err != nil {
		return fmt.Errorf("failed to insert/update phone number: %w", err)
	}

	return nil
}

// UpdatePhoneNumber updates the name for an existing phone number
func (c *Client) UpdatePhoneNumber(phoneNumber, name string) error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	var namePtr *string
	if name != "" {
		namePtr = &name
	}

	query := `UPDATE phone_numbers SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE phone_number = ?`

	result, err := c.db.Exec(query, namePtr, phoneNumber)
	if err != nil {
		return fmt.Errorf("failed to update phone number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("phone number not found: %s", phoneNumber)
	}

	return nil
}

// GetPhoneNumber retrieves a phone number and its associated name
func (c *Client) GetPhoneNumber(phoneNumber string) (*PhoneNumber, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT phone_number, name, created_at, updated_at FROM phone_numbers WHERE phone_number = ?`

	row := c.db.QueryRow(query, phoneNumber)

	var pn PhoneNumber
	err := row.Scan(&pn.PhoneNumber, &pn.Name, &pn.CreatedAt, &pn.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("phone number not found: %s", phoneNumber)
		}
		return nil, fmt.Errorf("failed to scan phone number: %w", err)
	}

	return &pn, nil
}

// GetPhoneNumberName retrieves only the name for a phone number (optimized lookup)
func (c *Client) GetPhoneNumberName(phoneNumber string) (string, error) {
	if c.db == nil {
		return "", fmt.Errorf("database not connected")
	}

	query := `SELECT name FROM phone_numbers WHERE phone_number = ?`

	var name *string
	err := c.db.QueryRow(query, phoneNumber).Scan(&name)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No name found, return empty string
		}
		return "", fmt.Errorf("failed to get phone number name: %w", err)
	}

	if name == nil {
		return "", nil
	}

	return *name, nil
}

// GetAllPhoneNumbers retrieves all phone numbers with their names, ordered by phone number
func (c *Client) GetAllPhoneNumbers(limit int) ([]PhoneNumber, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT phone_number, name, created_at, updated_at FROM phone_numbers 
		ORDER BY phone_number ASC LIMIT ?`

	rows, err := c.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query phone numbers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var phoneNumbers []PhoneNumber
	for rows.Next() {
		var pn PhoneNumber
		err := rows.Scan(&pn.PhoneNumber, &pn.Name, &pn.CreatedAt, &pn.UpdatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan phone number: %w", err)
		}

		phoneNumbers = append(phoneNumbers, pn)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating phone numbers: %w", err)
	}

	return phoneNumbers, nil
}

// DeletePhoneNumber removes a phone number and its associated name
func (c *Client) DeletePhoneNumber(phoneNumber string) error {
	if c.db == nil {
		return fmt.Errorf("database not connected")
	}

	query := `DELETE FROM phone_numbers WHERE phone_number = ?`

	result, err := c.db.Exec(query, phoneNumber)
	if err != nil {
		return fmt.Errorf("failed to delete phone number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("phone number not found: %s", phoneNumber)
	}

	return nil
}

// SearchPhoneNumbersByName searches for phone numbers by name (partial match)
func (c *Client) SearchPhoneNumbersByName(namePattern string, limit int) ([]PhoneNumber, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT phone_number, name, created_at, updated_at FROM phone_numbers 
		WHERE name LIKE ? ORDER BY name ASC LIMIT ?`

	pattern := "%" + namePattern + "%"
	rows, err := c.db.Query(query, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search phone numbers by name: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var phoneNumbers []PhoneNumber
	for rows.Next() {
		var pn PhoneNumber
		err := rows.Scan(&pn.PhoneNumber, &pn.Name, &pn.CreatedAt, &pn.UpdatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan phone number: %w", err)
		}

		phoneNumbers = append(phoneNumbers, pn)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating phone numbers: %w", err)
	}

	return phoneNumbers, nil
}
