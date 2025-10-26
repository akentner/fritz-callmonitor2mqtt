-- Migration 005: Add internal call tracking fields
-- Adds fields to detect and link internal calls between MSNs

-- Add internal call tracking columns
ALTER TABLE calls ADD COLUMN is_internal_call BOOLEAN DEFAULT FALSE NOT NULL;
ALTER TABLE calls ADD COLUMN linked_call_id TEXT;
ALTER TABLE calls ADD COLUMN internal_call_role TEXT; -- 'caller' or 'callee'

-- Create index for efficient linking queries
CREATE INDEX IF NOT EXISTS idx_calls_linked_call_id ON calls(linked_call_id) 
WHERE linked_call_id IS NOT NULL;

-- Create index for finding internal call pairs during migration
CREATE INDEX IF NOT EXISTS idx_calls_msn_timestamp ON calls(caller_msn, called_msn, timestamp)
WHERE caller_msn IS NOT NULL AND called_msn IS NOT NULL;
