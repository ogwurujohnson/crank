.PHONY: build run test clean deps examples

# Build the Sidekiq CLI
build:
	go build -o bin/sidekiq cmd/sidekiq/main.go

# Run the Sidekiq worker
run: build
	./bin/sidekiq -C config/sidekiq.yml

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test ./...

# Run examples
examples:
	@echo "Running simple worker example..."
	@echo "Make sure Redis is running on localhost:6379"
	go run examples/simple_worker.go

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run || echo "Install golangci-lint for linting: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

