---
phase: "04"
slug: "routerai-ru-integration-billing-engine"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-09"
validated: "2026-09-09T18:43:00Z"
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution and Nyquist compliance.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.27+), `golangci-lint`, PostgreSQL 18 |
| **Config file** | `backend/go.mod`, `backend/.golangci.yml`, `docker-compose.yml` |
| **Quick run command** | `go -C backend test -v ./...` |
| **Full suite command** | `make test && make lint` |
| **Concurrency / Race check** | `go -C backend test -race -v ./...` |
| **Estimated runtime** | ~4 seconds |

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
| 04-01-01 | 01 | 1 | LLM-01 | Pricing calculation formula with math.Ceil, currency conversions | unit | `go -C backend test -v ./internal/llm/...` | ✅ `pricing_test.go` | ✅ green |
| 04-01-02 | 01 | 1 | LLM-01 | RouterAI HTTP client & MockLLMProvider implementation with stream chunks | unit | `go -C backend test -v ./internal/llm/...` | ✅ `routerai_test.go`, `mock_test.go` | ✅ green |
| 04-01-03 | 01 | 1 | LLM-01 | In-memory TTL model cache & static catalog fallback | unit | `go -C backend test -v -run TestModelCache ./internal/llm/...` | ✅ `cache_test.go` | ✅ green |
| 04-02-01 | 02 | 1 | LLM-03 | ResponseController deadline removal & SSE event formatting (delta/done/error) | unit | `go -C backend test -v ./internal/server/handler/...` | ✅ `chat_test.go` | ✅ green |
| 04-02-02 | 02 | 1 | LLM-03 | SSE streaming integration test with MockLLMProvider and chunk delivery | integration | `go -C backend test -v -run TestStreamChat ./internal/server/handler/...` | ✅ `chat_test.go` | ✅ green |
| 04-03-01 | 03 | 2 | LLM-02, LLM-04 | BalanceRepository DeductUsage supporting overdraft and token_charge ledger | integration | `go -C backend test -v ./internal/repository/postgres/...` | ✅ `balance_test.go` | ✅ green |
| 04-03-02 | 03 | 2 | LLM-02, LLM-04 | Pre-generation balance gate (HTTP 402 on balance <= 0) and post-stream deduction | integration | `go -C backend test -v -run TestStreamChat ./internal/server/handler/...` | ✅ `chat_test.go`, `billing_test.go` | ✅ green |
| 04-03-03 | 03 | 2 | LLM-01..04 | Client disconnect handling, partial token billing & full route integration test | e2e / race | `go -C backend test -race ./... && make lint` | ✅ `chat_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `backend/internal/llm/pricing_test.go` — stubs and unit tests for LLM-01 pricing math
- [x] `backend/internal/llm/mock.go` — autonomous MockLLMProvider for isolated testing
- [x] `backend/internal/llm/routerai_test.go` — unit tests for RouterAIClient with httptest server
- [x] `backend/internal/repository/postgres/balance_test.go` — tests for DeductUsage overdraft & ledger
- [x] `backend/internal/service/billing_test.go` — billing service verification (CanGenerate, ChargeTokens)
- [x] `backend/internal/server/handler/chat_test.go` — SSE streaming, HTTP 402 gate, overdraft & client disconnect tests

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real RouterAI network call (optional) | LLM-01 | Requires valid paid API key | Run server with real `ROUTERAI_API_KEY` and test via `curl -N` |

---

## Validation Audit 2026-09-09

| Metric | Count |
|--------|-------|
| Gaps found | 0 (all requirements have executable automated tests; routerai_test.go added for direct HTTP client coverage) |
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

