package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"backend/internal/auth"
)

type userContextKey struct{}

// WithUserID injects the authenticated user ID into the request context.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userContextKey{}, userID)
}

// UserIDFromContext extracts the authenticated user ID from context if present.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	val, ok := ctx.Value(userContextKey{}).(int64)
	return val, ok
}

// AuthMiddleware creates a middleware that validates JWT session tokens from HttpOnly cookie or Bearer header.
func AuthMiddleware(tokenManager auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string

			// 1. Try to extract from auth_token cookie
			if cookie, err := r.Cookie("auth_token"); err == nil && cookie.Value != "" {
				tokenStr = cookie.Value
			}

			// 2. Fallback to Authorization: Bearer <token>
			if tokenStr == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenStr == "" {
				respondUnauthorized(w)
				return
			}

			claims, err := tokenManager.ValidateToken(tokenStr)
			if err != nil {
				respondUnauthorized(w)
				return
			}

			ctx := WithUserID(r.Context(), claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}
