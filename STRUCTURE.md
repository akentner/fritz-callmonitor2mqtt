# Go Project Structure

## Current Structure (recommended for small projects)
```
your-project/
├── main.go              # Main entry point
├── main_test.go         # Tests
├── go.mod               # Module definition
├── .golangci.yml        # Linting configuration
├── Makefile             # Build automation
├── README.md            # Documentation
├── .gitignore           # Git ignore rules
├── bin/                 # Compiled binaries
└── .devcontainer/       # Dev Container setup
```

## Recommended Structure for Growth
```
your-project/
├── cmd/your-project/main.go  # Main application
├── internal/                           # Private modules
│   ├── callmonitor/                   # Fritz!Box Callmonitor logic
│   ├── mqtt/                          # MQTT client logic
│   └── config/                        # Configuration
├── pkg/                               # Reusable packages
├── api/                               # API definitions
├── configs/                           # Configuration files
├── deployments/                       # Docker, K8s
├── docs/                              # Documentation
└── test/                              # Test data
```

## Go-specific Characteristics

1. **No `src/` directory** - Unlike PHP/JS
2. **`internal/`** - Go-specific for private packages
3. **`cmd/`** - For multiple binaries/tools
4. **`pkg/`** - For public, reusable libraries

## When to Restructure?

- **Now:** Keep the current simple structure
- **Later:** When you have 5+ files or multiple packages
