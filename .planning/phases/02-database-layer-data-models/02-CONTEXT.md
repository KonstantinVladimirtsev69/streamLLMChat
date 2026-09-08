# Phase 2: Database Layer & Data Models - Context

**Gathered:** 2026-09-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Фаза 2 обеспечивает создание слоя персистентности (DAL) для бэкенда на Go: схемы таблиц и миграции PostgreSQL 18+, коллекции и составные индексы MongoDB 8+, модели данных, интерфейсы репозиториев и финансовый транзакционный менеджер для безопасных ACID-операций с балансом пользователя.

</domain>

<spec_lock>
## Requirements (locked via SPEC.md)

**3 requirements are locked.** See `02-SPEC.md` for full requirements, boundaries, and acceptance criteria.

Downstream agents MUST read `02-SPEC.md` before planning or implementing. Requirements are not duplicated here.

**In scope (from SPEC.md):**
- Подключение зависимостей `jackc/pgx/v5` и `go.mongodb.org/mongo-driver/v2` в `backend/go.mod`.
- SQL-миграции для PostgreSQL в формате `golang-migrate` (`backend/migrations/`).
- Автоматическое применение миграций при старте и CLI/Makefile команды (`make migrate-up`, `make migrate-down`).
- Определение моделей Go для пользователей, балансов, транзакций, диалогов и сообщений.
- Репозиторный слой (DAL) с транзакционным выполнением финансовых операций.
- Автоматическая инициализация составных индексов в MongoDB.
- Unit и интеграционные тесты для PostgreSQL и MongoDB слоя с проверкой конкурентных списаний.

**Out of scope (from SPEC.md):**
- Эндпоинты аутентификации VK OAuth и выпуск JWT (это Фаза 3).
- Логика взаимодействия с API routerai.ru (это Фаза 4).
- HTTP-хэндлеры и SSE-потоки для чатов (это Фаза 4 и 5).
- Пользовательский интерфейс Next.js (это Фаза 6).

</spec_lock>

<decisions>
## Implementation Decisions

### 1. Архитектура транзакций (Transaction Management)
- **D-01 (Контекстный TxManager):** Реализуется паттерн `TxManager` с методом `WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error`. При старте транзакции объект `pgx.Tx` помещается в контекст запроса. Репозитории PostgreSQL автоматически проверяют наличие транзакции в контексте: если транзакция активна, запросы выполняются внутри неё, иначе используется базовый пул соединений `*pgxpool.Pool`. Сервисный слой не зависит от типов драйвера БД. — **Reversibility:** costly — изменение абстракции транзакций потребует переписывания вызовов во всех репозиториях и сервисах.

### 2. Финансовая целостность и баланс
- **D-02 (Хранение баланса в копейках):** Баланс пользователя хранится строго в целочисленном формате копеек (`BIGINT` `amount_kopecks`). 1 рубль = 100 копеек. Стартовый бонус = 500 копеек (+5 руб), реферальное вознаграждение = 200 копеек (+2 руб). На уровне таблицы `balances` наложено ограничение `CHECK (amount_kopecks >= 0)`. Каждое изменение баланса атомарно логируется в `balance_transactions`. — **Reversibility:** one-way — изменение типа поля баланса в БД потребует сложной миграции и пересчёта исторических транзакций.

### 3. Миграции и индексы баз данных
- **D-03 (Автомиграции PostgreSQL):** Миграции размещаются в `backend/migrations/*.sql` и встраиваются в бинарник через директиву Go `//go:embed`. При старте бэкенда автоматически запускается `migrate.Up()`. Дополнительно в корневой `Makefile` добавляются цели `migrate-up` и `migrate-down` для управления миграциями вручную. — **Reversibility:** reversible.
- **D-04 (Инициализация индексов MongoDB):** При подключении к MongoDB при старте приложения вызывается функция `EnsureIndexes(ctx)`, которая регистрирует составные индексы: `chats: { user_id: 1, updated_at: -1 }` и `messages: { chat_id: 1, created_at: 1 }`. Если индексы уже существуют, MongoDB пропускает их создание. — **Reversibility:** reversible.

### 4. Структура пакетов в Go Backend
- **D-05 (Идиоматичная модульная структура):**
  - `backend/internal/model/` — чистые структуры сущностей (`User`, `Balance`, `BalanceTransaction`, `Referral`, `Chat`, `Message`) и доменные ошибки (`ErrNotFound`, `ErrInsufficientBalance`, `ErrUserAlreadyExists`).
  - `backend/internal/database/` — инициализация пула `pgxpool.Pool` (`postgres.go`), клиента `mongo.Client` (`mongo.go`), контекстного транзакционного менеджера (`tx_manager.go`), запуск встроенных миграций (`migrate.go`).
  - `backend/internal/repository/` — интерфейсы репозиториев (`UserRepository`, `BalanceRepository`, `ChatRepository`, `MessageRepository`) и их реализации для PostgreSQL и MongoDB. — **Reversibility:** costly — реорганизация пакетов затронет импорты по всему коду.

### Agent's Discretion
- Точные имена SQL файлов миграций (`000001_create_users_and_balances.up.sql` и т.д.).
- Вспомогательные хелперы для сканирования строк `pgx.Rows`.
- Настройка таймаутов пулов соединений (например, `MaxConns`, `MinConns`, `MaxConnLifetime` в `pgxpool.Config`).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Contracts & Specifications
- `.planning/phases/02-database-layer-data-models/02-SPEC.md` — спецификация требований, границы и критерии приёмки Фазы 2.
- `.planning/REQUIREMENTS.md` §DATA — требования DATA-01, DATA-02, DATA-03.
- `.planning/ROADMAP.md` §Phase 2 — цели и ожидаемые результаты фазы.

### Infrastructure & Config
- `docker-compose.yml` — параметры сервисов `postgres:18-alpine` и `mongo:8.0`.
- `.env.example` — формат строк подключения `DATABASE_URL` и `MONGODB_URI`.
- `Makefile` — команды автоматизации сборки, тестов и линтеров.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/server/server.go` — HTTP сервер chi v5 с маршрутом `/health` и middleware (включая безопасный `ClientIPFromHeader`).
- `backend/internal/server/server_test.go` — паттерн unit-тестирования в Go с чистыми assertions.

### Established Patterns
- Go 1.27.1, строгий `golangci-lint` (gosec, revive, govet, errcheck, staticcheck). Любой создаваемый код обязан проходить `make lint` с 0 issues.
- Экспортируемые типы и функции должны сопровождаться doc-комментариями.
- Защита от race conditions: `go test -race ./...`.

### Integration Points
- `backend/main.go` — точка входа сервиса: здесь будет происходить инициализация соединений с Postgres и Mongo, запуск миграций и передача репозиториев в HTTP-сервер, а также закрытие пулов соединений в graceful shutdown.

</code_context>

<specifics>
## Specific Ideas

- Балансовые операции списания токенов должны использовать атомарный запрос:
  ```sql
  UPDATE balances
  SET amount_kopecks = amount_kopecks - $1, updated_at = NOW()
  WHERE user_id = $2 AND amount_kopecks >= $1
  RETURNING amount_kopecks;
  ```
  Это гарантирует, что даже при параллельных запросах баланс не уйдет в минус и не потребуется блокировать всю таблицу.
- Каждая финансовая операция сопровождается созданием записи в `balance_transactions` с типом (`welcome_bonus`, `referral_reward`, `token_charge`).

</specifics>

<deferred>
## Deferred Ideas

- Внешние платежные шлюзы (ЮKassa/Robokassa/СБП) — запланированы на v2 (BILL-01).
- Кэширование баланса в Redis — отложено до появления высоких нагрузок, сейчас PostgreSQL 18 с легкостью обеспечивает требуемую производительность.

</deferred>

---

*Phase: 02-database-layer-data-models*
*Context gathered: 2026-09-08*
*Next step: /gsd-plan-phase 2 — составление детального плана реализации*
