package handlers

import (
	"loyaltySys/internal/auth"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// NewRouter creates a new router for the handler
func (h *Handler) NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Route("/api/user", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(auth.TokenAuth))
			r.Use(jwtauth.Authenticator(auth.TokenAuth))
			r.Post("/orders", h.CreateOrder())
			r.Get("/orders", h.GetOrders())
			r.Get("/balance", h.GetBalance())
			r.Post("/balance/withdraw", h.Withdraw())
			r.Get("/balance/withdrawals", h.GetWithdrawals())
		})
		r.Post("/register", h.CreateUser())
		r.Post("/login", h.LoginUser())
	})

	return r
}
