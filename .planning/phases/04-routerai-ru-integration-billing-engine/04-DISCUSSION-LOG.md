# Phase 4: routerai.ru Integration & Billing Engine - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-09  
**Phase:** 04-routerai-ru-integration-billing-engine  
**Areas discussed:** Архитектура LLM-клиента и кэширование тарифов моделей, SSE-стриминг и таймауты HTTP-сервера  

---

## Архитектура LLM-клиента и кэширование тарифов моделей

### Вопрос 1: Абстракция клиента LLM
| Option | Description | Selected |
|--------|-------------|----------|
| Интерфейс LLMProvider | Интерфейс LLMProvider (GetModels, StreamChat) с реализациями RouterAIClient и MockLLMProvider | ✓ |
| Прямая структура RouterAIClient | Без интерфейса, подъем httptest.Server в тестах | |
| You decide | На усмотрение разработчика | |

**User's choice:** Интерфейс LLMProvider (GetModels, StreamChat) с реализациями RouterAIClient и MockLLMProvider (автономный мок со стримингом для тестов без сетевых вызовов)  
**Notes:** Обеспечивает быстрое модульное тестирование и легкую замену провайдеров.

### Вопрос 2: Кэширование каталога моделей и цен
| Option | Description | Selected |
|--------|-------------|----------|
| In-memory TTL кэш (15 мин) | Кэширование в памяти с fallback на встроенный справочник моделей при сбоях API | ✓ |
| Прямой запрос без кэша | Каждый вызов GET /api/v1/models идет в сеть к routerai.ru | |
| You decide | На усмотрение разработчика | |

**User's choice:** In-memory кэш (TTL 15 мин) с fallback на встроенный справочник моделей и тарифов при сбоях routerai.ru  
**Notes:** Снижает задержки и нагрузку на API routerai.ru, обеспечивает надежность при временных сбоях провайдера.

---

## SSE-стриминг и таймауты HTTP-сервера

### Вопрос 1: Таймауты HTTP-сервера
| Option | Description | Selected |
|--------|-------------|----------|
| ResponseController.SetWriteDeadline | Снятие WriteDeadline через http.ResponseController внутри SSE-хэндлера | ✓ |
| Глобальное увеличение WriteTimeout | Увеличение до 300с глобально в main.go для всех эндпоинтов | |
| You decide | На усмотрение разработчика | |

**User's choice:** Использовать http.ResponseController.SetWriteDeadline(time.Time{}) в SSE-хэндлере (и безопасный IdleTimeout/ReadHeaderTimeout в http.Server), чтобы длинные стримы не резались 15s таймаутом  
**Notes:** Сохраняет защиту сервера от зависших обычных HTTP-запросов и позволяет стримить ответы любой длины.

### Вопрос 2: Формат событий SSE
| Option | Description | Selected |
|--------|-------------|----------|
| Нормализованный JSON формат платформы | {"type":"delta","content":"..."}, {"type":"done","usage":{...}}, {"type":"error","error":"..."} | ✓ |
| Сырой OpenAI формат | data: {"choices":[{"delta":{"content":"..."}}]} и data: [DONE] | |
| You decide | На усмотрение разработчика | |

**User's choice:** Нормализованный JSON формат платформы: {"type":"delta","content":"..."}, {"type":"done","usage":{...}}, {"type":"error","error":"..."}  
**Notes:** Упрощает разбор на фронтенде в Phase 6 и прозрачно передает информацию о списанных средствах и токенах в событии done.

---

## the agent's Discretion

- Поддержка овердрафта в методе списания за токены (`DeductUsage`), чтобы генерация не прерывалась при нехватке копеек на завершающих токенах.
- Тарификация и списание фактически полученных токенов при преждевременном разрыве SSE соединения клиентом.
- Формат внутренней синхронизации горутин стриминга через каналы Go.

## Deferred Ideas

- Сохранение истории переписки и диалогов в MongoDB — Phase 5.
- Пользовательский веб-интерфейс на Next.js с рендерингом Markdown и переключателем моделей — Phase 6.
- Пополнение баланса реальными деньгами через платежные шлюзы (ЮKassa/СБП) — v2 (BILL-01).
