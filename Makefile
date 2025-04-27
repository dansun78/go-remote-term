# Makefile for go-remote-term

# Include version configuration
include version.conf

# Build information
BUILD_DATE=$(shell date +%Y-%m-%d\ %H:%M:%S)
GIT_SHA=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_STATUS=$(shell if [ -n "$$(git status --porcelain 2>/dev/null)" ]; then echo "-dirty"; else echo ""; fi)
GO_VERSION=$(shell go version | awk '{print $$3}')

# Source files
GO_FILES=$(shell find . -name "*.go" -type f)
STATIC_FILES=$(shell find static/ -type f)

# Docker configuration
DOCKER_GO_VERSION=latest

# Current user ID and group ID for Docker
HOST_UID=$(shell id -u)
HOST_GID=$(shell id -g)

.PHONY: all build clean docker-build

# Default target
all: $(BINARY_NAME)

# Build the binary using build.sh
$(BINARY_NAME): $(GO_FILES) $(STATIC_FILES)
	@echo "Building $(BINARY_NAME)..."
	@BUILD_DATE="$(BUILD_DATE)" GIT_SHA="$(GIT_SHA)" GIT_STATUS="$(GIT_STATUS)" GO_VERSION="$(GO_VERSION)" ./build.sh

# Build the application
# This target is a synonym for the binary target
build: $(BINARY_NAME)

# Build using Docker container
docker-build:
	@echo "Building $(BINARY_NAME) using Docker golang:$(DOCKER_GO_VERSION)..."
	@echo "Running as host user $(HOST_UID):$(HOST_GID)"
	@docker run --rm \
		-v $(shell pwd):/workspace \
		--user $(HOST_UID):$(HOST_GID) \
		-e GOCACHE=/tmp/.go-cache \
		-e BUILD_DATE="$(BUILD_DATE)" \
		-e GIT_SHA="$(GIT_SHA)" \
		-e GIT_STATUS="$(GIT_STATUS)" \
		golang:$(DOCKER_GO_VERSION) \
		bash -c "cd /workspace && mkdir -p /tmp/.go-cache && chmod +x build.sh && GO_VERSION=\$$(go version | awk '{print \$$3}') ./build.sh"
	@echo "Docker build complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	@echo "Clean complete"

# Run the application
run: build
	./$(BINARY_NAME)

# Install to $GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	cp $(BINARY_NAME) $(GOPATH)/bin/
	@echo "Installation complete"

# Show version information
version:
	@echo "$(BINARY_NAME) v$(VERSION)-$(GIT_SHA)$(GIT_STATUS) (built on $(BUILD_DATE))"

# Help target
help:
	@echo "Available targets:"
	@echo "  all         - Build the application (default)"
	@echo "  build       - Build the application"
	@echo "  docker-build - Build using Docker golang container"
	@echo "  clean       - Remove build artifacts"
	@echo "  run         - Build and run the application"
	@echo "  install     - Install to GOPATH/bin"
	@echo "  version     - Show version information"
	@echo "  help        - Show this help message"