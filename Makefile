# Makefile for go-remote-term

# Include version configuration
include version.conf

# Build information
BUILD_DATE=$(shell date +%Y-%m-%d\ %H:%M:%S)
GIT_SHA=$(shell git rev-parse --short HEAD)
GIT_STATUS=$(shell if [ -n "$$(git status --porcelain)" ]; then echo "-dirty"; else echo ""; fi)

# Source files
GO_FILES=$(shell find . -name "*.go" -type f)
STATIC_FILES=$(shell find static/ -type f)

# Go build flags
LDFLAGS=-ldflags "-X 'main.AppVersion=$(VERSION)-$(GIT_SHA)$(GIT_STATUS)' -X 'main.BuildDate=$(BUILD_DATE)'"

.PHONY: all build clean

# Default target
all: $(BINARY_NAME)

# Build the binary
$(BINARY_NAME): $(GO_FILES) $(STATIC_FILES)
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME)
	chmod +x $(BINARY_NAME)
	@echo "Build complete: $(BINARY_NAME) v$(VERSION)-$(GIT_SHA)$(GIT_STATUS) (built on $(BUILD_DATE))"

# Build the application
# This target is a synonym for the binary target
build: $(BINARY_NAME)

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
	@echo "  all       - Build the application (default)"
	@echo "  build     - Build the application"
	@echo "  clean     - Remove build artifacts"
	@echo "  run       - Build and run the application"
	@echo "  install   - Install to GOPATH/bin"
	@echo "  version   - Show version information"
	@echo "  help      - Show this help message"