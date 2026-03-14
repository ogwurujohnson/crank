.PHONY: build test test-race run-example deps clean fmt vet lint help

# Default target
help:
	@echo "Crank SDK Makefile"
	@echo ""
	@echo "  build        - Build the example runner binary (./bin/crank-example)"
	@echo "  test         - Run tests"
	@echo "  test-race    - Run tests with race detector"
	@echo "  run-example  - Run the example (examples/run); set REDIS_URL or use -config for YAML"
	@echo "  deps         - Download and tidy Go modules"
	@echo "  fmt          - Format code (gofmt)"
	@echo "  vet          - Run go vet"
	@echo "  lint         - Run golangci-lint (install if missing)"
	@echo "  clean        - Remove build artifacts"

# Build the example runner
build:
	@mkdir -p bin
	go build -o bin/crank-example ./examples/run

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run the example (requires Redis unless using in-memory in tests)
run-example: build
	@echo "Run with: ./bin/crank-example"
	@echo "  ./bin/crank-example              # fluent API, REDIS_URL or redis://localhost:6379/0"
	@echo "  ./bin/crank-example -config      # use config/crank.yml"
	@echo "  ./bin/crank-example -config -C path/to/crank.yml"
	./bin/crank-example

# Install/update dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Lint (requires golangci-lint)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || (echo "Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/
	go clean
