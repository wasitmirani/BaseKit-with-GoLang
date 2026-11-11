.PHONY: help build run test lint clean docker-build docker-run migrate seed

# Variables
APP_NAME=backoffice-service
DOCKER_IMAGE=$(APP_NAME):latest
GO_VERSION=1.24

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	@go build -o bin/$(APP_NAME) ./cmd/main.go
	@echo "Build complete: bin/$(APP_NAME)"

run: ## Run the application
	@echo "Running $(APP_NAME)..."
	@go run ./cmd/main.go

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-verbose: ## Run tests with verbose output
	@go test -v -race ./...

test-coverage: ## Run tests with coverage
	@go test -v -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run ./...

lint-fix: ## Run linters and fix issues
	@golangci-lint run --fix ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

deps-update: ## Update dependencies
	@go get -u ./...
	@go mod tidy

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run: ## Run Docker container
	@docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE)

docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image..."
	@docker push $(DOCKER_IMAGE)

migrate: ## Run database migrations
	@echo "Running migrations..."
	@go run scripts/migrate.go

seed: ## Seed database
	@echo "Seeding database..."
	@go run scripts/seed.go

dev: ## Run in development mode
	@LOG_CHANNEL=stdout LOG_LEVEL=debug go run ./cmd/main.go

prod: build ## Run in production mode
	@./bin/$(APP_NAME)

fmt: ## Format code
	@go fmt ./...

vet: ## Run go vet
	@go vet ./...

security: ## Run security scan
	@gosec ./...

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@GOOS=linux GOARCH=amd64 go build -o bin/$(APP_NAME)-linux-amd64 ./cmd/main.go
	@GOOS=linux GOARCH=arm64 go build -o bin/$(APP_NAME)-linux-arm64 ./cmd/main.go
	@GOOS=windows GOARCH=amd64 go build -o bin/$(APP_NAME)-windows-amd64.exe ./cmd/main.go
	@GOOS=darwin GOARCH=amd64 go build -o bin/$(APP_NAME)-darwin-amd64 ./cmd/main.go
	@GOOS=darwin GOARCH=arm64 go build -o bin/$(APP_NAME)-darwin-arm64 ./cmd/main.go
	@echo "Builds complete in bin/"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Tools installed"

ci: lint test ## Run CI checks locally
	@echo "CI checks complete"


messaging: ## Run the messaging engine with the NestJS settings (NestJS: DELIVERY_ENGINE=go)
	go run ./cmd/messaging-worker -env ../backend-nestjs/.env

messaging-build: ## Build the messaging engine binary
	go build -o bin/messaging-worker ./cmd/messaging-worker

messaging-test: ## Unit tests of the messaging engine (end to end: backend-nestjs test/go-engine.e2e-spec.ts)
	go test ./internal/messaging/... -count=1
