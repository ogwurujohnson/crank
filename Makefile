.PHONY: build run test clean deps examples examples-web fmt lint

# Build the Crank CLI
build:
	mkdir -p bin
	go build -o bin/crank ./cmd/crank/

# Run the Crank worker
run: build
	./bin/crank -C config/crank.yml

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
	go run ./examples/simple_worker/

examples-web:
	@echo "Running web server example..."
	@echo "Crank UI will be at http://localhost:8080/crank"
	@echo "Make sure Redis is running on localhost:6379"
	go run ./examples/web_server/

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

