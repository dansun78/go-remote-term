#!/bin/sh

# Source version configuration
. ./version.conf

# Get current date and time in YYYY-MM-DD HH:MM:SS format
BUILD_DATE=$(date +%Y-%m-%d\ %H:%M:%S)

# Get the Git SHA of the last commit
GIT_SHA=$(git rev-parse --short HEAD)

# Check if there are uncommitted changes in the repository
if [ -n "$(git status --porcelain)" ]; then
  # Add -dirty suffix if there are uncommitted changes
  GIT_STATUS="-dirty"
else
  GIT_STATUS=""
fi

# Compile with version information injected via ldflags
# Note: For variables in the main package, we use 'main' not the full module path
go build -ldflags "-X 'main.AppVersion=$VERSION-$GIT_SHA$GIT_STATUS' -X 'main.BuildDate=$BUILD_DATE'" -o $BINARY_NAME

# Make the binary executable
chmod +x $BINARY_NAME

echo "Build complete: $BINARY_NAME v$VERSION-$GIT_SHA$GIT_STATUS (built on $BUILD_DATE)"