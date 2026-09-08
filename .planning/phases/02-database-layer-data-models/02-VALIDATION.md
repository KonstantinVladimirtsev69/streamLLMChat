---
phase: "02"
slug: "database-layer-data-models"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-08"
validated: "2026-09-08T22:10:20Z"
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution and Nyquist compliance.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.27+), `golangci-lint`, Docker Compose (PostgreSQL 18, MongoDB 8) |
| **Config file** | `backend/go.mod`, `backend/.golangci.yml`, `docker-compose.yml` |
| **Quick run command** | `go -C backend test -v ./...` |
| **Full suite command** | `make test && make lint` |
| **Concurrency / Race check** | `go -C backend test -race -v ./...` |
| **Estimated runtime** | ~2 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go -C backend test -v ./...`
- **After every plan wave:** Run `make test && make lint`
- **Before `/gsd-verify-work`:** Full suite must be green + `go test -race` passes
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | DATA-01 | — | Clean driver dependency resolution | module check | `go -C backend list -m github.com/jackc/pgx/v5 github.com/golang-migrate/migrate/v4` | ✅ `backend/go.mod` | ✅ green |
| 02-01-02 | 01 | 1 | DATA-01 | T-02-01 | Positive balance constraint enforcement | migration check | `make migrate-up && make migrate-down && make migrate-up` | ✅ `backend/migrations/*.sql` | ✅ green |
| 02-01-03 | 01 | 1 | DATA-01 | — | Strict type safety for kopecks (BIGINT) | unit | `go -C backend test ./internal/model/...` | ✅ `backend/internal/model/*.go` | ✅ green |
| 02-01-04 | 01 | 1 | DATA-01 | — | Embedded migration runner execution | build / cli | `go -C backend build ./cmd/migrate && make -n migrate-up` | ✅ `backend/internal/database/migrate.go` | ✅ green |
| 02-02-01 | 02 | 1 | DATA-02 | — | Chat & Message BSON/JSON serialization | unit | `go -C backend test -run TestModelBSONSerialization ./internal/database/...` | ✅ `backend/internal/database/mongo_test.go` | ✅ green |
| 02-02-02 | 02 | 1 | DATA-02 | — | Compound index registration on MongoDB | integration | `go -C backend test -run TestChatAndMessageRepositories ./internal/repository/mongodb/...` | ✅ `backend/internal/database/mongo.go` | ✅ green |
| 02-03-01 | 03 | 2 | DATA-03 | T-02-02 | Contextual transaction isolation and auto-rollback | integration | `go -C backend test -run TestTransactionRollbackOnPanicOrError ./internal/repository/postgres/...` | ✅ `backend/internal/database/tx_manager.go` | ✅ green |
| 02-03-02 | 03 | 2 | DATA-03 | T-02-03 | Atomic balance deduction preventing negative balance | integration | `go -C backend test -run TestBalanceLifecycleAndTransactions ./internal/repository/postgres/...` | ✅ `backend/internal/repository/postgres/balance.go` | ✅ green |
| 02-03-03 | 03 | 2 | DATA-02 | — | Chronological retrieval of chat message chain | integration | `go -C backend test -run TestChatAndMessageRepositories ./internal/repository/mongodb/...` | ✅ `backend/internal/repository/mongodb/message.go` | ✅ green |
| 02-03-04 | 03 | 2 | DATA-03 | T-02-04 | High-concurrency race condition safety (10 goroutines) | concurrency / race | `go -C backend test -race -run TestConcurrentBalanceDeductions ./internal/repository/postgres/...` | ✅ `backend/internal/repository/postgres/balance_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `backend/internal/database/mongo_test.go` — BSON serialization and client unit tests
- [x] `backend/internal/repository/postgres/balance_test.go` — balance lifecycle, concurrency and transaction tests
- [x] `backend/internal/repository/mongodb/chat_test.go` — chat and message repository tests
- [x] `backend/.golangci.yml` — static analysis and security checks (0 issues)
- [x] `docker-compose.yml` — PostgreSQL 18 and MongoDB 8 service definitions

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Audit 2026-09-08

| Metric | Count |
|--------|-------|
| Gaps found | 0 (all requirements have executable automated tests) |
| Resolved | 0 |
| Escalated | 0 |

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-08
