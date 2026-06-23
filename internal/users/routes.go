package users

import (
	"github.com/duniandewon/inventory-service/internal/auth"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, authService *auth.Service, handler *Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authService.RequireAuth)
		r.Get("/api/roles", handler.ListRoles)
		r.Get("/api/users/{id}/roles", handler.ListUserRoles)
		r.Post("/api/users/{id}/roles", handler.AssignRole)
		r.Delete("/api/users/{id}/roles/{roleID}", handler.RevokeRole)
	})
}
