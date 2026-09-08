# Plan 03-01 Summary: JWT Primitives and Auth Middleware

## Completed Tasks
- [x] **03-01-01**: Installed `github.com/golang-jwt/jwt/v5`, implemented `TokenManager` in `backend/internal/auth/jwt.go` with HS256 signature, minimum 16-byte secret requirement, claims containing `UserID` and `VKID`, and comprehensive unit tests in `jwt_test.go`.
- [x] **03-01-02**: Implemented context helpers (`WithUserID`, `UserIDFromContext`) and `AuthMiddleware` in `backend/internal/server/middleware/auth.go` supporting both HttpOnly cookies (`auth_token`) and `Authorization: Bearer` headers, with 401 Unauthorized handling and unit tests in `auth_test.go`.

## Verification Results
- `go test -v ./internal/auth/...`: PASS (weak secret, valid tokens, expired tokens, key mismatch, malformed tokens).
- `go test -v ./internal/server/middleware/...`: PASS (cookie extraction, bearer header extraction, 401 unauthorized checks).
- `go test -race ./...`: PASS without data races.
