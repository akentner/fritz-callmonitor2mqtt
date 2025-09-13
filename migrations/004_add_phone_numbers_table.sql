-- Description: Add phone numbers table for contact management
-- Add phone_numbers table for storing names associated with phone numbers
-- This enables contact name resolution for caller and called numbers

-- +migrate Up

-- Table for storing phone number to name mappings
CREATE TABLE IF NOT EXISTS phone_numbers (
    phone_number TEXT PRIMARY KEY, -- Normalized phone number (e.g., +4961813698237)
    name TEXT DEFAULT NULL,        -- Display name for this number
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster name lookups (for reverse search)
CREATE INDEX IF NOT EXISTS idx_phone_numbers_name ON phone_numbers(name);

-- +migrate Down

-- Remove phone_numbers table and its indexes
DROP INDEX IF EXISTS idx_phone_numbers_name;
DROP TABLE IF EXISTS phone_numbers;
