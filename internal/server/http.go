package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/erizkiatama/ansara-gym/internal/auth"
	"github.com/erizkiatama/ansara-gym/internal/client"
	"github.com/erizkiatama/ansara-gym/internal/config"
	authhttp "github.com/erizkiatama/ansara-gym/internal/server/auth"
	clienthttp "github.com/erizkiatama/ansara-gym/internal/server/client"
	"github.com/erizkiatama/ansara-gym/internal/server/respond"
	"github.com/erizkiatama/ansara-gym/internal/trainer"
	"github.com/jmoiron/sqlx"
)

type Server struct {
	log     *slog.Logger
	db      *sqlx.DB
	auth    *authhttp.Handler
	clients *clienthttp.Handler
}

func New(log *slog.Logger, database *sqlx.DB, authCfg config.AuthConfig) http.Handler {
	s := &Server{
		log: log,
		db:  database,
		auth: authhttp.NewHandler(
			log,
			auth.NewTokens(authCfg.JWTSecret, authCfg.JWTTTL),
			trainer.NewRepo(database),
		),
		clients: clienthttp.NewHandler(log, client.NewRepo(database)),
	}
	return s.routes()
}

type healthResponse struct {
	Status string `json:"status"`
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		s.log.Error("health check db ping failed", "err", err)
		respond.JSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
		return
	}

	respond.JSON(w, http.StatusOK, healthResponse{Status: "ok"})
}
