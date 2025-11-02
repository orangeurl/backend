# OrangeURL Backend Makefile

.PHONY: help build up down dev logs clean

# Default target
help: ## Show this help message
	@echo "OrangeURL Backend - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development commands
dev: ## Start development environment
	docker compose -f docker-compose.dev.yml up --build

dev-detached: ## Start development environment in background
	docker compose -f docker-compose.dev.yml up --build -d

dev-logs: ## Show development logs
	docker compose -f docker-compose.dev.yml logs -f

dev-down: ## Stop development environment
	docker compose -f docker-compose.dev.yml down

# Production commands
build: ## Build production images
	docker compose --env-file .env build

up: ## Start production environment
	docker compose --env-file .env up -d

down: ## Stop production environment
	docker compose down

logs: ## Show production logs
	docker compose logs -f

# Utility commands
clean: ## Clean up containers and volumes
	docker compose down -v
	docker compose -f docker-compose.dev.yml down -v
	docker system prune -f

# Health checks
health: ## Check service health
	@echo "Checking API health..."
	@curl -f http://localhost:3000/ || echo "API is not healthy"
