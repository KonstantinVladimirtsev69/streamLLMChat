# Phase 5 Plan 02 Summary: Message Persistence, Context Sliding Window & Stateful Streaming (CHAT-02, CHAT-03)

**Execution Date:** 2026-09-09  
**Plan:** `05-02-PLAN.md`  
**Status:** Complete  

---

## 1. Summary of Changes

Реализовано сохранение истории сообщений в MongoDB, автоматическое именование диалогов по первому сообщению и формирование скользящего окна контекста для LLM (CHAT-02, CHAT-03):

1. **Модели запросов (`backend/internal/model/llm.go`)**:
   - Расширена структура `ChatCompletionRequest` полями `ChatID string` и `Content string` для поддержки указания ID диалога и упрощенной отправки текста запроса.

2. **Сервисный слой (`backend/internal/service/chat.go`)**:
   - Расширен интерфейс `ChatService` методами:
     - `SaveUserMessage(ctx, chatID, userID, content)` — сохранение реплики пользователя и триггер авто-наименования.
     - `SaveAssistantMessage(ctx, chatID, userID, content, usage)` — сохранение реплики ассистента с количеством токенов и стоимостью в копейках.
     - `AutoUpdateTitleIfNeeded(ctx, chatID, userID, content)` — авто-именование чата по первым 45 рунам (с суффиксом `...` при превышении), если заголовок был «Новый диалог».
     - `PreparePromptContext(ctx, chatID, userID, newMsg, maxHistory)` — проверка принадлежности чата пользователю (404 при несовпадении), выборка последних 20 сообщений из MongoDB с реверсом в хронологический порядок (`ASC`) и добавление нового пользовательского сообщения.

3. **HTTP Хэндлер (`backend/internal/server/handler/chat.go`)**:
   - Обновлен метод `StreamChat` с поддержкой гибридного стриминга (D-04):
     - При наличии `chat_id`: верификация доступа к диалогу, формирование скользящего окна контекста (`PreparePromptContext`), предварительное сохранение сообщения пользователя (`SaveUserMessage`), обновление активности диалога (`TouchChat`).
     - При завершении потока (`StreamEventDone`): списание баланса за токены, сохранение ответа ассистента (`SaveAssistantMessage`) и вызов `TouchChat`.
     - При обрыве соединения клиентом (`<-r.Context().Done()`): переключение на detached context с таймаутом (5 сек) для биллинга и гарантированного сохранения частичного ответа ассистента в MongoDB.
     - При отсутствии `chat_id`: сохранение полной обратной совместимости со stateless-режимом Phase 4.

4. **Тестирование и верификация**:
   - `backend/internal/service/chat_test.go`: unit-тесты для сборки 20-сообщенческого скользящего окна (`PreparePromptContext`), авто-титрования (`AutoUpdateTitleIfNeeded`), сохранения пользовательских и ассистентских реплик.
   - `backend/internal/server/handler/chat_crud_test.go`: интеграционные тесты хэндлера `StreamChat` на сохранение сообщений в MongoDB, передачу контекста, возврат HTTP 404 при передаче чужого `chat_id` и сохранение обратной совместимости для stateless-запросов.
   - `backend/internal/server/chat_e2e_test.go`: сквозной E2E-тест (`TestChatLifecycle_E2E`), проверяющий создание диалога, авто-переименование по первому сообщению, многоходовый диалог со скользящим контекстом памяти, проверку чужого доступа (HTTP 404), переименование чата (`PATCH`) и каскадное удаление сообщений при удалении диалога (`DELETE`).

---

## 2. Verification Results

- `go test -v ./internal/service/...` — PASS
- `go test -v ./internal/server/handler/...` — PASS
- `go test -v ./internal/server/...` — PASS
- `go test -v ./...` — PASS (весь бэкенд без ошибок)
- `go build ./...` — PASS
