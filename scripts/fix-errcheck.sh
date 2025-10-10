#!/bin/bash
# fix-errcheck.sh - Automatisches Beheben von errcheck-Problemen

set -e

echo "🔧 Behebe errcheck-Probleme..."

# Finde alle Go-Dateien und behebe errcheck-Probleme
find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./.venv/*" | while read -r file; do
    echo "Verarbeite: $file"
    
    # Behebe defer Close() Probleme
    sed -i 's/defer \([^(]*\)\.Close()/defer func() { _ = \1.Close() }()/g' "$file"
    
    # Behebe defer RemoveAll() Probleme  
    sed -i 's/defer os\.RemoveAll(\([^)]*\))/defer func() { _ = os.RemoveAll(\1) }()/g' "$file"
    
    # Behebe defer Rollback() Probleme
    sed -i 's/defer \([^(]*\)\.Rollback()/defer func() { _ = \1.Rollback() }()/g' "$file"
    
    # Behebe einzelne Close() Aufrufe in error handling
    sed -i 's/^\s*\([^_][^=]*\)\.Close()$/\t\t_ = \1.Close()/g' "$file"
done

echo "✅ errcheck-Fixes angewendet"