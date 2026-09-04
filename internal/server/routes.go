package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.healthz)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/signup", s.auth.Signup)
			r.Post("/login", s.auth.Login)

			r.Group(func(r chi.Router) {
				r.Use(s.auth.RequireTrainer)
				r.Get("/me", s.auth.Me)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(s.auth.RequireTrainer)
			r.Post("/clients", s.clients.Create)
			r.Get("/clients", s.clients.List)
			r.Get("/clients/{id}", s.clients.Get)
			r.Put("/clients/{id}", s.clients.Update)
			r.Delete("/clients/{id}", s.clients.Delete)
		})
	})

	return r
}
