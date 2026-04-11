package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zclaw/zclaw/internal/auth"
)

type contextKey string

const authContextKey contextKey = "auth_context"

// AuthMiddlewareConfig holds configuration for the auth middleware.
type AuthMiddlewareConfig struct {
	AuthService  auth.AuthService
	AuthEnabled  bool
	AdminAPIKey  string
	APIKeyPrefix string
	PublicPaths  map[string]bool
	HealthOnly   bool
}

// DefaultPublicPaths are paths that never require authentication.
func DefaultPublicPaths() map[string]bool {
	return map[string]bool{
		"/health":           true,
		"/api/v1/dashboard": false,
	}
}

// AuthMiddleware provides authentication for API requests.
// When AuthEnabled is false, it creates an admin AuthContext for all requests (backward compat).
func AuthMiddleware(cfg AuthMiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.AuthEnabled {
				ctx := context.WithValue(r.Context(), authContextKey, &auth.AuthContext{
					UserID:   "system",
					TenantID: "default",
					Role:     auth.RoleAdmin,
					Session:  false,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if cfg.PublicPaths != nil && cfg.PublicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			ac, err := authenticateRequest(r, cfg)
			if err != nil {
				slog.Warn("auth failed", "path", r.URL.Path, "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}

			ctx := context.WithValue(r.Context(), authContextKey, ac)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authenticateRequest(r *http.Request, cfg AuthMiddlewareConfig) (*auth.AuthContext, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errNoCredentials
	}

	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if cfg.AdminAPIKey != "" && token == cfg.AdminAPIKey {
			return &auth.AuthContext{
				UserID:   "admin",
				TenantID: "default",
				Role:     auth.RoleAdmin,
				Session:  false,
			}, nil
		}

		if strings.HasPrefix(token, cfg.APIKeyPrefix) {
			return cfg.AuthService.AuthenticateAPIKey(r.Context(), token)
		}

		return cfg.AuthService.AuthenticateSession(r.Context(), token)
	}

	if strings.HasPrefix(authHeader, "ApiKey ") {
		key := strings.TrimPrefix(authHeader, "ApiKey ")
		return cfg.AuthService.AuthenticateAPIKey(r.Context(), key)
	}

	return nil, errNoCredentials
}

// RequirePermission creates middleware that checks for a specific permission.
func RequirePermission(perm auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := GetAuthContext(r.Context())
			if !ok || ac == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
				return
			}

			if !auth.HasPermission(ac.Role, perm) {
				slog.Warn("permission denied",
					"path", r.URL.Path,
					"role", string(ac.Role),
					"permission", string(perm),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetAuthContext extracts the AuthContext from a request context.
func GetAuthContext(ctx context.Context) (*auth.AuthContext, bool) {
	ac, ok := ctx.Value(authContextKey).(*auth.AuthContext)
	return ac, ok
}

type authError struct {
	msg string
}

func (e *authError) Error() string { return e.msg }

var errNoCredentials = &authError{msg: "no credentials provided"}
