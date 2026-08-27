.PHONY: help run build test test-integration-auth test-worker-sql-alignment coverage lint fmt clean docker-up docker-down migrate-up migrate-down migrate-create migrate-status

# Variables
APP_NAME=labuda-backend
MAIN_PATH=./cmd/core_server
BUILD_DIR=./bin
MIGRATIONS_DIR=./migrations
DB_DSN=postgresql://labuda:labuda123@localhost:5432/labuda?sslmode=disable

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

# Database migration commands

migrate-up: ## Run database migrations (up)
	@echo "$(COLOR_YELLOW)Running migrations...$(COLOR_RESET)"
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" up
	@echo "$(COLOR_GREEN)Migrations complete$(COLOR_RESET)"

migrate-down: ## Rollback last migration
	@echo "$(COLOR_YELLOW)Rolling back migration...$(COLOR_RESET)"
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" down 1
	@echo "$(COLOR_GREEN)Rollback complete$(COLOR_RESET)"

migrate-create: ## Create new migration (usage: make migrate-create name=create_users_table)
	@echo "$(COLOR_YELLOW)Creating migration: $(name)$(COLOR_RESET)"
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)
	@echo "$(COLOR_GREEN)Migration files created$(COLOR_RESET)"

migrate-force: ## Force migration version (usage: make migrate-force version=1)
	@echo "$(COLOR_YELLOW)Forcing migration version to $(version)$(COLOR_RESET)"
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" force $(version)

migrate-status: ## Show migration status
	@echo "$(COLOR_YELLOW)Checking migration status...$(COLOR_RESET)"
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" version

migrate-reset: ## Drop all tables and re-run migrations
	@echo "$(COLOR_YELLOW)Resetting database...$(COLOR_RESET)"
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" drop -f
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" up
	@echo "$(COLOR_GREEN)Database reset complete$(COLOR_RESET)"

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

db-backup: ## Backup database
	@echo "$(COLOR_YELLOW)Creating database backup...$(COLOR_RESET)"
	docker exec labuda-postgres pg_dump -U labuda labuda > backup_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "$(COLOR_GREEN)Backup created$(COLOR_RESET)"

db-restore: ## Restore database (usage: make db-restore file=backup.sql)
	@echo "$(COLOR_YELLOW)Restoring database from $(file)...$(COLOR_RESET)"
	docker exec -i labuda-postgres psql -U labuda labuda < $(file)
	@echo "$(COLOR_GREEN)Restore complete$(COLOR_RESET)"
