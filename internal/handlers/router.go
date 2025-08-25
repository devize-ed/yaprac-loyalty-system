package handlers

import (
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", h.CreateUser)
		r.Post("/login", h.LoginUser)
		r.Post("/orders", h.LogoutUser)
		r.Get("/orders", h.GetOrders)
		r.Get("/balance", h.GetBalance)
		r.Post("/balance/withdraw", h.WithdrawBalance)
		r.Get("/balance/withdrawals", h.GetWithdrawals)
	})

	return r
}
