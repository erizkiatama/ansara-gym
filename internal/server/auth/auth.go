package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authkit "github.com/erizkiatama/ansara-gym/internal/auth"
	"github.com/erizkiatama/ansara-gym/internal/server/respond"
	store "github.com/erizkiatama/ansara-gym/internal/trainer"
)

type Handler struct {
	log      *slog.Logger
	tokens   *authkit.Tokens
	trainers Repository
}

func NewHandler(log *slog.Logger, tokens *authkit.Tokens, trainers Repository) *Handler {
	return &Handler{log: log, tokens: tokens, trainers: trainers}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if !respond.Bind(w, r, &req) {
		return
	}

	trainer := store.Trainer{
		Email: strings.ToLower(strings.TrimSpace(req.Email)),
		Name:  strings.TrimSpace(req.Name),
	}
	if err := validateSignup(trainer, req.Password); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := authkit.Hash(req.Password)
	if err != nil {
		h.log.Error("hash password", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	trainer.PasswordHash = hash

	row, err := h.trainers.Insert(r.Context(), trainer)
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			respond.Error(w, http.StatusConflict, "email already registered")
			return
		}
		h.log.Error("insert trainer", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	token, err := h.tokens.Issue(row.ID)
	if err != nil {
		h.log.Error("issue token", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	respond.JSON(w, http.StatusCreated, tokenResponse{
		Token:   token,
		Trainer: toResponse(row),
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !respond.Bind(w, r, &req) {
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	row, err := h.trainers.GetByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			authkit.DummyCompare(req.Password)
			respond.Error(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		h.log.Error("get trainer", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := authkit.Compare(row.PasswordHash, req.Password); err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.tokens.Issue(row.ID)
	if err != nil {
		h.log.Error("issue token", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	respond.JSON(w, http.StatusOK, tokenResponse{
		Token:   token,
		Trainer: toResponse(row),
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := authkit.TrainerIDFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	row, err := h.trainers.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		h.log.Error("get trainer", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	respond.JSON(w, http.StatusOK, toResponse(row))
}

func (h *Handler) RequireTrainer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		scheme, raw, ok := strings.Cut(header, " ")
		raw = strings.TrimSpace(raw)
		if !ok || !strings.EqualFold(scheme, "Bearer") || raw == "" {
			respond.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := h.tokens.Verify(raw)
		if err != nil {
			respond.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r.WithContext(authkit.WithTrainerID(r.Context(), id)))
	})
}
