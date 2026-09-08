---
phase: "03"
slug: "authentication-referral-system"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-08"
validated: "2026-09-08T22:30:00Z"
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution and Nyquist compliance.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.27+), `golangci-lint`, Docker Compose (PostgreSQL 18) |
| **Config file** | `backend/go.mod`, `backend/.golangci.yml`, `docker-compose.yml` |
| **Quick run command** | `go -C backend test -v ./...` |
| **Full suite command** | `make test && make lint` |
| **Concurrency / Race check** | `go -C backend test -race -v ./...` |
| **Estimated runtime** | ~3 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go -C backend test -v ./...`
- **After every plan wave:** Run `make test && make lint`
- **Before `/gsd-verify-work`:** Full suite must be green + `go test -race` passes
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|--------|
| 03-01-01 | 01 | 1 | AUTH-02 | HS256 algorithm enforcement, rejection of expired/tampered tokens | unit | `go -C backend test -v ./internal/auth/...` | ✅ green |
| 03-01-02 | 01 | 1 | AUTH-02 | HttpOnly cookie & Bearer token extraction, 401 unauthorized handling | unit | `go -C backend test -v ./internal/server/middleware/...` | ✅ green |
| 03-02-01 | 02 | 1 | AUTH-05 | Referral repository persistence and foreign key enforcement | integration | `go -C backend test -v ./internal/repository/postgres/...` | ✅ green |
| 03-02-02 | 02 | 1 | AUTH-04 | Base58 un-ambiguous ref_code generation without collisions | unit | `go -C backend test -v -run TestRefCode ./internal/auth/...` | ✅ green |
| 03-02-03 | 02 | 1 | AUTH-03, AUTH-05 | Atomic welcome (+500 kop) and referral (+200 kop) bonus crediting via TxManager | integration | `go -C backend test -v ./internal/service/...` | ✅ green |
| 03-03-01 | 03 | 2 | AUTH-01 | VK OAuth URL generation with CSRF state & Mock client token exchange | unit | `go -C backend test -v -run TestVK ./internal/auth/...` | ✅ green |
| 03-03-02 | 03 | 2 | AUTH-01, AUTH-02 | HTTP auth handlers (/login, /callback, /mock, /me, /logout) cookie & JSON response | integration | `go -C backend test -v ./internal/server/handler/...` | ✅ green |
| 03-03-03 | 03 | 2 | AUTH-01..05 | Full server route wiring, environment config & race condition checks | e2e / lint | `go -C backend test -race ./... && make lint` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*


---

## Validation Audit 2026-09-08

| Metric | Count |
|--------|-------|
| Gaps found | 0 (all requirements AUTH-01..AUTH-05 have executable automated tests) |
| Resolved | 0 |
| Escalated | 0 |

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all dependencies
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-08

