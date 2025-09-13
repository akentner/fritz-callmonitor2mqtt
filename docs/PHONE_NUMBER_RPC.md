# Phone Number RPC API

## Overview

The fritz-callmonitor2mqtt service provides a RPC-style MQTT interface for managing phone number to name mappings. This enables automatic caller/called name resolution in call events and provides a complete contact management system.

## MQTT Topics

### Request Topic
```
{prefix}/phone_number/request
```

### Response Topic  
```
{prefix}/phone_number/response
```

## RPC Request Format

All requests must be valid JSON with the following structure:

```json
{
  "id": "unique-request-id",
  "method": "get|set|delete|list|search",
  "phone_number": "string (optional)",
  "name": "string (optional)",
  "limit": 100,
  "pattern": "string (optional)",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### Request Fields

- **id** (required): Unique identifier for request/response correlation
- **method** (required): Operation to perform (`get`, `set`, `delete`, `list`, `search`)
- **phone_number** (optional): Target phone number for get/set/delete operations
- **name** (optional): Contact name for set operations
- **limit** (optional): Maximum results for list/search operations (default: 100)
- **pattern** (optional): Search pattern for search operations
- **timestamp** (required): Request timestamp in ISO 8601 format

## RPC Response Format

All responses use the following JSON structure:

```json
{
  "id": "matching-request-id",
  "success": true,
  "error": "string (only if success=false)",
  "phone_number": {
    "phone_number": "string",
    "name": "string",
    "created_at": "2025-09-13T12:30:00Z",
    "updated_at": "2025-09-13T12:30:00Z"
  },
  "phone_numbers": [
    {
      "phone_number": "string", 
      "name": "string",
      "created_at": "2025-09-13T12:30:00Z",
      "updated_at": "2025-09-13T12:30:00Z"
    }
  ],
  "count": 42,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### Response Fields

- **id**: Matches the request ID for correlation
- **success**: Boolean indicating operation success
- **error**: Error message (only present if success=false)
- **phone_number**: Single phone number result (for get operations)
- **phone_numbers**: Array of phone numbers (for list/search operations)
- **count**: Total number of results
- **timestamp**: Response timestamp

## RPC Methods

### 1. Set Phone Number

Associates a name with a phone number (creates or updates).

**Request:**
```json
{
  "id": "req-001",
  "method": "set",
  "phone_number": "+1234567890",
  "name": "John Doe",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

**Response:**
```json
{
  "id": "req-001",
  "success": true,
  "phone_number": {
    "phone_number": "+1234567890",
    "name": "John Doe",
    "created_at": "2025-09-13T12:30:00Z",
    "updated_at": "2025-09-13T12:30:00Z"
  },
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### 2. Get Phone Number

Retrieves information for a specific phone number.

**Request:**
```json
{
  "id": "req-002",
  "method": "get",
  "phone_number": "+1234567890",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

**Response (found):**
```json
{
  "id": "req-002",
  "success": true,
  "phone_number": {
    "phone_number": "+1234567890",
    "name": "John Doe",
    "created_at": "2025-09-13T12:30:00Z",
    "updated_at": "2025-09-13T12:30:00Z"
  },
  "timestamp": "2025-09-13T12:30:00Z"
}
```

**Response (not found):**
```json
{
  "id": "req-002",
  "success": false,
  "error": "phone number not found",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### 3. Delete Phone Number

Removes a phone number and its associated name.

**Request:**
```json
{
  "id": "req-003",
  "method": "delete",
  "phone_number": "+1234567890",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

**Response:**
```json
{
  "id": "req-003",
  "success": true,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### 4. List All Phone Numbers

Retrieves all stored phone numbers with optional limit.

**Request:**
```json
{
  "id": "req-004",
  "method": "list",
  "limit": 50,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

**Response:**
```json
{
  "id": "req-004",
  "success": true,
  "phone_numbers": [
    {
      "phone_number": "+1234567890",
      "name": "John Doe",
      "created_at": "2025-09-13T12:30:00Z",
      "updated_at": "2025-09-13T12:30:00Z"
    },
    {
      "phone_number": "+0987654321",
      "name": "Jane Smith",
      "created_at": "2025-09-13T12:29:00Z",
      "updated_at": "2025-09-13T12:29:00Z"
    }
  ],
  "count": 2,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### 5. Search Phone Numbers

Searches for phone numbers by name pattern (case-insensitive).

**Request:**
```json
{
  "id": "req-005",
  "method": "search",
  "pattern": "John",
  "limit": 10,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

**Response:**
```json
{
  "id": "req-005",
  "success": true,
  "phone_numbers": [
    {
      "phone_number": "+1234567890",
      "name": "John Doe",
      "created_at": "2025-09-13T12:30:00Z",
      "updated_at": "2025-09-13T12:30:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

## Error Handling

All errors are returned with `success: false` and an `error` field:

```json
{
  "id": "req-001",
  "success": false,
  "error": "Invalid phone number format",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### Common Error Messages

- `"Invalid method: <method>"` - Unsupported RPC method
- `"Missing phone number"` - Required phone_number field missing
- `"Missing name for set operation"` - Name required for set method
- `"Missing pattern for search operation"` - Pattern required for search method
- `"Phone number not found"` - Get/delete operation on non-existent number
- `"Invalid JSON: <details>"` - Malformed request JSON

## Integration Examples

See the [examples directory](../examples/) for complete integration examples:

- `examples/phone-rpc-client.py` - Python MQTT client
- `examples/phone-rpc-client.js` - Node.js MQTT client  
- `examples/phone-rpc-curl.sh` - curl-based HTTP-to-MQTT bridge examples
- `examples/home-assistant/` - Home Assistant integration examples

## Database Schema

The phone number data is stored in SQLite with the following schema:

```sql
CREATE TABLE phone_numbers (
    phone_number TEXT PRIMARY KEY,
    name TEXT DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_phone_numbers_name ON phone_numbers(name);
```

## Performance Considerations

- **Name Resolution**: Optimized lookup queries for real-time call processing
- **Concurrent Access**: Thread-safe database operations
- **Caching**: Consider implementing client-side caching for frequently accessed numbers
- **Limits**: Default limit of 100 results for list/search operations to prevent memory issues
