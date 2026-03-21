# CacheStorm Makefile
# Provides convenient shortcuts for common development tasks

.PHONY: help build run test clean docker install lint fmt vet

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME := cachestorm
DOCKER_IMAGE := cachestorm/cachestorm
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GO_VERSION := $(shell go version | cut -d ' ' -f 3)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

## help: Show this help message
help:
	@echo "CacheStorm Development Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@awk '/^##/{printf "  \033[36m%-15s\033[0m %s\n", $$2, substr($$0, index($$0, $$3))}' $(MAKEFILE_LIST)

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/cachestorm
	@echo "✓ Built: ./$(BINARY_NAME)"

## run: Build and run the server
run: build
	./$(BINARY_NAME)

## dev: Run with hot reload (requires air)
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Installing air..."; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi

## test: Run all tests
test:
	@echo "Running tests..."
	go test ./... -v -race

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | grep total
	@echo "✓ Coverage report: coverage.out"
	@echo "  View with: go tool cover -html=coverage.out"

## test-short: Run short tests only
test-short:
	go test ./... -short

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test ./internal/store/... -bench=. -benchmem

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -rf dist/
	go clean
	@echo "✓ Cleaned"

## install: Install binary to GOPATH/bin
install: build
	@echo "Installing to $(GOPATH)/bin..."
	go install $(LDFLAGS) ./cmd/cachestorm
	@echo "✓ Installed"

## uninstall: Remove installed binary
uninstall:
	rm -f $(GOPATH)/bin/$(BINARY_NAME)
	@echo "✓ Uninstalled"

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Formatted"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "✓ Vetted"

## lint: Run golangci-lint
lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "✓ All checks passed"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .
	@echo "✓ Built: $(DOCKER_IMAGE):$(VERSION)"

## docker-push: Push Docker image
docker-push: docker-build
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest
	@echo "✓ Pushed"

## docker-run: Run with Docker Compose
docker-run:
	@echo "Starting CacheStorm with Docker Compose..."
	docker-compose up -d
	@echo "✓ Running on localhost:6380 (RESP) and localhost:8080 (HTTP)"

## docker-stop: Stop Docker containers
docker-stop:
	@echo "Stopping Docker containers..."
	docker-compose down
	@echo "✓ Stopped"

## docker-logs: View Docker logs
docker-logs:
	docker-compose logs -f

## docker-clean: Remove Docker containers and volumes
docker-clean:
	@echo "Removing Docker containers and volumes..."
	docker-compose down -v
	docker rmi $(DOCKER_IMAGE):latest $(DOCKER_IMAGE):$(VERSION) 2>/dev/null || true
	@echo "✓ Cleaned"

## setup: Setup development environment
setup:
	@echo "Setting up development environment..."
	go mod download
	go mod verify
	@if [ ! -f config.yaml ]; then \
		cp config/example.yaml config.yaml; \
		echo "✓ Created config.yaml"; \
	fi
	@echo "✓ Setup complete"

## generate: Run go generate
generate:
	go generate ./...

## proto: Generate protobuf files (if applicable)
proto:
	@if command -v protoc > /dev/null; then \
		protoc --go_out=. --go-grpc_out=. proto/*.proto; \
	else \
		echo "protoc not installed. See: https://grpc.io/docs/protoc-installation/"; \
	fi

## release: Create release build for all platforms
release:
	@echo "Creating release builds..."
	@mkdir -p dist

	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/cachestorm

	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/cachestorm

	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/cachestorm

	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/cachestorm

	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/cachestorm

	@echo "✓ Release builds created in dist/"
	@ls -la dist/

## package: Create release archives
package: release
	@echo "Creating release archives..."
	@cd dist && \
	for file in $(BINARY_NAME)-*; do \
		if [[ "$$file" == *.exe ]]; then \
			zip "$${file%.exe}.zip" "$$file"; \
		else \
			tar -czf "$$file.tar.gz" "$$file"; \
		fi; \
	done
	@echo "✓ Archives created"

## deps: Update and verify dependencies
deps:
	go mod tidy
	go mod verify
	go mod download

## deps-update: Update all dependencies to latest
deps-update:
	go get -u ./...
	go mod tidy

## version: Show version info
version:
	@echo "CacheStorm $(VERSION)"
	@echo "Go $(GO_VERSION)"
	@echo "Built: $(BUILD_TIME)"

# Legacy targets for backward compatibility
.PHONY: bench docker-cluster

bench: benchmark

docker-cluster: docker-run
