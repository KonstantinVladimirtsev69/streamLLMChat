# Root Cause Analysis: VK OAuth Security Error

## Problem Description
При попытке авторизации через VK на проде запрос уходит на:
`https://oauth.vk.com/authorize?client_id=txdyBX88P1a8kTsGusSK&redirect_uri=https%3A%2F%2Fstreamchat-api.velvet-sin.com%2Fapi%2Fv1%2Fauth%2Fvk%2Fcallback&response_type=code&state=...&v=5.131`
После 302 редиректа на `https://oauth.vk.ru/authorize?...` возвращается ошибка:
`HTTP 401 Unauthorized`
`{"error":"invalid_request","error_description":"Security Error"}`

## Root Cause
В параметре `client_id` передаётся строка `txdyBX88P1a8kTsGusSK`.

В OAuth 2.0 API ВКонтакте (`oauth.vk.com` / `oauth.vk.ru`):
1. **`client_id` (ID приложения)** обязан быть **целым положительным числом** (например, `51912345`).
2. Значение `txdyBX88P1a8kTsGusSK` имеет длину 20 символов и содержит буквы и цифры. Это классический формат **«Защищённого ключа» (Client Secret)** приложения ВКонтакте.
3. При получении нечислового `client_id` шлюз авторизации ВКонтакте мгновенно отклоняет запрос с ошибкой:
   `{"error":"invalid_request","error_description":"Security Error"}`.

### Подтверждение (Reproduction Test)
- `curl "https://oauth.vk.com/authorize?client_id=txdyBX88P1a8kTsGusSK&..."` -> `401 Unauthorized {"error":"invalid_request","error_description":"Security Error"}`
- `curl "https://oauth.vk.com/authorize?client_id=123456&..."` -> `401 Unauthorized {"error":"invalid_request","error_description":"redirect_uri is incorrect, check application redirect uri in the settings page"}` (числовой client_id проходит валидацию и переходит к проверке настроек приложения).

## Как исправить в окружении (Dokploy / .env)
1. Открыть панель разработчика ВКонтакте: [https://vk.com/apps?act=manage](https://vk.com/apps?act=manage) или [https://id.vk.com/about/business/go](https://id.vk.com/about/business/go).
2. Выбрать нужное приложение и перейти в раздел **«Настройки»**.
3. Проверить переменные:
   - `VK_CLIENT_ID`: скопировать поле **«ID приложения»** (только цифры, например `51848392`).
   - `VK_CLIENT_SECRET`: скопировать поле **«Защищённый ключ»** (строка `txdyBX88P1a8kTsGusSK`).
   - `VK_REDIRECT_URI`: `https://streamchat-api.velvet-sin.com/api/v1/auth/vk/callback`.
4. В настройках самого приложения ВКонтакте убедиться, что заполнены:
   - **Доверенный redirect URI**: `https://streamchat-api.velvet-sin.com/api/v1/auth/vk/callback`
   - **Базовый домен**: `velvet-sin.com`
   - **Адрес сайта**: `https://streamchat.velvet-sin.com`
