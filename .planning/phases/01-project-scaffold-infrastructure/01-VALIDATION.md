---
phase: "01"
slug: "project-scaffold-infrastructure"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-08"
validated: "2026-09-08T21:23:00Z"
---

# Phase 01 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution and Nyquist compliance.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.27+), `eslint` + Next.js build |
| **Config file** | `backend/go.mod`, `backend/.golangci.yml`, `frontend/package.json` |
| **Quick run command** | `go -C backend test ./...` |
| **Full suite command** | `make test && make lint` |
| **Estimated runtime** | ~3 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go -C backend test ./...`
- **After every plan wave:** Run `make test && make lint`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | INFRA-01 | T-01-01 | Reverse proxy header validation (ClientIPFromHeader) | unit | `go -C backend test -v ./...` | ✅ `backend/internal/server/server_test.go` | ✅ green |
| 01-01-02 | 01 | 1 | INFRA-01 | — | Next.js standalone build & typecheck | build / lint | `npm --prefix frontend run lint && npm --prefix frontend run build` | ✅ `frontend/package.json` | ✅ green |
| 01-02-01 | 02 | 1 | INFRA-03 | — | Compose syntax & persistent volumes | infra validation | `docker compose config -q` | ✅ `docker-compose.yml` | ✅ green |
| 01-02-02 | 02 | 1 | INFRA-03 | T-01-02 | Secret isolation & .env protection | static check | `git check-ignore -v .env` | ✅ `.env.example`, `.gitignore` | ✅ green |
| 01-03-01 | 03 | 1 | INFRA-02 | — | Makefile automated targets | integration | `make help && make lint && make test` | ✅ `Makefile` | ✅ green |
| 01-03-02 | 03 | 1 | INFRA-04 | T-01-03 | Autonomous unprivileged container execution | container check | `grep -E "USER|appuser|nextjs" backend/Dockerfile frontend/Dockerfile` | ✅ Dockerfiles | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `backend/internal/server/server_test.go` — unit tests for Chi router, `/health`, CORS, ClientIP, and RequestID
- [x] `backend/.golangci.yml` — linter configuration for static analysis and security checks
- [x] `make lint` / `make test` — automation commands in root Makefile

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Audit 2026-09-08

| Metric | Count |
|--------|-------|
| Gaps found | 1 (missing Go unit tests for server router) |
| Resolved | 1 (`backend/internal/server/server_test.go` implemented and passing) |
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
