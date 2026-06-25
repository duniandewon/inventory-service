package logistics

import (
	"github.com/duniandewon/inventory-service/internal/auth"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, authService *auth.Service, handler *Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authService.RequireAuth)

		// Process types — mutations are OWNER-only
		r.Get("/api/process-types", handler.ListProcessTypes)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("OWNER"))
			r.Post("/api/process-types", handler.CreateProcessType)
			r.Patch("/api/process-types/{id}", handler.UpdateProcessType)
			r.Delete("/api/process-types/{id}", handler.DeleteProcessType)
		})

		// Work orders
		r.Post("/api/work-orders", handler.CreateWorkOrder)
		r.Get("/api/work-orders", handler.ListWorkOrders)
		r.Get("/api/work-orders/{id}", handler.GetWorkOrder)
		r.Post("/api/work-orders/{id}/inputs", handler.AssignInputs)
		r.Post("/api/work-orders/{id}/outputs", handler.ReceiveOutputs)

		// Delivery notes
		r.Post("/api/delivery-notes", handler.CreateDeliveryNote)
		r.Get("/api/delivery-notes", handler.ListDeliveryNotes)
		r.Get("/api/delivery-notes/{id}", handler.GetDeliveryNote)

		// Lineage — mounted here because the linkage data lives in logistics,
		// even though the path looks like it belongs to inventory.
		r.Get("/api/inventory/{id}/lineage", handler.GetLineage)
	})
}
