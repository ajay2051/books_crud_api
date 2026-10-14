# Load .env if it exists so DATABASE_URL is available to make targets.
-include .env
export

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/booksdb?sslmode=disable

.PHONY: help run build test fmt vet tidy db-up db-down migrate-install migrate-up migrate-down migrate-create docker-up docker-down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

run: ## Run the API locally
	go run ./cmd/api

build: ## Build binary into ./bin/api
	go build -o bin/api ./cmd/api

test: ## Run tests
	go test ./... -cover

fmt: ## Format code
	gofmt -w .

vet: ## Static checks
	go vet ./...

tidy: ## Sync go.mod / go.sum with the code
	go mod tidy

# db-up: ## Start only PostgreSQL in Docker
# 	docker compose up -d db
# 
# db-down: ## Stop PostgreSQL
# 	docker compose stop db

migrate-install: ## Install the golang-migrate CLI
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate-up: ## Apply all pending migrations
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back the last migration
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create: ## Create a new migration: make migrate-create name=add_isbn
	migrate create -ext sql -dir migrations -seq $(name)

# docker-up: ## Build and run API + PostgreSQL in Docker
# 	docker compose up --build -d
# 
# docker-down: ## Stop and remove containers (keeps data volume)
# 	docker compose down