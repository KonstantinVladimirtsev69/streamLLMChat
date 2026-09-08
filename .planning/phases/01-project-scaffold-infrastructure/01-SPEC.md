# Phase 1: Project Scaffold & Infrastructure — Specification

**Created:** 2026-09-08
**Ambiguity score:** 0.08 (gate: ≤ 0.20)
**Requirements:** 4 locked

## Goal

Создать базовую архитектуру монорепозитория, включающую структуру директорий `backend/` (Go 1.27+, chi v5) и `frontend/` (Next.js 16+, TypeScript, TailwindCSS), корневой `Makefile` для автоматизации разработки, локальный `docker-compose.yml` с PostgreSQL 18 и MongoDB 8, а также изолированные многоэтапные `Dockerfile`, готовые для деплоя в Dokploy.

## Background

Проект находится на этапе гринфилд (greenfield) разработки — исходный код отсутствует. Для быстрого и безопасного масштабирования следующих слоев (БД, аутентификация, чат, UI) требуется заложить стандартизированный каркас сервисов, унифицированные команды сборки и воспроизводимое локальное окружение в Docker.

## Requirements

1. **Монорепозиторий и сервисные каркасы (INFRA-01)**:
   - **Current**: В репозитории отсутствуют директории исходного кода сервисов.
   - **Target**: Созданы директории `backend/` и `frontend/`. В `backend/` инициализирован модуль Go 1.27+ (`go.mod`), подключен HTTP-роутер `github.com/go-chi/chi/v5` и создан базовый сервер с проверкой работоспособности (`GET /health`). В `frontend/` развернут проект Next.js 16+ с App Router, TypeScript и TailwindCSS.
   - **Acceptance**: `go build ./...` внутри `backend/` компилируется без ошибок; `npm run build` внутри `frontend/` успешно собирает статический/SSR бандл.

2. **Автоматизация разработки через Makefile (INFRA-02)**:
   - **Current**: Команды для локальной сборки и запуска необходимо вызывать вручную.
   - **Target**: Корневой `Makefile` предоставляет цели: `help`, `build`, `run`, `test`, `docker-up`, `docker-down`, `clean`.
   - **Acceptance**: Вызов `make help` выводит документированный список команд; вызов `make build` компилирует backend и frontend; `make test` запускает тесты обоих сервисов.

3. **Локальное контейнерное окружение баз данных (INFRA-03)**:
   - **Current**: Локальные базы данных PostgreSQL и MongoDB не настроены.
   - **Target**: Корневой `docker-compose.yml` поднимает контейнеры PostgreSQL 18 (порт 5432) и MongoDB 8 (порт 27017) с сохранением данных через named volumes и healthcheck-проверками. Подготовлен `.env.example` с переменными подключения, портами и возможностью переопределения (например, `FRONTEND_PORT=${FRONTEND_PORT:-3001}`, учитывая занятость порта 3000).
   - **Acceptance**: Команда `make docker-up` (или `docker compose up -d`) успешно запускает сервисы PostgreSQL и MongoDB; оба контейнера переходят в статус healthy.

4. **Автономные Dockerfile для деплоя в Dokploy (INFRA-04)**:
   - **Current**: Конфигурации контейнеризации сервисов отсутствуют.
   - **Target**: Созданы независимые multi-stage `backend/Dockerfile` (на базе golang:1.27-alpine / alpine) и `frontend/Dockerfile` (на базе node:alpine с поддержкой standalone output Next.js), оптимизированные по размеру и не зависящие друг от друга при сборке.
   - **Acceptance**: `docker build -t llm-chat-backend:test backend/` и `docker build -t llm-chat-frontend:test frontend/` собираются без ошибок и запускаются автономно.

## Boundaries

**In scope:**
- Структура монорепозитория: `backend/` и `frontend/`
- Каркас Go backend с `chi/v5` и эндпоинтом `GET /health`
- Каркас Next.js 16+ (App Router, TypeScript, TailwindCSS)
- Корневой `Makefile` со всеми базовыми таргетами
- Локальный `docker-compose.yml` для PostgreSQL 18 и MongoDB 8
- Шаблоны переменных окружения `.env.example`
- Автономные `Dockerfile` для `backend` и `frontend` (Dokploy-ready)

**Out of scope:**
- Схемы таблиц, миграции PostgreSQL и подключение репозиториев к базам (это Phase 2)
- Авторизация через VK OAuth, токены и реферальная механика (это Phase 3)
- Интеграция с routerai.ru и SSE потоковый биллинг (это Phase 4)
- Бизнес-логика чатов и сохранение сообщений в MongoDB (это Phase 5)
- UI компоненты чата, сайдбара и диалогов на фронтенде (это Phase 6)

## Constraints

- **Go**: Версия 1.27+
- **Next.js**: Версия 16+ (App Router, TypeScript, TailwindCSS)
- **PostgreSQL**: Версия 18+
- **MongoDB**: Версия 8+
- **Backend Router**: `github.com/go-chi/chi/v5`
- **Frontend Port**: Порт 3001 по умолчанию (`${FRONTEND_PORT:-3001}`), так как порт 3000 занят внешним процессом в системе
- **Dokploy Compatibility**: Dockerfile сервисов должны быть полностью автономными и собираться из контекста своих папок (`backend/` и `frontend/`)

## Acceptance Criteria

- [ ] В `backend/` инициализирован Go-модуль (Go 1.27+) с `chi/v5` и рабочим эндпоинтом `GET /health`
- [ ] В `frontend/` инициализирован проект Next.js 16+ с TypeScript и TailwindCSS
- [ ] В корне репозитория присутствует `Makefile` с целями `docker-up`, `docker-down`, `build`, `run`, `test`, `clean`
- [ ] `docker-compose.yml` запускает PostgreSQL 18 и MongoDB 8 с persistent volumes и healthchecks
- [ ] Создан файл `.env.example` с документированными переменными окружения для всех сервисов
- [ ] `backend/Dockerfile` и `frontend/Dockerfile` успешно собирают production-ready Docker образы

## Edge Coverage

**Coverage:** 3/3 applicable edges resolved · 0 unresolved

| Category | Requirement | Status | Resolution / Reason |
|----------|-------------|--------|---------------------|
| Port Conflicts | INFRA-03 | ✅ covered | Порт 3000 занят в ОС; фронтенд вынесен на порт 3001 по умолчанию с переопределением через FRONTEND_PORT |
| Data Persistence | INFRA-03 | ✅ covered | Базы данных используют named volumes (`pg_data`, `mongo_data`), чтобы перезапуск контейнеров не удалял данные |
| Independent Build Context | INFRA-04 | ✅ covered | Сборка Dockerfile не ссылается на родительские папки или соседние каталоги монорепозитория |

## Prohibitions (must-NOT)

**Coverage:** 3/3 applicable prohibitions resolved · 0 unresolved

| Prohibition (must-NOT statement) | Requirement | Status | Verification / Reason |
|----------------------------------|-------------|--------|------------------------|
| MUST NOT коммитить реальные секреты и пароли в git | INFRA-03 | resolved | verification: test (наличие `.env` в `.gitignore` и только `.env.example` в git) |
| MUST NOT использовать версии ниже согласованных (Go <1.27, Next <16, Postgres <18, Mongo <8) | INFRA-01 | resolved | verification: test |
| MUST NOT создавать общие зависимости между Dockerfile backend и frontend | INFRA-04 | resolved | verification: test (изолированная сборка в Dokploy) |

## Ambiguity Report

| Dimension          | Score | Min  | Status | Notes                                             |
|--------------------|-------|------|--------|---------------------------------------------------|
| Goal Clarity       | 0.95  | 0.75 | ✓      | Специфицированы каркасы монорепозитория            |
| Boundary Clarity   | 0.90  | 0.70 | ✓      | Четко разграничены инфраструктурный каркас и Phase 2 |
| Constraint Clarity | 0.90  | 0.65 | ✓      | Go 1.27+, Next.js 16+, PG 18+, Mongo 8+, chi, 3001 |
| Acceptance Criteria| 0.90  | 0.70 | ✓      | 6 конкретных бинарных критериев готовности         |
| **Ambiguity**      | 0.08  | ≤0.20| ✓      | Gate пройден с запасом                            |

## Interview Log

| Round | Perspective     | Question summary                         | Decision locked                                         |
|-------|-----------------|------------------------------------------|---------------------------------------------------------|
| 1     | Researcher      | HTTP-роутер в Go                         | Выбран `go-chi/chi/v5` (легковесный, совместим с SSE)   |
| 1     | Researcher      | Стек стилизации Next.js 16               | Выбран TypeScript + TailwindCSS (App Router)            |
| 1     | Researcher      | Конфигурация портов                      | Порт 3000 занят; фронтенд настроен на 3001; PG 5432, Mongo 27017 |

---

*Phase: 01-project-scaffold-infrastructure*
*Spec created: 2026-09-08*
*Next step: /gsd-discuss-phase 1 — implementation decisions (структура пакетов Go, alias в Next.js и т.д.)*
