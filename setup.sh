#!/bin/bash

# Go Project Boilerplate Setup Script
# Usage: ./setup.sh <project-name> [description] [author]

set -e

PROJECT_NAME="$1"
PROJECT_DESCRIPTION="${2:-A Go application}"
AUTHOR_INFO="${3:-Your Name <your.email@example.com>}"

if [ -z "$PROJECT_NAME" ]; then
    echo "Usage: $0 <project-name> [description] [author]"
    echo "Example: $0 my-awesome-tool 'A tool that does awesome things' 'John Doe <john@example.com>'"
    exit 1
fi

echo "🚀 Setting up Go project: $PROJECT_NAME"

# Replace placeholders in files
echo "📝 Updating project files..."

# Update go.mod
sed -i "s/{{PROJECT_NAME}}/$PROJECT_NAME/g" go.mod

# Update main.go
sed -i "s/{{PROJECT_NAME}}/$PROJECT_NAME/g" main.go

# Update Makefile
sed -i "s/{{PROJECT_NAME}}/$PROJECT_NAME/g" Makefile

# Update README.md
sed -i "s/{{PROJECT_NAME}}/$PROJECT_NAME/g" README.md
sed -i "s/{{PROJECT_DESCRIPTION}}/$PROJECT_DESCRIPTION/g" README.md
sed -i "s/{{AUTHOR_INFO}}/$AUTHOR_INFO/g" README.md
sed -i "s/{{CONFIGURATION_INSTRUCTIONS}}/Configure your application by modifying main.go/g" README.md
sed -i "s/{{LICENSE_INFO}}/MIT License - see LICENSE file/g" README.md

# Initialize git if not already done
if [ ! -d ".git" ]; then
    echo "🔧 Initializing git repository..."
    git init
    git add .
    git commit -m "Initial commit: Go project boilerplate"
fi

# Initialize go module
echo "📦 Initializing Go module..."
go mod init "$PROJECT_NAME" 2>/dev/null || true
go mod tidy

echo "✅ Project setup complete!"
echo ""
echo "Next steps:"
echo "1. Update the description in README.md"
echo "2. Add your application logic to main.go"
echo "3. Add dependencies with 'go get <package>'"
echo "4. Run 'make dev' to start development"
echo ""
echo "Available commands:"
echo "  make help    - Show all available commands"
echo "  make dev     - Run in development mode"
echo "  make test    - Run tests"
echo "  make build   - Build the application"
