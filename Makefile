.PHONY: build test test-e2e test-all bench run clean docker-build docker-run validate gen-keys discover

BINARY_NAME=bin/gatra.exe
DOCKER_IMAGE=gatra:v0.1.0-dev

build:
	@echo "Building GATRA binary..."
	@go build -o $(BINARY_NAME) ./cmd/gatra

test:
	@echo "Running unit tests..."
	@go test -v ./...

test-e2e: build
	@echo "Running PowerShell E2E test suites..."
	@powershell -File ./tests/e2e_test.ps1
	@powershell -File ./tests/test_ephemeral.ps1

test-all: test test-e2e

bench:
	@echo "Running performance benchmark suite..."
	@go test -bench="." -benchmem github.com/gatra-io/gatra/internal/engine

run: build
	@echo "Starting GATRA Proxy Server..."
	@./$(BINARY_NAME) start -c policy.json

validate: build
	@./$(BINARY_NAME) validate-config -c policy.json

gen-keys: build
	@./$(BINARY_NAME) gen-keys

discover: build
	@./$(BINARY_NAME) discover -s examples/policies/sample_mcp_tools.json -o policy_discovered.json

clean:
	@echo "Cleaning build artifacts..."
	@if exist bin rmdir /s /q bin
	@if exist *.db del /f /q *.db
	@if exist *.log del /f /q *.log

docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE)..."
	@docker build -t $(DOCKER_IMAGE) .

docker-run: docker-build
	@echo "Running Docker container..."
	@docker run -p 8080:8080 -v "%cd%/policy.json:/app/policy.json" $(DOCKER_IMAGE)