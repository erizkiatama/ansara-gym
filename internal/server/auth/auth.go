package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authkit "github.com/erizkiatama/ansara-gym/internal/auth"
	"github.com/erizkiatama/ansara-gym/internal/server/respond"
	"github.com/erizkiatama/ansara-gym/internal/trainer"
)

type Handler struct {
	log      *slog.Logger
	tokens   *authkit.Tokens
	trainers TrainerStore
}

func NewHandler(log *slog.Logger, tokens *authkit.Tokens, trainers TrainerStore) *Handler {
	return &Handler{log: log, tokens: tokens, trainers: trainers}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	email, err := normalizeEmail(req.Email)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid email")
		return
	}
	name, err := normalizeName(req.Name)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid name")
		return
	}
	if err := validatePassword(req.Password); err != nil {
		respond.Error(w, http.StatusBadRequest, "password must be 8 to 128 characters")
		return
	}

	hash, err := authkit.Hash(req.Password)
	if err != nil {
		h.log.Error("hash password", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	row, err := h.trainers.Insert(r.Context(), email, hash, name)
	if err != nil {
		if errors.Is(err, trainer.ErrEmailTaken) {
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
		Trainer: trainerResponse{ID: row.ID, Email: row.Email, Name: row.Name},
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	email, err := normalizeEmail(req.Email)
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if req.Password == "" {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	row, err := h.trainers.GetByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, trainer.ErrNotFound) {
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
		Trainer: trainerResponse{ID: row.ID, Email: row.Email, Name: row.Name},
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
		if errors.Is(err, trainer.ErrNotFound) {
			respond.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		h.log.Error("get trainer", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	respond.JSON(w, http.StatusOK, trainerResponse{ID: row.ID, Email: row.Email, Name: row.Name})
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
