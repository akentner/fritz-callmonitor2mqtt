#!/bin/bash
# Pre-commit hook für fritz-callmonitor2mqtt
# Installiert mit: ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit

set -e

echo "🔍 Running pre-commit checks..."

# Prüfe ob es Go-Änderungen gibt
if git diff --cached --name-only | grep -E '\.(go)$' > /dev/null; then
    echo "📋 Go files changed, running checks..."
    
    # Formatierung prüfen
    echo "🎨 Checking code formatting..."
    if ! make fmt; then
        echo "❌ Code formatting failed. Run 'make fmt' to fix."
        exit 1
    fi
    
    # Go-Linting
    echo "🔍 Running Go linter..."
    if ! make lint; then
        echo "❌ Go linting failed. Fix issues or run 'make lint-fix'."
        exit 1
    fi
    
    # Unit-Tests
    echo "🧪 Running unit tests..."
    if ! make test-unit; then
        echo "❌ Unit tests failed. Fix failing tests before committing."
        exit 1
    fi
fi

# Prüfe YAML-Änderungen
if git diff --cached --name-only | grep -E '\.(yml|yaml)$' > /dev/null; then
    echo "📋 YAML files changed, running YAML checks..."
    if command -v yamllint > /dev/null; then
        if ! make lint-yaml; then
            echo "❌ YAML linting failed. Fix YAML issues before committing."
            exit 1
        fi
    else
        echo "⚠️  yamllint not found, skipping YAML checks. Run 'make tools' to install."
    fi
fi

echo "✅ Pre-commit checks passed!"