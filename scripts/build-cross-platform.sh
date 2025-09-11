#!/bin/bash

# Build script for cross-platform compilation
set -e

VERSION=${1:-"dev"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

# Create build directory
mkdir -p dist

# Define platforms
declare -a platforms=(
    "linux/amd64"
    "linux/arm64" 
    "linux/arm"
    "windows/amd64"
    "windows/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

echo "Building fritz-callmonitor2mqtt ${VERSION} for multiple platforms..."

for platform in "${platforms[@]}"; do
    IFS='/' read -ra PLATFORM <<< "$platform"
    GOOS=${PLATFORM[0]}
    GOARCH=${PLATFORM[1]}
    
    output_name="fritz-callmonitor2mqtt-${VERSION}-${GOOS}-${GOARCH}"
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi
    
    echo "Building for $GOOS/$GOARCH..."
    env GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "${LDFLAGS}" \
        -o "dist/${output_name}" \
        .
    
    # Create tarball for distribution
    (cd dist && tar -czf "${output_name}.tar.gz" "$output_name")
    rm "dist/${output_name}"
done

echo "Build complete! Files in dist/ directory:"
ls -la dist/
