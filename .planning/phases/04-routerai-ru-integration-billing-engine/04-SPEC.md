# Phase 4: routerai.ru Integration & Billing Engine — Specification

**Created:** 2026-09-09  
**Ambiguity score:** 0.07 (gate: ≤ 0.20)  
**Requirements:** 4 locked  

## Goal

Интегрировать API routerai.ru в Go бэкенд, предоставив защищенный эндпоинт потокового вывода ответов через Server-Sent Events (SSE) с предварительной проверкой баланса и атомарным списанием стоимости израсходованных токенов в рублях (копейках) в PostgreSQL.

## Background

В фазах 1–3 реализованы фундамент монорепозитория, база данных PostgreSQL с таблицами `balances` и `balance_transactions`, аутентификация через VK OAuth со стартовым бонусом 5 руб и реферальными начислениями 2 руб. Однако сейчас у платформы отсутствует интеграция с провайдером нейросетей routerai.ru: нет клиента для каталога моделей и отправки сообщений, нет эндпоинта стриминга SSE, и баланс пользователя не уменьшается при генерациях. Пользователи имеют баланс, но не могут расходовать его на диалог с LLM.

## Requirements

1. **RouterAI API Client & Models Catalog (R1)**: Реализован HTTP-клиент к API routerai.ru с поддержкой каталога моделей, тарифов и потоковых запросов.
   - Current: В проекте нет кода взаимодействия с API routerai.ru.
   - Target: Клиент `routerai.Client` с таймаутами и поддержкой контекста выполняет запросы к `/api/v1/models` и `/api/v1/chat/completions` (OpenAI-compatible SSE stream), а бэкенд предоставляет защищенный эндпоинт `GET /api/v1/models` со списком доступных моделей и их тарифами.
   - Acceptance: Запрос `GET /api/v1/models` с валидным JWT возвращает HTTP 200 и JSON-массив моделей с идентификаторами и тарифами (цена за 1M входных/выходных токенов в рублях); при недоступности внешнего API возвращается HTTP 502/503.

2. **Pre-generation Balance Gate (R2)**: Проверка наличия положительного баланса пользователя перед стартом генерации.
   - Current: Отсутствует проверка баланса перед вызовами инференса.
   - Target: Запрос на стриминг чата блокируется с кодом HTTP 402 Payment Required до отправки запроса в routerai.ru, если баланс пользователя `amount_kopecks <= 0`.
   - Acceptance: При обращении пользователя с `amount_kopecks <= 0` к стриминг-эндпоинту сервер немедленно возвращает HTTP 402 с JSON `{ "error": "insufficient_balance" }` и не отправляет исходящий запрос к routerai.ru.

3. **Real-time SSE Chat Stream (R3)**: Потоковая передача ответа модели клиенту через Server-Sent Events (SSE).
   - Current: В бэкенде нет эндпоинтов потокового вывода.
   - Target: Эндпоинт `POST /api/v1/chat/stream` принимает JSON с выбранной моделью и массивом сообщений `messages`, проверяет JWT-сессию и удерживает HTTP-соединение с заголовками `Content-Type: text/event-stream`, отдавая входящие дельты текста в реальном времени. В конце стрима отправляется событие с метаданными о потраченных токенах и списанной сумме.
   - Acceptance: Вызов `POST /api/v1/chat/stream` передает непрерывный поток SSE-событий с `data: {"delta":"..."}` и завершающее событие `data: {"event":"done","usage":{...}}`, после чего соединение корректно закрывается.

4. **Token Cost Calculation & Atomic Balance Ledger Deduction (R4)**: Расчет стоимости генерации по тарифам модели и атомарное списание средств в транзакции PostgreSQL.
   - Current: Метод `Deduct` в `BalanceRepository` существует, но списание за токены нигде не вызывается.
   - Target: По завершении стриминга (или при обрыве связи) количество входных и выходных токенов умножается на тариф модели за 1M токенов; итоговая сумма в копейках округляется вверх до целого (`math.Ceil`), после чего атомарно списывается с баланса пользователя с записью транзакции типа `token_charge` в `balance_transactions`. Если генерация превысила остаток на балансе, баланс уходит в минус/овердрафт с сохранением целостности истории.
   - Acceptance: После завершения стриминга на 150 prompt-токенов и 350 completion-токенов в таблице `balance_transactions` появляется запись с `type = 'token_charge'`, корректной суммой списания в копейках и обновленным `balances.amount_kopecks`.

## Boundaries

**In scope:**
- Go HTTP-клиент к routerai.ru с поддержкой OpenAI-совместимого API (каталог моделей и chat completions streaming).
- Эндпоинт `GET /api/v1/models` для получения списка доступных моделей и цен.
- Эндпоинт `POST /api/v1/chat/stream` для потокового вывода текста по протоколу Server-Sent Events (SSE).
- Валидация баланса перед началом генерации (`amount_kopecks > 0`).
- Формула расчета стоимости токенов: `ceil((prompt_tokens * prompt_price + completion_tokens * completion_price) / 1_000_000 * 100)`.
- Списание средств с баланса пользователя в PostgreSQL и фиксация в журнале `balance_transactions` с типом `token_charge`.
- Корректная обработка обрыва связи клиентом: отмена upstream-запроса через контекст и тарификация полученных до обрыва токенов.
- Потокобезопасный мок/стаб routerai.ru для unit- и integration-тестов без необходимости реального API-ключа.

**Out of scope:**
- Сохранение диалогов и истории переписки в MongoDB — это предмет Phase 5 (Chat Management & Persistence).
- Создание веб-интерфейса чата и селектора моделей в Next.js — это предмет Phase 6 (Next.js Frontend Interface).
- Пополнение баланса реальными деньгами через платежные шлюзы (ЮKassa/СБП) — запланировано в v2 (BILL-01).
- Мультимодальные запросы (картинки, документы) — запланировано в v2 (FEAT-01).

## Constraints

- **Streaming Timeouts**: Настройки HTTP-сервера и роутера должны позволять длительные SSE-соединения (отключение или адаптивная настройка `WriteTimeout` для стриминговых обработчиков с использованием `http.ResponseController` или flush-механизма).
- **Security**: API-ключ `ROUTERAI_API_KEY` хранится исключительно на сервере в переменных окружения и ни при каких условиях не передается клиенту.
- **Financial Integrity**: Списание средств за токены выполняется строго в рамках транзакции PostgreSQL с защитой от race conditions; расчет копеек производится с округлением вверх (`math.Ceil`), исключая потерю выручки из-за дробных частей копеек.
- **Stateless Streaming**: Входной контракт `POST /api/v1/chat/stream` принимает массив сообщений формата `[{"role": "user"|"assistant"|"system", "content": "..."}]` без обязательной привязки к MongoDB ID чата.

## Acceptance Criteria

- [ ] Запрос `GET /api/v1/models` без авторизации возвращает HTTP 401 Unauthorized.
- [ ] Запрос `GET /api/v1/models` с валидным JWT возвращает HTTP 200 и список моделей с тарифами в рублях.
- [ ] Запрос `POST /api/v1/chat/stream` от пользователя с балансом <= 0 возвращает HTTP 402 Payment Required без вызова routerai.ru.
- [ ] Запрос `POST /api/v1/chat/stream` от пользователя с балансом > 0 возвращает заголовки `Content-Type: text/event-stream` и стримит чанки ответа.
- [ ] По завершении генерации отправляется завершающий SSE-пакет с метаданными usage (токены и списанная сумма).
- [ ] Стоимость токенов рассчитывается по тарифам модели с округлением вверх до целых копеек (`math.Ceil`).
- [ ] Сумма списывается с баланса пользователя в таблице `balances` и создается запись в `balance_transactions` с `type = 'token_charge'`.
- [ ] При обрыве соединения клиентом контекст upstream-запроса отменяется, а сгенерированные до момента отмены токены тарифицируются и списываются.
- [ ] При параллельных запросах баланс списывается корректно без race conditions.

## Edge Coverage

**Coverage:** 5/5 applicable edges resolved · 0 unresolved

| Category | Requirement | Status | Resolution / Reason |
|----------|-------------|--------|---------------------|
| concurrency | R1 | ✅ covered | Потокобезопасный HTTP-клиент с пулом соединений и контекстными таймаутами |
| concurrency | R2 | ✅ covered | Атомарная проверка баланса исключает двойной старт при параллельных запросах |
| unclassified (disconnect) | R3 | ✅ covered | Отмена upstream context при закрытии клиентского сокета; биллинг накопленных токенов |
| boundary | R4 | ✅ covered | При стоимости 0 коп транзакция не пишется; при овердрафте баланс уходит в минус с фиксацией в ledger |
| precision | R4 | ✅ covered | Математическое округление вверх (`math.Ceil`) в копейках за 1M токенов исключает потерю долей копеек |

## Prohibitions (must-NOT)

**Coverage:** 3/3 applicable prohibitions resolved · 0 unresolved

| Prohibition (must-NOT statement) | Requirement | Status | Verification / Reason |
|----------------------------------|-------------|--------|------------------------|
| MUST NOT передавать или логировать `ROUTERAI_API_KEY` в ответах клиенту или открытых логах | R1 | resolved | verification: test (автотест проверяет отсутствие API ключа в хедерах и теле ответа) |
| MUST NOT инициировать генерацию к внешнему LLM API, если баланс пользователя <= 0 | R2 | resolved | verification: test (автотест на возврат HTTP 402 при балансе 0 и отрицательном) |
| MUST NOT обнулять или игнорировать списание токенов при преждевременном разрыве SSE-соединения | R3, R4 | resolved | verification: test (автотест на частичное списание при context.Canceled) |

## Ambiguity Report

| Dimension          | Score | Min  | Status | Notes                                |
|--------------------|-------|------|--------|--------------------------------------|
| Goal Clarity       | 0.95  | 0.75 | ✓      | Четко определены клиент, стрим, биллинг |
| Boundary Clarity   | 0.95  | 0.70 | ✓      | Сохранение в Mongo отнесено к Phase 5  |
| Constraint Clarity | 0.90  | 0.65 | ✓      | Округление вверх, обработка таймаутов |
| Acceptance Criteria| 0.90  | 0.70 | ✓      | 9 строгих pass/fail критериев        |
| **Ambiguity**      | 0.07  | ≤0.20| ✓      | Gate успешно пройден                 |

## Interview Log

| Round | Perspective     | Question summary                         | Decision locked                                       |
|-------|-----------------|------------------------------------------|-------------------------------------------------------|
| 1     | Researcher      | Тарификация и валюта (расчет стоимости)? | Тарифы routerai.ru за 1M токенов, округление вверх до копеек (`math.Ceil`) |
| 1     | Boundary Keeper | Скоуп Phase 4 vs Phase 5 (MongoDB)?      | Phase 4 — stateless стриминг; сохранение истории диалогов в MongoDB в Phase 5 |
| 1     | Failure Analyst | Овердрафт баланса и обрыв соединения?    | Списание за фактически полученные токены при обрыве; допуск овердрафта |
| 2     | Gate & Probes   | Граничные случаи и запреты (must-NOT)?   | Зафиксированы concurrency, precision, disconnect rules и 3 ключевых запрета |

---

*Phase: 04-routerai-ru-integration-billing-engine*  
*Spec created: 2026-09-09*  
*Next step: /gsd-discuss-phase 4 — обсуждение технических деталей реализации (архитектура пакетов, mock-провайдер, SSE форматирование)*
