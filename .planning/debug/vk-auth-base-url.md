# Debug Report: VK Auth Outdated Base URL & VK ID Security Error

**Status:** Resolved
**Date:** 2026-09-25
**Branch:** fix/vk-auth-base-url

## 1. Issue Description
1. Изначальная проблема: устаревшие базовые URL `oauth.vk.com` и `api.vk.com`.
2. Вторая проблема при боевом тестировании приложения `54752238`:
   - При редиректе на `https://oauth.vk.ru/authorize?client_id=54752238...` сервер VK возвращает HTTP 401:
     `{"error":"invalid_request","error_description":"Security Error"}`.
   - Пользователь в браузере видит ошибку безопасности, а в DevTools Network запросы выглядят так, будто редирект не уходит на VK.

## 2. Root Cause Analysis
1. **Тип приложения VK:**
   - Приложение `54752238` создано в современном кабинете **VK ID** (`id.vk.com`/`id.vk.ru`).
   - Серверы устаревшего VK OAuth (`oauth.vk.ru` и `oauth.vk.com`) жестко блокируют новые приложения VK ID и отклоняют запросы авторизации ошибкой `Security Error`.
2. **Протокол VK ID (OAuth 2.1 + PKCE):**
   - Приложения VK ID обязаны использовать базовый URL `https://id.vk.ru` (или `https://id.vk.ru/authorize`).
   - Протокол VK ID требует обязательного использования **PKCE** (`code_challenge` и `code_challenge_method=S256`). Без `code_challenge` эндпоинт `id.vk.ru/authorize` возвращает ошибку `code_challenge or code_challenge_method is invalid`.
   - При передаче `code_challenge` эндпоинт `https://id.vk.ru/authorize` возвращает HTTP 200 с полноценной страницей входа VK ID.
   - Обмен кода в VK ID выполняется методом `POST https://id.vk.ru/oauth2/auth` с передачей `code_verifier`, `device_id` и `grant_type=authorization_code`.
   - Профиль пользователя запрашивается через `POST https://id.vk.ru/oauth2/user_info`.

## 3. Solution Applied
1. **PKCE генератор (`internal/auth/pkce.go`):**
   - Реализована функция `GeneratePKCE()` (генерация 32 криптографически стойких байт, кодирование в raw URL Base64 верификатора и вычисление SHA256-хеша для челленджа по стандарту RFC 7636).
   - Покрыто изолированным тестом `internal/auth/pkce_test.go`.
2. **VK ID & Legacy Двухрежимный Клиент (`internal/auth/vk.go`):**
   - Дефолтный `BaseURL` переведён на `https://id.vk.ru`.
   - В `GetAuthURL` передаётся `code_challenge`, `code_challenge_method=S256` и `scope=vkid.personal_info`.
   - Добавлен метод `ExchangeCodeWithParams` с поддержкой `code_verifier`, `device_id` и `state`.
   - Если `BaseURL` содержит `id.vk`, клиент работает по протоколу VK ID (`/oauth2/auth` + `/oauth2/user_info`).
   - Если `BaseURL` legacy (`oauth.vk.ru` или mock server), сохраняется обратная совместимость со стандартным OAuth 2.0 (`/access_token` + `users.get`).
3. **HTTP Auth Handler (`internal/server/handler/auth.go`):**
   - В `Login`: генерируется PKCE, верификатор сохраняется в защищённую cookie `oauth_state`, а `code_challenge` передаётся в `GetAuthURL`.
   - В `Callback`: считываются `device_id` и `code` из query-параметров и передаются в `ExchangeCodeWithParams` вместе с сохранённым `code_verifier`.
4. **Конфигурация и окружение:**
   - `.env.example`: `VK_BASE_URL=https://id.vk.ru`.
   - `backend/main.go`: логирование эффективного `base_url`.
5. **Тесты (`internal/auth/vk_test.go`):**
   - Добавлены проверки формирования ссылки VK ID с PKCE и `vkid.personal_info`.
   - Добавлен тест мок-сервера для эндпоинтов `/oauth2/auth` и `/oauth2/user_info`.

## 4. Verification
- `go test -v ./internal/auth`: Все тесты пройдены.
- `go test -v ./...`: Все тесты пройдены.
- `go vet ./...`: Чисто (0 замечаний).
- Ручной curl-тест `id.vk.ru/authorize` с `client_id=54752238`: HTTP 200 OK.
