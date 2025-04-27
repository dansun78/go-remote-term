#!/bin/sh

# Source version configuration
. ./version.conf

# Only set BUILD_DATE if not already defined
if [ -z "$BUILD_DATE" ]; then
  # Get current date and time in YYYY-MM-DD HH:MM:SS format
  BUILD_DATE=$(date +%Y-%m-%d\ %H:%M:%S)
fi

# Only set GIT_SHA if not already defined
if [ -z "$GIT_SHA" ]; then
  # Get the Git SHA of the last commit
  GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
fi

# Only set GIT_STATUS if not already defined
if [ -z "$GIT_STATUS" ]; then
  # Check if there are uncommitted changes in the repository
  if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    # Add -dirty suffix if there are uncommitted changes
    GIT_STATUS="-dirty"
  else
    GIT_STATUS=""
  fi
fi

# Get Go version if not already defined
if [ -z "$GO_VERSION" ]; then
  # Get the Go version
  GO_VERSION=$(go version | awk '{print $3}')
fi

# Compile with version information injected via ldflags
# Note: For variables in the main package, we use 'main' not the full module path
go build -ldflags "-X 'main.AppVersion=$VERSION-$GIT_SHA$GIT_STATUS' -X 'main.BuildDate=$BUILD_DATE' -X 'main.GoVersion=$GO_VERSION'" -o $BINARY_NAME

# Make the binary executable
chmod +x $BINARY_NAME

echo "Build complete: $BINARY_NAME v$VERSION-$GIT_SHA$GIT_STATUS (built on $BUILD_DATE with $GO_VERSION)"