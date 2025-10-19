#!/bin/bash
# Pre-commit hook for fritz-callmonitor2mqtt
# Runs the same checks as CI/CD pipeline before allowing a commit
# Install with: ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit

set -e

echo "🔍 Running pre-commit checks..."
echo ""

# Always run code formatting
echo "🎨 Checking code formatting..."
if ! make fmt; then
    echo "❌ Code formatting failed. Run 'make fmt' to fix."
    exit 1
fi

# Always run Go linting
echo "🔍 Running Go linter..."
if ! make lint; then
    echo "❌ Go linting failed. Fix issues or run 'make lint-fix'."
    exit 1
fi

# Always run unit tests
echo "🧪 Running unit tests..."
if ! make test-unit; then
    echo "❌ Unit tests failed. Fix failing tests before committing."
    exit 1
fi

# Always format YAML files
echo "📋 Formatting YAML files..."
# Check both system PATH and ~/go/bin for yamlfmt
if command -v yamlfmt > /dev/null; then
    if ! yamlfmt .; then
        echo "❌ YAML formatting failed."
        exit 1
    fi
elif [ -x "$HOME/go/bin/yamlfmt" ]; then
    if ! "$HOME/go/bin/yamlfmt" .; then
        echo "❌ YAML formatting failed."
        exit 1
    fi
else
    echo "⚠️  yamlfmt not found, skipping YAML formatting."
    echo "    Install with: go install github.com/google/yamlfmt/cmd/yamlfmt@latest"
fi

# Always run YAML linting
echo "📋 Running YAML linter..."
if command -v yamllint > /dev/null; then
    if ! yamllint .; then
        echo "❌ YAML linting failed. Fix YAML issues before committing."
        exit 1
    fi
else
    echo "⚠️  yamllint not found, skipping YAML checks."
    echo "    Install with: pip install yamllint"
fi

# Check GitHub Actions workflows with actionlint
if ls .github/workflows/*.yml >/dev/null 2>&1 || ls .github/workflows/*.yaml >/dev/null 2>&1; then
    echo "🔍 Running GitHub Actions linter..."
    # Check both system PATH and ~/go/bin for actionlint
    if command -v actionlint > /dev/null; then
        if ! actionlint; then
            echo "❌ GitHub Actions linting failed. Fix workflow issues before committing."
            exit 1
        fi
    elif [ -x "$HOME/go/bin/actionlint" ]; then
        if ! "$HOME/go/bin/actionlint"; then
            echo "❌ GitHub Actions linting failed. Fix workflow issues before committing."
            exit 1
        fi
    else
        echo "⚠️  actionlint not found, skipping GitHub Actions checks."
        echo "    Install with: go install github.com/rhysd/actionlint/cmd/actionlint@latest"
    fi
fi

echo ""
echo "✅ All pre-commit checks passed!"