# Root Cause Analysis: VK OAuth / VK ID Security Error

## Problem Description
1. Изначально при попытке авторизации через VK на проде запрос уходил на:
   `https://oauth.vk.com/authorize?client_id=txdyBX88P1a8kTsGusSK&...`
   и возвращал `401 Unauthorized {"error":"invalid_request","error_description":"Security Error"}`.
2. После замены `client_id` на числовой `54752238` и секрета на `rrvnrYBOEELBlMZSdntg`, ошибка `Security Error` сохранилась при обращении к `oauth.vk.com`.

## Root Cause Analysis
Были выявлены две взаимосвязанные причины:

### 1. Подмена Client ID секретным ключом
Строка `txdyBX88P1a8kTsGusSK` являлась 20-значным «Защищённым ключом» (Client Secret), а не «ID приложения». Шлюз ВКонтакте мгновенно отклоняет нечисловой `client_id` с кодом `401 Security Error`.

### 2. Несовместимость приложений VK ID с устаревшим шлюзом `oauth.vk.com`
Приложение `54752238` зарегистрировано в новом кабинете **VK ID / VK Бизнес ID** (`id.vk.com` / `id.vk.ru`).
- Все новые приложения VK ID работают исключительно по протоколу **OAuth 2.1**.
- Старый шлюз `oauth.vk.com` / `oauth.vk.ru` **полностью блокирует приложения VK ID** с ошибкой `401 Security Error`.
- Прямой запрос к шлюзу VK ID:
  `https://id.vk.ru/authorize?client_id=54752238&redirect_uri=...&response_type=code&state=...&code_challenge=...&code_challenge_method=s256&scope=vkid.personal_info`
  успешно отдаёт `302 Found` на `https://id.vk.ru/auth?...` и открывает форму входа.

## Реализованное решение (VK ID OAuth 2.1)
1. **PKCE генератор (`backend/internal/auth/pkce.go`)**:
   - Реализована генерация криптостойкого `code_verifier` (RFC 7636, base64url, 43 символа) и `code_challenge` (S256).
2. **VK ID клиент (`backend/internal/auth/vk.go`)**:
   - Базовый URL по умолчанию переведён на `https://id.vk.ru` (настраивается через `VK_BASE_URL`).
   - `GetAuthURL` передаёт `code_challenge`, `code_challenge_method=s256` и `scope=vkid.personal_info`.
   - `ExchangeCode` принимает `ExchangeParams` (`code`, `code_verifier`, `device_id`, `state`) и отправляет POST на `https://id.vk.ru/oauth2/auth`.
   - Профиль пользователя запрашивается через эндпоинт `https://id.vk.ru/oauth2/user_info` с fallback на `users.get`.
3. **Обработчик авторизации (`backend/internal/server/handler/auth.go`)**:
   - `Login`: генерирует пару PKCE, сохраняет `code_verifier` в защищённую `HttpOnly` cookie `oauth_state` и перенаправляет пользователя на `id.vk.ru/authorize` с `code_challenge`.
   - `Callback`: считывает `code`, `state` и `device_id`, валидирует state и выполняет обмен кода с верификацией PKCE.
4. **Конфигурация (`backend/main.go`, `.env.example`)**:
   - Добавлена поддержка `VK_BASE_URL` (по умолчанию `https://id.vk.ru`).
   - Сохранена проверка на числовой формат `VK_CLIENT_ID`.

## Статус верификации
- Все юнит-тесты пакета `backend/internal/auth` и всего бэкенда (`go test ./...`) успешно пройдены.
- Сборка и typecheck фронтенда (`npx tsc --noEmit`) успешно пройдены.
