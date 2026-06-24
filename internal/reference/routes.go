package reference

import (
	"github.com/duniandewon/inventory-service/internal/auth"
	"github.com/go-chi/chi/v5"
)

// Mutations are gated to OWNER. There is no ADMIN role seeded yet (see
// db/migrations/002_seed_owner_and_roles.sql), so OWNER stands in for the
// "OWNER / ADMIN" access described in context/features/units-of-measure.md §4.
func RegisterRoutes(r chi.Router, authService *auth.Service, handler *Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authService.RequireAuth)
		r.Get("/api/uom", handler.ListUnits)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("OWNER"))
			r.Post("/api/uom", handler.CreateUnit)
			r.Patch("/api/uom/{id}", handler.UpdateUnit)
			r.Delete("/api/uom/{id}", handler.DeleteUnit)
		})
	})
}
