# Requirements: LLM Chat Platform (routerai.ru)

**Defined:** 2026-09-08
**Core Value:** Быстрый, надёжный веб-чат с LLM и прозрачной тарификацией за фактически израсходованные токены с баланса пользователя.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Infrastructure & Monorepo (INFRA)

- [x] **INFRA-01**: Монорепозиторий с разделением на директории `backend/` (Go) и `frontend/` (Next.js)
- [x] **INFRA-02**: Корневой `Makefile` с командами сборки, запуска, тестирования и контейнеризации
- [x] **INFRA-03**: Локальный `docker-compose.yml` для запуска PostgreSQL, MongoDB, backend и frontend сервисов
- [x] **INFRA-04**: Автономные `Dockerfile` для `backend` и `frontend`, оптимизированные для деплоя в Dokploy

### Database Layer & Data Models (DATA)

- [ ] **DATA-01**: Схема и миграции PostgreSQL для пользователей, балансов, истории транзакций и реферальных связей
- [ ] **DATA-02**: Схема и индексы MongoDB для хранения сессий чатов и сообщений
- [ ] **DATA-03**: Слой доступа к данным (DAL/Repository) в Go с поддержкой ACID-транзакций для балансовых операций

### Authentication & Referral System (AUTH)

- [ ] **AUTH-01**: Аутентификация через VK OAuth (редирект на VK ID, обмен кода, получение профиля)
- [ ] **AUTH-02**: Управление пользовательскими сессиями через JWT и middleware проверки авторизации
- [ ] **AUTH-03**: Автоматическое начисление приветственного бонуса 5 руб на баланс новому пользователю при первой регистрации
- [ ] **AUTH-04**: Генерация уникальной реферальной ссылки для каждого пользователя
- [ ] **AUTH-05**: Фиксация реферального перехода при регистрации и автоматическое начисление 2 руб на баланс пригласившего пользователя

### LLM Integration & Billing Engine (LLM)

- [ ] **LLM-01**: Клиент взаимодействия с API routerai.ru (каталог доступных моделей, отправка запросов)
- [ ] **LLM-02**: Проверка наличия положительного баланса перед началом генерации (блокировка при нулевом или отрицательном балансе)
- [ ] **LLM-03**: Потоковая передача ответов модели в реальном времени через Server-Sent Events (SSE)
- [ ] **LLM-04**: Точный расчет стоимости израсходованных токенов по тарифам routerai.ru и атомарное списание средств с баланса пользователя в PostgreSQL

### Chat Management & Persistence (CHAT)

- [ ] **CHAT-01**: Управление диалогами в MongoDB (создание диалога, получение списка, удаление)
- [ ] **CHAT-02**: Сохранение сообщений пользователя и ответов ассистента в MongoDB с метаданными токенов и модели
- [ ] **CHAT-03**: Формирование контекста предыдущих сообщений диалога для передачи в LLM

### Next.js Frontend Interface (UI)

- [ ] **UI-01**: Адаптивный веб-интерфейс на Next.js с авторизацией через кнопку VK OAuth и сессионным состоянием
- [ ] **UI-02**: Боковая панель (сайдбар) с историей диалогов, кнопкой создания нового чата и удалением диалогов
- [ ] **UI-03**: Окно чата с отображением потокового текста в реальном времени, рендерингом Markdown и подсветкой синтаксиса кода
- [ ] **UI-04**: Селектор доступных моделей routerai.ru с отображением активной модели
- [ ] **UI-05**: Виджет текущего баланса пользователя в рублях и модальное окно реферальной программы со ссылкой и кнопкой копирования

### Dokploy Deployment & Production Readiness (DEPLOY)

- [ ] **DEPLOY-01**: Шаблоны переменных окружения (`.env.example`) для backend, frontend и баз данных
- [ ] **DEPLOY-02**: Конфигурация и инструкция по развертыванию контейнеров backend и frontend в Dokploy с подключением к БД

## v2 Requirements

### Billing & Payments

- **BILL-01**: Интеграция платёжного шлюза (ЮKassa / Robokassa / СБП) для пополнения баланса рублями
- **BILL-02**: Выписка чеков и уведомления об успешном пополнении баланса

### Advanced Features

- **FEAT-01**: Мультимодальные запросы (загрузка изображений, файлов в контекст)
- **FEAT-02**: Дополнительные провайдеры авторизации (Telegram Login Widget, Google OAuth)
- **FEAT-03**: Экспорт переписки в Markdown и PDF

## Out of Scope

| Feature | Reason |
|---------|--------|
| Внешние платёжные шлюзы в v1 | Для запуска первой версии достаточно приветственных 5 руб и реферальных начислений |
| Альтернативные OAuth провайдеры | Фокус на аудитории VK в русскоязычном сегменте |
| Локальные LLM модели / self-hosted инференс | Используется облачный провайдер routerai.ru с широким набором готовых моделей |
| Генерация изображений и аудио | В рамках v1 поддерживаются исключительно текстовые модели |

## Traceability

Which phases cover which requirements.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 1 | Complete |
| INFRA-02 | Phase 1 | Complete |
| INFRA-03 | Phase 1 | Complete |
| INFRA-04 | Phase 1 | Complete |
| DATA-01 | Phase 2 | Pending |
| DATA-02 | Phase 2 | Pending |
| DATA-03 | Phase 2 | Pending |
| AUTH-01 | Phase 3 | Pending |
| AUTH-02 | Phase 3 | Pending |
| AUTH-03 | Phase 3 | Pending |
| AUTH-04 | Phase 3 | Pending |
| AUTH-05 | Phase 3 | Pending |
| LLM-01 | Phase 4 | Pending |
| LLM-02 | Phase 4 | Pending |
| LLM-03 | Phase 4 | Pending |
| LLM-04 | Phase 4 | Pending |
| CHAT-01 | Phase 5 | Pending |
| CHAT-02 | Phase 5 | Pending |
| CHAT-03 | Phase 5 | Pending |
| UI-01 | Phase 6 | Pending |
| UI-02 | Phase 6 | Pending |
| UI-03 | Phase 6 | Pending |
| UI-04 | Phase 6 | Pending |
| UI-05 | Phase 6 | Pending |
| DEPLOY-01 | Phase 7 | Pending |
| DEPLOY-02 | Phase 7 | Pending |

**Coverage:**

- v1 requirements: 26 total
- Mapped to phases: 26
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-08*
*Last updated: 2026-09-08 after initial definition*
