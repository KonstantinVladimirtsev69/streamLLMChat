# Phase 3: Authentication & Referral System — Technical Research

**Researched:** 2026-09-08
**Phase:** 03-authentication-referral-system
**Scope:** VK OAuth 2.0 Client & Mock Mode, JWT Sessions & Cookie Management, Auth Middleware, Referral Logic & Transactional Billing Engine

---

## 1. Executive Summary

Фаза 3 реализует подсистему идентификации и сессий пользователей веб-сервиса:
1. **VK OAuth 2.0 Integration**: серверная авторизация через VK ID / OAuth (`oauth.vk.ru/authorize`, `oauth.vk.ru/access_token`, `api.vk.ru/method/users.get`, конфигурируемые через `VK_BASE_URL` и `VK_API_URL`). Для изолированной разработки и тестов без внешних ключей создаётся автономный Mock-режим (`VK_MOCK_AUTH=true` или при пустом `VK_CLIENT_ID`).
2. **JWT Sessions**: генерация и валидация токенов HS256 (`golang-jwt/jwt/v5`) в защищённых cookies (`auth_token`, `HttpOnly=true`, `SameSite=Lax`, `Path=/`).
3. **Auth Middleware**: извлечение сессии из cookies или `Authorization: Bearer`, валидация подписи и сроков, внедрение `userID` в `context.Context`.
4. **Referral Engine & Financial Transactions**: при первой регистрации пользователя в БД запускается неделимая PostgreSQL транзакция через существующий `TxManager`:
   - Создание пользователя с уникальным криптографическим кодом приглашения `ref_code`.
   - Начисление приветственного бонуса 500 копеек (+5.00 руб) с проводкой `welcome_bonus`.
   - При наличии валидного реферера: запись связи в таблицу `referrals` и начисление рефереру 200 копеек (+2.00 руб) с проводкой `referral_reward`.

---

## 2. Dependencies & Libraries

| Dependency | Package | Version | Purpose |
|---|---|---|---|
| JWT Library | `github.com/golang-jwt/jwt/v5` | `v5.2.1` | Стандарт индустрии в Go для выпуска, подписи (HS256) и валидации JWT claims |
| HTTP Client / Router | `net/http`, `github.com/go-chi/chi/v5` | `v5.2.1` | Роутинг эндпоинтов авторизации и middleware сессий |
| Cryptography | `crypto/rand`, `crypto/hmac`, `crypto/sha256` | stdlib | Генерация криптографически стойких `state` токенов и уникальных `ref_code` |
| Database / DAL | `backend/internal/database`, `backend/internal/repository` | internal | Существующий пул `pgxpool`, транзакционный менеджер `TxManager` и репозитории |

---

## 3. Protocol & API Flows

### 3.1 VK OAuth 2.0 Authorization Flow

```
[Пользователь]                 [Backend API]                    [VK OAuth / API]
      |                               |                                 |
      | 1. GET /api/v1/auth/vk/login  |                                 |
      |    (?ref=XYZ&return_to=/chat) |                                 |
      |------------------------------>|                                 |
      |                               | Генерирует CSRF-state           |
      |                               | Сохраняет ref & state в cookie  |
      | 2. 302 Redirect oauth.vk.ru   |                                 |
      |<------------------------------|                                 |
      |                               |                                 |
      | 3. Авторизация и подтверждение прав                             |
      |---------------------------------------------------------------->|
      |                                                                 |
      | 4. 302 Redirect callback с code & state                         |
      |<----------------------------------------------------------------|
      |                                                                 |
      | 5. GET /api/v1/auth/vk/callback?code=...&state=...              |
      |------------------------------>|                                 |
      |                               | Проверяет state                 |
      |                               | 6. POST /access_token           |
      |                               |-------------------------------->|
      |                               | 7. Access Token + VK user_id    |
      |                               |<--------------------------------|
      |                               | 8. GET users.get?v=5.131        |
      |                               |-------------------------------->|
      |                               | 9. Profile (name, avatar)       |
      |                               |<--------------------------------|
      |                               | 10. AuthService: TxManager      |
      |                               |     - Create user if new        |
      |                               |     - Credit welcome bonus      |
      |                               |     - Process referral reward   |
      |                               | 11. Issue JWT cookie            |
      | 12. 302 Redirect /chat        |                                 |
      |<------------------------------|                                 |
```

### 3.2 Dev/Mock Mode Flow
Если `VK_CLIENT_ID` не сконфигурирован или `VK_MOCK_AUTH=true`:
- При запросе `GET /api/v1/auth/vk/login` бэкенд сразу перенаправляет на `/api/v1/auth/mock` (или обрабатывает на месте).
- Эндпоинт `GET /api/v1/auth/mock?vk_id=12345&name=TestUser&ref=XYZ` моментально регистрирует или аутентифицирует пользователя, выставляет JWT cookie и перенаправляет на `${FRONTEND_URL}/chat`.
- Это позволяет запускать E2E и интеграционные тесты без обращения к внешним серверам VK.

### 3.3 Referral Code Format
- Длина: 8 символов.
- Алфавит: base58 / un-ambiguous alphanumeric: `23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz` (исключены похожие символы `0, O, o, 1, l, I`).
- Энтропия: 58^8 ≈ 1.28 × 10^14 комбинаций, что полностью исключает случайные угадывания.

---

## 4. Transaction Architecture for Welcome & Referral Bonus

Все операции первого входа пользователя выполняются в рамках единой неделимой транзакции через `TxManager.WithinTransaction`:

```go
func (s *authService) RegisterNewUser(ctx context.Context, profile VKProfile, refCode string) (*model.User, error) {
    var newUser *model.User
    err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
        // 1. Поиск реферера, если передан refCode
        var referrerID *int64
        if refCode != "" {
            refUser, err := s.userRepo.GetByRefCode(txCtx, refCode)
            if err == nil && refUser != nil {
                referrerID = &refUser.ID
            }
        }

        // 2. Создание пользователя
        uniqueRefCode := s.generator.GenerateRefCode()
        user := &model.User{
            VKID:       profile.ID,
            FirstName:  profile.FirstName,
            LastName:   profile.LastName,
            AvatarURL:  profile.AvatarURL,
            RefCode:    uniqueRefCode,
            ReferredBy: referrerID,
        }
        if err := s.userRepo.Create(txCtx, user); err != nil {
            return err
        }
        newUser = user

        // 3. Начисление стартового бонуса (500 копеек)
        desc := "Приветственный бонус при регистрации"
        if _, err := s.balanceRepo.AddBonus(txCtx, user.ID, model.WelcomeBonusKopecks, model.TxWelcomeBonus, nil, desc); err != nil {
            return err
        }

        // 4. Если есть реферер и реферер != новый пользователь:
        if referrerID != nil && *referrerID != user.ID {
            // Запись в таблицу referrals
            if err := s.referralRepo.Create(txCtx, *referrerID, user.ID, model.ReferralRewardKopecks); err != nil {
                return err
            }
            // Начисление вознаграждения рефереру (200 копеек)
            refDesc := fmt.Sprintf("Реферальное вознаграждение за пользователя %d", user.ID)
            refIDStr := strconv.FormatInt(user.ID, 10)
            if _, err := s.balanceRepo.AddBonus(txCtx, *referrerID, model.ReferralRewardKopecks, model.TxReferralReward, &refIDStr, refDesc); err != nil {
                return err
            }
        }

        return nil
    })
    return newUser, err
}
```

---

## 5. Security & Session Integrity

1. **CSRF-защита OAuth**:
   - `state` формируется из случайных 32 байт, закодированных в hex.
   - Значение сохраняется в cookie `oauth_state` (`HttpOnly=true`, `SameSite=Lax`, `Max-Age=600`).
   - Callback сверяет параметр query `state` со значением cookie и сразу удаляет cookie.
2. **Хранение JWT в HttpOnly Cookie**:
   - Исключает доступ вредоносного JS к токену сессии в браузере (XSS mitigation).
   - Флаг `Secure` гарантирует отправку только по HTTPS в продакшене.
   - Флаг `SameSite=Lax` предотвращает CSRF-атаки при межсайтовых POST-запросах, но сохраняет cookie при переходе по ссылке.
3. **Защита от накрутки рефералов**:
   - Ограничение `chk_no_self_referral CHECK (referrer_id <> referee_id)` на уровне PostgreSQL.
   - Ограничение `UNIQUE (referee_id)`: пользователь может быть приглашён ровно один раз.
   - Обработка ошибок валидации реферального кода — если код невалиден, пользователь создаётся без реферера, регистрация не падает.

---

## 6. Verification Strategy

1. **Unit-тесты**:
   - `jwt_test.go`: генерация токена, валидация срока действия, обработка невалидной подписи или просроченного токена.
   - `refcode_test.go`: генерация кодов, проверка длины, алфавита и уникальности в цикле (1000 итераций).
2. **Интеграционные тесты сервисного слоя**:
   - Тест регистрации нового пользователя с проверкой баланса 500 копеек и создания транзакции.
   - Тест регистрации по реферальному коду: проверка начисления 200 копеек рефереру и 500 копеек новичку.
   - Тест защиты от повторной регистрации (возврат существующего пользователя без повторных бонусов).
   - Тест защиты от само-реферала.
3. **HTTP Integration Tests (`httptest`)**:
   - `/api/v1/auth/mock`: создание сессии, установка cookie `auth_token`, редирект.
   - `/api/v1/auth/me`: 401 Unauthorized без токена; 200 OK с правильным JSON профиля и баланса при наличии токена.
   - `/api/v1/auth/logout`: очистка cookie (Max-Age=0).
   - Проверка `AuthMiddleware`.

---

*Research completed: 2026-09-08*
