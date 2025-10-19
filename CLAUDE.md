# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

fritz-callmonitor2mqtt is a Go service that monitors FRITZ!Box callmonitor events (via TCP port 1012) and publishes them to MQTT topics. It provides real-time call tracking, contact management, call history, and Home Assistant integration.

## Common Commands

### Development & Testing
```bash
make dev              # Run without building binary
make run              # Build and run
make build            # Build for current platform

make test             # Run all unit tests
make test-unit        # Run unit tests only (excludes integration)
make test-integration # Run integration tests only
make test-coverage    # Generate coverage report

make bench            # Run benchmarks
```

### Code Quality
```bash
make lint             # Run linter (golangci-lint)
make lint-fix         # Auto-fix linting issues
make fmt              # Format code with gofmt and goimports
make pre-commit       # Run all pre-commit checks (fmt, lint-all, test-unit)
```

### Database
The application uses SQLite with embedded migrations. Database location: `./data/fritz-callmonitor.db`
- Migrations run automatically on startup
- Manual access: `sqlite3 ./data/fritz-callmonitor.db`

### Running Single Tests
```bash
# Run a specific test
go test -v ./pkg/types/ -run TestStateMachine

# Run tests in a specific package
go test -v ./internal/callmonitor/

# Run with race detection
go test -race ./...
```

## Architecture Overview

### Package Structure
```
internal/              # Private application packages
├── callmonitor/      # FRITZ!Box TCP connection & event parsing
├── config/           # Configuration loading & validation
├── database/         # SQLite client & migrations
└── mqtt/             # MQTT client & publishing logic

pkg/types/            # Shared types and core business logic
├── call.go           # Call event types
├── fsm.go            # Finite State Machine for call states
├── line_state_machine.go  # Multi-line FSM management
└── call_manager.go   # High-level call management with FSM
```

### Core Components

**1. Finite State Machine (FSM)**
- Location: `pkg/types/fsm.go`, `line_state_machine.go`, `call_manager.go`
- Purpose: Manages call state transitions with strict validation
- State flow: `idle` → `ringing`/`calling` → `talking` → `finished` → `idle` (1s timeout)
- Thread-safe with mutex protection and automatic timeout management
- See `docs/FSM.md` for detailed state diagram

**2. Callmonitor Client**
- Location: `internal/callmonitor/client.go`
- Connects to FRITZ!Box TCP port 1012 for raw call events
- Parses events: RING, CALL, CONNECT, DISCONNECT
- Handles phone number normalization (country code, local area code)
- Supports extension mapping and MSN detection

**3. MQTT Client**
- Location: `internal/mqtt/client.go`
- Publishes to topics: `{prefix}/line/{id}/status`, `{prefix}/history`, `{prefix}/events/{type}`
- Implements birth/last will messages for availability tracking
- Handles RPC-style phone number management on `{prefix}/phone_number/request|response`
- Supports Home Assistant MQTT Discovery

**4. Database Layer**
- Location: `internal/database/client.go`, `migrations.go`
- SQLite with WAL mode for concurrency
- Versioned migrations with automatic execution
- Stores: call events, phone numbers, configuration
- Call persistence with UUID v7 for chronological tracking

**5. Call Manager**
- Location: `pkg/types/call_manager.go`
- Integrates FSM, MQTT publishing, and database persistence
- Processes events through validation → FSM → publish → persist pipeline
- Manages multiple phone lines concurrently

### Data Flow

1. **FRITZ!Box Event** → Callmonitor Client parses raw TCP data
2. **CallEvent** → Enriched with extensions, MSN, phone number lookups
3. **Call Manager** → Processes through FSM for state validation
4. **MQTT Publish** → Events published to appropriate topics
5. **Database** → Events persisted for history and recovery

### Configuration

Configuration is loaded from environment variables with prefix `FRITZ_CALLMONITOR_`:
- `FRITZBOX_HOST` / `FRITZBOX_PORT` - FRITZ!Box connection
- `MQTT_BROKER` / `MQTT_PORT` / `MQTT_USERNAME` / `MQTT_PASSWORD` - MQTT broker
- `PBX_MSN` - Multiple Subscriber Numbers (comma-separated)
- `PBX_EXTENSIONS` - Extension mapping in JSON format
- `APP_LOG_LEVEL` - Logging level (debug, info, warn, error)
- `DATABASE_DATA_DIR` - Database directory (default: ./data)

See `internal/config/config.go` for complete configuration structure.

### Extension Support

Extensions (Nebenstellen) are configured via environment variables and mapped to types:
- `DECT` - Cordless phones
- `VOIP` - VoIP devices
- `ANALOG` - Analog phones
- `VOICEBOX` - Answering machine (special handling in FSM with `voiceBox` state)
- `UNKNOWN` - Unrecognized extensions

Extension info is enriched in events via `CallerExtension` and `CalledExtension` fields.

### Phone Number Normalization

- Raw numbers are normalized using country code and local area code
- MSN detection: checks if normalized number ends with configured MSN
- Lookup: phone number → contact name via database RPC interface
- Format: E.164 international format preferred (+49...)

## Testing Conventions

- Unit tests alongside source files: `*_test.go`
- Integration tests in: `test/integration/`
- Use table-driven tests for multiple scenarios
- Mock MQTT/database for unit tests; real components for integration tests
- FSM has comprehensive test coverage including timeout and concurrency tests

## Development Notes

### Adding New Call States
1. Add state constant to `pkg/types/call.go` (CallStatus enum)
2. Update FSM transition map in `pkg/types/fsm.go`
3. Update state diagram in `docs/FSM.md`
4. Add tests for new transitions in `fsm_test.go`

### Adding Database Migrations
1. Create migration in `internal/database/migrations.go`
2. Increment version number
3. Provide UpSQL and DownSQL
4. Test with both fresh DB and existing DB upgrade

### Home Assistant Integration
- MQTT Discovery support for sensors and binary sensors
- Device registration with manufacturer/model info
- Special enum sensor support for call status (idle/ringing/calling/talking)
- See `examples/home-assistant/` for configuration examples

### Logging
- Uses structured logging with `log/slog`
- Log levels: debug, info, warn, error
- Format: human-readable text with simplified timestamps
- Set via `FRITZ_CALLMONITOR_APP_LOG_LEVEL`

### Error Handling
- Automatic reconnection for FRITZ!Box and MQTT disconnects
- Parse errors logged but don't crash application
- Failed MQTT publishes are logged and retried on next connection
- Database errors are fatal on startup, logged during runtime

## Important Implementation Details

### UUID v7 for Call IDs
- Each call receives a UUID v7 identifier that persists across all state transitions
- Time-based sorting capability for chronological ordering
- Enables correlation of complete call lifecycles across ring/call → connect → disconnect

### Thread Safety
- All FSM components use `sync.RWMutex` for safe concurrent access
- MQTT client handles concurrent publishing
- Database uses connection pooling (single connection with WAL mode)

### Timeout Management
- Final FSM states (finished, notReached, missedCall) auto-transition to idle after 1 second
- Timers are properly cleaned up on FSM reset or shutdown
- Configurable reconnection delay for connection failures (default: 10s)

### MQTT Retained Messages
- Line status and history are retained for client reconnection recovery
- Birth/LWT messages track service availability
- Bootstrap from database on startup restores last known state

## Building & Distribution

```bash
make build-all          # Build for Linux, Windows, macOS (amd64/arm64)
make release-snapshot   # Create snapshot release with goreleaser
make install            # Install to $GOPATH/bin
```

Cross-compilation uses CGO_ENABLED=0 for static binaries.
