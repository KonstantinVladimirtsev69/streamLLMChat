---
phase: "05"
slug: "chat-management-persistence"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-09"
validated: "2026-09-09T19:08:00Z"
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution and Nyquist compliance.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.27+), `golangci-lint`, MongoDB 8+, PostgreSQL 18 |
| **Config file** | `backend/go.mod`, `backend/.golangci.yml`, `docker-compose.yml` |
| **Quick run command** | `go -C backend test -v ./...` |
| **Full suite command** | `make test && make lint` |
| **Concurrency / Race check** | `go -C backend test -race -v ./...` |
| **Estimated runtime** | ~3.5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go -C backend test -v ./...`
- **After every plan wave:** Run `make test && make lint`
- **Before `/gsd-verify-work`:** Full suite must be green + `go test -race` passes
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | CHAT-01 | MongoDB repository methods (Touch, DeleteByChatID) | unit | `go -C backend test -v ./internal/repository/mongodb/...` | ✅ `chat_test.go` | ✅ green |
| 05-01-02 | 01 | 1 | CHAT-01 | ChatService CRUD business logic with tenant isolation & cascade delete | unit | `go -C backend test -v ./internal/service/...` | ✅ `chat_test.go` | ✅ green |
| 05-01-03 | 01 | 1 | CHAT-01 | REST API HTTP handlers (/api/v1/chats) and 404 security checks | integration | `go -C backend test -v ./internal/server/handler/...` | ✅ `chat_crud_test.go` | ✅ green |
| 05-02-01 | 02 | 2 | CHAT-02, CHAT-03 | Context assembly (20-message sliding window), auto-titling (45 runes), message persistence | unit | `go -C backend test -v -run TestChatService ./internal/service/...` | ✅ `chat_test.go` | ✅ green |
| 05-02-02 | 02 | 2 | CHAT-02, CHAT-03 | Hybrid streaming handler (stateful with chat_id, stateless fallback, client disconnect resilience) | integration | `go -C backend test -v -run TestStreamChat_Stateful ./internal/server/handler/...` | ✅ `chat_crud_test.go` | ✅ green |
| 05-02-03 | 02 | 2 | CHAT-01..03 | Full lifecycle E2E: chat creation, multi-turn stream with context, auto-title, rename, 404 checks, cascade delete | e2e / race | `go -C backend test -race -v -run TestChatLifecycle_E2E ./internal/server/...` | ✅ `chat_e2e_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `backend/internal/repository/mongodb/chat_test.go` — Touch repository unit test
- [x] `backend/internal/service/chat_test.go` — unit tests for ChatService CRUD, tenant isolation, sliding window context & auto-titling
- [x] `backend/internal/server/handler/chat_crud_test.go` — HTTP handler integration tests for chat CRUD and hybrid streaming
- [x] `backend/internal/server/chat_e2e_test.go` — full end-to-end multi-turn chat lifecycle test

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real RouterAI multi-turn chat over network (optional) | CHAT-03 | Requires active paid API token and live network | Run backend with `ROUTERAI_API_KEY` and perform multi-turn requests with `chat_id` |

---

## Validation Audit 2026-09-09

| Metric | Count |
|--------|-------|
| Gaps found | 0 (all requirements covered by unit, integration, and E2E automated tests) |
| Resolved | 0 |
| Escalated | 0 |

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all missing test files
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-09
