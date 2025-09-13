package database

// GetEmbeddedMigrations returns the built-in migrations as a slice
func GetEmbeddedMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Name:        "initial_schema",
			Description: "Initial schema setup - Create the initial database schema for fritz-callmonitor2mqtt",
			UpSQL: `-- Table for storing call events
CREATE TABLE IF NOT EXISTS calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('incoming', 'outgoing', 'connect', 'disconnect')),
    caller TEXT,
    called TEXT,
    line INTEGER,
    trunk TEXT,
    duration INTEGER, -- Duration in seconds (for connect/disconnect events)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster queries by timestamp
CREATE INDEX IF NOT EXISTS idx_calls_timestamp ON calls(timestamp);

-- Index for faster queries by call_id
CREATE INDEX IF NOT EXISTS idx_calls_call_id ON calls(call_id);

-- Index for faster queries by event_type
CREATE INDEX IF NOT EXISTS idx_calls_event_type ON calls(event_type);

-- Table for storing application configuration
CREATE TABLE IF NOT EXISTS config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster config lookups
CREATE INDEX IF NOT EXISTS idx_config_key ON config(key);`,
			DownSQL: `DROP INDEX IF EXISTS idx_config_key;
DROP TABLE IF EXISTS config;
DROP INDEX IF EXISTS idx_calls_event_type;
DROP INDEX IF EXISTS idx_calls_call_id;
DROP INDEX IF EXISTS idx_calls_timestamp;
DROP TABLE IF EXISTS calls;`,
		},
		{
			Version:     2,
			Name:        "add_msn_fields",
			Description: "Add MSN fields to calls table for caller and called party MSN detection",
			UpSQL: `-- Add caller_msn and called_msn columns to calls table
ALTER TABLE calls ADD COLUMN caller_msn TEXT;
ALTER TABLE calls ADD COLUMN called_msn TEXT;

-- Index for faster queries by caller_msn
CREATE INDEX IF NOT EXISTS idx_calls_caller_msn ON calls(caller_msn);

-- Index for faster queries by called_msn
CREATE INDEX IF NOT EXISTS idx_calls_called_msn ON calls(called_msn);`,
			DownSQL: `-- Remove indexes
DROP INDEX IF EXISTS idx_calls_called_msn;
DROP INDEX IF EXISTS idx_calls_caller_msn;

-- Note: SQLite doesn't support DROP COLUMN, so we can't easily remove the columns
-- In a real rollback scenario, you'd need to recreate the table without these columns`,
		},
	}
}
