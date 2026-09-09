# Phase 5: Chat Management & Persistence — Technical Research

**Researched:** 2026-09-09  
**Phase:** 05-chat-management-persistence  
**Scope:** MongoDB Persistence, Chat CRUD REST API, Message History, Sliding Window Context Assembly, Hybrid SSE Streaming & Cascade Deletion  

---

## 1. Executive Summary

Фаза 5 завершает формирование стейтфул-платформы для работы с LLM, обеспечивая долговечное хранение диалогов и сообщений в MongoDB:
1. **Chat Session CRUD & Tenant Isolation (CHAT-01)**: Реализация полного жизненного цикла управления диалогами (`POST/GET/PATCH/DELETE /api/v1/chats`) и получения сообщений (`GET /api/v1/chats/{id}/messages`). Все операции защищены `AuthMiddleware` и строго изолированы по `user_id`. При попытке обратиться к чужому или удаленному диалогу возвращается `HTTP 404 Not Found`.
2. **Persistence & Auto-titling (CHAT-02)**: При стриминге сообщений с указанием `chat_id` реплика пользователя сохраняется в MongoDB до вызова LLM. Если чат имеет заголовок по умолчанию («Новый диалог»), заголовок автоматически обновляется первыми 40–50 символами текста первого сообщения. Ответ ассистента со всеми метаданными токенов и стоимости сохраняется по окончании стриминга (или в detached context при обрыве клиентского соединения).
3. **Sliding Window Context Assembly (CHAT-03)**: Для передачи в LLM бэкенд извлекает из MongoDB последние 20 сообщений диалога в хронологическом порядке, добавляет новое сообщение пользователя и передает сформированную историю в `llm.Provider.StreamChat`. Сохраняется обратная совместимость: если `chat_id` не передан, обработчик работает в stateless-режиме Phase 4.
4. **Каскадное удаление (CHAT-01/CHAT-02)**: При удалении диалога (`DELETE /api/v1/chats/{id}`) из MongoDB удаляется документ чата и атомарно вызывается `coll.DeleteMany` по `chat_id` в коллекции `messages`, исключая появление осиротевших записей.

---

## 2. Dependencies & Existing Assets

| Component / File | Purpose | Changes in Phase 5 |
|---|---|---|
| `backend/internal/model/chat.go` | Доменная модель `Chat` | Существует, готова к использованию |
| `backend/internal/model/message.go` | Доменная модель `Message` | Существует, готова к использованию |
| `backend/internal/model/llm.go` | DTO `ChatCompletionRequest` | Добавить опциональные поля `ChatID string` и `Content string` |
| `backend/internal/repository/mongodb/chat.go` | Репозиторий диалогов | Добавить метод `Touch(ctx, id, userID, model)` для обновления `updated_at` и активной модели |
| `backend/internal/repository/mongodb/message.go` | Репозиторий сообщений | Добавить метод `DeleteByChatID(ctx, chatID)` для каскадного удаления и метод `ListRecentByChatID(ctx, chatID, limit)` |
| `backend/internal/repository/repository.go` | Интерфейсы репозиториев | Обновить сигнатуры `ChatRepository` и `MessageRepository` |
| `backend/internal/service/chat.go` | Сервисный слой `ChatService` | **[NEW]** Бизнес-логика диалогов, авто-тайтлинг, каскадное удаление, сборка контекста |
| `backend/internal/server/handler/chat.go` | Обработчики HTTP | Добавить REST-методы CRUD для чатов и расширить `StreamChat` интеграцией с `ChatService` |
| `backend/internal/server/server.go` | Роутер | Зарегистрировать маршруты `/api/v1/chats` в `RegisterChatRoutes` |
| `backend/main.go` | Точка входа | Инициализировать репозитории MongoDB, `ChatService` и передать в хэндлеры |

---

## 3. Architecture & Data Flow

### 3.1 Жизненный цикл отправки сообщения и сборки контекста

```
[Client]                      [Backend Server]                         [MongoDB]             [routerai.ru API]
   |                                 |                                     |                        |
   | 1. POST /api/v1/chat/stream     |                                     |                        |
   |    {chat_id, model, content}    |                                     |                        |
   |-------------------------------->|                                     |                        |
   |                                 | 2. Auth Check -> userID             |                        |
   |                                 | 3. Balance Check -> amount > 0      |                        |
   |                                 | 4. GetChat(chat_id, userID)         |                        |
   |                                 |------------------------------------>|                        |
   |                                 |<-- chat ok / not found (404)        |                        |
   |                                 |                                     |                        |
   |                                 | 5. SaveUserMessage(chat_id, content)|                        |
   |                                 |------------------------------------>|                        |
   |                                 | 6. AutoUpdateTitleIfNeeded()        |                        |
   |                                 | 7. ListRecent(chat_id, limit=20)    |                        |
   |                                 |------------------------------------>|                        |
   |                                 |<-- messages history                 |                        |
   |                                 |                                     |                        |
   |                                 | 8. Assemble []ChatMessage           |                        |
   |                                 |    (history + new message)          |                        |
   |                                 | 9. Provider.StreamChat()            |                        |
   |                                 |------------------------------------------------------------->|
   |                                 |                                     |                        |
   |                                 |<-- 10. SSE tokens                   |                        |<-- SSE stream
   | 11. SSE tokens (delta)          |                                     |                        |
   |<--------------------------------|                                     |                        |
   |                                 |                                     |                        |
   |                                 |<-- 12. Done + Usage                 |                        |
   |                                 | 13. DeductUsage(userID, cost)       |                        |
   |                                 | 14. SaveAssistantMessage(tokens,...) |                        |
   |                                 |------------------------------------>|                        |
   |                                 | 15. Touch(chat_id, model)           |                        |
   |                                 |------------------------------------>|                        |
   | 16. SSE done event              |                                     |                        |
   |<--------------------------------|                                     |                        |
```

### 3.2 Каскадное удаление диалога

```
DELETE /api/v1/chats/{id}
   │
   ├──> 1. ChatRepository.Delete(ctx, chatID, userID)
   │       └── filter: {_id: chatID, user_id: userID} -> if MatchedCount==0 return 404
   │
   └──> 2. MessageRepository.DeleteByChatID(ctx, chatID)
           └── filter: {chat_id: chatID} -> DeleteMany
```

---

## 4. Specific Implementation Details

### 4.1 Авто-генерация заголовка (Auto-titling)
- При сохранении первого сообщения в чат проверяется текущий заголовок (`chat.Title == "Новый диалог"`).
- Текст первого сообщения нормализуется (удаление ведущих/замыкающих пробелов и переводов строк).
- Извлекаются первые 45 рун:
```go
runes := []rune(strings.TrimSpace(prompt))
if len(runes) > 45 {
    title = string(runes[:45]) + "..."
} else {
    title = string(runes)
}
```
- Если сформированный заголовок не пустой, вызывается `chatRepo.UpdateTitle`.

### 4.2 Сборка скользящего окна контекста
- В MongoDB сообщения упорядочены по `created_at ASC` с индексом `{chat_id: 1, created_at: 1}`.
- Для получения последних $N=20$ сообщений:
```go
opts := options.Find().
    SetSort(bson.D{{Key: "created_at", Value: -1}}).
    SetLimit(int64(limit))
// Затем инвертировать срез в хронологический порядок перед отправкой в LLM
```
- Это гарантирует, что даже в диалогах с сотнями сообщений объем контекста остается строго ограниченным 20 последними репликами, не превышая лимиты токенов моделей.

### 4.3 Устойчивость к обрывам связи (Disconnect Resilience)
- Если клиент отключается во время генерации (`r.Context().Done()`):
```go
cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
// 1. Списание баланса за накопленные токены
chargeDeltas(cleanupCtx)
// 2. Сохранение частичного ответа в MongoDB
if chatID != bson.NilObjectID && generatedText != "" {
    _ = h.chatSvc.SaveAssistantMessage(cleanupCtx, chatID, userID, generatedText, *lastUsage, req.Model)
}
```

---

## 5. Risk Assessment & Verification Strategy

| Risk | Mitigation |
|---|---|
| Race condition при обновлении названия или последнего сообщения | Обновления идемпотентны; Mongo атомарно обновляет документы по `_id` |
| Потеря ответа ассистента при прерывании SSE | Выделенный `detached timeout context` для сохранения в Mongo |
| Утечка диалогов между пользователями | Принудительная фильтрация по `user_id` во всех запросах; возврат 404 |
| Неконсистентность при удалении чата | Метод `DeleteChat` в `ChatService` сначала удаляет чат, затем каскадно очищает сообщения |

---

*Research finalized: 2026-09-09*  
*Ready for plan generation (05-01-PLAN.md, 05-02-PLAN.md)*
