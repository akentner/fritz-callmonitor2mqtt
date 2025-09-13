-- Description: Initial schema setup
-- Create the initial database schema for fritz-callmonitor2mqtt
-- This includes basic tables for call logging and system configuration

-- +migrate Up

-- Table for storing call events
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
CREATE INDEX IF NOT EXISTS idx_config_key ON config(key);

-- +migrate Down

DROP INDEX IF EXISTS idx_config_key;
DROP TABLE IF EXISTS config;
DROP INDEX IF EXISTS idx_calls_event_type;
DROP INDEX IF EXISTS idx_calls_call_id;
DROP INDEX IF EXISTS idx_calls_timestamp;
DROP TABLE IF EXISTS calls;
