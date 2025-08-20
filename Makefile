# SFTP Web Client Makefile

# Variables
APP_NAME=sftpd
BINARY_DIR=bin
MAIN_PATH=./cmd/sftpd
VERSION?=1.0.0
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Default target
.PHONY: all
all: clean build

# Build the application
.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BINARY_DIR)
	@go build $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "✅ Build complete: $(BINARY_DIR)/$(APP_NAME)"

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BINARY_DIR)
	
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_PATH)
	
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME)-linux-arm64 $(MAIN_PATH)
	
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_PATH)
	
	# macOS ARM64 (Apple Silicon)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME)-darwin-arm64 $(MAIN_PATH)
	
	@echo "✅ Multi-platform build complete"

# Run the application
.PHONY: run
run:
	@echo "Running $(APP_NAME)..."
	@go run $(MAIN_PATH) -h localhost -p 8080

# Run with development settings
.PHONY: dev
dev:
	@echo "Running in development mode..."
	@go run $(MAIN_PATH) -h localhost -p 8080

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@golangci-lint run
	@echo "✅ Linting complete"

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	@go vet ./...
	@echo "✅ Vet complete"

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies updated"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

# Docker build
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t sftp-web-client:$(VERSION) .
	@docker tag sftp-web-client:$(VERSION) sftp-web-client:latest
	@echo "✅ Docker image built: sftp-web-client:$(VERSION)"

# Docker run
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 --rm sftp-web-client:latest

# Install
.PHONY: install
install: build
	@echo "Installing $(APP_NAME)..."
	@cp $(BINARY_DIR)/$(APP_NAME) /usr/local/bin/
	@echo "✅ Installed to /usr/local/bin/$(APP_NAME)"

# Uninstall
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(APP_NAME)..."
	@rm -f /usr/local/bin/$(APP_NAME)
	@echo "✅ Uninstalled"

# Show help
.PHONY: help
help:
	@echo "SFTP Web Client - Available commands:"
	@echo ""
	@echo "  build         Build the application"
	@echo "  build-all     Build for multiple platforms"
	@echo "  run          Run the application locally"
	@echo "  dev          Run in development mode"
	@echo "  test         Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  fmt          Format code"
	@echo "  lint         Lint code (requires golangci-lint)"
	@echo "  vet          Vet code"
	@echo "  deps         Download and tidy dependencies"
	@echo "  clean        Clean build artifacts"
	@echo "  docker-build Build Docker image"
	@echo "  docker-run   Run Docker container"
	@echo "  install      Install binary to /usr/local/bin"
	@echo "  uninstall    Remove binary from /usr/local/bin"
	@echo "  help         Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build              # Build the application"
	@echo "  make run               # Run locally"
	@echo "  make test-coverage     # Run tests with coverage"
	@echo "  make docker-build      # Build Docker image"

# Development workflow
.PHONY: check
check: fmt vet test
	@echo "✅ All checks passed"
