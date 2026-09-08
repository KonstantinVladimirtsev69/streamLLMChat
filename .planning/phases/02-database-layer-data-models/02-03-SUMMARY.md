---
phase: 02-database-layer-data-models
plan: 03
status: complete
requirements:
  - DATA-03
created: 2026-09-08T21:59:20Z
---

# Plan 02-03 Summary: Data Access Layer, Repositories & Transaction Manager

## Plan Goal
Реализовать пул соединений PostgreSQL (`pgxpool`), контекстный `TxManager`, интерфейсы репозиториев (`UserRepository`, `BalanceRepository`, `ChatRepository`, `MessageRepository`) с реализациями для PostgreSQL и MongoDB, и покрыть финансовые операции атомарного списания и конкурентных изменений тестами.

## Delivered Artifacts
1. **Connection Pool & Contextual TxManager (`backend/internal/database/`):**
   - `postgres.go`: `NewPostgresPool` с параметрами пула (MaxConns: 25, MinConns: 5, timeouts, healthcheck).
   - `tx_manager.go`: реализация контекстного менеджера `WithinTransaction`, интерфейс `DBTX`, инъекция и извлечение `pgx.Tx` из `context.Context` с авто-откатом при панике или ошибке.
2. **Repository Interfaces & Implementations (`backend/internal/repository/`):**
   - `repository.go`: интерфейсы `UserRepository`, `BalanceRepository`, `ChatRepository`, `MessageRepository`.
   - `postgres/user.go`: `UserRepository` (CRUD по ID, VKID, RefCode с обработкой уникальности).
   - `postgres/balance.go`: `BalanceRepository` с атомарным начислением бонусов (`AddBonus`), атомарным списанием (`Deduct` с `WHERE amount_kopecks >= $1`) и выборкой истории `balance_transactions`.
   - `mongodb/chat.go`: `ChatRepository` (Create, GetByID, ListByUserID, UpdateTitle, Delete).
   - `mongodb/message.go`: `MessageRepository` (Create, ListByChatID с хронологической сортировкой).
3. **Application Lifecycle Integration (`backend/main.go`):**
   - Автоматическая инициализация пулов БД при наличии переменных окружения `DATABASE_URL` и `MONGODB_URI`.
   - Автоматический запуск миграций PostgreSQL и регистрация составных индексов MongoDB при старте сервиса.
   - Корректный graceful shutdown с закрытием пулов соединений.
4. **Integration & Concurrency Tests:**
   - `balance_test.go`:
     - `TestBalanceLifecycleAndTransactions`: создание пользователя + стартовый бонус 500 коп (+5 руб) в транзакции -> списание 150 коп -> отказ с `ErrInsufficientBalance` при попытке уйти в минус -> проверка записей в `balance_transactions`.
     - `TestConcurrentBalanceDeductions`: 10 параллельных горутин списывают по 100 коп с баланса 350 коп. Ровно 3 успешных списания, 7 отказов, итоговый баланс 50 коп, 0 race conditions.
     - `TestTransactionRollbackOnPanicOrError`: проверка отката транзакции при ошибке.
   - `chat_test.go`: интеграционный тест создания чата, добавления сообщений, упорядоченной выборки, обновления заголовка и удаления диалога.

## Verification
- `go -C backend test -race -v ./...` — 100% PASS, гонок данных нет.
- `make test && make lint` — 0 issues.
