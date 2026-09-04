package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound        = errors.New("session not found")
	ErrUnknownExercise = errors.New("unknown exercise_id")
)

// Session is a trainer-logged workout for one client on one date.
// Exercises and Sets are the nested graph persisted in the same transaction.
type Session struct {
	ID          string         `db:"id"`
	ClientID    string         `db:"client_id"`
	TrainerID   string         `db:"trainer_id"`
	SessionDate time.Time      `db:"session_date"`
	Notes       sql.NullString `db:"notes"`
	Exercises   []SessionExercise
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// SessionExercise is one movement card under a session (catalog exercise_id + order).
type SessionExercise struct {
	ID         string         `db:"id"`
	SessionID  string         `db:"session_id"`
	ExerciseID string         `db:"exercise_id"`
	OrderIndex int            `db:"order_index"`
	Notes      sql.NullString `db:"notes"`
	Sets       []Set
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Set is one logged set under a session exercise card.
type Set struct {
	ID                string          `db:"id"`
	SessionExerciseID string          `db:"session_exercise_id"`
	SetNumber         int             `db:"set_number"`
	Reps              int             `db:"reps"`
	Weight            float64         `db:"weight"`
	RPE               sql.NullFloat64 `db:"rpe"`
	IsWarmup          bool            `db:"is_warmup"`
	CreatedAt         time.Time       `db:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"`
}

type Repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

// Insert writes the session graph in one transaction. trainerID comes from the JWT;
// clientID from the URL. Either belongs-to-another-trainer or unknown client is ErrNotFound.
func (r *Repo) Insert(ctx context.Context, trainerID, clientID string, session Session) (Session, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback() }()

	out, err := r.insertTx(ctx, tx, trainerID, clientID, session)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return out, nil
}

func (r *Repo) insertTx(ctx context.Context, tx *sqlx.Tx, trainerID, clientID string, session Session) (Session, error) {
	var out Session
	err := tx.QueryRowxContext(ctx, `
		INSERT INTO workout_sessions (client_id, trainer_id, session_date, notes)
		SELECT $1, $2, $3, $4
		FROM clients
		WHERE clients.id = $1 AND clients.trainer_id = $2
		RETURNING id, client_id, trainer_id, session_date, notes, created_at, updated_at
	`, clientID, trainerID, session.SessionDate, session.Notes).StructScan(&out)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}

	if err := assertExercisesExist(ctx, tx, uniqueExerciseIDs(session.Exercises)); err != nil {
		return Session{}, err
	}

	out.Exercises = make([]SessionExercise, 0, len(session.Exercises))
	for _, ex := range session.Exercises {
		row, err := insertExercise(ctx, tx, out.ID, ex)
		if err != nil {
			return Session{}, err
		}
		out.Exercises = append(out.Exercises, row)
	}
	return out, nil
}

func insertExercise(ctx context.Context, tx *sqlx.Tx, sessionID string, ex SessionExercise) (SessionExercise, error) {
	var row SessionExercise
	err := tx.QueryRowxContext(ctx, `
		INSERT INTO workout_session_exercises (session_id, exercise_id, order_index, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, session_id, exercise_id, order_index, notes, created_at, updated_at
	`, sessionID, ex.ExerciseID, ex.OrderIndex, ex.Notes).StructScan(&row)
	if err != nil {
		return SessionExercise{}, err
	}

	row.Sets = make([]Set, 0, len(ex.Sets))
	for _, st := range ex.Sets {
		set, err := insertSet(ctx, tx, row.ID, st)
		if err != nil {
			return SessionExercise{}, err
		}
		row.Sets = append(row.Sets, set)
	}
	return row, nil
}

func insertSet(ctx context.Context, tx *sqlx.Tx, sessionExerciseID string, st Set) (Set, error) {
	var row Set
	err := tx.QueryRowxContext(ctx, `
		INSERT INTO workout_sets (session_exercise_id, set_number, reps, weight, rpe, is_warmup)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, session_exercise_id, set_number, reps, weight, rpe, is_warmup, created_at, updated_at
	`, sessionExerciseID, st.SetNumber, st.Reps, st.Weight, st.RPE, st.IsWarmup).StructScan(&row)
	if err != nil {
		return Set{}, err
	}
	return row, nil
}

func uniqueExerciseIDs(exercises []SessionExercise) []string {
	seen := make(map[string]struct{}, len(exercises))
	ids := make([]string, 0, len(exercises))
	for _, ex := range exercises {
		if _, ok := seen[ex.ExerciseID]; ok {
			continue
		}
		seen[ex.ExerciseID] = struct{}{}
		ids = append(ids, ex.ExerciseID)
	}
	return ids
}

func assertExercisesExist(ctx context.Context, tx *sqlx.Tx, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	parts := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	var found []string
	q := `SELECT id FROM exercises WHERE id IN (` + strings.Join(parts, ", ") + `)`
	if err := tx.SelectContext(ctx, &found, q, args...); err != nil {
		return err
	}
	if len(found) != len(ids) {
		return ErrUnknownExercise
	}
	return nil
}
