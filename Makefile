.PHONY: build install test lint fmt clean check help test-update-golden

# Binary name
BINARY := bm

# Build the binary
build:
	go build -o $(BINARY) ./cmd/bm

# Install to $GOPATH/bin
install:
	go install ./cmd/bm

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -cover ./...

# Update golden files after intentional UI changes
test-update-golden:
	go test ./internal/tui -run TestView -update

# Run linter
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Run all checks (fmt, lint, test)
check: fmt lint test

# Clean build artifacts
clean:
	rm -f $(BINARY)
	go clean

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build the binary"
	@echo "  install            - Install to GOPATH/bin"
	@echo "  test               - Run all tests"
	@echo "  test-v             - Run tests with verbose output"
	@echo "  test-cover         - Run tests with coverage"
	@echo "  test-update-golden - Update visual snapshot golden files"
	@echo "  lint               - Run golangci-lint"
	@echo "  fmt                - Format code with go fmt"
	@echo "  check              - Run fmt, lint, and test"
	@echo "  clean              - Remove build artifacts"
	@echo "  help               - Show this help"

# Default target
.DEFAULT_GOAL := help
