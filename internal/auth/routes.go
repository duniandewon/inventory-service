package auth

import "github.com/go-chi/chi/v5"

func RegisterRoutes(r chi.Router, service *Service, handler *Handler) {
	r.Post("/auth/otp/request", handler.RequestOTP)
	r.Post("/auth/otp/verify", handler.VerifyOTP)
	r.Post("/auth/refresh", handler.Refresh)
	r.Post("/auth/logout", handler.Logout)

	r.Group(func(r chi.Router) {
		r.Use(service.RequireAuth)
		r.Get("/auth/me", handler.Me)
	})
}
