---
phase: "06"
slug: "next-js-frontend-interface"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-18"
validated: "2026-09-18T20:57:00Z"
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution and Nyquist compliance.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Next.js 16 (App Router), TypeScript strict (`npx tsc --noEmit`), ESLint (`npm run lint`), Production build (`npm run build`) |
| **Config file** | `frontend/tsconfig.json`, `frontend/package.json`, `frontend/next.config.ts`, `frontend/eslint.config.mjs` |
| **Quick run command** | `cd frontend && npx tsc --noEmit` |
| **Full suite command** | `cd frontend && npx tsc --noEmit && npm run build` |
| **Lint check** | `cd frontend && npm run lint` |
| **Estimated runtime** | ~6-8 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx tsc --noEmit`
- **After every plan wave:** Run `cd frontend && npx tsc --noEmit && npm run build`
- **Before `/gsd-verify-work`:** Full build passes with 0 errors and 0 warnings + manual interactive verification in browser
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | R1, UI-01 | Установка зависимостей, дизайн-система темы Modern AI Dark, шрифты и базовые токены | build | `cd frontend && npx tsc --noEmit` | ✅ `globals.css` | ⬜ pending |
| 06-01-02 | 01 | 1 | R1, UI-01 | Приветственный лендинг (`/`), Dev Mock вход и серверный `middleware.ts` на куке `auth_token` | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `middleware.ts` | ⬜ pending |
| 06-01-03 | 01 | 1 | R1, UI-01 | Глобальный Zustand-стор (`useChatStore`), типизация и клиентский guard авторизации | unit/typecheck | `cd frontend && npx tsc --noEmit` | ✅ `useChatStore.ts` | ⬜ pending |
| 06-02-01 | 02 | 2 | R2, UI-02 | Компонент сайдбара (`ChatSidebar`, `ChatItem`) с интеграцией `GET /api/v1/chats` | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `ChatSidebar.tsx` | ⬜ pending |
| 06-02-02 | 02 | 2 | R2, UI-02 | Создание нового диалога (`POST /api/v1/chats`) и удаление (`DELETE /api/v1/chats/{id}`) | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `ChatItem.tsx` | ⬜ pending |
| 06-02-03 | 02 | 2 | R2, UI-02 | Адаптивная выезжающая шторка (drawer) для экранов `<768px` с кнопкой-гамбургером | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `ChatSidebar.tsx` | ⬜ pending |
| 06-03-01 | 03 | 3 | R3, UI-03 | Клиентский хук `useChatStream` с чтением `ReadableStreamDefaultReader` и `AbortController` | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `useChatStream.ts` | ⬜ pending |
| 06-03-02 | 03 | 3 | R3, UI-03 | Компоненты переписки: `ChatWindow`, `MessageItem`, `CodeBlock` с подсветкой и кнопкой копирования | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `CodeBlock.tsx` | ⬜ pending |
| 06-03-03 | 03 | 3 | R4, UI-04 | Селектор моделей `ModelSelector` с загрузкой из `/api/v1/models` и переключением на лету | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `ModelSelector.tsx` | ⬜ pending |
| 06-04-01 | 04 | 4 | R5, UI-05 | Виджет баланса в рублях (`BalanceChip`) в шапке с обновлением после стриминга | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `BalanceChip.tsx` | 06 pending |
| 06-04-02 | 04 | 4 | R5, UI-05 | Блокировка отправки сообщений при нулевом балансе (`ZeroBalanceAlert`) | typecheck | `cd frontend && npx tsc --noEmit` | ✅ `ZeroBalanceAlert.tsx` | ⬜ pending |
| 06-04-03 | 04 | 4 | R6, UI-05 | Модальное окно реферальной программы (`ReferralModal`) с копированием ссылки и бонусами | typecheck/build | `cd frontend && npm run build` | ✅ `ReferralModal.tsx` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
