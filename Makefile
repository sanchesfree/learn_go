# ============================================
# Makefile для booking-service
# ============================================
# make — показывает help
# make run — запускает локально (нужен PostgreSQL и Redis)
# make test — запускает тесты
# make docker-up — поднимает всё через docker-compose
# ============================================

.PHONY: help run build test lint docker-up docker-down clean fmt vet

# Default target
help: ## Показать этот help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# ============================================
# Development
# ============================================

run: ## Запустить API локально
	go run ./cmd/api

worker: ## Запустить worker локально
	go run ./cmd/worker

run-all: ## Запустить API и worker одновременно
	go run ./cmd/api &
	go run ./cmd/worker

# ============================================
# Build
# ============================================

build: ## Собрать бинарники
	CGO_ENABLED=0 go build -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -o bin/worker ./cmd/worker

build-linux: ## Собрать для Linux (для Docker)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/worker ./cmd/worker

# ============================================
# Testing
# ============================================

test: ## Запустить все тесты
	go test -v -race -cover ./...

test-short: ## Быстрые тесты (без интеграции)
	go test -v -short ./...

bench: ## Запустить бенчмарки
	go test -bench=. -benchmem ./...

# ============================================
# Code quality
# ============================================

fmt: ## Форматировать код (go fmt + goimports style)
	go fmt ./...

vet: ## Статический анализ
	go vet ./...

lint: fmt vet ## Всё вместе
	@echo "✅ Code is clean"

# ============================================
# Docker
# ============================================

docker-up: ## Поднять инфраструктуру (postgres + redis + api + worker)
	docker compose up --build -d

docker-down: ## Остановить docker-compose
	docker compose down

docker-logs: ## Логи всех сервисов
	docker compose logs -f

docker-clean: ## Удалить всё включая volumes
	docker compose down -v

# ============================================
# Database
# ============================================

migrate-up: ## Применить миграции (нужен psql)
	psql -h localhost -U postgres -d booking -f migrations/001_init.sql

# ============================================
# Cleanup
# ============================================

clean: ## Удалить бинарники
	rm -rf bin/

# ============================================
# Dependencies
# ============================================

tidy: ## Скачать зависимости
	go mod tidy

deps: tidy ## Алиас
