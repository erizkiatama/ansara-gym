package session

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authkit "github.com/erizkiatama/ansara-gym/internal/auth"
	"github.com/erizkiatama/ansara-gym/internal/server/respond"
	store "github.com/erizkiatama/ansara-gym/internal/session"
	"github.com/erizkiatama/ansara-gym/internal/utils"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log      *slog.Logger
	sessions Repository
}

func NewHandler(log *slog.Logger, sessions Repository) *Handler {
	return &Handler{log: log, sessions: sessions}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	trainerID, clientID, ok := h.scopedClient(w, r)
	if !ok {
		return
	}

	var req sessionRequest
	if !respond.Bind(w, r, &req) {
		return
	}

	sessionDate, err := parseSessionDate(strings.TrimSpace(req.SessionDate))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid session_date")
		return
	}

	notes := strings.TrimSpace(req.Notes)
	session := store.Session{
		SessionDate: sessionDate,
		Notes: sql.NullString{
			String: notes,
			Valid:  notes != "",
		},
		Exercises: make([]store.SessionExercise, 0, len(req.Exercises)),
	}
	for _, ex := range req.Exercises {
		exNotes := strings.TrimSpace(ex.Notes)
		card := store.SessionExercise{
			ExerciseID: strings.TrimSpace(ex.ExerciseID),
			OrderIndex: ex.OrderIndex,
			Notes: sql.NullString{
				String: exNotes,
				Valid:  exNotes != "",
			},
			Sets: make([]store.Set, 0, len(ex.Sets)),
		}
		for _, st := range ex.Sets {
			set := store.Set{
				SetNumber: st.SetNumber,
				Reps:      st.Reps,
				Weight:    st.Weight,
				IsWarmup:  st.IsWarmup,
			}
			if st.RPE != nil {
				set.RPE = sql.NullFloat64{Float64: *st.RPE, Valid: true}
			}
			card.Sets = append(card.Sets, set)
		}
		session.Exercises = append(session.Exercises, card)
	}

	if err := validateSession(session); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	row, err := h.sessions.Insert(r.Context(), trainerID, clientID, session)
	if err != nil {
		h.writeStoreErr(w, err, "insert session")
		return
	}

	respond.JSON(w, http.StatusCreated, toResponse(row))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	trainerID, clientID, ok := h.scopedClient(w, r)
	if !ok {
		return
	}

	page, err := parseListPage(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, hasMore, err := h.sessions.List(r.Context(), trainerID, clientID, page)
	if err != nil {
		h.writeStoreErr(w, err, "list sessions")
		return
	}

	out := make([]sessionHeader, 0, len(rows))
	for _, row := range rows {
		out = append(out, toHeader(row))
	}
	resp := listResponse{Sessions: out}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		resp.Next = &listCursor{
			BeforeDate: last.SessionDate.Format("2006-01-02"),
			BeforeID:   last.ID,
		}
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	trainerID, clientID, ok := h.scopedClient(w, r)
	if !ok {
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	if !utils.ValidID(sessionID) {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	row, err := h.sessions.Get(r.Context(), trainerID, clientID, sessionID)
	if err != nil {
		h.writeStoreErr(w, err, "get session")
		return
	}

	respond.JSON(w, http.StatusOK, toResponse(row))
}

func (h *Handler) Progress(w http.ResponseWriter, r *http.Request) {
	trainerID, clientID, ok := h.scopedClient(w, r)
	if !ok {
		return
	}

	exerciseID := chi.URLParam(r, "exerciseId")
	if !utils.ValidID(exerciseID) {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	points, err := h.sessions.Progress(r.Context(), trainerID, clientID, exerciseID)
	if err != nil {
		h.writeStoreErr(w, err, "get progress")
		return
	}

	respond.JSON(w, http.StatusOK, toProgressResponse(points))
}

func (h *Handler) scopedClient(w http.ResponseWriter, r *http.Request) (trainerID, clientID string, ok bool) {
	trainerID, ok = authkit.TrainerIDFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "unauthorized")
		return "", "", false
	}
	clientID = chi.URLParam(r, "id")
	if !utils.ValidID(clientID) {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return "", "", false
	}
	return trainerID, clientID, true
}

func (h *Handler) writeStoreErr(w http.ResponseWriter, err error, op string) {
	if errors.Is(err, store.ErrNotFound) {
		respond.Error(w, http.StatusNotFound, "not found")
		return
	}
	if errors.Is(err, store.ErrUnknownExercise) {
		respond.Error(w, http.StatusBadRequest, "unknown exercise_id")
		return
	}
	h.log.Error(op, "err", err)
	respond.Error(w, http.StatusInternalServerError, "internal error")
}
