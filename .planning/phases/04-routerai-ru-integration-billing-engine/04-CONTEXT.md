# Phase 4: routerai.ru Integration & Billing Engine - Context

**Gathered:** 2026-09-09  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Фаза 4 реализует интеграцию с API routerai.ru в Go бэкенде: клиент каталога моделей и стриминга чата, защищенный эндпоинт `GET /api/v1/models` с кэшированием тарифов, защищенный эндпоинт `POST /api/v1/chat/stream` для потокового вывода через Server-Sent Events (SSE), предварительную проверку положительного баланса пользователя и атомарное списание стоимости израсходованных токенов в рублях (копейках) в PostgreSQL с фиксацией в журнале транзакций.
</domain>

<spec_lock>
## Requirements (locked via SPEC.md)

**4 requirements are locked.** See `04-SPEC.md` for full requirements, boundaries, and acceptance criteria.

Downstream agents MUST read `04-SPEC.md` before planning or implementing. Requirements are not duplicated here.

**In scope (from SPEC.md):**
- Go HTTP-клиент к routerai.ru с поддержкой OpenAI-совместимого API (каталог моделей и chat completions streaming).
- Эндпоинт `GET /api/v1/models` для получения списка доступных моделей и цен.
- Эндпоинт `POST /api/v1/chat/stream` для потокового вывода текста по протоколу Server-Sent Events (SSE).
- Валидация баланса перед началом генерации (`amount_kopecks > 0`).
- Формула расчета стоимости токенов: `ceil((prompt_tokens * prompt_price + completion_tokens * completion_price) / 1_000_000 * 100)`.
- Списание средств с баланса пользователя в PostgreSQL и фиксация в журнале `balance_transactions` с типом `token_charge`.
- Корректная обработка обрыва связи клиентом: отмена upstream-запроса через контекст и тарификация полученных до обрыва токенов.
- Потокобезопасный мок/стаб routerai.ru для unit- и integration-тестов без необходимости реального API-ключа.

**Out of scope (from SPEC.md):**
- Сохранение диалогов и истории переписки в MongoDB — это предмет Phase 5 (Chat Management & Persistence).
- Создание веб-интерфейса чата и селектора моделей в Next.js — это предмет Phase 6 (Next.js Frontend Interface).
- Пополнение баланса реальными деньгами через платежные шлюзы (ЮKassa/СБП) — запланировано в v2 (BILL-01).
- Мультимодальные запросы (картинки, документы) — запланировано в v2 (FEAT-01).

</spec_lock>

<decisions>
## Implementation Decisions

### 1. Архитектура LLM-клиента и провайдера
- **D-01 (LLMProvider Interface & Mock):** В пакете `internal/llm` выделяется интерфейс `LLMProvider` с методами `GetModels(ctx context.Context) ([]model.LLMModel, error)` и `StreamChat(ctx context.Context, req model.ChatCompletionRequest) (<-chan model.StreamEvent, <-chan error)`. Реализуется боевой клиент `RouterAIClient` и автономный `MockLLMProvider` для детерминированного тестирования без внешних сетевых вызовов. — **Reversibility:** costly — интерфейс провайдера внедряется в хэндлеры и сервисы чата.
- **D-02 (In-memory TTL кэш каталога моделей):** Список моделей и тарифов кэшируется в оперативной памяти с TTL 15 минут, исключая повторные сетевые запросы к routerai.ru на каждый запрос пользователя. Предусмотрен встроенный статический список моделей с ценами по умолчанию на случай сбоя или недоступности внешнего API. — **Reversibility:** reversible.

### 2. SSE-стриминг и управление таймаутами
- **D-03 (SSE Timeout Management):** Внутри HTTP-хэндлера стриминга используется `rc := http.NewResponseController(w); rc.SetWriteDeadline(time.Time{})` для снятия глобального 15-секундного ограничения `WriteTimeout`, что позволяет передавать генерации любой длительности. Глобальный сервер сохраняет безопасные `ReadTimeout` и `IdleTimeout`. — **Reversibility:** reversible.
- **D-04 (Normalized SSE Protocol):** Клиенту передается нормализованный прикладной поток SSE-событий в формате JSON:
  - `data: {"type":"delta","content":"..."}` — текстовые дельты ответа в реальном времени.
  - `data: {"type":"done","usage":{"prompt_tokens":N,"completion_tokens":N,"total_tokens":N,"cost_kopecks":N}}` — завершающее событие с метаданными токенов и биллинга.
  - `data: {"type":"error","error":"..."}` — уведомление об ошибке инференса. — **Reversibility:** costly — контракт формата сообщений напрямую потребляется фронтендом в Phase 6.

### 3. Биллинг и механизм овердрафта (the agent's Discretion)
- **D-05 (Atomic Deduction with Overdraft):** В `BalanceRepository` добавляется поддержка списания за использование токенов (`DeductUsage`), позволяющая балансу уходить в минус при генерации ответа, стоимость которого превысила остаток средств. Это гарантирует, что пользователь получит сгенерированный ответ, а платформа зафиксирует точную сумму задолженности в транзакционном журнале `balance_transactions`. — **Reversibility:** costly — логика списания баланса и целостность финансового регистра.
- **D-06 (Disconnect Token Accounting):** При закрытии соединения клиентом (отмена контекста `r.Context()`) upstream-запрос к routerai.ru отменяется, а фактически полученные до разрыва токены тарифицируются и списываются с баланса. — **Reversibility:** reversible.

### the agent's Discretion
- Выбор внутренней структуры очередей/каналов для стриминга SSE чанков внутри Go горутин.
- Конкретные наименования DTO структур в пакете `internal/model` для запросов к routerai.ru.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements & Contracts
- `.planning/phases/04-routerai-ru-integration-billing-engine/04-SPEC.md` — Зафиксированные требования, границы, edge coverage и критерии приемки Phase 4.

### Architecture & Prior Decisions
- `.planning/phases/03-authentication-referral-system/03-CONTEXT.md` — Сессии, Auth middleware, сервисный слой и конфигурация окружения.
- `.planning/phases/02-database-layer-data-models/02-CONTEXT.md` — Схема таблиц `balances`, `balance_transactions` и транзакционный менеджер.
- `.env.example` — Переменные конфигурации `ROUTERAI_API_KEY`, `ROUTERAI_BASE_URL`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/database/tx.go`: `TxManager` для выполнения операций в транзакции PostgreSQL.
- `backend/internal/repository/postgres/balance.go`: Репозиторий баланса и журнала транзакций.
- `backend/internal/server/middleware/auth.go`: `AuthMiddleware` для извлечения `userID` из сессионного токена.
- `backend/internal/auth/token.go`: Валидация JWT токенов.

### Established Patterns
- Чистый сервисный слой (`internal/service`), разделяющий бизнес-логику и HTTP-транспорт.
- Выделение интерфейсов провайдеров (`internal/auth/vk.go` -> `VKClientInterface`) с реализациями боевого клиента и мока для тестов.
- Использование chi-роутера с middleware и защищенными группами роутов (`r.Group(...)`).

### Integration Points
- Роуты моделей и чата: `internal/server/server.go` (`RegisterChatRoutes` или `RegisterLLMRoutes`).
- Инициализация и внедрение зависимостей в `backend/main.go`.

</code_context>

<specifics>
## Specific Ideas

- Формула округления стоимости: `ceil((prompt_tokens * prompt_price + completion_tokens * completion_price) / 1_000_000 * 100)` kopecks.
- Корректная обработка `text/event-stream` с обязательным вызовом `w.(http.Flusher).Flush()` после каждого отправленного чанка.

</specifics>

<deferred>
## Deferred Ideas

- Сохранение истории диалогов и сообщений в MongoDB — Фаза 5 (Chat Management & Persistence).
- Пользовательский веб-интерфейс чата с рендерингом Markdown и выбором моделей — Фаза 6 (Next.js Frontend Interface).
- Платежные шлюзы для пополнения баланса (ЮKassa / СБП) — v2 (BILL-01).

</deferred>

---

*Phase: 04-routerai-ru-integration-billing-engine*  
*Context gathered: 2026-09-09*  
