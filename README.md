# {{PROJECT_NAME}}

{{PROJECT_DESCRIPTION}}

## Features

- ✅ Modern Go development setup
- ✅ Docker Dev Container
- ✅ Automated testing and linting
- ✅ Cross-platform builds
- ✅ Version information embedding
- ✅ Comprehensive Makefile

## Quick Start

### Prerequisites

- Docker
- Visual Studio Code with Remote-Containers extension

### Development Setup

1. Clone this repository
```bash
git clone <repository-url>
cd {{PROJECT_NAME}}
```

2. Open in VS Code
```bash
code .
```

3. Reopen in Container when prompted

4. Start developing!

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
{{PROJECT_NAME}}/
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

{{CONFIGURATION_INSTRUCTIONS}}

## Usage

```bash
# Show help
./{{PROJECT_NAME}} -help

# Show version
./{{PROJECT_NAME}} -version

# Run application
./{{PROJECT_NAME}}
```

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

{{LICENSE_INFO}}

## Author

{{AUTHOR_INFO}}
