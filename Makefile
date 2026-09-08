# ==============================================================================
# LLM Chat Platform - Automation Makefile
# ==============================================================================

.DEFAULT_GOAL := help
SHELL := /bin/bash

.PHONY: help build run-backend run-frontend run test lint lint-fix docker-up docker-down docker-build clean

help: ## Показать справку по доступным командам
	@echo "Использование: make [цель]"
	@echo ""
	@echo "Доступные команды:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Собрать Go бэкенд и Next.js фронтенд
	@echo "==> Сборка Backend..."
	go -C backend build -o bin/server ./main.go
	@echo "==> Сборка Frontend..."
	npm --prefix frontend run build
	@echo "==> Сборка успешно завершена."

run-backend: ## Запустить Go бэкенд локально
	@echo "==> Запуск Backend на порту $${PORT:-8080}..."
	go -C backend run ./main.go

run-frontend: ## Запустить Next.js фронтенд локально в режиме разработки
	@echo "==> Запуск Frontend в режиме dev..."
	npm --prefix frontend run dev

test: ## Запустить тесты бэкенда и проверки фронтенда
	@echo "==> Тестирование Backend..."
	go -C backend test -v ./...
	@echo "==> Проверка типов Frontend..."
	npm --prefix frontend run lint || true

lint: ## Запустить линтеры для backend и frontend
	@echo "==> Запуск golangci-lint для Backend..."
	@cd backend && PATH="$$(go env GOPATH)/bin:$$PATH" golangci-lint run ./...
	@echo "==> Запуск ESLint для Frontend..."
	npm --prefix frontend run lint
	@echo "==> Все проверки линтеров успешно пройдены."

lint-fix: ## Автоматически исправить замечания линтеров
	@echo "==> Автоматическое исправление в Backend..."
	@cd backend && PATH="$$(go env GOPATH)/bin:$$PATH" golangci-lint run --fix ./...
	@echo "==> Автоматическое исправление в Frontend..."
	npm --prefix frontend run lint -- --fix

docker-up: ## Запустить PostgreSQL 18 и MongoDB 8 в Docker Compose
	@echo "==> Запуск баз данных в Docker Compose..."
	docker compose up -d

docker-down: ## Остановить локальные контейнеры баз данных
	@echo "==> Остановка контейнеров Docker Compose..."
	docker compose down

docker-build: ## Собрать локальные Docker-образы для backend и frontend
	@echo "==> Сборка Docker образа backend..."
	docker build -t llm-chat-backend:latest backend/
	@echo "==> Сборка Docker образа frontend..."
	docker build -t llm-chat-frontend:latest frontend/
	@echo "==> Образы успешно собраны."

clean: ## Очистить скомпилированные бинарники и кэш
	@echo "==> Очистка временных файлов..."
	rm -rf backend/bin
	rm -rf frontend/.next
	@echo "==> Очистка завершена."
