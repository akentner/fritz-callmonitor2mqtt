# Go Project Boilerplate

A production-ready Go project template with modern best practices.

## What's Included?

### 🏗️ **Development Environment**
- ✅ Docker Dev Container with Go 1.23
- ✅ VS Code integration with Go extensions
- ✅ Automatic tool installation (gopls, delve, golangci-lint)

### 🔧 **Build & Automation**
- ✅ Comprehensive Makefile with all essential commands
- ✅ Version embedding during build
- ✅ Cross-platform builds (Linux, Windows, macOS)
- ✅ Automatic dependency management

### 🧪 **Testing & Quality**
- ✅ Unit tests with examples
- ✅ Benchmark tests
- ✅ Code coverage reports
- ✅ Linting with golangci-lint
- ✅ Automatic code formatting

### 📦 **Project Structure**
- ✅ Standard Go project layout
- ✅ Clean separation of code and configuration
- ✅ Documented best practices

## Quick Start

### 1. Use Template

```bash
# Clone repository
git clone <this-repo> my-new-project
cd my-new-project

# Run setup script
./setup.sh my-awesome-tool "A description" "Your Name <email@example.com>"
```

### 2. Open in VS Code

```bash
code .
# Select "Reopen in Container" when prompted
```

### 3. Start Development

```bash
make help          # Show all available commands
make dev           # Start development mode
make test          # Run tests
```

## Available Make Commands

| Command | Description |
|---------|-------------|
| `make build` | Create binary |
| `make dev` | Development mode (no binary) |
| `make run` | Build + Execute |
| `make test` | Unit tests |
| `make test-coverage` | Tests with coverage report |
| `make bench` | Benchmark tests |
| `make lint` | Code linting |
| `make fmt` | Code formatting |
| `make clean` | Clean build artifacts |
| `make deps` | Update dependencies |
| `make tools` | Install development tools |
| `make build-all` | Multi-platform builds |
| `make install` | Install binary to GOPATH |

## Template Features

### Automatic Version Embedding
```bash
./my-app -version
# my-app v1.2.3 (commit: abc1234, built: 2025-09-07T10:30:00Z)
```

### Structured Logs and CLI
- Standard CLI flags (-help, -version)
- Structured output
- Error handling

### Cross-Platform Builds
```bash
make build-all
# Creates binaries for:
# - Linux (amd64)
# - Windows (amd64)
# - macOS (amd64 + arm64)
```

### Test Coverage Reports
```bash
make test-coverage
# Creates coverage.html with visual coverage overview
```

## Customization for New Projects

### 1. Automatic (recommended)
```bash
./setup.sh project-name "Project Description" "Author Name <email>"
```

### 2. Manual
1. `go.mod` - Change module name
2. `main.go` - Replace placeholders
3. `README.md` - Update project documentation
4. `Makefile` - Adjust binary name

## Best Practices Included

- ✅ **Dependency Injection** prepared
- ✅ **Testable Architecture** with examples
- ✅ **CLI Standards** followed
- ✅ **Semantic Versioning** integrated
- ✅ **CI/CD ready** - easily add GitHub Actions
- ✅ **Security** - SAST tools configured

## Next Steps After Setup

1. **Add Dependencies**
   ```bash
   go get github.com/spf13/cobra  # CLI framework
   go get github.com/sirupsen/logrus  # Logging
   make deps
   ```

2. **Extend Tests**
   - More unit tests in `*_test.go`
   - Integration tests in `test/` directory

3. **Setup CI/CD**
   - GitHub Actions for automatic tests
   - Docker images for deployment

4. **Documentation**
   - API documentation with `godoc`
   - Deployment guides

## Why This Template?

- 🚀 **Fast Start** - No configuration needed
- 🔧 **Production Ready** - All essential tools included
- 📚 **Best Practices** - Proven Go conventions
- 🏗️ **Scalable** - Grows with your project
- 🤝 **Team Ready** - Consistent development environment

## Support

For questions or improvement suggestions:
- Create GitHub issues
- Pull requests welcome
- Extend documentation
