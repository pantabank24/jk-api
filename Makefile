.PHONY: help dev build run clean docker-build docker-up docker-down docker-restart docker-logs migrate test

# Variables
APP_NAME=jk-api
MAIN_FILE=./cmd/main.go
BINARY=./bin/server
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

# Load .env
include .env
export

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==================== Development ====================

dev: ## Run in development mode
	go run $(MAIN_FILE)

air: ## Run in development mode with live reload (requires: go install github.com/air-verse/air@latest)
	air

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	@mkdir -p ./bin
	go build -ldflags="-w -s" -o $(BINARY) $(MAIN_FILE)
	@echo "Build complete: $(BINARY)"

run: build ## Build and run the application
	$(BINARY)

clean: ## Clean build artifacts
	@rm -rf ./bin
	@echo "Cleaned!"

tidy: ## Tidy go modules
	go mod tidy

test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# ==================== Database Migrations ====================

migrate-up: ## Run all pending migrations
	migrate -path migrations -database "$(DB_URL)" -verbose up

migrate-down: ## Rollback last migration
	migrate -path migrations -database "$(DB_URL)" -verbose down 1

migrate-down-all: ## Rollback all migrations
	migrate -path migrations -database "$(DB_URL)" -verbose down

migrate-status: ## Show migration status
	migrate -path migrations -database "$(DB_URL)" version

migrate-force: ## Force migration version (use: make migrate-force version=N)
	migrate -path migrations -database "$(DB_URL)" force $(version)

migrate-create: ## Create new migration (use: make migrate-create name=xxx)
	migrate create -ext sql -dir migrations -seq $(name)

# ==================== Docker ====================

docker-build: ## Build Docker image
	docker compose build

docker-up: ## Start all containers
	docker compose up -d

docker-down: ## Stop all containers
	docker compose down

docker-restart: ## Restart all containers
	docker compose down
	docker compose up -d

docker-logs: ## Show container logs
	docker compose logs -f

docker-clean: ## Remove all containers, volumes, and images
	docker compose down -v --rmi all

# ==================== Database ====================

db-up: ## Start only PostgreSQL
	docker compose up -d postgres

db-down: ## Stop PostgreSQL
	docker compose stop postgres

# ==================== Deploy ====================

deploy: docker-build docker-up ## Build and deploy with Docker
	@echo "✅ Deployed successfully!"

deploy-clean: docker-clean deploy ## Clean deploy (rebuild everything)
	@echo "✅ Clean deploy complete!"
