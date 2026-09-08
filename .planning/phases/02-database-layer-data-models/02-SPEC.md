# Phase 2: Database Layer & Data Models — Specification

**Created:** 2026-09-08
**Ambiguity score:** 0.13 (gate ≤ 0.20 passed)
**Requirements:** 3 locked (DATA-01, DATA-02, DATA-03)

---

## Goal

Спроектировать и реализовать слой персистентности и доступа к данным (DAL) для PostgreSQL 18+ и MongoDB 8+ в Go бэкенде: схемы данных, миграции, репозитории и транзакционный менеджер для безопасных финансовых операций с балансом пользователя (в копейках) и сохранением истории чатов.

## Background

В рамках Фазы 1 был развёрнут монорепозиторий, запущены контейнеры PostgreSQL 18 и MongoDB 8 в Docker Compose, настроен базовый HTTP-сервер на Go. На текущий момент в бэкенде отсутствуют драйверы баз данных, схемы таблиц/коллекций, миграции и структуры моделей. Для реализации последующих фаз (авторизация через VK, биллинг токенов routerai.ru и диалоги с LLM) требуется надёжный слой данных с гарантией ACID для финансовых операций.

---

## Requirements

### 1. DATA-01: Схема и миграции PostgreSQL (Users, Balances, Transactions, Referrals)
- **Current:** PostgreSQL запущен в docker-compose, но база данных `llmchat` пуста, файлов миграций нет.
- **Target:** Созданы SQL-миграции (`golang-migrate`) в директории `backend/migrations/`:
  - Таблица `users`: `id` (BIGSERIAL PRIMARY KEY), `vk_id` (BIGINT UNIQUE NOT NULL), `first_name` (VARCHAR(128)), `last_name` (VARCHAR(128)), `avatar_url` (TEXT), `ref_code` (VARCHAR(32) UNIQUE NOT NULL), `referred_by` (BIGINT REFERENCES users(id)), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW()), `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW()).
  - Таблица `balances`: `user_id` (BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE), `amount_kopecks` (BIGINT NOT NULL DEFAULT 0 CHECK (amount_kopecks >= 0)), `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW()).
  - Таблица `balance_transactions`: `id` (BIGSERIAL PRIMARY KEY), `user_id` (BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE), `amount_kopecks` (BIGINT NOT NULL), `balance_after_kopecks` (BIGINT NOT NULL), `type` (VARCHAR(32) NOT NULL — 'welcome_bonus', 'referral_reward', 'token_charge', 'deposit', 'adjustment'), `reference_id` (VARCHAR(64)), `description` (TEXT), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW()).
  - Таблица `referrals`: `id` (BIGSERIAL PRIMARY KEY), `referrer_id` (BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE), `referee_id` (BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE), `reward_kopecks` (BIGINT NOT NULL DEFAULT 200), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW()).
  - Индексы: `idx_users_vk_id`, `idx_users_ref_code`, `idx_transactions_user_id`, `idx_referrals_referrer_id`.
- **Acceptance:** Миграции успешно применяются (`up`) и откатываются (`down`) на PostgreSQL 18 без ошибок; схема содержит все таблицы, ограничения внешних ключей и `CHECK (amount_kopecks >= 0)`.

### 2. DATA-02: Схема и индексы MongoDB (Chats, Messages)
- **Current:** MongoDB 8 запущена в docker-compose, база `llmchat` не содержит коллекций.
- **Target:** Определены Go-структуры моделей BSON/JSON и инициализатор индексов:
  - Коллекция `chats`: `_id` (primitive.ObjectID), `user_id` (int64), `title` (string), `model` (string), `created_at` (time.Time), `updated_at` (time.Time).
  - Коллекция `messages`: `_id` (primitive.ObjectID), `chat_id` (primitive.ObjectID), `user_id` (int64), `role` (string: "user" | "assistant" | "system"), `content` (string), `prompt_tokens` (int), `completion_tokens` (int), `total_tokens` (int), `cost_kopecks` (int64), `created_at` (time.Time).
  - Индексы: составной индекс `chats: { user_id: 1, updated_at: -1 }`, составной индекс `messages: { chat_id: 1, created_at: 1 }`.
- **Acceptance:** Функция инициализации MongoDB создаёт требуемые коллекции и индексы; проверка через драйвер подтверждает наличие составных индексов.

### 3. DATA-03: Go DAL (Repository Layer) & Финансовый транзакционный менеджер
- **Current:** В `backend/` нет пакетов для взаимодействия с базами данных.
- **Target:** 
  - Подключение к PostgreSQL через пул `jackc/pgx/v5/pgxpool` с чтением параметров из переменной `DATABASE_URL`.
  - Подключение к MongoDB через официальный драйвер `go.mongodb.org/mongo-driver/v2` (или v1) с чтением из `MONGODB_URI`.
  - Интерфейсы и реализации репозиториев:
    - `UserRepository`: создание/поиск по `vk_id`, получение по `ref_code`.
    - `BalanceRepository`: получение текущего баланса, атомарное изменение баланса в рамках транзакции с добавлением записи в `balance_transactions`.
    - `ChatRepository`: CRUD операции для диалогов пользователя в MongoDB.
    - `MessageRepository`: добавление сообщений, постраничная выборка истории сообщений диалога.
  - Транзакционный менеджер для PostgreSQL: метод выполнения операций внутри транзакции с уровнем изоляции `Read Committed` / `Repeatable Read` и блокировкой `SELECT ... FOR UPDATE` (или атомарным декрементом с проверкой положительного остатка), предотвращающий race conditions при одновременных списаниях.
- **Acceptance:** 
  - Unit-тесты покрывают валидацию логики и ограничений моделей.
  - Интеграционные тесты проверяют:
    1. Регистрацию пользователя с созданием записи в `users` и начального баланса 0.
    2. Начисление стартового бонуса (500 копеек / 5 руб) с созданием транзакции.
    3. Атомарное списание за токены с предотвращением ухода баланса в минус при конкурентных запросах.
    4. Создание чата и добавление цепочки сообщений в MongoDB с последующей корректной выборкой.

---

## Boundaries

**In scope:**
- Подключение зависимостей `pgx/v5` и `mongo-driver` в `backend/go.mod`.
- SQL-миграции для PostgreSQL (`backend/migrations/`).
- Модуль автоматического или CLI-применения миграций (`backend/pkg/migrate` или `internal/database`).
- Определение моделей Go для пользователей, балансов, транзакций, диалогов и сообщений.
- Репозиторный слой (DAL) с транзакционным выполнением финансовых операций.
- Unit и интеграционные тесты для PostgreSQL и MongoDB слоя.

**Out of scope:**
- Эндпоинты аутентификации VK OAuth и выпуск JWT (это Фаза 3).
- Логика взаимодействия с API routerai.ru (это Фаза 4).
- HTTP-хэндлеры и SSE-потоки для чатов (это Фаза 4 и 5).
- Пользовательский интерфейс Next.js (это Фаза 6).

---

## Constraints

- **PostgreSQL 18+**: использование connection pool `pgxpool`, параметризованные запросы без конкатенации строк (защита от SQL-инъекций).
- **MongoDB 8+**: использование официального Go-драйвера, контекст с таймаутами при выполнении запросов.
- **Денежный формат**: Баланс хранится строго в копейках (`BIGINT`), 1 рубль = 100 копеек. Стартовый бонус = 500 копеек, реферальный бонус = 200 копеек. Баланс не может быть отрицательным (`CHECK (amount_kopecks >= 0)`).
- **ACID и финансовая безопасность**: Любое изменение баланса обязано сопровождаться созданием записи в таблице `balance_transactions` внутри одной неделимой транзакции PostgreSQL.

---

## Acceptance Criteria

- [ ] Файлы миграций PostgreSQL созданы в `backend/migrations/` и успешно поднимают/откатывают схему.
- [ ] Таблицы `users`, `balances`, `balance_transactions`, `referrals` создаются со всеми внешними ключами и ограничениями целостности.
- [ ] Ограничение `amount_kopecks >= 0` отклоняет попытку списания суммы, превышающей текущий баланс.
- [ ] Модели и индексы MongoDB (`chats`, `messages`) инициализируются корректно.
- [ ] Пакет `internal/database` (или `internal/dal`) предоставляет методы подключения к PostgreSQL и MongoDB с graceful shutdown.
- [ ] Реализованы репозитории с транзакционными операциями изменения баланса.
- [ ] Автотесты `go test -v ./...` проверяют транзакционность и конкурентные операции без гонок данных (`go test -race`).
- [ ] `make lint` и `make test` отрабатывают без ошибок.

---

## Ambiguity Report

| Dimension          | Score | Min  | Status | Notes                                       |
|--------------------|-------|------|--------|---------------------------------------------|
| Goal Clarity       | 0.90  | 0.75 | ✓      | Чёткий фокус на DAL, схемах и транзакциях  |
| Boundary Clarity   | 0.85  | 0.70 | ✓      | Явно исключены HTTP-хэндлеры и OAuth       |
| Constraint Clarity | 0.85  | 0.65 | ✓      | Зафиксированы pgx, mongo-driver, копейки    |
| Acceptance Criteria| 0.85  | 0.70 | ✓      | Сформулированы конкретные проверяемые тесты |
| **Ambiguity**      | **0.13** | **≤0.20** | **✓** | **Готово к переходу на discuss-phase**   |

---

## Interview Log

| Round | Perspective | Question summary | Decision locked |
|-------|-------------|------------------|-----------------|
| 1 | Researcher | Драйверы и библиотеки для PostgreSQL и MongoDB | `pgx/v5` (pgxpool) + `golang-migrate`; `mongo-driver` для Go |
| 1 | Researcher | Формат и точность хранения баланса | `BIGINT` в копейках (1 руб = 100 копеек, бонус 5 руб = 500 коп) |
| 1 | Boundary Keeper | Границы тестирования фазы | Репозитории, схемы, unit-тесты + интеграционные тесты с docker-compose |

---

*Phase: 02-database-layer-data-models*
*Spec created: 2026-09-08*
*Next step: /gsd-discuss-phase 2 — планирование архитектурных деталей реализации DAL*
