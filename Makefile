# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=fritz-callmonitor2mqtt
BINARY_UNIX=$(BINARY_NAME)_unix

# Build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build test lint fmt clean run deps help install

# Default target
all: test build

# Build the application
build:
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) -v .

# Build and run
run:
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) -v .
	./bin/$(BINARY_NAME)

# Run directly without building binary
dev:
	$(GOCMD) run .

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Cross-platform builds
build-all: clean
	./scripts/build-cross-platform.sh $(VERSION)

# Build for Linux AMD64
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .

# Build for Windows AMD64  
build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .

# Build for macOS AMD64
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .

# Build for ARM64 (Apple Silicon)
build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .

# Build for Linux ARM64 (Raspberry Pi 4)
build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 .

# Build for Linux ARM (Raspberry Pi 3)
build-linux-arm:
	GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-armv7 .

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	$(GOCMD) fmt ./...
	goimports -w . 2>/dev/null || true

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Install tools
tools:
	$(GOGET) golang.org/x/tools/cmd/goimports@latest
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Build for multiple platforms
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .

# Install binary to GOPATH/bin
install:
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) .

# Development targets
dev-config:
	@echo "Loading development configuration..."
	@if [ -f dev.env ]; then \
		echo "✅ dev.env found"; \
	else \
		echo "❌ dev.env not found - please create it first"; \
		exit 1; \
	fi

dev-run: build dev-config
	@echo "🚀 Starting development server..."
	@bash -c "source dev.env && ./bin/$(BINARY_NAME)"

dev-test-config: dev-config
	@bash -c "source dev.env && ./bin/$(BINARY_NAME) -config-test"

dev-mqtt-test: dev-config
	@echo "🧪 Testing MQTT connection..."
	@bash -c 'source dev.env && echo "Testing MQTT broker at $$FRITZ_CALLMONITOR_MQTT_BROKER:$$FRITZ_CALLMONITOR_MQTT_PORT"'
	@bash -c 'source dev.env && mosquitto_pub -h $$FRITZ_CALLMONITOR_MQTT_BROKER -p $$FRITZ_CALLMONITOR_MQTT_PORT -t "$$FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX/test" -m "Hello from fritz-callmonitor2mqtt dev environment" && echo "✅ MQTT test message sent successfully"' || echo "❌ MQTT connection failed - check if broker is running at 192.168.178.3:1883"

dev-mqtt-listen: dev-config
	@echo "👂 Listening to MQTT topics (Ctrl+C to stop)..."
	@bash -c 'source dev.env && echo "Subscribing to: $$FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX/#"'
	@bash -c 'source dev.env && mosquitto_sub -h $$FRITZ_CALLMONITOR_MQTT_BROKER -p $$FRITZ_CALLMONITOR_MQTT_PORT -t "$$FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX/#" -v' || echo "❌ MQTT connection failed - check if broker is running at 192.168.178.3:1883"

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Building & Running:"
	@echo "  build          Build the binary"
	@echo "  run            Build and run the application"
	@echo "  dev            Run without building binary"
	@echo ""
	@echo "Development:"
	@echo "  dev-run        Build and run with development config"
	@echo "  dev-test-config Test development configuration"
	@echo "  dev-mqtt-test   Test MQTT connection"
	@echo "  dev-mqtt-listen Listen to MQTT topics"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  bench          Run benchmarks"
	@echo "  lint           Run linter"
	@echo "  fmt            Format code"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          Clean build artifacts"
	@echo "  deps           Download dependencies"
	@echo "  tools          Install development tools"
	@echo "  build-all      Build for multiple platforms"
	@echo "  install        Install binary to GOPATH/bin"
	@echo "  help           Show this help"
