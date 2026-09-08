# Phase 3: Authentication & Referral System - Context

**Gathered:** 2026-09-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Фаза 3 реализует аутентификацию пользователей через VK OAuth 2.0, выпуск и валидацию JWT-сессий в защищённых HttpOnly cookies, выдачу профиля пользователя и реферальную программу с атомарным начислением приветственного бонуса (+5 руб / 500 копеек) и реферального вознаграждения (+2 руб / 200 копеек) в единой транзакции PostgreSQL.
</domain>

<spec_lock>
## Requirements (locked via SPEC.md)

**5 requirements are locked.** See `03-SPEC.md` for full requirements, boundaries, and acceptance criteria.

Downstream agents MUST read `03-SPEC.md` before planning or implementing. Requirements are not duplicated here.

**In scope (from SPEC.md):**
- Пакет `internal/auth`: клиент VK OAuth 2.0, генерация/валидация JWT токенов (HS256), генератор реферальных кодов (`ref_code`).
- Сервисный слой `internal/service/auth.go` для координации транзакций регистрации, начисления бонуса новичку и реферального вознаграждения через `TxManager`.
- Расширение репозиторного слоя (методы работы с таблицей `referrals` и поиск реферера).
- HTTP-хэндлеры в `internal/server/handler/auth.go`:
  - `GET /api/v1/auth/vk/login` — формирование URL редиректа на VK с защитой от CSRF (`state`) и сохранением `ref`
  - `GET /api/v1/auth/vk/callback` — обмен кода авторизации на токен, получение профиля, создание пользователя и выдача cookie
  - `GET /api/v1/auth/mock` (или авто-mock в dev) — мгновенный вход для локального тестирования
  - `GET /api/v1/auth/me` — получение данных текущего пользователя и баланса
  - `POST /api/v1/auth/logout` — завершение сессии и удаление cookie
- Auth Middleware `internal/server/middleware/auth.go` для защиты приватных роутов.
- Модели и конфигурации окружения (`VK_CLIENT_ID`, `VK_CLIENT_SECRET`, `VK_REDIRECT_URI`, `JWT_SECRET`, `FRONTEND_URL`, `VK_MOCK_AUTH`).
- Unit и интеграционные тесты для авторизации, сессий и транзакционного реферального биллинга.

**Out of scope (from SPEC.md):**
- Интеграция с API routerai.ru (каталог моделей, генерация) — Фаза 4.
- Проверка баланса перед генерацией и списание за токены LLM — Фаза 4.
- Хранение переписки и диалогов в MongoDB — Фаза 5.
- Пользовательский интерфейс Next.js (кнопка входа, отображение баланса, модалка рефералки) — Фаза 6.

</spec_lock>

<decisions>
## Implementation Decisions

### 1. Архитектура сервисного слоя и хэндлеров
- **D-01 (Выделение AuthService):** Реализуется чистый сервисный слой `internal/service/auth.go`. Сервис принимает `UserRepository`, `BalanceRepository`, `ReferralRepository` (или методы в users), `database.TxManager` и конфигурацию. Хэндлеры `internal/server/handler/auth.go` отвечают исключительно за HTTP-контекст: чтение параметров запроса, установку HttpOnly cookie, HTTP-редиректы и сериализацию JSON. — **Reversibility:** costly.

### 2. OAuth Flow и обработка редиректов
- **D-02 (Server-side Redirect Flow):** Классический OAuth 2.0 flow:
  1. Бэкенд на `/api/v1/auth/vk/login` генерирует случайный криптографический `state`, привязывает к нему `ref` (реферальный код) и `return_to` (желаемый URL возврата), сохраняет `state` во временную cookie с коротким TTL (5–10 минут) и отправляет 302 Redirect на `oauth.vk.com/authorize`.
  2. При возврате на `/api/v1/auth/vk/callback` бэкенд проверяет совпадение `state` с cookie (защита от CSRF), обменивает `code` на `access_token`, запрашивает профиль пользователя в VK API (`users.get`) и вызывает `AuthService.AuthenticateOrRegister`.
  3. При успехе устанавливается сессионная cookie и выполняется 302 Redirect на `${FRONTEND_URL}/chat` (или `return_to`). При ошибке — редирект на `${FRONTEND_URL}/?error=auth_failed`. — **Reversibility:** reversible.
- **D-03 (Dev/Mock режим):** Если `VK_CLIENT_ID` пуст или `VK_MOCK_AUTH=true`, при обращении к `/api/v1/auth/vk/login` или `/api/v1/auth/mock` бэкенд не пытается стучаться в VK, а генерирует или авторизует тестового пользователя с предустановленным VK ID (например, `vk_id=999001` или передаваемым параметром `?vk_id=...`). Это обеспечивает мгновенный локальный цикл разработки и надёжные CI-тесты без внешних сетевых зависимостей. — **Reversibility:** reversible.

### 3. Сессии и безопасность JWT
- **D-04 (HttpOnly Secure Cookie):** Токен сессии хранится в cookie `auth_token` с флагами:
  - `HttpOnly = true` (недоступен для JavaScript, защита от XSS)
  - `SameSite = Lax` (передается при переходах по внешним ссылкам)
  - `Path = /`
  - `Secure = isProd` (активируется через флаг `COOKIE_SECURE` или в production окружении)
  - `Max-Age = 30 days` (долговечная сессия для удобства пользователя)
- **D-05 (JWT Claims и валидация):** JWT подписывается алгоритмом `HS256` с использованием `JWT_SECRET`. Claims содержат:
  - `sub`: строковый ID пользователя в БД
  - `vk_id`: числовой VK ID
  - `iat`: время выдачи
  - `exp`: время истечения
  Middleware `auth.go` извлекает токен из cookie `auth_token` (а если cookie отсутствует — проверяет заголовок `Authorization: Bearer <token>` для удобства API-клиентов и интеграционных тестов), валидирует подпись и сохраняет `userID` в `context.Context`. — **Reversibility:** reversible.

### 4. Реферальная программа и финансовые транзакции
- **D-06 (Атомарная регистрация в TxManager):** Вся процедура первого входа нового пользователя выполняется строго в транзакции:
  1. Создание записи в `users` (с генерацией уникального `ref_code`).
  2. Начисление приветственного бонуса 500 копеек (+5.00 руб) новому пользователю с проводкой в `balance_transactions` (`type = 'welcome_bonus'`).
  3. Если в запросе был передан валидный `ref_code`, принадлежащий существующему пользователю (и не совпадающий с новым):
     - В `users.referred_by` проставляется `referrer.ID`.
     - Создается строка в таблице `referrals`: `referrer_id`, `referee_id`, `reward_kopecks = 200`.
     - Рефереру начисляется 200 копеек (+2.00 руб) на баланс через `balanceRepo.AddBonus` с проводкой `type = 'referral_reward'` и `reference_id = referee.ID`.
  При сбое любого шага транзакция откатывается целиком. При повторной авторизации уже существующего пользователя транзакция начислений не запускается, обновляются только данные профиля. — **Reversibility:** one-way.

### Agent's Discretion
- Точный алгоритм генерации `ref_code` (например, 8 символов base62/alphanumeric с повторной попыткой при коллизии).
- Вспомогательные DTO для ответов `/api/v1/auth/me`.
- Механизм упаковки временного OAuth state (хеш со временем жизни или зашифрованный токен).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Contracts & Specifications
- `.planning/phases/03-authentication-referral-system/03-SPEC.md` — спецификация требований, границы и критерии приёмки Фазы 3.
- `.planning/REQUIREMENTS.md` §AUTH — требования AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05.
- `.planning/ROADMAP.md` §Phase 3 — цели и этапы реализации фазы.

### Database & Repository Contracts
- `backend/internal/model/user.go` — модель `User`.
- `backend/internal/model/balance.go` — модель `Balance` и константы `WelcomeBonusKopecks` (500), `ReferralRewardKopecks` (200).
- `backend/internal/model/transaction.go` — модель `BalanceTransaction` и типы транзакций (`TxWelcomeBonus`, `TxReferralReward`).
- `backend/internal/model/referral.go` — модель `Referral`.
- `backend/internal/database/tx_manager.go` — транзакционный менеджер `TxManager` с `WithinTransaction`.
- `backend/internal/repository/postgres/user.go` — `UserRepository`.
- `backend/internal/repository/postgres/balance.go` — `BalanceRepository`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/database/tx_manager.go` — готовый транзакционный менеджер с сохранением `pgx.Tx` в `context.Context`.
- `backend/internal/repository/postgres/balance.go` — методы `AddBonus`, `GetByUserID`, `GetTransactions`.
- `backend/internal/repository/postgres/user.go` — методы `Create`, `GetByID`, `GetByVKID`, `GetByRefCode`.
- `backend/internal/server/server.go` — маршрутизатор chi v5 с middleware логирования, cors и recoverer.

### Established Patterns
- Чистый Go, Go 1.27.1, без сторонних фреймворков сверх `chi`, `pgx/v5`, `golang-jwt/jwt/v5`.
- Финансовая целостность: баланс строго в `BIGINT` копейках.
- Линтер `golangci-lint` со строгими проверками ошибок (`errcheck`, `gosec`, `govet`).
- Юнит и интеграционные тесты с параллельным запуском (`t.Parallel()`) и проверкой race conditions (`go test -race`).

### Integration Points
- `backend/internal/server/server.go` — добавление роутов `/api/v1/auth/...` и подключение `AuthMiddleware`.
- `backend/main.go` — чтение переменных конфигурации VK и JWT, инициализация `AuthService` и передача в хэндлеры сервера.
- `.env.example` — дополнение переменными `VK_CLIENT_ID`, `VK_CLIENT_SECRET`, `VK_REDIRECT_URI`, `JWT_SECRET`, `FRONTEND_URL`, `VK_MOCK_AUTH`.

</code_context>

<specifics>
## Specific Ideas

- Пакет `internal/auth/jwt`:
  ```go
  type TokenClaims struct {
      UserID int64 `json:"uid"`
      VKID   int64 `json:"vk_id"`
      jwt.RegisteredClaims
  }
  ```
- Реферальная ссылка генерируется в `/api/v1/auth/me`:
  ```go
  RefLink: fmt.Sprintf("%s/?ref=%s", frontendURL, user.RefCode)
  ```
- Генерация `ref_code`: генератор безопасной случайной строки (например, криптостойкий псевдослучайный генератор `crypto/rand` из 8 символов алфавита `23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz`, исключающий похожие символы `0, O, 1, l, I`).

</specifics>

<deferred>
## Deferred Ideas

- Дополнительные провайдеры авторизации (Telegram Login Widget, Google OAuth, Яндекс ID) — запланированы на v2 (FEAT-02).
- Двухфакторная аутентификация (2FA / SMS) — не требуется для текущего этапа MVP.
- Выплата реферального вознаграждения в рублях на карты — в рамках v1 начисления используются исключительно для оплаты токенов в сервисе.

</deferred>

---

*Phase: 03-authentication-referral-system*
*Context gathered: 2026-09-08*
*Next step: /gsd-plan-phase 3 — создание детального пошагового плана реализации фазы*
