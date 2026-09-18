# Roadmap: LLM Chat Platform (routerai.ru)

## Overview

Полноценная разработка сервиса чата с LLM через routerai.ru, построенного по архитектуре Horizontal Layers: от базовой инфраструктуры монорепозитория, моделей данных PostgreSQL и MongoDB, через бэкенд-авторизацию VK OAuth и реферальный биллинг, к интеграции потокового инференса routerai.ru, современному Next.js фронтенду и финальной валидации с развертыванием в Dokploy.

## Phases

- [x] **Phase 1: Project Scaffold & Infrastructure** - Инициализация монорепозитория, Docker-конфигураций и Makefile (completed 2026-09-08)
- [x] **Phase 2: Database Layer & Data Models** - Схемы и репозитории PostgreSQL (пользователи, биллинг) и MongoDB (чаты) (completed 2026-09-08)
- [x] **Phase 3: Authentication & Referral System** - VK OAuth, JWT сессии, начисление 5 руб и реферальная программа (+2 руб) (completed 2026-09-08)
- [x] **Phase 4: routerai.ru Integration & Billing Engine** - Клиент API, SSE стриминг, учет токенов и списание средств (completed 2026-09-09)

- [x] **Phase 5: Chat Management & Persistence** - Управление диалогами, сохранение истории сообщений и сборка контекста (completed 2026-09-09)
- [ ] **Phase 6: Next.js Frontend Interface** - Веб-интерфейс чата, сайдбар, Markdown/код, выбор моделей, баланс и рефералы
- [ ] **Phase 7: End-to-End Integration & Dokploy Deployment** - Сквозное тестирование, финализация Dockerfile и запуск в Dokploy

## Phase Details

### Phase 1: Project Scaffold & Infrastructure

**Goal**: Создать структуру монорепозитория с директориями backend и frontend, настроить локальный docker-compose для баз данных и разработать Makefile для автоматизации разработки.
**Depends on**: Nothing (первая фаза)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04
**Success Criteria**:

  1. Репозиторий имеет чистую структуру с `backend/` (Go) и `frontend/` (Next.js)
  2. Команда `make docker-up` успешно поднимает контейнеры PostgreSQL и MongoDB
  3. Для `backend` и `frontend` подготовлены независимые многоэтапные `Dockerfile`, готовые для Dokploy
  4. Команды `make build` и `make run` компилируют и запускают каркасы сервисов

**Plans**: 3 plans

Plans:
**Wave 1**

- [x] 01-01: Инициализация монорепозитория, Go-модуля и базовой структуры Next.js
- [x] 01-02: Конфигурация локального `docker-compose.yml` (PostgreSQL 18, MongoDB 8) и `.env.example`

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-03: Создание корневого `Makefile` и многоэтапных `Dockerfile` для backend и frontend

### Phase 2: Database Layer & Data Models

**Goal**: Разработать схемы данных, миграции и слой доступа к данным для PostgreSQL и MongoDB с поддержкой финансовых ACID-транзакций.
**Depends on**: Phase 1
**Requirements**: DATA-01, DATA-02, DATA-03
**Success Criteria**:

  1. Миграции PostgreSQL создают таблицы `users`, `balances`, `balance_transactions`, `referrals`
  2. MongoDB инициализирует коллекции `chats` и `messages` с соответствующими индексами
  3. В Go реализован репозиторный слой с поддержкой транзакций для безопасного изменения балансов

**Plans**: 3 plans

Plans:

- [x] 02-01: Миграции и схема PostgreSQL для пользователей, баланса и транзакций
- [x] 02-02: Схема и индексы коллекций MongoDB для диалогов и сообщений
- [x] 02-03: Реализация Go-репозиториев и транзакционного менеджера баланса

### Phase 3: Authentication & Referral System

**Goal**: Реализовать авторизацию через VK OAuth, выдачу JWT, начисление стартового баланса 5 руб и реферальную программу с вознаграждением 2 руб.
**Depends on**: Phase 2
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05
**Success Criteria**:

  1. Пользователь может пройти аутентификацию через VK OAuth и получить валидный JWT-токен
  2. При первой регистрации создается профиль с автоматическим зачислением +5.00 руб на баланс
  3. Каждому пользователю генерируется уникальный реферальный код и ссылка
  4. При регистрации по реферальной ссылке пригласившему пользователю начисляется +2.00 руб с записью в истории транзакций

**Plans**: 3 plans

Plans:

- [x] 03-01: Core Auth Primitives — JWT Token Manager, Session Context, and Auth Middleware
- [x] 03-02: Referral DAL, Code Generator & Transactional AuthService with Welcome and Referral Bonuses
- [x] 03-03: VK OAuth Client, Mock Provider, HTTP Handlers, Server Integration & E2E Tests

### Phase 4: routerai.ru Integration & Billing Engine

**Goal**: Интегрировать API routerai.ru, реализовать потоковый вывод ответов через Server-Sent Events (SSE) и биллинг списания токенов с баланса в PostgreSQL.
**Depends on**: Phase 3
**Requirements**: LLM-01, LLM-02, LLM-03, LLM-04
**Success Criteria**:

  1. Backend получает актуальный список доступных моделей от routerai.ru
  2. Запрос к LLM блокируется с ошибкой 402/403, если баланс пользователя <= 0 руб
  3. Ответ LLM стримится клиенту в реальном времени через Server-Sent Events (SSE)
  4. По завершении стриминга рассчитывается стоимость токенов и сумма списывается с баланса пользователя

**Plans**: 3 plans

Plans:
**Wave 1**

- [x] 04-01: Клиент API routerai.ru (каталог моделей, аутентификация, расчет стоимости)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-02: Эндпоинт стриминга чата на основе Server-Sent Events (SSE) в Go

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 04-03: Проверка баланса перед генерацией и атомарное списание за токены после завершения ответа

### Phase 5: Chat Management & Persistence

**Goal**: Реализовать полный цикл управления чатами и сохранения переписки в MongoDB с контекстной памятью диалогов.
**Depends on**: Phase 4
**Requirements**: CHAT-01, CHAT-02, CHAT-03
**Success Criteria**:

  1. Пользователь может создавать, просматривать список, переименовывать и удалять чаты
  2. Каждое сообщение пользователя и ответ ассистента сохраняются в MongoDB с метками модели и токенов
  3. При продолжении диалога предыдущие сообщения корректно передаются в routerai.ru как контекст

**Plans**: 2 plans

Plans:
**Wave 1**

- [x] 05-01: REST API для управления диалогами (CRUD) в MongoDB

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 05-02: Механизм сохранения сообщений и сборки истории контекста для отправки в LLM

### Phase 6: Next.js Frontend Interface

**Goal**: Создать современный, интерактивный веб-интерфейс чата на Next.js с авторизацией, стримингом, Markdown, селектором моделей и реферальным виджетом.
**Depends on**: Phase 5
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05
**Success Criteria**:

  1. Пользователь видит адаптивный интерфейс с кнопкой авторизации через VK
  2. В сайдбаре отображается список чатов с возможностью создания нового и переключения
  3. Сообщения ассистента выводятся потоком в реальном времени с поддержкой Markdown и подсветки кода
  4. Доступен селектор модели из списка routerai.ru
  5. В шапке/профиле отображается баланс в рублях, а в модальном окне — реферальная ссылка со статистикой

**Plans**: 4 plans

Plans:
**Wave 1**

- [ ] 06-01: Фундамент, тема Modern AI Dark, авторизация VK/Mock, middleware и Zustand стор

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 06-02: Сайдбар диалогов (CRUD /api/v1/chats) и адаптивный мобильный drawer

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 06-03: Окно переписки со стримингом SSE, Markdown/Code рендерингом и селектором моделей

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 06-04: Виджет баланса в рублях, блокировка при 0.00 ₽, модальное окно рефералов и production-сборка

### Phase 7: End-to-End Integration & Dokploy Deployment

**Goal**: Провести сквозную интеграцию, проверку всех пользовательских сценариев и подготовить проект к бесшовному развертыванию в Dokploy.
**Depends on**: Phase 6
**Requirements**: DEPLOY-01, DEPLOY-02
**Success Criteria**:

  1. Полный сквозной сценарий работает: регистрация через VK -> получение 5 руб -> приглашение друга (+2 руб) -> выбор модели -> чат со стримингом -> списание баланса
  2. Готовы эталонные конфигурации окружения `.env.example`
  3. Dokploy-совместимые Dockerfile собираются без ошибок и запускаются в изолированных контейнерах
  4. Документирован процесс деплоя в Dokploy

**Plans**: 2 plans

Plans:

- [ ] 07-01: Сквозное интеграционное тестирование полного пользовательского и биллингового цикла
- [ ] 07-02: Финализация Dockerfile, Dokploy-конфигурации и инструкции по развертыванию

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Project Scaffold & Infrastructure | 3/3 | Complete    | 2026-09-08 |
| 2. Database Layer & Data Models | 3/3 | Complete    | 2026-09-08 |
| 3. Authentication & Referral System | 3/3 | Complete    | 2026-09-08 |
| 4. routerai.ru Integration & Billing Engine | 3/3 | Complete    | 2026-09-18 |
| 5. Chat Management & Persistence | 2/2 | Complete    | 2026-09-18 |
| 6. Next.js Frontend Interface | 0/4 | Planned     | - |
| 7. End-to-End Integration & Dokploy Deployment | 0/2 | Not started | - |
