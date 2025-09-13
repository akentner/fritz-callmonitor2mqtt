# Database Schema Migrations

## Overview

The fritz-callmonitor2mqtt application uses SQLite for data persistence. The database is automatically created and maintained using a versioned migration system.

## Database Location

- **Data Directory**: Configurable via `FRITZ_CALLMONITOR_DATABASE_DATA_DIR` environment variable (default: `./data`)
- **Database File**: `{data_dir}/database/fritz-callmonitor.db`

## Schema Versioning

The application uses a migration system to manage database schema changes:

- Migration files are embedded in the application binary
- Each migration has a version number, name, and SQL statements
- Migrations are tracked in the `schema_migrations` table
- Only new migrations are applied on startup

## Current Schema (Version 2)

### Tables

#### `calls`
Stores call events from the Fritz!Box callmonitor:

- `id` - Primary key
- `call_id` - Fritz!Box call identifier
- `timestamp` - When the call event occurred
- `event_type` - Type of event (incoming, outgoing, connect, disconnect)
- `caller` - Caller phone number
- `called` - Called phone number
- `caller_msn` - MSN if caller number ends with configured MSN *(Version 2+)*
- `called_msn` - MSN if called number ends with configured MSN *(Version 2+)*
- `line` - Fritz!Box line number
- `trunk` - Network trunk information
- `duration` - Call duration in seconds (for connect/disconnect events)
- `created_at` - Record creation timestamp
- `updated_at` - Record update timestamp

#### `config`
Stores application configuration:

- `id` - Primary key
- `key` - Configuration key (unique)
- `value` - Configuration value
- `created_at` - Record creation timestamp
- `updated_at` - Record update timestamp

#### `schema_migrations`
Tracks applied database migrations:

- `version` - Migration version (primary key)
- `name` - Migration name
- `applied_at` - When migration was applied
- `description` - Migration description

## Adding New Migrations

To add a new database migration:

1. Create a new migration in `internal/database/migrations.go`
2. Add it to the `GetEmbeddedMigrations()` function
3. Increment the version number
4. Provide both UP and DOWN SQL statements
5. Test thoroughly before deployment

Example:
```go
{
    Version:     2,
    Name:        "add_user_table",
    Description: "Add user management table",
    UpSQL: `CREATE TABLE users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT UNIQUE NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );`,
    DownSQL: `DROP TABLE IF EXISTS users;`,
},
```

## Database Features

- **WAL Mode**: Enabled for better concurrency
- **Foreign Keys**: Enabled for data integrity
- **Indexes**: Optimized for common query patterns
- **Automatic Backups**: SQLite creates automatic WAL backups

## Maintenance

### Manual Database Access

You can access the database directly using sqlite3:

```bash
sqlite3 ./data/database/fritz-callmonitor.db

# View current schema version
SELECT * FROM schema_migrations;

# Query recent calls
SELECT * FROM calls ORDER BY timestamp DESC LIMIT 10;
```

### Database Cleanup

The database will grow over time. Consider implementing periodic cleanup of old call records based on your retention requirements.

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `FRITZ_CALLMONITOR_DATABASE_DATA_DIR` | `./data` | Directory for all data files |

## Troubleshooting

### Database Connection Issues

- Ensure the data directory is writable
- Check disk space availability
- Verify SQLite version compatibility

### Migration Failures

- Check application logs for detailed error messages
- Verify migration SQL syntax
- Ensure no concurrent access during migration

### Performance Issues

- Monitor database size and consider cleanup
- Check if WAL mode is enabled
- Review query patterns and indexes
