---
phase: 02-database-layer-data-models
plan: 01
status: complete
requirements:
  - DATA-01
created: 2026-09-08T21:43:40Z
---

# Plan 02-01 Summary: PostgreSQL Migrations & Domain Models

## Plan Goal
Создать схемы и миграции PostgreSQL для пользователей, балансов, истории транзакций и рефералов, определить Go-модели сущностей и реализовать встроенный запуск миграций через `golang-migrate` с поддержкой команд в Makefile.

## Delivered Artifacts
1. **PostgreSQL Migrations (`backend/migrations/`):**
   - `000001_init_schema.up.sql`: таблицы `users`, `balances` (с ограничением `amount_kopecks >= 0`), `balance_transactions`, `referrals` (с ограничением `referrer_id <> referee_id`) и B-tree индексы.
   - `000001_init_schema.down.sql`: безопасный откат схемы.
   - `migrations.go`: внедрение миграций в бинарник через `//go:embed *.sql`.
2. **Go Domain Models (`backend/internal/model/`):**
   - `user.go`: структура `User` с JSON тегами.
   - `balance.go`: структура `Balance` и константы `WelcomeBonusKopecks = 500` (+5 руб), `ReferralRewardKopecks = 200` (+2 руб).
   - `transaction.go`: структура `BalanceTransaction` и типы транзакций (`welcome_bonus`, `referral_reward`, `token_charge`, `deposit`, `adjustment`).
   - `referral.go`: структура `Referral`.
   - `errors.go`: доменные ошибки `ErrNotFound`, `ErrInsufficientBalance`, `ErrUserAlreadyExists`.
3. **Migration Runner & CLI (`backend/internal/database/`, `backend/cmd/migrate/`):**
   - `migrate.go`: функции `Up(databaseURL)` и `Down(databaseURL)` на базе `golang-migrate/migrate/v4` и `source/iofs`.
   - `main.go`: CLI утилита применения и отката миграций.
   - `Makefile`: добавлены цели `migrate-up` и `migrate-down`.

## Verification
- `go -C backend build ./...` успешно компилирует все новые пакеты.
- `make test && make lint` проходит со 100% успехом и 0 замечаний линтеров.
