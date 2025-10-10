# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=fritz-callmonitor2mqtt
BINARY_UNIX=$(BINARY_NAME)_unix

# Tool paths (auto-detect)
GOLANGCI_LINT=$(shell which golangci-lint 2>/dev/null || echo "$(HOME)/go/bin/golangci-lint")

# Debug target to show detected tool paths
debug-tools:
	@echo "🔧 Detected tool paths:"
	@echo "  GOLANGCI_LINT: $(GOLANGCI_LINT)"
	@echo "  Exists: $$(test -f '$(GOLANGCI_LINT)' && echo '✅ yes' || echo '❌ no')"

# Build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build test test-unit test-integration test-all lint lint-fix lint-yaml lint-actions lint-all lint-fix-all fix-errcheck pre-commit init status debug-tools fmt clean clean-all run deps deps-update deps-check deps-clean tools tools-venv cache-info help install build-all build-cross-platform release-check release-snapshot release-dy-run

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

# Run tests excluding integration tests
test-unit:
	@echo "🧪 Running unit tests (excluding integration tests)..."
	$(GOTEST) -v ./internal/... ./pkg/... .

# Run integration tests
test-integration:
	@echo "🧪 Running integration tests..."
	@cd test/integration && $(GOTEST) -v .

# Run all tests including integration
test-all: test test-integration

# Run tests with coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Cross-platform builds using script
build-cross-platform: clean
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
	@echo "🔍 Running linter..."
	@if [ -f "$(GOLANGCI_LINT)" ]; then \
		$(GOLANGCI_LINT) run; \
	else \
		echo "❌ golangci-lint not found. Run 'make tools' to install."; \
		exit 1; \
	fi

# Run linter with fixes
lint-fix:
	@echo "🔧 Running linter with auto-fixes..."
	@if [ -f "$(GOLANGCI_LINT)" ]; then \
		$(GOLANGCI_LINT) run --fix; \
	else \
		echo "❌ golangci-lint not found. Run 'make tools' to install."; \
		exit 1; \
	fi

# Lint YAML files
lint-yaml:
	@echo "🔍 Linting YAML files..."
	@if [ -f ".venv/bin/yamllint" ]; then \
		.venv/bin/yamllint .; \
	elif which yamllint > /dev/null; then \
		yamllint .; \
	else \
		echo "❌ yamllint not found. Run: make tools or make tools-venv"; \
		exit 1; \
	fi

# Lint GitHub Actions workflows
lint-actions:
	@echo "🔍 Linting GitHub Actions workflows..."
	@if which actionlint > /dev/null 2>&1; then \
		actionlint; \
	else \
		echo "⚠️  actionlint not available - skipping GitHub Actions linting"; \
		echo "💡 Manual validation: Check workflows in .github/workflows/ for syntax errors"; \
	fi

# Run all linting
lint-all: lint lint-yaml lint-actions
	@echo "✅ All linting completed"

# Initialize development environment
init: tools
	@echo "🚀 Initializing development environment..."
	@echo ""
	@echo "📦 Installing pre-commit hook..."
	@if [ -d ".git" ]; then \
		ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit && \
		chmod +x .git/hooks/pre-commit && \
		echo "✅ Pre-commit hook installed"; \
	else \
		echo "⚠️  Not a git repository, skipping pre-commit hook"; \
	fi
	@echo ""
	@echo "🔧 Running initial checks..."
	@$(MAKE) --no-print-directory fmt || echo "⚠️  Code formatting issues found - run 'make fmt'"
	@echo ""
	@echo "📊 Environment status:"
	@echo -n "Go version: " && go version 2>/dev/null || echo "Go not found"
	@echo -n "golangci-lint: " && (which golangci-lint > /dev/null && echo "✅ installed") || echo "❌ not found (will install with tools)"
	@echo -n "yamllint: " && (which yamllint > /dev/null && echo "✅ installed") || echo "❌ not found (will install with tools)"
	@echo ""
	@echo "🎉 Development environment initialized!"
	@echo ""
	@echo "🚀 Quick start:"
	@echo "  make help          - Show all available targets"
	@echo "  make pre-commit    - Run pre-commit checks manually"
	@echo "  make lint-fix-all  - Auto-fix common linting issues"
	@echo "  make test          - Run tests"
	@echo "  make build         - Build the application"
	@echo ""

# Quick development environment check
status:
	@echo "📊 Development Environment Status:"
	@echo ""
	@echo "🔧 Tools:"
	@echo -n "  Go: " && go version 2>/dev/null || echo "❌ not found"
	@echo -n "  golangci-lint: " && (which golangci-lint > /dev/null || [ -f ~/go/bin/golangci-lint ]) && echo "✅ installed" || echo "❌ not found"
	@echo -n "  yamllint: " && (which yamllint > /dev/null && echo "✅ installed") || echo "❌ not found"
	@echo -n "  actionlint: " && (which actionlint > /dev/null && echo "✅ installed") || echo "⚠️  not available (optional)"
	@echo ""
	@echo "🪝 Git Hooks:"
	@if [ -f ".git/hooks/pre-commit" ]; then \
		echo "  pre-commit: ✅ installed"; \
	else \
		echo "  pre-commit: ❌ not installed (run 'make init')"; \
	fi
	@echo ""
	@$(MAKE) --no-print-directory cache-info

# Pre-commit checks
pre-commit: fmt lint-all test-unit
	@echo "🔍 Running pre-commit checks..."
	@echo "✅ Pre-commit checks passed - ready to commit!"

# Fix common linting issues automatically
lint-fix-all: fmt lint-fix fix-errcheck
	@echo "🔧 Auto-fixing common linting issues..."
	@echo "✅ Auto-fixes applied"

# Fix errcheck issues (ignore errors in defer/cleanup)
fix-errcheck:
	@echo "🔧 Fixing errcheck issues..."
	@# Add blank identifier to ignore errors in defer statements where appropriate
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./.venv/*" | \
		xargs grep -l "defer.*Close()" | \
		xargs sed -i 's/defer \([^(]*Close()\)/defer func() { _ = \1 }()/g' || true
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./.venv/*" | \
		xargs grep -l "defer.*RemoveAll" | \
		xargs sed -i 's/defer os\.RemoveAll(\([^)]*\))/defer func() { _ = os.RemoveAll(\1) }()/g' || true
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./.venv/*" | \
		xargs grep -l "defer.*Rollback" | \
		xargs sed -i 's/defer \([^(]*Rollback()\)/defer func() { _ = \1 }()/g' || true
	@echo "✅ errcheck fixes applied"

# Format code
fmt:
	$(GOCMD) fmt ./...
	goimports -w . 2>/dev/null || true

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

# Clean everything including dependency cache
clean-all: clean deps-clean
	@echo "🧹 Cleaned build artifacts and dependency cache"

# Show cache information
cache-info:
	@echo "📊 Build Cache Information:"
	@echo ""
	@echo "Go Module Cache:"
	@go env GOMODCACHE
	@echo ""
	@echo "Go Build Cache:"
	@go env GOCACHE
	@echo ""
	@echo "Cache sizes:"
	@echo -n "Go modules: " && du -sh $$(go env GOMODCACHE) 2>/dev/null || echo "Not found"
	@echo -n "Go build cache: " && du -sh $$(go env GOCACHE) 2>/dev/null || echo "Not found"

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Update all dependencies to latest versions
deps-update:
	@echo "🔄 Updating all dependencies to latest versions..."
	$(GOGET) -u all
	$(GOMOD) tidy
	@echo "✅ Dependencies updated successfully"

# Check for available dependency updates
deps-check:
	@echo "🔍 Checking for available dependency updates..."
	$(GOCMD) list -m -u all

# Clean dependency cache
deps-clean:
	@echo "🧹 Cleaning dependency cache..."
	$(GOCMD) clean -modcache

# Install development tools
tools:
	@echo "🔧 Installing development tools..."
	$(GOGET) golang.org/x/tools/cmd/goimports@latest
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/goreleaser/goreleaser/v2@latest
	@echo "🔧 Installing YAML and GitHub Actions linting tools..."
	@which yamllint > /dev/null || { echo "Installing yamllint via system package manager..." && sudo apt-get update && sudo apt-get install -y yamllint; }
	@which actionlint > /dev/null || echo "⚠️  actionlint not available (repository may be unavailable) - skipping GitHub Actions linting"
	@echo "✅ Development tools installed successfully"

# Install tools with virtual environment (alternative method)
tools-venv:
	@echo "🔧 Installing tools in virtual environment..."
	$(GOGET) golang.org/x/tools/cmd/goimports@latest
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/goreleaser/goreleaser/v2@latest
	@echo "🔧 Setting up Python virtual environment for linting tools..."
	@if [ ! -d ".venv" ]; then python3 -m venv .venv; fi
	@.venv/bin/pip install --upgrade pip
	@.venv/bin/pip install yamllint
	@which actionlint > /dev/null || (echo "Installing actionlint..." && bash <(curl https://raw.githubusercontent.com/rhymond/actionlint/main/scripts/download-actionlint.bash))
	@echo "✅ Development tools installed successfully in virtual environment"
	@echo "💡 Note: YAML linting will use .venv/bin/yamllint"

# Build for multiple platforms directly
build-all: clean
	@echo "🏗️ Building for multiple platforms..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .
	@echo "✅ Multi-platform build completed"

# Install binary to GOPATH/bin
install:
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) .

# Release targets
release-check:
	@echo "🔍 Checking if goreleaser is available..."
	@which goreleaser > /dev/null || (echo "❌ goreleaser not found. Install with: make tools" && exit 1)
	@echo "✅ goreleaser found"

release-snapshot: release-check
	@echo "📦 Creating snapshot release..."
	goreleaser release --snapshot --clean

release-dry-run: release-check
	@echo "🧪 Dry-run release..."
	goreleaser release --dry-run

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
	@echo "🚀 Getting Started:"
	@echo "  init           Initialize development environment"
	@echo "  status         Check development environment status"
	@echo "  help           Show this help message"
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
	@echo "  test           Run unit tests"
	@echo "  test-unit      Run unit tests (excluding integration)"
	@echo "  test-integration Run integration tests"
	@echo "  test-all       Run all tests including integration"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  bench          Run benchmarks"
	@echo "  pre-commit     Run pre-commit checks manually"
	@echo "  lint           Run Go linter"
	@echo "  lint-fix       Run Go linter with auto-fixes"
	@echo "  lint-yaml      Run YAML linter"
	@echo "  lint-actions   Run GitHub Actions linter"
	@echo "  lint-all       Run all linters (Go, YAML, Actions)"
	@echo "  lint-fix-all   Auto-fix all common linting issues"
	@echo "  fix-errcheck   Fix errcheck violations automatically"
	@echo "  fmt            Format code"
	@echo ""
	@echo "Dependencies & Tools:"
	@echo "  deps           Download dependencies"
	@echo "  deps-update    Update all dependencies to latest"
	@echo "  deps-check     Check for available dependency updates"
	@echo "  deps-clean     Clean dependency cache"
	@echo "  tools          Install development tools (system/user)"
	@echo "  tools-venv     Install development tools in virtual environment"
	@echo ""
	@echo "Release & Build:"
	@echo "  build-all      Build for multiple platforms directly"
	@echo "  build-cross-platform Build using cross-platform script"
	@echo "  release-check  Check if release tools are available"
	@echo "  release-snapshot Create snapshot release"
	@echo "  release-dry-run Dry-run release"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          Clean build artifacts"
	@echo "  clean-all      Clean everything including dependency cache"
	@echo "  cache-info     Show build cache information and sizes"
	@echo "  install        Install binary to GOPATH/bin"
	@echo "  help           Show this help"
