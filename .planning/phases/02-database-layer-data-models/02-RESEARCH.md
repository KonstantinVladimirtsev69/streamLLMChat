# Phase 2: Database Layer & Data Models — Technical Research

**Researched:** 2026-09-08
**Phase:** 02-database-layer-data-models
**Scope:** PostgreSQL 18+ (pgx/v5 + golang-migrate), MongoDB 8+ (mongo-driver/v2), Go DAL & Financial Transaction Management

---

## 1. Executive Summary

Фаза 2 закладывает фундамент хранения данных проекта:
1. **Реляционная часть (PostgreSQL 18+)**: отвечает за учетные записи пользователей, реферальную сеть, балансы в копейках и неизменяемую историю транзакций. Обеспечивает строгие гарантии ACID для предотвращения двойных списаний и ухода баланса в минус при конкурентных запросах.
2. **Документная часть (MongoDB 8+)**: отвечает за гибкое хранение сессий диалогов и цепочек сообщений чатов с токеновыми метаданными.
3. **Data Access Layer (DAL)**: предоставляет чистые Go-интерфейсы репозиториев (`UserRepository`, `BalanceRepository`, `ChatRepository`, `MessageRepository`) и контекстный транзакционный менеджер (`TxManager`).

---

## 2. Stack & Dependency Analysis

| Component | Library / Driver | Version | Purpose |
|---|---|---|---|
| PostgreSQL Client | `github.com/jackc/pgx/v5` | `v5.11.0` | Высокопроизводительный пул соединений (`pgxpool`), нативная поддержка типов Postgres, работа с транзакциями |
| PostgreSQL Migrations | `github.com/golang-migrate/migrate/v4` | `v4.19.1` | Версионирование схемы БД через SQL файлы (`.up.sql` / `.down.sql`), поддержка `source/iofs` для `//go:embed` |
| Migrate PGX Driver | `github.com/golang-migrate/migrate/v4/database/pgx/v5` | `v4.19.1` | Прямая интеграция `golang-migrate` с пулом `pgx/v5` |
| MongoDB Client | `go.mongodb.org/mongo-driver/v2` | `v2.9.0` | Официальный современный драйвер для MongoDB 8+ с улучшенной производительностью и строгой типизацией BSON |

---

## 3. Database Schemas & Data Modeling

### 3.1 PostgreSQL DDL (`backend/migrations/000001_init_schema.up.sql`)

```sql
-- Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    vk_id BIGINT NOT NULL UNIQUE,
    first_name VARCHAR(128) NOT NULL DEFAULT '',
    last_name VARCHAR(128) NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    ref_code VARCHAR(32) NOT NULL UNIQUE,
    referred_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица балансов (1 пользователь = 1 баланс)
CREATE TABLE IF NOT EXISTS balances (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    amount_kopecks BIGINT NOT NULL DEFAULT 0 CHECK (amount_kopecks >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица транзакций баланса (append-only ledger)
CREATE TABLE IF NOT EXISTS balance_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_kopecks BIGINT NOT NULL,
    balance_after_kopecks BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('welcome_bonus', 'referral_reward', 'token_charge', 'deposit', 'adjustment')),
    reference_id VARCHAR(64),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица реферальных связей
CREATE TABLE IF NOT EXISTS referrals (
    id BIGSERIAL PRIMARY KEY,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referee_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    reward_kopecks BIGINT NOT NULL DEFAULT 200,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_no_self_referral CHECK (referrer_id <> referee_id)
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_users_vk_id ON users(vk_id);
CREATE INDEX IF NOT EXISTS idx_users_ref_code ON users(ref_code);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON balance_transactions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer_id ON referrals(referrer_id);
```

### 3.2 MongoDB Collections & Indexes

1. **Коллекция `chats`**:
   - Поля: `_id` (ObjectID), `user_id` (int64), `title` (string), `model` (string), `created_at` (time.Time), `updated_at` (time.Time).
   - Индекс: `{ user_id: 1, updated_at: -1 }` — обеспечивает быструю выборку списка диалогов пользователя, отсортированных по последней активности.
2. **Коллекция `messages`**:
   - Поля: `_id` (ObjectID), `chat_id` (ObjectID), `user_id` (int64), `role` (string), `content` (string), `prompt_tokens` (int), `completion_tokens` (int), `total_tokens` (int), `cost_kopecks` (int64), `created_at` (time.Time).
   - Индекс: `{ chat_id: 1, created_at: 1 }` — обеспечивает упорядоченную выборку истории сообщений диалога для сборки контекста LLM.

---

## 4. Pattern Design: Context-based Transaction Manager

Для исключения привязки бизнес-слоя к `pgx.Tx` и предотвращения передачи транзакций параметрами в каждый метод используется паттерн контекстного `TxManager`:

```go
type txKey struct{}

func InjectTx(ctx context.Context, tx pgx.Tx) context.Context {
    return context.WithValue(ctx, txKey{}, tx)
}

func ExtractTx(ctx context.Context) (pgx.Tx, bool) {
    tx, ok := ctx.Value(txKey{}).(pgx.Tx)
    return tx, ok
}
```

Вспомогательный интерфейс `DBTX`:
```go
type DBTX interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

Репозиторий получает исполнителя:
```go
func (r *balanceRepo) getDB(ctx context.Context) DBTX {
    if tx, ok := ExtractTx(ctx); ok {
        return tx
    }
    return r.pool
}
```

### Финансовая безопасность списаний:
Запрос списания токенов:
```sql
UPDATE balances
SET amount_kopecks = amount_kopecks - $1, updated_at = NOW()
WHERE user_id = $2 AND amount_kopecks >= $1
RETURNING amount_kopecks;
```
Если возвращено 0 строк, значит баланс меньше требуемой суммы списания -> возвращается доменная ошибка `model.ErrInsufficientBalance`.

---

## 5. Verification Strategy

1. **Unit-тесты (`go test -v ./...`)**:
   - Тестирование моделей и валидаторов.
   - Тестирование парсинга ошибок БД в доменные ошибки.
2. **Интеграционные тесты (`go test -tags=integration ./...`)**:
   - Автоматический запуск миграций на реальном PostgreSQL контейнере.
   - Тест сценария: создание пользователя -> начисление 500 коп (+5 руб) -> списание 150 коп -> проверка баланса 350 коп и записей в `balance_transactions`.
   - Тест конкурентных списаний (10 параллельных горутин списывают сумму, когда на балансе хватает только на 2) с проверкой отсутствия гонок и `CHECK` нарушений.
   - Тест MongoDB: создание диалога, добавление сообщений, проверка составных индексов через `EnsureIndexes`.

---

*Research completed: 2026-09-08*
