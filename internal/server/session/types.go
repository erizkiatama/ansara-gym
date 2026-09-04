package session

import (
	"context"

	store "github.com/erizkiatama/ansara-gym/internal/session"
)

// Repository is the slice of session persistence this HTTP package needs.
type Repository interface {
	Insert(ctx context.Context, trainerID, clientID string, session store.Session) (store.Session, error)
	List(ctx context.Context, trainerID, clientID string, params store.ListParams) ([]store.Session, bool, error)
	Get(ctx context.Context, trainerID, clientID, sessionID string) (store.Session, error)
	Progress(ctx context.Context, trainerID, clientID, exerciseID string) ([]store.ProgressPoint, error)
}

type sessionRequest struct {
	SessionDate string            `json:"session_date"`
	Notes       string            `json:"notes"`
	Exercises   []exerciseRequest `json:"exercises"`
}

type exerciseRequest struct {
	ExerciseID string       `json:"exercise_id"`
	OrderIndex int          `json:"order_index"`
	Notes      string       `json:"notes"`
	Sets       []setRequest `json:"sets"`
}

type setRequest struct {
	SetNumber int      `json:"set_number"`
	Reps      int      `json:"reps"`
	Weight    float64  `json:"weight"`
	RPE       *float64 `json:"rpe"`
	IsWarmup  bool     `json:"is_warmup"`
}

type sessionResponse struct {
	ID          string             `json:"id"`
	SessionDate string             `json:"session_date"`
	Notes       string             `json:"notes,omitempty"`
	Exercises   []exerciseResponse `json:"exercises"`
}

type exerciseResponse struct {
	ID         string        `json:"id"`
	ExerciseID string        `json:"exercise_id"`
	OrderIndex int           `json:"order_index"`
	Notes      string        `json:"notes,omitempty"`
	Sets       []setResponse `json:"sets"`
}

type setResponse struct {
	ID        string   `json:"id"`
	SetNumber int      `json:"set_number"`
	Reps      int      `json:"reps"`
	Weight    float64  `json:"weight"`
	RPE       *float64 `json:"rpe,omitempty"`
	IsWarmup  bool     `json:"is_warmup"`
}

func toResponse(row store.Session) sessionResponse {
	exercises := make([]exerciseResponse, 0, len(row.Exercises))
	for _, ex := range row.Exercises {
		exercises = append(exercises, toExerciseResponse(ex))
	}
	notes := ""
	if row.Notes.Valid {
		notes = row.Notes.String
	}
	return sessionResponse{
		ID:          row.ID,
		SessionDate: row.SessionDate.Format("2006-01-02"),
		Notes:       notes,
		Exercises:   exercises,
	}
}

func toExerciseResponse(row store.SessionExercise) exerciseResponse {
	sets := make([]setResponse, 0, len(row.Sets))
	for _, st := range row.Sets {
		sets = append(sets, toSetResponse(st))
	}
	notes := ""
	if row.Notes.Valid {
		notes = row.Notes.String
	}
	return exerciseResponse{
		ID:         row.ID,
		ExerciseID: row.ExerciseID,
		OrderIndex: row.OrderIndex,
		Notes:      notes,
		Sets:       sets,
	}
}

func toSetResponse(row store.Set) setResponse {
	var rpe *float64
	if row.RPE.Valid {
		v := row.RPE.Float64
		rpe = &v
	}
	return setResponse{
		ID:        row.ID,
		SetNumber: row.SetNumber,
		Reps:      row.Reps,
		Weight:    row.Weight,
		RPE:       rpe,
		IsWarmup:  row.IsWarmup,
	}
}

type sessionHeader struct {
	ID          string `json:"id"`
	SessionDate string `json:"session_date"`
	Notes       string `json:"notes,omitempty"`
}

type listCursor struct {
	BeforeDate string `json:"before_date"`
	BeforeID   string `json:"before_id"`
}

type listResponse struct {
	Sessions []sessionHeader `json:"sessions"`
	Next     *listCursor     `json:"next,omitempty"`
}

type progressPointResponse struct {
	SessionID   string  `json:"session_id"`
	SessionDate string  `json:"session_date"`
	MaxWeight   float64 `json:"max_weight"`
}

type progressResponse struct {
	Points []progressPointResponse `json:"points"`
}

func toHeader(row store.Session) sessionHeader {
	notes := ""
	if row.Notes.Valid {
		notes = row.Notes.String
	}
	return sessionHeader{
		ID:          row.ID,
		SessionDate: row.SessionDate.Format("2006-01-02"),
		Notes:       notes,
	}
}

func toProgressResponse(points []store.ProgressPoint) progressResponse {
	out := make([]progressPointResponse, 0, len(points))
	for _, p := range points {
		out = append(out, progressPointResponse{
			SessionID:   p.SessionID,
			SessionDate: p.SessionDate.Format("2006-01-02"),
			MaxWeight:   p.MaxWeight,
		})
	}
	return progressResponse{Points: out}
}
