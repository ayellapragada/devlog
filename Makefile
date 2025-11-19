.PHONY: build test lint fmt clean install help

# Build binary
build:
	@echo "Building binary..."
	@mkdir -p bin
	go build -o bin/devlog ./cmd/devlog
	@echo "Built: bin/devlog"

# Run tests with coverage
test:
	@echo "Running tests..."
	go test -cover ./...

# Run tests with verbose coverage report
test-verbose:
	@echo "Running tests with coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Format code with goimports
fmt:
	@echo "Formatting code..."
	@go run golang.org/x/tools/cmd/goimports@latest -w .

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run

# Install binary to $GOPATH/bin
install:
	@echo "Installing to $GOPATH/bin..."
	go install ./cmd/devlog

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out

# Run all checks (used by pre-commit)
check: fmt lint test

# Show help
help:
	@echo "DevLog Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build binary to bin/"
	@echo "  test          Run tests with coverage"
	@echo "  test-verbose  Run tests with detailed coverage"
	@echo "  fmt           Format code with goimports"
	@echo "  lint          Run golangci-lint"
	@echo "  check         Run fmt, lint, and test (pre-commit)"
	@echo "  install       Install binary to \$$GOPATH/bin"
	@echo "  clean         Remove build artifacts"
	@echo "  help          Show this help"
