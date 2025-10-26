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
		{
			Version:     3,
			Name:        "refactor_calls_for_fsm",
			Description: "Refactor calls table to use UUID primary key and FSM-compatible structure",
			UpSQL: `-- Create new calls table with UUID primary key and FSM fields
CREATE TABLE IF NOT EXISTS calls_new (
    call_id TEXT PRIMARY KEY, -- UUID as text for better readability
    line INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('idle', 'ringing', 'calling', 'talking', 'missedCall', 'notReached', 'finished')),
    finish_state TEXT CHECK (finish_state IN ('missedCall', 'notReached', 'finished')),
    caller TEXT,
    called TEXT,
    caller_msn TEXT,
    called_msn TEXT,
    trunk TEXT,
    start_timestamp DATETIME,
    connect_timestamp DATETIME,
    end_timestamp DATETIME,
    duration INTEGER, -- Duration in seconds
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for the new table
CREATE INDEX IF NOT EXISTS idx_calls_new_line ON calls_new(line);
CREATE INDEX IF NOT EXISTS idx_calls_new_status ON calls_new(status);
CREATE INDEX IF NOT EXISTS idx_calls_new_start_timestamp ON calls_new(start_timestamp);
CREATE INDEX IF NOT EXISTS idx_calls_new_caller_msn ON calls_new(caller_msn);
CREATE INDEX IF NOT EXISTS idx_calls_new_called_msn ON calls_new(called_msn);

-- Drop old table and rename new one
DROP TABLE IF EXISTS calls;
ALTER TABLE calls_new RENAME TO calls;`,
			DownSQL: `-- Rollback: recreate old calls table structure
DROP TABLE IF EXISTS calls;
CREATE TABLE IF NOT EXISTS calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('incoming', 'outgoing', 'connect', 'disconnect')),
    caller TEXT,
    called TEXT,
    caller_msn TEXT,
    called_msn TEXT,
    line INTEGER,
    trunk TEXT,
    duration INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Recreate old indexes
CREATE INDEX IF NOT EXISTS idx_calls_timestamp ON calls(timestamp);
CREATE INDEX IF NOT EXISTS idx_calls_call_id ON calls(call_id);
CREATE INDEX IF NOT EXISTS idx_calls_event_type ON calls(event_type);
CREATE INDEX IF NOT EXISTS idx_calls_caller_msn ON calls(caller_msn);
CREATE INDEX IF NOT EXISTS idx_calls_called_msn ON calls(called_msn);`,
		},
		{
			Version:     4,
			Name:        "add_phone_numbers_table",
			Description: "Add phone_numbers table for storing names associated with phone numbers",
			UpSQL: `-- Table for storing phone number to name mappings
CREATE TABLE IF NOT EXISTS phone_numbers (
    phone_number TEXT PRIMARY KEY, -- Normalized phone number (e.g., +4961813698237)
    name TEXT DEFAULT NULL,        -- Display name for this number
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster name lookups (for reverse search)
CREATE INDEX IF NOT EXISTS idx_phone_numbers_name ON phone_numbers(name);`,
			DownSQL: `-- Remove phone_numbers table and its indexes
DROP INDEX IF EXISTS idx_phone_numbers_name;
DROP TABLE IF EXISTS phone_numbers;`,
		},
		{
			Version:     5,
			Name:        "add_caller_called_indexes",
			Description: "Add indexes for calls.caller and calls.called for faster phone number lookups",
			UpSQL: `-- Add indexes for faster phone number queries
CREATE INDEX IF NOT EXISTS idx_calls_caller ON calls(caller);
CREATE INDEX IF NOT EXISTS idx_calls_called ON calls(called);`,
			DownSQL: `-- Remove caller and called indexes
DROP INDEX IF EXISTS idx_calls_called;
DROP INDEX IF EXISTS idx_calls_caller;`,
		},
		{
			Version:     6,
			Name:        "add_extension_fields",
			Description: "Add extension fields to calls table to store caller and called extension information",
			UpSQL: `-- Add caller_extension and called_extension columns to calls table as JSON
-- JSON format: {"number": "610", "name": "Büro", "type": "DECT"}
ALTER TABLE calls ADD COLUMN caller_extension_json TEXT;
ALTER TABLE calls ADD COLUMN called_extension_json TEXT;

-- Index for faster queries by caller extension (extract number from JSON)
CREATE INDEX IF NOT EXISTS idx_calls_caller_ext_number ON calls(json_extract(caller_extension_json, '$.number'));

-- Index for faster queries by called extension (extract number from JSON)
CREATE INDEX IF NOT EXISTS idx_calls_called_ext_number ON calls(json_extract(called_extension_json, '$.number'));

-- Index for faster queries by extension type
CREATE INDEX IF NOT EXISTS idx_calls_caller_ext_type ON calls(json_extract(caller_extension_json, '$.type'));
CREATE INDEX IF NOT EXISTS idx_calls_called_ext_type ON calls(json_extract(called_extension_json, '$.type'));`,
			DownSQL: `-- Remove indexes
DROP INDEX IF EXISTS idx_calls_called_ext_type;
DROP INDEX IF EXISTS idx_calls_caller_ext_type;
DROP INDEX IF EXISTS idx_calls_called_ext_number;
DROP INDEX IF EXISTS idx_calls_caller_ext_number;

-- Note: SQLite doesn't support DROP COLUMN, so we can't easily remove the columns
-- In a real rollback scenario, you'd need to recreate the table without these columns`,
		},
		{
			Version:     7,
			Name:        "add_voicebox_status",
			Description: "Add voiceBox status to support voicemail detection",
			UpSQL: `-- In SQLite, we cannot modify CHECK constraints directly
-- We need to recreate the table with the new constraint

-- Create a temporary table with the updated constraint
CREATE TABLE calls_temp (
    call_id TEXT PRIMARY KEY, -- UUID as text for better readability
    line INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('idle', 'ringing', 'calling', 'talking', 'voiceBox', 'missedCall', 'notReached', 'finished')),
    finish_state TEXT CHECK (finish_state IN ('missedCall', 'notReached', 'finished')),
    caller TEXT,
    called TEXT,
    caller_msn TEXT,
    called_msn TEXT,
    trunk TEXT,
    start_timestamp DATETIME,
    connect_timestamp DATETIME,
    end_timestamp DATETIME,
    duration INTEGER, -- Duration in seconds
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    caller_extension_json TEXT,
    called_extension_json TEXT
);

-- Copy all existing data
INSERT INTO calls_temp SELECT * FROM calls;

-- Drop the old table
DROP TABLE calls;

-- Rename the new table
ALTER TABLE calls_temp RENAME TO calls;

-- Recreate all indexes
CREATE INDEX idx_calls_new_line ON calls(line);
CREATE INDEX idx_calls_new_status ON calls(status);
CREATE INDEX idx_calls_new_start_timestamp ON calls(start_timestamp);
CREATE INDEX idx_calls_new_caller_msn ON calls(caller_msn);
CREATE INDEX idx_calls_new_called_msn ON calls(called_msn);
CREATE INDEX idx_calls_caller ON calls(caller);
CREATE INDEX idx_calls_called ON calls(called);
CREATE INDEX idx_calls_caller_ext_number ON calls(json_extract(caller_extension_json, '$.number'));
CREATE INDEX idx_calls_called_ext_number ON calls(json_extract(called_extension_json, '$.number'));
CREATE INDEX idx_calls_caller_ext_type ON calls(json_extract(caller_extension_json, '$.type'));
CREATE INDEX idx_calls_called_ext_type ON calls(json_extract(called_extension_json, '$.type'));`,
			DownSQL: `-- Rollback by recreating table without voiceBox status
-- This is a complex rollback that would need table recreation`,
		},
		{
			Version:     8,
			Name:        "add_voicebox_finish_state",
			Description: "Add voiceBox as valid finish_state for VOICEBOX calls",
			UpSQL: `-- Create new table with updated constraint for both status and finish_state
CREATE TABLE IF NOT EXISTS calls_new (
    call_id TEXT PRIMARY KEY,
    line INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('idle', 'ringing', 'calling', 'talking', 'voiceBox', 'missedCall', 'notReached', 'finished')),
    finish_state TEXT CHECK (finish_state IN ('missedCall', 'notReached', 'finished', 'voiceBox')),
    caller TEXT,
    called TEXT,
    caller_msn TEXT,
    called_msn TEXT,
    trunk TEXT,
    start_timestamp DATETIME,
    connect_timestamp DATETIME,
    end_timestamp DATETIME,
    duration INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    caller_extension_json TEXT,
    called_extension_json TEXT
);

-- Copy data from old table
INSERT INTO calls_new SELECT * FROM calls;

-- Drop the old table
DROP TABLE calls;

-- Rename the new table
ALTER TABLE calls_new RENAME TO calls;

-- Recreate all indexes
CREATE INDEX idx_calls_new_line ON calls(line);
CREATE INDEX idx_calls_new_status ON calls(status);
CREATE INDEX idx_calls_new_start_timestamp ON calls(start_timestamp);
CREATE INDEX idx_calls_new_caller_msn ON calls(caller_msn);
CREATE INDEX idx_calls_new_called_msn ON calls(called_msn);
CREATE INDEX idx_calls_caller ON calls(caller);
CREATE INDEX idx_calls_called ON calls(called);
CREATE INDEX idx_calls_caller_ext_number ON calls(json_extract(caller_extension_json, '$.number'));
CREATE INDEX idx_calls_called_ext_number ON calls(json_extract(called_extension_json, '$.number'));
CREATE INDEX idx_calls_caller_ext_type ON calls(json_extract(caller_extension_json, '$.type'));
CREATE INDEX idx_calls_called_ext_type ON calls(json_extract(called_extension_json, '$.type'));`,
			DownSQL: `-- Rollback by recreating table without voiceBox in finish_state
-- This is a complex rollback that would need table recreation`,
		},
		{
			Version:     9,
			Name:        "update_messagebox_to_voicebox",
			Description: "Update existing messageBox status values to voiceBox for consistency",
			UpSQL: `-- Update any existing messageBox status to voiceBox
UPDATE calls SET status = 'voiceBox' WHERE status = 'messageBox';`,
			DownSQL: `-- Rollback: Update voiceBox back to messageBox
UPDATE calls SET status = 'messageBox' WHERE status = 'voiceBox';`,
		},
		{
			Version:     10,
			Name:        "add_events_table",
			Description: "Add events table for storing raw callmonitor events",
			UpSQL: `-- Table for storing raw callmonitor events
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY, -- UUID v7 as text
    timestamp DATETIME NOT NULL,
    raw_value TEXT NOT NULL, -- Raw event string from callmonitor
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster queries by timestamp
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

-- Index for faster queries by created_at
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);`,
			DownSQL: `-- Remove events table and its indexes
DROP INDEX IF EXISTS idx_events_created_at;
DROP INDEX IF EXISTS idx_events_timestamp;
DROP TABLE IF EXISTS events;`,
		},
		{
			Version:     11,
			Name:        "add_internal_call_fields",
			Description: "Add fields to track internal calls between MSNs",
			UpSQL: `-- Add internal call tracking columns
ALTER TABLE calls ADD COLUMN is_internal_call BOOLEAN DEFAULT FALSE NOT NULL;
ALTER TABLE calls ADD COLUMN linked_call_id TEXT;
ALTER TABLE calls ADD COLUMN internal_call_role TEXT; -- 'caller' or 'callee'

-- Create index for efficient linking queries
CREATE INDEX IF NOT EXISTS idx_calls_linked_call_id ON calls(linked_call_id) 
WHERE linked_call_id IS NOT NULL;

-- Create index for finding internal call pairs
CREATE INDEX IF NOT EXISTS idx_calls_msn_timestamp ON calls(caller_msn, called_msn, start_timestamp)
WHERE caller_msn IS NOT NULL AND called_msn IS NOT NULL;`,
			DownSQL: `-- Remove internal call tracking
DROP INDEX IF EXISTS idx_calls_msn_timestamp;
DROP INDEX IF EXISTS idx_calls_linked_call_id;
-- Note: SQLite doesn't support DROP COLUMN, would need table recreation`,
		},
	}
}
