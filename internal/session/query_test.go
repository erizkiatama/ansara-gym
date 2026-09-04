package session

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func seedTrainerClient(t *testing.T, db *sqlx.DB, label string) (trainerID, clientID string) {
	t.Helper()
	ctx := context.Background()
	err := db.QueryRowxContext(ctx, `
		INSERT INTO trainers (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`, fmt.Sprintf("week6-%s-%d@example.com", label, time.Now().UnixNano()), "not-a-real-hash", "Week6").Scan(&trainerID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM clients WHERE trainer_id = $1`, trainerID)
		_, _ = db.Exec(`DELETE FROM trainers WHERE id = $1`, trainerID)
	})

	err = db.QueryRowxContext(ctx, `
		INSERT INTO clients (trainer_id, name)
		VALUES ($1, $2)
		RETURNING id
	`, trainerID, "Rina").Scan(&clientID)
	if err != nil {
		t.Fatal(err)
	}
	return trainerID, clientID
}

func catalogID(t *testing.T, db *sqlx.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Get(&id, `SELECT id FROM exercises WHERE name = $1`, name); err != nil {
		t.Fatalf("seeded %s: %v", name, err)
	}
	return id
}

func TestListKeysetAndGetGraph(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	trainerID, clientID := seedTrainerClient(t, db, "list")
	repo := NewRepo(db)
	bench := catalogID(t, db, "Bench Press")

	dates := []time.Time{
		time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC),
	}
	var created []Session
	for i, day := range dates {
		row, err := repo.Insert(ctx, trainerID, clientID, Session{
			SessionDate: day,
			Exercises: []SessionExercise{{
				ExerciseID: bench,
				OrderIndex: 0,
				Sets:       []Set{{SetNumber: 1, Reps: 5, Weight: float64(70 + i)}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, row)
	}

	page1, hasMore, err := repo.List(ctx, trainerID, clientID, ListParams{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(page1) != 2 {
		t.Fatalf("page1 len=%d hasMore=%v", len(page1), hasMore)
	}
	if page1[0].SessionDate.Format("2006-01-02") != "2026-09-11" || page1[1].SessionDate.Format("2006-01-02") != "2026-09-09" {
		t.Fatalf("order %#v %#v", page1[0].SessionDate, page1[1].SessionDate)
	}

	page2, hasMore, err := repo.List(ctx, trainerID, clientID, ListParams{
		Limit:      2,
		BeforeDate: page1[1].SessionDate,
		BeforeID:   page1[1].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasMore || len(page2) != 1 || page2[0].SessionDate.Format("2006-01-02") != "2026-09-07" {
		t.Fatalf("page2 len=%d hasMore=%v date=%v", len(page2), hasMore, page2[0].SessionDate)
	}

	got, err := repo.Get(ctx, trainerID, clientID, created[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Exercises) != 1 || len(got.Exercises[0].Sets) != 1 || got.Exercises[0].Sets[0].Weight != 72 {
		t.Fatalf("graph %#v", got.Exercises)
	}

	_, err = repo.Get(ctx, trainerID, clientID, "00000000-0000-0000-0000-000000000099")
	if err != ErrNotFound {
		t.Fatalf("missing session: %v", err)
	}
	_, _, err = repo.List(ctx, trainerID, "00000000-0000-0000-0000-000000000099", ListParams{Limit: 20})
	if err != ErrNotFound {
		t.Fatalf("missing client list: %v", err)
	}
}

func TestProgressExcludesWarmup(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	trainerID, clientID := seedTrainerClient(t, db, "progress")
	repo := NewRepo(db)
	bench := catalogID(t, db, "Bench Press")
	squat := catalogID(t, db, "Back Squat")

	_, err := repo.Insert(ctx, trainerID, clientID, Session{
		SessionDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{{
			ExerciseID: bench,
			OrderIndex: 0,
			Sets: []Set{
				{SetNumber: 1, Reps: 5, Weight: 60, IsWarmup: true},
				{SetNumber: 2, Reps: 5, Weight: 80, IsWarmup: false},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Insert(ctx, trainerID, clientID, Session{
		SessionDate: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{{
			ExerciseID: squat,
			OrderIndex: 0,
			Sets:       []Set{{SetNumber: 1, Reps: 5, Weight: 140, IsWarmup: false}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	points, err := repo.Progress(ctx, trainerID, clientID, bench)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].MaxWeight != 80 {
		t.Fatalf("bench %#v", points)
	}

	none, err := repo.Progress(ctx, trainerID, clientID, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("unknown exercise %#v", none)
	}
}

func TestProgressQueryUsesIndex(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	var idx int
	if err := db.GetContext(ctx, &idx, `
		SELECT COUNT(*) FROM pg_indexes WHERE indexname = $1
	`, "workout_session_exercises_exercise_session_idx"); err != nil {
		t.Fatal(err)
	}
	if idx == 0 {
		t.Skip("run go run ./cmd/migrate up (00004 week6 indexes)")
	}

	trainerID, clientID := seedTrainerClient(t, db, "explain")
	squat := catalogID(t, db, "Back Squat")
	bench := catalogID(t, db, "Bench Press")

	_, err := db.ExecContext(ctx, `
		WITH s AS (
			INSERT INTO workout_sessions (client_id, trainer_id, session_date)
			SELECT $1, $2, DATE '2020-01-01' + g
			FROM generate_series(0, 4999) AS g
			RETURNING id
		)
		INSERT INTO workout_session_exercises (session_id, exercise_id, order_index)
		SELECT id, $3, 0 FROM s
	`, clientID, trainerID, squat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ANALYZE workout_sessions`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ANALYZE workout_session_exercises`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ANALYZE workout_sets`); err != nil {
		t.Fatal(err)
	}

	var lines []string
	if err := db.SelectContext(ctx, &lines, `EXPLAIN `+progressQuery, clientID, trainerID, bench); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(lines, "\n")
	if !strings.Contains(plan, "workout_session_exercises_exercise_session_idx") {
		t.Fatalf("expected index in plan:\n%s", plan)
	}
	if strings.Contains(plan, "Seq Scan on workout_session_exercises") {
		t.Fatalf("seq scan on workout_session_exercises:\n%s", plan)
	}
}
