package client

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authkit "github.com/erizkiatama/ansara-gym/internal/auth"
	store "github.com/erizkiatama/ansara-gym/internal/client"
	"github.com/erizkiatama/ansara-gym/internal/server/respond"
	"github.com/erizkiatama/ansara-gym/internal/utils"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log     *slog.Logger
	clients Repository
}

func NewHandler(log *slog.Logger, clients Repository) *Handler {
	return &Handler{log: log, clients: clients}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	trainerID, ok := authkit.TrainerIDFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req clientRequest
	if !respond.Bind(w, r, &req) {
		return
	}

	name := strings.TrimSpace(req.Name)
	notes := strings.TrimSpace(req.Notes)
	client := store.Client{
		Name: name,
		Notes: sql.NullString{
			String: notes,
			Valid:  notes != "",
		},
	}
	if err := validateClient(client); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	row, err := h.clients.Insert(r.Context(), trainerID, client)
	if err != nil {
		h.log.Error("insert client", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	respond.JSON(w, http.StatusCreated, toResponse(row))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	trainerID, ok := authkit.TrainerIDFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.clients.List(r.Context(), trainerID)
	if err != nil {
		h.log.Error("list clients", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]clientResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResponse(row))
	}
	respond.JSON(w, http.StatusOK, listResponse{Clients: out})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	trainerID, id, ok := h.scopedID(w, r)
	if !ok {
		return
	}

	row, err := h.clients.Get(r.Context(), trainerID, id)
	if err != nil {
		h.writeStoreErr(w, err, "get client")
		return
	}

	respond.JSON(w, http.StatusOK, toResponse(row))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	trainerID, id, ok := h.scopedID(w, r)
	if !ok {
		return
	}

	var req clientRequest
	if !respond.Bind(w, r, &req) {
		return
	}

	name := strings.TrimSpace(req.Name)
	notes := strings.TrimSpace(req.Notes)
	client := store.Client{
		Name: name,
		Notes: sql.NullString{
			String: notes,
			Valid:  notes != "",
		},
	}
	if err := validateClient(client); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	row, err := h.clients.Update(r.Context(), trainerID, id, client)
	if err != nil {
		h.writeStoreErr(w, err, "update client")
		return
	}

	respond.JSON(w, http.StatusOK, toResponse(row))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	trainerID, id, ok := h.scopedID(w, r)
	if !ok {
		return
	}

	if err := h.clients.Delete(r.Context(), trainerID, id); err != nil {
		h.writeStoreErr(w, err, "delete client")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) scopedID(w http.ResponseWriter, r *http.Request) (trainerID, id string, ok bool) {
	trainerID, ok = authkit.TrainerIDFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return "", "", false
	}
	id = chi.URLParam(r, "id")
	if !utils.ValidID(id) {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return "", "", false
	}
	return trainerID, id, true
}

func (h *Handler) writeStoreErr(w http.ResponseWriter, err error, op string) {
	if errors.Is(err, store.ErrNotFound) {
		respond.Error(w, http.StatusNotFound, "not found")
		return
	}
	h.log.Error(op, "err", err)
	respond.Error(w, http.StatusInternalServerError, "internal error")
}
