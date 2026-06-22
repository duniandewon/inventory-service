package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

type contextKey string

const (
	userIDContextKey contextKey = "auth_user_id"
	rolesContextKey  contextKey = "auth_roles"
)

func UserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDContextKey).(int)
	return id, ok
}

func RolesFromContext(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(rolesContextKey).([]string)
	return roles, ok
}

// RequireAuth validates the access token's signature and expiry and loads
// the user id + roles from its claims into the request context. It never
// hits Redis or Postgres.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		userID, roles, err := s.ParseAccessToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		ctx = context.WithValue(ctx, rolesContextKey, roles)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole must be mounted after RequireAuth. It rejects requests whose
// token roles don't include the required role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles, ok := RolesFromContext(r.Context())
			if !ok || !hasRole(roles, role) {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasRole(roles []string, role string) bool {
	return slices.Contains(roles, role)
}
