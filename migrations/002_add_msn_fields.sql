-- Description: Add MSN fields to calls table
-- Add caller_msn and called_msn columns for MSN detection based on phone number endings
-- This allows tracking which MSN (Multiple Subscriber Number) was involved in each call

-- +migrate Up

-- Add caller_msn and called_msn columns to calls table
ALTER TABLE calls ADD COLUMN caller_msn TEXT;
ALTER TABLE calls ADD COLUMN called_msn TEXT;

-- Index for faster queries by caller_msn
CREATE INDEX IF NOT EXISTS idx_calls_caller_msn ON calls(caller_msn);

-- Index for faster queries by called_msn
CREATE INDEX IF NOT EXISTS idx_calls_called_msn ON calls(called_msn);

-- +migrate Down

-- Remove indexes
DROP INDEX IF EXISTS idx_calls_called_msn;
DROP INDEX IF EXISTS idx_calls_caller_msn;

-- Note: SQLite doesn't support DROP COLUMN, so we can't easily remove the columns
-- In a real rollback scenario, you'd need to recreate the table without these columns
