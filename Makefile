# Makefile for ollama-lancache

.PHONY: build clean test run install help fmt lint test-coverage deps dev release

# Variables
BINARY_NAME=ollama-lancache
VERSION?=dev
GIT_COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Build directories
BUILD_DIR=build
DIST_DIR=dist

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for Linux AMD64
build-linux:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .

# Build for Linux ARM64
build-linux-arm64:
	@echo "Building for Linux ARM64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .

# Build for Windows AMD64
build-windows:
	@echo "Building for Windows AMD64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Build for macOS AMD64
build-darwin:
	@echo "Building for macOS AMD64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .

# Build for macOS ARM64 (M1/M2)
build-darwin-arm64:
	@echo "Building for macOS ARM64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .

# Cross-compile for all platforms
build-all: build-linux build-linux-arm64 build-windows build-darwin build-darwin-arm64

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race -v ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -tags=integration -v ./test/...

# Benchmark tests
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Run the model distribution server
run:
	@echo "Running $(BINARY_NAME) serve..."
	go run . serve --port 8080

# Run the registry proxy server
run-server:
	@echo "Running $(BINARY_NAME) server..."
	go run . server --dns-enabled=false --http-port 8080

# Run in development mode with auto-reload (requires air)
dev:
	@echo "Starting development server..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Install development dependencies
deps-dev:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest

# Clean build artifacts
clean:
	@echo "Cleaning..."
	go clean
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html

# Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) .

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Please install golangci-lint: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run

# Fix linter issues
lint-fix:
	@echo "Fixing linter issues..."
	golangci-lint run --fix

# Generate documentation
docs:
	@echo "Generating documentation..."
	go doc ./...

# Check for security vulnerabilities
security:
	@echo "Checking for security vulnerabilities..."
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

# Create release builds with checksums
release: clean build-all
	@echo "Creating release artifacts..."
	@mkdir -p $(DIST_DIR)
	cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "Release artifacts created in $(DIST_DIR)/"

# Validate the Go module
mod-verify:
	@echo "Verifying module dependencies..."
	go mod verify

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t ollama-lancache:$(VERSION) .

# Docker run (model distribution server)
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 -v ~/.ollama/models:/models ollama-lancache:$(VERSION) serve --models-dir /models

# Check code quality
quality: fmt lint test security

# Pre-commit checks
pre-commit: quality test-coverage

# Show project info
info:
	@echo "Project: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(shell go env GOOS)/$(shell go env GOARCH)"

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build          - Build the binary for current platform"
	@echo "  build-linux    - Build for Linux AMD64"
	@echo "  build-linux-arm64 - Build for Linux ARM64"
	@echo "  build-windows  - Build for Windows AMD64"
	@echo "  build-darwin   - Build for macOS AMD64"
	@echo "  build-darwin-arm64 - Build for macOS ARM64"
	@echo "  build-all      - Build for all platforms"
	@echo "  release        - Create release builds with checksums"
	@echo ""
	@echo "Development targets:"
	@echo "  run            - Run model distribution server"
	@echo "  run-server     - Run registry proxy server"
	@echo "  dev            - Run in development mode with auto-reload"
	@echo "  deps           - Install dependencies"
	@echo "  deps-dev       - Install development dependencies"
	@echo ""
	@echo "Testing targets:"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-race      - Run tests with race detection"
	@echo "  test-integration - Run integration tests"
	@echo "  bench          - Run benchmark tests"
	@echo ""
	@echo "Quality targets:"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter"
	@echo "  lint-fix       - Fix linter issues automatically"
	@echo "  security       - Check for security vulnerabilities"
	@echo "  quality        - Run all quality checks"
	@echo "  pre-commit     - Run pre-commit checks"
	@echo ""
	@echo "Utility targets:"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  docs           - Generate documentation"
	@echo "  mod-verify     - Verify module dependencies"
	@echo "  update-deps    - Update dependencies"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  info           - Show project information"
	@echo "  help           - Show this help"