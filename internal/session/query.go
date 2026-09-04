package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const progressQuery = `
	SELECT s.id AS session_id, s.session_date, MAX(st.weight) AS max_weight
	FROM workout_sessions s
	INNER JOIN workout_session_exercises e ON e.session_id = s.id
	INNER JOIN workout_sets st ON st.session_exercise_id = e.id AND st.is_warmup = false
	WHERE s.client_id = $1
	  AND s.trainer_id = $2
	  AND e.exercise_id = $3
	GROUP BY s.id, s.session_date
	ORDER BY s.session_date ASC, s.id ASC
`

func (r *Repo) assertClient(ctx context.Context, trainerID, clientID string) error {
	var id string
	err := r.db.GetContext(ctx, &id, `
		SELECT id FROM clients WHERE trainer_id = $1 AND id = $2
	`, trainerID, clientID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// List returns up to params.Limit sessions, newest session_date first.
// hasMore is true when another page exists (fetched limit+1 internally).
func (r *Repo) List(ctx context.Context, trainerID, clientID string, params ListParams) ([]Session, bool, error) {
	if err := r.assertClient(ctx, trainerID, clientID); err != nil {
		return nil, false, err
	}

	fetch := params.Limit + 1
	var rows []Session
	var err error
	if params.BeforeID == "" {
		err = r.db.SelectContext(ctx, &rows, `
			SELECT id, client_id, trainer_id, session_date, notes, created_at, updated_at
			FROM workout_sessions
			WHERE client_id = $1 AND trainer_id = $2
			ORDER BY session_date DESC, id DESC
			LIMIT $3
		`, clientID, trainerID, fetch)
	} else {
		err = r.db.SelectContext(ctx, &rows, `
			SELECT id, client_id, trainer_id, session_date, notes, created_at, updated_at
			FROM workout_sessions
			WHERE client_id = $1 AND trainer_id = $2
			  AND (session_date, id) < ($3, $4)
			ORDER BY session_date DESC, id DESC
			LIMIT $5
		`, clientID, trainerID, params.BeforeDate, params.BeforeID, fetch)
	}
	if err != nil {
		return nil, false, err
	}
	if rows == nil {
		rows = []Session{}
	}
	hasMore := len(rows) > params.Limit
	if hasMore {
		rows = rows[:params.Limit]
	}
	return rows, hasMore, nil
}

// Get returns one session graph scoped to the trainer's client.
func (r *Repo) Get(ctx context.Context, trainerID, clientID, sessionID string) (Session, error) {
	if err := r.assertClient(ctx, trainerID, clientID); err != nil {
		return Session{}, err
	}

	var row Session
	err := r.db.GetContext(ctx, &row, `
		SELECT id, client_id, trainer_id, session_date, notes, created_at, updated_at
		FROM workout_sessions
		WHERE trainer_id = $1 AND client_id = $2 AND id = $3
	`, trainerID, clientID, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}

	if err := r.loadGraph(ctx, &row); err != nil {
		return Session{}, err
	}
	return row, nil
}

func (r *Repo) loadGraph(ctx context.Context, session *Session) error {
	var exercises []SessionExercise
	err := r.db.SelectContext(ctx, &exercises, `
		SELECT id, session_id, exercise_id, order_index, notes, created_at, updated_at
		FROM workout_session_exercises
		WHERE session_id = $1
		ORDER BY order_index, id
	`, session.ID)
	if err != nil {
		return err
	}
	if exercises == nil {
		exercises = []SessionExercise{}
	}

	ids := make([]string, 0, len(exercises))
	for _, ex := range exercises {
		ids = append(ids, ex.ID)
	}
	setsByCard, err := r.setsByExerciseIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range exercises {
		sets := setsByCard[exercises[i].ID]
		if sets == nil {
			sets = []Set{}
		}
		exercises[i].Sets = sets
	}
	session.Exercises = exercises
	return nil
}

func (r *Repo) setsByExerciseIDs(ctx context.Context, ids []string) (map[string][]Set, error) {
	out := map[string][]Set{}
	if len(ids) == 0 {
		return out, nil
	}

	parts := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	var rows []Set
	q := `
		SELECT id, session_exercise_id, set_number, reps, weight, rpe, is_warmup, created_at, updated_at
		FROM workout_sets
		WHERE session_exercise_id IN (` + strings.Join(parts, ", ") + `)
		ORDER BY session_exercise_id, set_number, id
	`
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	for _, st := range rows {
		out[st.SessionExerciseID] = append(out[st.SessionExerciseID], st)
	}
	return out, nil
}

// Progress returns max working-set kg per session for a catalog exercise.
// Unknown or never-logged exercise_id yields an empty slice, not an error.
func (r *Repo) Progress(ctx context.Context, trainerID, clientID, exerciseID string) ([]ProgressPoint, error) {
	if err := r.assertClient(ctx, trainerID, clientID); err != nil {
		return nil, err
	}

	var points []ProgressPoint
	if err := r.db.SelectContext(ctx, &points, progressQuery, clientID, trainerID, exerciseID); err != nil {
		return nil, err
	}
	if points == nil {
		points = []ProgressPoint{}
	}
	return points, nil
}
