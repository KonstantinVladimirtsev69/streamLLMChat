---
phase: 02-database-layer-data-models
plan: 02
status: complete
requirements:
  - DATA-02
created: 2026-09-08T21:45:10Z
---

# Plan 02-02 Summary: MongoDB Models & Indexing

## Plan Goal
Подключить официальный драйвер `go.mongodb.org/mongo-driver/v2`, определить структуры моделей для чатов и сообщений с метаданными токенов, реализовать модуль инициализации MongoDB клиента и автоматического создания составных индексов.

## Delivered Artifacts
1. **MongoDB Dependencies & Configuration:**
   - Подключена актуальная версия `go.mongodb.org/mongo-driver/v2` (v2.9.0).
2. **Go Domain Models (`backend/internal/model/`):**
   - `chat.go`: модель `Chat` с BSON/JSON тегами (`_id`, `user_id`, `title`, `model`, `created_at`, `updated_at`).
   - `message.go`: модель `Message` с поддержкой токенов (`prompt_tokens`, `completion_tokens`, `total_tokens`, `cost_kopecks`) и ролей сообщений.
3. **MongoDB Client & Index Management (`backend/internal/database/`):**
   - `mongo.go`: `NewMongoClient` с настройкой пула и таймаутов, `EnsureIndexes` для регистрации составных индексов:
     - `chats: { user_id: 1, updated_at: -1 }` (быстрый доступ к диалогам пользователя с сортировкой)
     - `messages: { chat_id: 1, created_at: 1 }` (упорядоченная хронология диалога)
4. **Unit Tests:**
   - `mongo_test.go`: `TestModelBSONSerialization` (проверка маршалинга/анмаршалинга BSON структур) и `TestNewMongoClientInvalidURI`.

## Verification
- `go -C backend test -v ./internal/database/...` — 100% green.
- `make test && make lint` — 0 issues.
