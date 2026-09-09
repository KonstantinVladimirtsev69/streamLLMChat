# Phase 4: routerai.ru Integration & Billing Engine — Technical Research

**Researched:** 2026-09-09  
**Phase:** 04-routerai-ru-integration-billing-engine  
**Scope:** RouterAI API Client, Models Catalog & In-Memory TTL Cache, Real-time SSE Streaming & HTTP ResponseController, Token Billing Engine & PostgreSQL Overdraft Ledger  

---

## 1. Executive Summary

Фаза 4 интегрирует сервис с облачным агрегатором нейросетей **routerai.ru** и замыкает базовый экономический цикл продукта:
1. **RouterAI API Client & Mock Mode**: HTTP-клиент к OpenAI-совместимому API routerai.ru (`https://routerai.ru/api/v1`) для получения списка моделей и стриминга ответов. Автономный `MockLLMProvider` обеспечивает 100% покрытие unit- и integration-тестами без внешних сетевых вызовов и без утечки боевых токенов.
2. **Каталог моделей и In-Memory TTL кэш**: Эндпоинт `GET /api/v1/models` возвращает список доступных текстовых моделей и их тарифов за 1M входных/выходных токенов в рублях. Кэширование в памяти (TTL 15 минут) предотвращает спам запросов к провайдеру, а встроенный статический словарь моделей служит отказоустойчивым fallback при недоступности внешнего API.
3. **Pre-generation Balance Gate**: Перед отправкой запроса к LLM бэкенд проверяет `amount_kopecks > 0`. Если у пользователя нулевой или отрицательный баланс, запрос немедленно отклоняется с кодом `HTTP 402 Payment Required` (`{"error": "insufficient_balance"}`).
4. **SSE-стриминг (Server-Sent Events)**: Защищенный эндпоинт `POST /api/v1/chat/stream` принимает JSON с выбранной моделью и массивом сообщений `messages`. Используется `http.NewResponseController(w).SetWriteDeadline(time.Time{})` для снятия 15-секундного ограничения `WriteTimeout` сервера Go. Клиенту отправляются нормализованные JSON-события (`delta`, `done` с деталями токенов/стоимости, `error`). Заголовок `X-Accel-Buffering: no` гарантирует мгновенную доставку чанков через reverse proxy (Dokploy / Nginx / Traefik).
5. **Атомарный биллинг и овердрафт**: По завершении стриминга (или при обрыве клиентского соединения) вычисляется стоимость: `ceil((prompt_tokens * prompt_price + completion_tokens * completion_price) / 1_000_000 * 100)` копеек. Метод `BalanceRepository.DeductUsage` списывает сумму с баланса (допуская уход в минус, если генерация превысила остаток) и атомарно регистрирует транзакцию `token_charge` в `balance_transactions`.

---

## 2. Dependencies & Standard Libraries

| Dependency / Package | Version | Purpose |
|---|---|---|
| `net/http` | stdlib | HTTP-клиент к routerai.ru с `http.Client`, SSE-хэндлер и `http.ResponseController` |
| `github.com/go-chi/chi/v5` | `v5.2.1` | Маршрутизация эндпоинтов `/api/v1/models` и `/api/v1/chat/stream` |
| `context` | stdlib | Отмена исходящих запросов к LLM при разрыве клиентского соединения (`r.Context()`) |
| `math` | stdlib | Строгое математическое округление вверх `math.Ceil` при переводе рублей в копейки |
| `sync` | stdlib | Потокобезопасная синхронизация кэша моделей (`sync.RWMutex`) |
| `backend/internal/database` | internal | `TxManager` и подключение к PostgreSQL `pgxpool` |
| `backend/internal/repository` | internal | `BalanceRepository` для проверки и списания баланса |
| `backend/internal/server/middleware` | internal | `AuthMiddleware` для извлечения `userID` из сессионных JWT cookies |

---

## 3. Protocol & API Flows

### 3.1 Архитектура взаимодействия с RouterAI и SSE стриминга

```
[Client / Frontend]             [Backend Server]                  [routerai.ru API]
        |                               |                                 |
        | 1. POST /api/v1/chat/stream   |                                 |
        |    {model, messages}          |                                 |
        |------------------------------>|                                 |
        |                               | 2. AuthMiddleware (JWT)         |
        |                               |    -> userID                    |
        |                               | 3. Balance Check:               |
        |                               |    GetByUserID(userID)          |
        |                               |    if balance <= 0 -> 402 Exit  |
        |                               |                                 |
        |                               | 4. POST /v1/chat/completions    |
        |                               |    {model, messages, stream:true}|
        |                               |-------------------------------->|
        |                               |                                 |
        |                               |<-- 5. SSE stream (OpenAI format)|
        |                               |                                 |
        | 6. HTTP 200 text/event-stream |                                 |
        |    data: {"type":"delta",...} |                                 |
        |<------------------------------|                                 |
        |    data: {"type":"delta",...} |                                 |
        |<------------------------------|                                 |
        |                               |<-- 7. Stream finish (usage info)|
        |                               |                                 |
        |                               | 8. Calculate Cost (math.Ceil)   |
        |                               | 9. DeductUsage(userID, cost)    |
        |                               |    -> INSERT balance_transaction|
        | 10. data: {"type":"done",     |                                 |
        |     "usage":{"cost_kop":..}}  |                                 |
        |<------------------------------|                                 |
        | (Connection closed cleanly)   |                                 |
```

### 3.2 Формат SSE событий платформы

1. **Текстовый чанк (дельты ответа)**:
```
data: {"type":"delta","content":"Привет"}

data: {"type":"delta","content":", чем могу"}

data: {"type":"delta","content":" помочь?"}

```

2. **Завершающий чанк (метаданные и биллинг)**:
```
data: {"type":"done","usage":{"prompt_tokens":18,"completion_tokens":42,"total_tokens":60,"cost_kopecks":4}}

```

3. **Событие ошибки**:
```
data: {"type":"error","error":"upstream_provider_unavailable"}

```

---

## 4. Расчет стоимости и правила списания баланса

1. **Формула расчета**:
   - Цена входных токенов: $P_{in}$ руб / 1 000 000 токенов.
   - Цена выходных токенов: $P_{out}$ руб / 1 000 000 токенов.
   - Итоговая стоимость в копейках:
     $$\text{cost\_kopecks} = \left\lceil \left( \frac{T_{in} \cdot P_{in} + T_{out} \cdot P_{out}}{1\,000\,000} \right) \cdot 100 \right\rceil$$
   - Если $\text{cost\_kopecks} = 0$ (например, пустой или бесплатный тестовый запрос), транзакция не пишется.
   - Если $\text{cost\_kopecks} > 0$, списывается ровно вычисленное число копеек.

2. **Механика овердрафта в PostgreSQL**:
   ```sql
   UPDATE balances
   SET amount_kopecks = amount_kopecks - $1,
       updated_at = NOW()
   WHERE user_id = $2
   RETURNING amount_kopecks, updated_at;
   ```
   В отличие от существующего метода `Deduct`, здесь отсутствует условие `AND amount_kopecks >= $1`. Это позволяет балансу пользователя при необходимости стать отрицательным (например, было 2 коп, а генерация стоила 5 коп -> баланс стал -3 коп). После этого пользователь не сможет начать новый диалог (сработает pre-check `balance <= 0` с HTTP 402), пока не пополнит баланс.

3. **Обработка разрыва соединения (Client Disconnect)**:
   - Контекст запроса клиента `r.Context()` связан с отменой исходящего HTTP-запроса к routerai.ru.
   - Если клиент отключается посреди генерации, чтение стрима прерывается.
   - Накопленные до момента разрыва токены тарифицируются, и стоимость списывается с баланса через независимый контекст с таймаутом (`context.WithTimeout(context.Background(), 5*time.Second)`), чтобы предотвратить потерю выручки из-за прерывания контекста запроса.

---

## 5. Security & Architectural Patterns

1. **Скрытие API ключа (`ROUTERAI_API_KEY`)**:
   - API ключ провайдера считывается из переменных окружения и хранится строго в экземпляре `RouterAIClient`.
   - Запрещено передавать ключ в заголовках ответов клиенту, сериализовать в DTO моделей или выводить в логи.
2. **Изоляция мок-режима**:
   - `MockLLMProvider` реализует тот же интерфейс `LLMProvider`, что и `RouterAIClient`.
   - Поддерживает конфигурируемый стриминг с эмуляцией задержек или мгновенным сбросом чанков, генерацию токенов usage и эмуляцию сетевых ошибок.
3. **Защита от зависших стримов**:
   - `RouterAIClient` использует HTTP-транспорт с `ResponseHeaderTimeout: 30s` и контекстными таймаутами на старт генерации.
   - Пинг/Keep-alive комментарии `: keep-alive\n\n` могут отправляться в SSE поток при задержках первого токена.

---

## 6. Validation Architecture

1. **Unit-тесты (`backend/internal/llm/`)**:
   - `pricing_test.go`: Тестирование формулы расчета стоимости токенов, граничных значений (0 токенов, 1 токен, огромные значения) и математического округления вверх (`math.Ceil`).
   - `cache_test.go`: Проверка in-memory TTL кэша каталога моделей (истечение TTL, возврат кэшированных данных, fallback на статические модели при ошибке).
   - `mock_test.go`: Проверка `MockLLMProvider` на корректную генерацию стриминговых чанков и расчет usage.
2. **Интеграционные тесты репозитория (`backend/internal/repository/postgres/`)**:
   - `balance_usage_test.go`: Проверка метода `DeductUsage` на успешное списание, уход в овердрафт (отрицательный баланс) и корректную запись в `balance_transactions` с типом `token_charge`.
3. **HTTP Integration Tests (`backend/internal/server/handler/`)**:
   - `GET /api/v1/models`:
     - 401 Unauthorized без JWT сессии.
     - 200 OK с JSON списком моделей при валидной сессии.
   - `POST /api/v1/chat/stream`:
     - 401 Unauthorized без JWT сессии.
     - 402 Payment Required при балансе <= 0 коп без вызова провайдера.
     - 200 OK text/event-stream при балансе > 0: получение чанков `delta`, финального `done` с usage, и проверка списания баланса в БД.
     - Тест клиентского дисконнекта: прерывание контекста клиента и проверка частичного списания средств.

---

*Research completed: 2026-09-09*
