# Phase 5: Chat Management & Persistence - Context

**Gathered:** 2026-09-09  
**Status:** Ready for planning  

<domain>
## Phase Boundary

Фаза 5 переводит платформу из stateless-режима в полноценный stateful-чат с сохранением сессий диалогов и истории сообщений в MongoDB. Фаза охватывает реализацию сервисного слоя `ChatService`, REST API для управления диалогами (`POST/GET/PATCH/DELETE /api/v1/chats` и `GET /api/v1/chats/{id}/messages`), расширение эндпоинта стриминга `POST /api/v1/chat/stream` поддержкой `chat_id` со сборкой скользящего окна контекста (последние 20 сообщений), сохранением реплик пользователя и ассистента в MongoDB, авто-именованием диалогов по тексту первого сообщения, обновлением активной модели и каскадной очисткой сообщений при удалении чата со строгой изоляцией пользователей (HTTP 404 для чужих ресурсов).
</domain>

<spec_lock>
## Requirements (locked via SPEC.md)

**4 requirements are locked.** See `05-SPEC.md` for full requirements, boundaries, and acceptance criteria.

Downstream agents MUST read `05-SPEC.md` before planning or implementing. Requirements are not duplicated here.

**In scope (from SPEC.md):**
- Выделенный сервисный слой `ChatService` для диалогов и сообщений.
- REST API для диалогов:
  - `POST /api/v1/chats` (создание с дефолтным названием «Новый диалог»)
  - `GET /api/v1/chats` (список с пагинацией и сортировкой по `updated_at DESC`)
  - `GET /api/v1/chats/{id}` (получение информации о диалоге)
  - `PATCH /api/v1/chats/{id}` (переименование заголовка)
  - `DELETE /api/v1/chats/{id}` (удаление диалога с каскадным удалением сообщений)
  - `GET /api/v1/chats/{id}/messages` (хронологическая история сообщений с пагинацией)
- Расширение `POST /api/v1/chat/stream`:
  - Гибридный контракт: stateful при передаче `chat_id`, stateless при его отсутствии (Phase 4 backward compatibility).
  - Сборка скользящего окна контекста (до 20 предыдущих реплик из MongoDB).
  - Сохранение реплики пользователя перед началом вызова LLM.
  - Автоматическое обновление названия диалога по первым 40–50 символам первого сообщения.
  - Сохранение ответа ассистента по окончании генерации (`StreamEventDone`) с токенами и стоимостью.
  - Сохранение частичного ответа ассистента в detached context при обрыве связи клиентом.
  - Обновление поля `chat.Model` и `chat.UpdatedAt` при генерации.
- Метод `DeleteByChatID` в `MessageRepository` для каскадного удаления.
- Подключение репозиториев MongoDB и регистрация роутов в `backend/main.go`.
- Unit и E2E тесты на CRUD, изоляцию (404), скользящее окно, авто-тайтлинг и каскадное удаление.

**Out of scope (from SPEC.md):**
- Пользовательский интерфейс Next.js (сайдбар, селектор моделей, Markdown-рендеринг) — предмет Phase 6 (Next.js Frontend Interface).
- Полнотекстовый поиск по сообщениям в MongoDB (full-text search).
- Экспорт переписки в PDF или Markdown (запланировано в v2, FEAT-03).
- Ветвление диалогов (message branching / edit prior message).

</spec_lock>

<decisions>
## Implementation Decisions

### 1. Архитектура сервисного слоя и репозиториев MongoDB
- **D-01 (Dedicated ChatService):** Вся бизнес-логика диалогов инкапсулируется в `internal/service/chat.go` (`ChatService`), следуя паттерну `AuthService` и `BillingService`. Сервис принимает `ChatRepository` и `MessageRepository`, проверяет владение чатом (`user_id`), управляет авто-тайтлингом и формирует скользящее окно контекста. — **Reversibility:** costly — интерфейс сервиса пронизывает обработчики и роуты.
- **D-02 (Cascade Delete in MessageRepository):** В `MessageRepository` добавляется метод `DeleteByChatID(ctx context.Context, chatID bson.ObjectID) error`, реализующий `coll.DeleteMany(ctx, bson.D{{Key: "chat_id", Value: chatID}})`. При вызове `ChatService.DeleteChat` сначала удаляется запись чата, затем каскадно очищаются все его сообщения. — **Reversibility:** costly.
- **D-03 (Chat Model & Timestamp Updates):** В `ChatRepository` добавляется метод `Touch(ctx context.Context, id bson.ObjectID, userID int64, model string) error` (или `UpdateActivity`), обновляющий `updated_at = time.Now()` и `model` (если передан идентификатор модели), поддерживая актуальное состояние чата в сортировке списка. — **Reversibility:** reversible.

### 2. Контракт эндпоинтов и гибридный стриминг
- **D-04 (Hybrid Stream Endpoint Contract):** Эндпоинт `POST /api/v1/chat/stream` расширяет `ChatCompletionRequest`:
  - Поле `chat_id` (`string`, опциональное).
  - Поле `content` (`string`, опциональное) или массив `messages`.
  - Если `chat_id` указан:
    1. Проверяется существование и принадлежность чата текущему `user_id` (если нет — HTTP 404 Not Found).
    2. Извлекается текст сообщения пользователя.
    3. Сообщение пользователя сохраняется в MongoDB (`Role: "user"`).
    4. Если чат имеет заголовок по умолчанию («Новый диалог») и это первое сообщение, заголовок обновляется первыми 40–50 символами текста.
    5. Загружаются последние 20 сообщений из MongoDB для формирования контекста LLM.
    6. Выполняется стриминг через `llm.Provider`.
    7. Ответ ассистента сохраняется в MongoDB по событию `StreamEventDone` (или в detached context при обрыве клиентом).
  - Если `chat_id` не указан: выполняется стандартный stateless-стриминг Phase 4. — **Reversibility:** costly — публичный контракт API.
- **D-05 (404 on Unauthorized Access):** Любые попытки доступа к несуществующему или чужому диалогу возвращают HTTP 404 Not Found (`model.ErrNotFound`), предотвращая утечку информации о существовании идентификаторов других пользователей. — **Reversibility:** reversible.

### 3. Сборка контекста диалога (CHAT-03)
- **D-06 (Sliding Window Context Assembly):** Для передачи в LLM извлекаются последние $N=20$ сообщений чата из MongoDB:
  - Выборка `MessageRepository.ListByChatID` с лимитом 20 и сортировкой по `created_at DESC` (с последующим реверсом в хронологический порядок `ASC`) либо выборка последних сообщений.
  - Сообщения маппятся в `model.ChatMessage` (`role`, `content`).
  - Новая реплика пользователя ставится последней.
  - Это предотвращает раздувание контекста и контролирует затраты токенов пользователя. — **Reversibility:** reversible.

### the agent's Discretion
- Конкретные названия DTO-структур для ответов списка чатов (`ChatResponse`, `ChatListResponse`).
- Вспомогательная функция обрезки текста для авто-тайтлинга (ограничение до 50 рун с отсечением по границе слова и добавлением многоточия `...`).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements & Contracts
- `.planning/phases/05-chat-management-persistence/05-SPEC.md` — Зафиксированные требования, границы, edge coverage и критерии приемки Phase 5.
- `.planning/phases/04-routerai-ru-integration-billing-engine/04-SPEC.md` — Спецификация стриминга и биллинга токенов.

### Architecture & Prior Decisions
- `.planning/phases/04-routerai-ru-integration-billing-engine/04-CONTEXT.md` — SSE протокол, управление таймаутами и обработка обрыва связи.
- `.planning/phases/02-database-layer-data-models/02-CONTEXT.md` — Схемы `chats` и `messages` в MongoDB и составные индексы.
- `backend/internal/database/mongo.go` — Подключение к MongoDB и `EnsureIndexes`.
- `backend/internal/repository/mongodb/chat.go` — Базовый репозиторий диалогов.
- `backend/internal/repository/mongodb/message.go` — Базовый репозиторий сообщений.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/repository/mongodb/chat.go`: CRUD методы для коллекции `chats`.
- `backend/internal/repository/mongodb/message.go`: Методы `Create` и `ListByChatID` для коллекции `messages`.
- `backend/internal/database/mongo.go`: Индексы `(user_id, updated_at DESC)` и `(chat_id, created_at ASC)`.
- `backend/internal/server/handler/chat.go`: `ChatHandler` с обработчиками `GetModels` и `StreamChat`.
- `backend/internal/server/middleware/auth.go`: `UserIDFromContext` для безопасного получения `userID`.

### Established Patterns
- Сервисный слой (`internal/service`), разделяющий бизнес-логику и HTTP-транспорт.
- Репозиторные интерфейсы в `internal/repository/repository.go`.
- Защита эндпоинтов через middleware авторизации `appMiddleware.AuthMiddleware(tokenManager)`.
- Graceful shutdown для пулов соединений в `backend/main.go`.

### Integration Points
- Расширение `backend/internal/repository/repository.go` (`ChatRepository`, `MessageRepository`).
- Создание `backend/internal/service/chat.go` (`ChatService`).
- Добавление REST маршрутов для чатов в `backend/internal/server/server.go` (`RegisterChatRoutes`).
- Обновление `backend/internal/server/handler/chat.go` (`ChatHandler`).
- Инициализация и связывание компонентов в `backend/main.go`.

</code_context>

<specifics>
## Specific Ideas

- Функция авто-генерации заголовка: `generateChatTitle(content string) string` берет первые 40–50 символов (unicode-aware runes), обрезает лишние пробелы и добавляет `...` при превышении длины.
- Обработка обрыва связи: использование detached `context.WithTimeout(context.Background(), 5*time.Second)` для гарантированного сохранения частичного ответа ассистента в MongoDB параллельно со списанием токенов.

</specifics>

<deferred>
## Deferred Ideas

- Пользовательский веб-интерфейс чата на Next.js с сайдбаром диалогов — Фаза 6 (Next.js Frontend Interface).
- Полнотекстовый поиск по истории сообщений в MongoDB — Фаза 6 / v2.
- Внешние платежные шлюзы для пополнения баланса (ЮKassa / СБП) — v2 (BILL-01).
- Экспорт диалогов в Markdown / PDF — v2 (FEAT-03).

</deferred>

---

*Phase: 05-chat-management-persistence*  
*Context gathered: 2026-09-09*  
