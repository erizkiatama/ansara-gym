package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
)

type Server struct {
	log *slog.Logger
	db  *sqlx.DB
}

func New(log *slog.Logger, database *sqlx.DB) http.Handler {
	s := &Server{log: log, db: database}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.healthz)

	return r
}

type healthResponse struct {
	Status string `json:"status"`
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		s.log.Error("health check db ping failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}
