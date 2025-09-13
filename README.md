# fritz-callmonitor2mqtt

Fritz!Box Callmonitor to MQTT Bridge

Alexander Kentner <github@akentner.de>

A Go backend service that monitors the Fritz!Box callmonitor interface (TCP port 1012) and publishes call events to MQTT topics.

## Features

- **Real-time Call Monitoring**: Connects to Fritz!Box callmonitor interface
- **MQTT Integration**: Publishes call events to MQTT broker with configurable topics
- **Line Status Tracking**: Maintains current status for each phone line (idle/ring/active)
- **Phone Number Name Mapping**: Store and retrieve contact names for phone numbers via RPC-style MQTT interface
- **Call History**: Keeps track of the last 50 calls in JSON format with caller/called names
- **SQLite Database**: Persistent storage of call events with versioned migrations
- **MSN Detection**: Automatically detects Multiple Subscriber Numbers (MSNs) in phone calls
- **RPC-Style MQTT API**: Manage phone number contacts via MQTT request/response topics
- **Automatic Reconnection**: Robust connection handling with automatic reconnection
- **Environment-based Configuration**: Configure via environment variables
- **Lightweight**: Single binary, minimal dependencies


## MQTT Topics

The service publishes to the following MQTT topics (with configurable prefix):

### Call Event Topics
- `{prefix}/status` - Service availability with Birth/Last Will (retained)
- `{prefix}/line/{line_id}/status` - Current status of each phone line (retained)
- `{prefix}/line/{line_id}/last_event` - Last event for each line (retained)
- `{prefix}/history` - Last 50 calls as JSON array (retained) 
- `{prefix}/events/{call_type}` - Individual call events by type:
  - `ring` - Incoming call started
  - `call` - Outgoing call started  
  - `connect` - Call connected/answered
  - `disconnect` - Call ended

### Phone Number Management (RPC-Style)
- `{prefix}/phone_number/request` - Submit phone number management requests
- `{prefix}/phone_number/response` - Receive responses for phone number operations

### Call Tracking with UUID v7
Each call receives a unique UUID v7 identifier that:
- **Persists across all call states**: Same ID for ring/call → connect → disconnect
- **Time-based sorting**: UUID v7 contains timestamp, enabling chronological sorting
- **Correlation**: Enables tracking complete call lifecycles in monitoring systems
- **Example ID**: `01933e88-a140-7d2c-b0a8-123456789abc`

### Service Availability
The service implements MQTT Birth and Last Will Testament:
- **Birth Message**: `{"state":"online", "last_changed":"2025-09-09T10:30:45Z"}` on connect
- **Last Will**: `{"state":"offline", "last_changed":"2025-09-09T10:30:45Z"}` on unexpected disconnect  
- **Graceful Shutdown**: Explicit offline message before clean disconnect

### Phone Number Name Mapping
The service includes a contact management system that allows associating phone numbers with names:
- **Automatic Name Resolution**: Call events automatically include caller/called names when available
- **RPC-Style MQTT Interface**: Manage contacts via MQTT request/response pattern
- **Persistent Storage**: Contact data stored in SQLite database
- **Search Capabilities**: Find contacts by name pattern or phone number

**Enhanced Call Events with Names:**
```json
{
  "id": "01933e88-a140-7d2c-b0a8-123456789abc",
  "caller": "+1234567890",
  "called": "+0987654321",
  "caller_name": "John Doe",
  "called_name": "Jane Smith",
  "type": "call",
  "direction": "outbound"
}
```

## Quick Start

### Prerequisites

- Fritz!Box router with callmonitor enabled
- MQTT broker (e.g., Mosquitto, HiveMQ)
- Go 1.21+ (for building from source)


### Enable Fritz!Box Callmonitor

First, enable the callmonitor on your Fritz!Box by dialing:
```
#96*5*
```

This activates the TCP interface on port 1012.

### Binary Installation

Download the latest binary from the releases page and run:
```bash
./fritz-callmonitor2mqtt
```

### Docker (TODO)
```bash
docker run -d \
  --name fritz-callmonitor2mqtt \
  -e FRITZ_CALLMONITOR_FRITZBOX_HOST=fritz.box \
  -e FRITZ_CALLMONITOR_MQTT_BROKER=mqtt.example.com \
  akentner/fritz-callmonitor2mqtt
```

### Development Setup

1. Clone this repository
```bash
git clone https://github.com/akentner/fritz-callmonitor2mqtt.git
cd fritz-callmonitor2mqtt
```

2. Build and run
```bash
make build
./bin/fritz-callmonitor2mqtt
```

Or run directly:
```bash
make run
```

### Available Commands

```bash
# Development
make dev             # Run without building
make run             # Build and run
make build           # Build binary

# Testing
make test            # Run tests
make test-coverage   # Run tests with coverage
make bench           # Run benchmarks

# Code Quality
make lint            # Run linter
make fmt             # Format code

# Maintenance
make clean           # Clean build artifacts
make deps            # Update dependencies
make tools           # Install dev tools

# Distribution
make build-all       # Build for multiple platforms
make install         # Install to GOPATH/bin
```

## Project Structure

```
fritz-callmonitor2mqtt/
├── .devcontainer/       # Dev Container configuration
├── bin/                 # Compiled binaries (generated)
├── main.go              # Application entry point
├── main_test.go         # Tests
├── go.mod               # Go module definition
├── Makefile             # Build automation
├── .golangci.yml       # Linting configuration
├── .gitignore          # Git ignore rules
├── README.md           # This file
└── STRUCTURE.md        # Project structure guide
```

## Configuration

Configure the application using environment variables:

### Fritz!Box Settings
- `FRITZ_CALLMONITOR_FRITZBOX_HOST` - Fritz!Box hostname (default: `fritz.box`)
- `FRITZ_CALLMONITOR_FRITZBOX_PORT` - Callmonitor port (default: `1012`)

### MQTT Settings  
- `FRITZ_CALLMONITOR_MQTT_BROKER` - MQTT broker hostname (default: `localhost`)
- `FRITZ_CALLMONITOR_MQTT_PORT` - MQTT broker port (default: `1883`)
- `FRITZ_CALLMONITOR_MQTT_USERNAME` - MQTT username (optional)
- `FRITZ_CALLMONITOR_MQTT_PASSWORD` - MQTT password (optional)
- `FRITZ_CALLMONITOR_MQTT_CLIENT_ID` - MQTT client ID (default: `fritz-callmonitor2mqtt`)
- `FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX` - Topic prefix (default: `fritz/callmonitor`)
- `FRITZ_CALLMONITOR_MQTT_QOS` - QoS level (default: `1`)
- `FRITZ_CALLMONITOR_MQTT_RETAIN` - Retain messages (default: `true`)

### Application Settings
- `FRITZ_CALLMONITOR_APP_LOG_LEVEL` - Log level (default: `info`)
- `FRITZ_CALLMONITOR_APP_CALL_HISTORY_SIZE` - Number of calls to keep (default: `50`)
- `FRITZ_CALLMONITOR_APP_RECONNECT_DELAY` - Reconnection delay (default: `10s`)
- `FRITZ_CALLMONITOR_APP_HEALTH_CHECK_PORT` - Health check port (default: `8080`)
- `FRITZ_CALLMONITOR_APP_TIMEZONE` - Timezone for timestamp parsing (default: `Europe/Berlin`)

## Usage

```bash
# Show help
./fritz-callmonitor2mqtt -help

# Show version
./fritz-callmonitor2mqtt -version

# Run application with default settings
./fritz-callmonitor2mqtt

# Run with custom timezone (e.g. for US East Coast)
FRITZ_CALLMONITOR_APP_TIMEZONE=America/New_York ./fritz-callmonitor2mqtt

# Run with multiple custom settings
FRITZ_CALLMONITOR_FRITZBOX_HOST=192.168.1.1 \
FRITZ_CALLMONITOR_MQTT_BROKER=mqtt.home.lan \
FRITZ_CALLMONITOR_APP_TIMEZONE=Europe/Vienna \
./fritz-callmonitor2mqtt
```

## Phone Number Management

The service includes a complete contact management system accessible via RPC-style MQTT interface:

### Quick Examples

```bash
# Python client
python3 examples/phone-rpc-client.py set "+1234567890" "John Doe"
python3 examples/phone-rpc-client.py get "+1234567890"
python3 examples/phone-rpc-client.py list

# Node.js client
node examples/phone-rpc-client.js set "+1234567890" "John Doe"
node examples/phone-rpc-client.js search "John"

# Shell script (using mosquitto tools)
./examples/phone-rpc-shell.sh set "+1234567890" "John Doe"
./examples/phone-rpc-shell.sh monitor  # Watch all RPC responses
```

### Documentation

- **[Phone Number RPC API](docs/PHONE_NUMBER_RPC.md)** - Complete API documentation
- **[Examples](examples/README.md)** - Client implementations and usage examples
- **[Home Assistant Integration](examples/home-assistant/)** - Ready-to-use HA configuration

## Development

### Adding Dependencies

```bash
go get github.com/package/name
make deps
```

### Running Tests

```bash
make test                # Run all tests
make test-coverage       # Generate coverage report
```

### Building

```bash
make build              # Build for current platform
make build-all          # Build for all platforms
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
6. Submit a pull request

## License

MIT License - see LICENSE file

## Author

Alexander Kentner <github@akentner.de>
