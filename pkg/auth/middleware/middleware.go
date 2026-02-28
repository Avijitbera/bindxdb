package middleware

import (
	"bindxdb/pkg/auth"
	"context"
	"net/http"
	"strings"
)

type AuthMiddleware struct {
	authorizer  auth.Authorizer
	providers   map[string]auth.AuthProvider
	exemptPaths []string
}

func NewAuthMiddleware(authorizer auth.Authorizer) *AuthMiddleware {
	return &AuthMiddleware{
		providers:  make(map[string]auth.AuthProvider),
		authorizer: authorizer,
		exemptPaths: []string{
			"/health",
			"/metrics",
			"/auth/login",
			"/auth/refresh",
		},
	}
}

func (m *AuthMiddleware) AddProvider(provider auth.AuthProvider) {
	m.providers[provider.Name()] = provider
}

func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, path := range m.exemptPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		var authResult *auth.AuthResult
		// var err error

		for _, provider := range m.providers {
			authResult, err := provider.ValidateToken(r.Context(), token)
			if err == nil && authResult.Success {
				break
			}
		}
		if authResult == nil || !authResult.Success {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		authCtx := &auth.AuthContext{
			UserID:        authResult.UserID,
			Username:      authResult.Username,
			Roles:         authResult.Roles,
			Token:         token,
			Authenticated: true,
			ExpiresAt:     authResult.ExpiresAt,
		}

		ctx := context.WithValue(r.Context(), "auth", authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}

func (m *AuthMiddleware) RequirePermission(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx, ok := r.Context().Value("auth").(*auth.AuthContext)
			if !ok {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			authorized, err := m.authorizer.Authorize(r.Context(), authCtx, resource, action)
			if err != nil || !authorized {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GetAuthContext(r *http.Request) *auth.AuthContext {
	authCtx, _ := r.Context().Value("auth").(*auth.AuthContext)
	return authCtx
}
