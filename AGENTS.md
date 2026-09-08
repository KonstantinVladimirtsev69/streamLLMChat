<!-- GSD:project-start source:PROJECT.md -->

## Project

**LLM Chat Platform (routerai.ru)**

Сервис чата с большими языковыми моделями (LLM) через провайдера routerai.ru с авторизацией через VK OAuth, поддержкой потокового вывода ответов (SSE) и выбором моделей. Продукт включает биллинг с балансом в рублях, приветственный бонус (5 руб) и реферальную программу (+2 руб пригласившему), позволяя пользователям удобно общаться с ИИ и платить за фактически использованные токены.

**Core Value:** Быстрый, надёжный веб-чат с LLM и прозрачной тарификацией за фактически израсходованные токены с баланса пользователя.

### Constraints

- **Tech Stack**: Go 1.22+ (backend), Next.js 14/15 (frontend), PostgreSQL 16+, MongoDB 7+, routerai.ru API
- **Deployment**: Dokploy совместимость с автономными Dockerfile для frontend и backend
- **Auth**: VK OAuth / OpenID Connect
- **Financial Integrity**: Все операции изменения баланса (начисление стартового бонуса, реферального вознаграждения, списание за токены) должны выполняться в транзакциях PostgreSQL с защитой от race conditions

<!-- GSD:project-end -->

<!-- GSD:stack-start source:STACK.md -->

## Technology Stack

Technology stack not yet documented. Will populate after codebase mapping or first phase.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.agents/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
