# Debug Report: VK Auth Outdated Base URL

**Status:** Resolved
**Date:** 2026-09-25
**Branch:** fix/vk-auth-base-url

## 1. Issue Description
Авторизация через VK завершалась сбоем из-за устаревшего базового URL `https://oauth.vk.com` и эндпоинта `https://api.vk.com`.

## 2. Root Cause Analysis
В соответствии с официальными изменениями инфраструктуры VK и документацией VK ID (проверено через `@mcp:context7` по библиотекам `/websites/id_vk_ru_about_business_go_en_vkid_vk-id` и `/kaktaknet/discourse-vkid-oauth`):
1. VK перенес основную инфраструктуру авторизации и API из доменной зоны `.com` в национальную доменную зону `.ru`:
   - OAuth авторизация: `https://oauth.vk.com` -> `https://oauth.vk.ru`
   - Основное API: `https://api.vk.com` -> `https://api.vk.ru`
   - Единая точка входа VK ID: `https://id.vk.ru`
2. В клиенте `backend/internal/auth/vk.go` базовые URL были жестко захардкожены на `https://oauth.vk.com` и `https://api.vk.com`, без возможности переопределения через переменные окружения.

## 3. Solution Applied
1. **Configurable Base URLs with Modern Defaults (`internal/auth/vk.go`):**
   - В структуру `VKConfig` добавлены поля `BaseURL` и `APIBaseURL`.
   - По умолчанию `BaseURL` инициализируется значением `https://oauth.vk.ru`.
   - По умолчанию `APIBaseURL` инициализируется значением `https://api.vk.ru`.
   - Запрос профиля `users.get` использует настроенный `apiBaseURL`.
   - Метод `SetBaseURL` синхронно переопределяет оба URL для изолированных HTTP-тестов.
2. **Environment Variable Integration (`backend/main.go`):**
   - Добавлено считывание `VK_BASE_URL=os.Getenv("VK_BASE_URL")` и `VK_API_URL=os.Getenv("VK_API_URL")`.
   - Обновлен вывод лога при старте сервиса.
3. **Environment Documentation (`.env.example`):**
   - Добавлены примеры `VK_BASE_URL=https://oauth.vk.ru` и `VK_API_URL=https://api.vk.ru`.
4. **Tests (`backend/internal/auth/vk_test.go`):**
   - Тест `GetAuthURL` проверяет `oauth.vk.ru`.
   - Добавлен тест `GetAuthURL respects custom BaseURL from VKConfig`.
5. **Project Planning Documentation (`.planning/`):**
   - Актуализированы спецификации и архитектурные описания в `03-SPEC.md`, `03-RESEARCH.md`, `03-CONTEXT.md` и `03-03-PLAN.md`.

## 4. Verification
- `go test -v ./internal/auth`: Все тесты пройдены.
- `go test -v ./...`: Полный сьют бэкенда пройден.
- `go vet ./...`: Чисто.
- Frontend `npm run build`: Сборка успешна без ошибок.
