.PHONY: help run build test test-integration-auth test-worker-sql-alignment coverage lint fmt clean docker-up docker-down

# Variables
APP_NAME=labuda-backend
MAIN_PATH=./cmd/core_server
BUILD_DIR=./bin

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m

help: ## Show this help message
	@echo '$(COLOR_BOLD)Available targets:$(COLOR_RESET)'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Run the application
	@echo "$(COLOR_YELLOW)Starting application...$(COLOR_RESET)"
	go run $(MAIN_PATH)/main.go

build: ## Build the application
	@echo "$(COLOR_YELLOW)Building application...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)/main.go
	@echo "$(COLOR_GREEN)Build complete: $(BUILD_DIR)/$(APP_NAME)$(COLOR_RESET)"

test: ## Run tests
	@echo "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	go test -v ./...

test-integration-auth: ## Run auth integration tests (requires local Postgres with labuda_test database)
	@echo "$(COLOR_YELLOW)Running auth integration tests...$(COLOR_RESET)"
	go test -tags integration -v -timeout 120s ./internal/identity/auth/delivery/http/...
	@echo "$(COLOR_GREEN)Auth integration tests complete$(COLOR_RESET)"

test-worker-sql-alignment: ## Run worker SQL alignment proof against a disposable Postgres database
	@echo "$(COLOR_YELLOW)Running worker SQL alignment proof...$(COLOR_RESET)"
	go run ./cmd/worker_sql_alignment
	@echo "$(COLOR_GREEN)Worker SQL alignment proof complete$(COLOR_RESET)"

coverage: ## Run tests with coverage
	@echo "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)Coverage report generated: coverage.html$(COLOR_RESET)"

lint: ## Run linter
	@echo "$(COLOR_YELLOW)Running linter...$(COLOR_RESET)"
	golangci-lint run

fmt: ## Format code
	@echo "$(COLOR_YELLOW)Formatting code...$(COLOR_RESET)"
	go fmt ./...
	gofmt -w .

clean: ## Clean build artifacts
	@echo "$(COLOR_YELLOW)Cleaning...$(COLOR_RESET)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "$(COLOR_GREEN)Clean complete$(COLOR_RESET)"

# Docker commands

docker-up: ## Start Docker containers
	@echo "$(COLOR_YELLOW)Starting Docker containers...$(COLOR_RESET)"
	docker-compose up -d
	@echo "$(COLOR_GREEN)Docker containers started$(COLOR_RESET)"

docker-down: ## Stop Docker containers
	@echo "$(COLOR_YELLOW)Stopping Docker containers...$(COLOR_RESET)"
	docker-compose down
	@echo "$(COLOR_GREEN)Docker containers stopped$(COLOR_RESET)"

docker-logs: ## Show Docker logs
	docker-compose logs -f

docker-ps: ## Show running containers
	docker-compose ps

# Development commands

dev: ## Run with hot reload (requires air)
	@echo "$(COLOR_YELLOW)Starting development server with hot reload...$(COLOR_RESET)"
	air

install-tools: ## Install development tools
	@echo "$(COLOR_YELLOW)Installing development tools...$(COLOR_RESET)"
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(COLOR_GREEN)Tools installed$(COLOR_RESET)"

deps: ## Download dependencies
	@echo "$(COLOR_YELLOW)Downloading dependencies...$(COLOR_RESET)"
	go mod download
	go mod tidy
	@echo "$(COLOR_GREEN)Dependencies downloaded$(COLOR_RESET)"

# Database commands

db-connect: ## Connect to PostgreSQL
	docker exec -it labuda-postgres psql -U labuda -d labuda
