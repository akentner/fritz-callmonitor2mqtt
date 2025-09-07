# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME={{PROJECT_NAME}}
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

# Show help
help:
	@echo "Available targets:"
	@echo "  build        Build the binary"
	@echo "  run          Build and run the application"
	@echo "  dev          Run without building binary"
	@echo "  test         Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  bench        Run benchmarks"
	@echo "  lint         Run linter"
	@echo "  fmt          Format code"
	@echo "  clean        Clean build artifacts"
	@echo "  deps         Download dependencies"
	@echo "  tools        Install development tools"
	@echo "  build-all    Build for multiple platforms"
	@echo "  install      Install binary to GOPATH/bin"
	@echo "  help         Show this help"
