# LLM Chat Platform (routerai.ru)

## What This Is

Сервис чата с большими языковыми моделями (LLM) через провайдера routerai.ru с авторизацией через VK OAuth, поддержкой потокового вывода ответов (SSE) и выбором моделей. Продукт включает биллинг с балансом в рублях, приветственный бонус (5 руб) и реферальную программу (+2 руб пригласившему), позволяя пользователям удобно общаться с ИИ и платить за фактически использованные токены.

## Core Value

Быстрый, надёжный веб-чат с LLM и прозрачной тарификацией за фактически израсходованные токены с баланса пользователя.

## Business Context

- **Customer**: Русскоязычные пользователи и разработчики, желающие доступ к передовым LLM через единый удобный интерфейс с оплатой в рублях.
- **Revenue model**: Стартовый бонус (5 руб) и реферальная программа (+2 руб); монетизация через наценку на стоимость токенов routerai при последующем внедрении пополнения.
- **Success metric**: Доля активных пользователей, исчерпавших приветственный баланс, и коэффициент виральности реферальных ссылок (K-factor).
- **Strategy notes**: Быстрый запуск MVP без усложнения платёжными шлюзами на старте.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Архитектура монорепозитория: `backend/` (Go) и `frontend/` (Next.js), корневой `Makefile`, `docker-compose.yml` для локальной разработки и отдельные `Dockerfile` для backend и frontend
- [ ] Аутентификация через VK OAuth (сохранение профиля в PostgreSQL, выпуск JWT/session токенов)
- [ ] Модели пользователей и биллинга в PostgreSQL (баланс в рублях, журнал транзакций, реферальный код и ссылки)
- [ ] Приветственный баланс: автоматическое начисление 5 руб каждому новому пользователю при первой авторизации
- [ ] Реферальная программа: уникальные пригласительные ссылки, начисление +2 руб рефереру при регистрации нового пользователя по его ссылке
- [ ] Интеграция с API routerai.ru (получение списка доступных моделей, проксирование запросов, расчет стоимости токенов)
- [ ] Хранение истории чатов и сообщений в MongoDB (диалоги, сообщения, метаданные токенов и ролей)
- [ ] Потоковый вывод ответов LLM в реальном времени через Server-Sent Events (SSE) с backend на frontend
- [ ] Автоматическое списание средств с баланса пользователя в PostgreSQL по факту завершения генерации на основе стоимости токенов
- [ ] Современный веб-интерфейс Next.js: сайдбар с историей диалогов, окно чата с поддержкой Markdown и подсветки кода, выбор модели, индикатор баланса и модальное окно реферальной программы
- [ ] Конфигурация деплоя для Dokploy (изолированные Dockerfile, переменные окружения, подключение к базам данных)

### Out of Scope

- Подключение внешних платёжных шлюзов (ЮKassa, Robokassa, банковские карты) — отложено до v2, в v1 фокус на приветственном и реферальном балансе
- Дополнительные методы авторизации (Google, Telegram, логин/пароль) — в первой версии авторизация строго через VK OAuth
- Мультимодальные запросы (генерация изображений, аудио, распознавание голоса) — в v1 поддержка исключительно текстовых моделей routerai.ru
- Внутренний fine-tuning или обучение собственных моделей — использование только внешнего API routerai

## Context

- Монорепозиторий с директориями `backend/` (Go, idiomatic project structure, маршрутизация, pgx/sqlx, mongo-go-driver) и `frontend/` (Next.js App Router, TypeScript).
- Гибридная модель данных: PostgreSQL гарантирует строгую ACID-согласованность финансовых операций, баланса и реферальной структуры; MongoDB обеспечивает быстрое и гибкое хранение вложенных структур сообщений и истории переписки.
- Внешние интеграции: VK OAuth API для аутентификации и routerai.ru (OpenAI-совместимый API) для инференса и биллинга токенов.
- Инфраструктура: деплой на Dokploy через отдельные Dockerfile сервисов; локальное окружение поднимается одной командой через `docker-compose.yml` и `Makefile`.

## Constraints

- **Tech Stack**: Go 1.27+ (backend), Next.js 16+ (frontend), PostgreSQL 18+, MongoDB 8+, routerai.ru API
- **Deployment**: Dokploy совместимость с автономными Dockerfile для frontend и backend
- **Auth**: VK OAuth / OpenID Connect
- **Financial Integrity**: Все операции изменения баланса (начисление стартового бонуса, реферального вознаграждения, списание за токены) должны выполняться в транзакциях PostgreSQL с защитой от race conditions

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| PostgreSQL для пользователей и балансов, MongoDB для переписки | Балансы и рефералы требуют строгих ACID-транзакций, а сообщения чата — гибких JSON-документов | — Pending |
| SSE (Server-Sent Events) для потокового вывода ответов | Оптимальный и простой протокол для однонаправленного стриминга текста из LLM в браузер | — Pending |
| Списание средств по тарифам routerai.ru | Прозрачный и масштабируемый учёт стоимости генераций с возможностью задания наценки | — Pending |
| Отдельные Dockerfile + root docker-compose.yml и Makefile | Удобная локальная разработка и чистая интеграция в Dokploy | — Pending |
| Ограничение авторизации только VK OAuth на первом этапе | Упрощение onboarding и целевая ориентация на русскоязычную аудиторию | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Business Context check (if present) — customer, revenue model, success metric still accurate?
4. Audit Out of Scope — reasons still valid?
5. Update Context with current state

---
*Last updated: 2026-09-08 after initialization*
