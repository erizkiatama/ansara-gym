package session

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	database, err := sqlx.Connect("pgx", "postgres://erizkiatama@localhost:5432/ansara_gym?sslmode=disable")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Ping(); err != nil {
		t.Skipf("postgres ping: %v", err)
	}
	return database
}

func TestInsertUnknownExerciseRollsBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	var trainerID string
	err := db.QueryRowxContext(ctx, `
		INSERT INTO trainers (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`, fmt.Sprintf("week5-rollback-%d@example.com", time.Now().UnixNano()), "not-a-real-hash", "Week5").Scan(&trainerID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM clients WHERE trainer_id = $1`, trainerID)
		_, _ = db.Exec(`DELETE FROM trainers WHERE id = $1`, trainerID)
	})

	var clientID string
	err = db.QueryRowxContext(ctx, `
		INSERT INTO clients (trainer_id, name)
		VALUES ($1, $2)
		RETURNING id
	`, trainerID, "Rina").Scan(&clientID)
	if err != nil {
		t.Fatal(err)
	}

	var benchID string
	err = db.GetContext(ctx, &benchID, `SELECT id FROM exercises WHERE name = $1`, "Bench Press")
	if err != nil {
		t.Fatalf("seeded Bench Press: %v", err)
	}

	var before int
	if err := db.GetContext(ctx, &before, `
		SELECT COUNT(*) FROM workout_sessions WHERE client_id = $1
	`, clientID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepo(db)
	_, err = repo.Insert(ctx, trainerID, clientID, Session{
		SessionDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{
			{ExerciseID: benchID, OrderIndex: 0, Sets: []Set{
				{SetNumber: 1, Reps: 8, Weight: 80},
			}},
			{ExerciseID: "00000000-0000-0000-0000-000000000001", OrderIndex: 1},
		},
	})
	if err != ErrUnknownExercise {
		t.Fatalf("got %v, want ErrUnknownExercise", err)
	}

	var after int
	if err := db.GetContext(ctx, &after, `
		SELECT COUNT(*) FROM workout_sessions WHERE client_id = $1
	`, clientID); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("session row leaked: before=%d after=%d", before, after)
	}

	var orphanExercises int
	if err := db.GetContext(ctx, &orphanExercises, `
		SELECT COUNT(*) FROM workout_session_exercises e
		JOIN workout_sessions s ON s.id = e.session_id
		WHERE s.client_id = $1
	`, clientID); err != nil {
		t.Fatal(err)
	}
	if orphanExercises != 0 {
		t.Fatalf("orphan exercises: %d", orphanExercises)
	}
}

func TestInsertHappyPathAndEmptySession(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	var trainerID string
	err := db.QueryRowxContext(ctx, `
		INSERT INTO trainers (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`, fmt.Sprintf("week5-happy-%d@example.com", time.Now().UnixNano()), "not-a-real-hash", "Week5").Scan(&trainerID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM clients WHERE trainer_id = $1`, trainerID)
		_, _ = db.Exec(`DELETE FROM trainers WHERE id = $1`, trainerID)
	})

	var clientID string
	err = db.QueryRowxContext(ctx, `
		INSERT INTO clients (trainer_id, name)
		VALUES ($1, $2)
		RETURNING id
	`, trainerID, "Rina").Scan(&clientID)
	if err != nil {
		t.Fatal(err)
	}

	var squatID string
	err = db.GetContext(ctx, &squatID, `SELECT id FROM exercises WHERE name = $1`, "Back Squat")
	if err != nil {
		t.Fatalf("seeded Back Squat: %v", err)
	}

	repo := NewRepo(db)

	empty, err := repo.Insert(ctx, trainerID, clientID, Session{
		SessionDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.ID == "" || len(empty.Exercises) != 0 {
		t.Fatalf("empty %#v", empty)
	}

	planned, err := repo.Insert(ctx, trainerID, clientID, Session{
		SessionDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{
			{ExerciseID: squatID, OrderIndex: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Exercises) != 1 || planned.Exercises[0].ID == "" || len(planned.Exercises[0].Sets) != 0 {
		t.Fatalf("planned %#v", planned.Exercises)
	}

	logged, err := repo.Insert(ctx, trainerID, clientID, Session{
		SessionDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{
			{
				ExerciseID: squatID,
				OrderIndex: 0,
				Sets: []Set{{
					SetNumber: 1,
					Reps:      5,
					Weight:    100.5,
					RPE:       sql.NullFloat64{Float64: 8, Valid: true},
					IsWarmup:  false,
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(logged.Exercises) != 1 || len(logged.Exercises[0].Sets) != 1 {
		t.Fatalf("logged %#v", logged.Exercises)
	}
	set := logged.Exercises[0].Sets[0]
	if set.ID == "" || set.Weight != 100.5 || !set.RPE.Valid || set.RPE.Float64 != 8 {
		t.Fatalf("set %#v", set)
	}

	_, err = repo.Insert(ctx, trainerID, "00000000-0000-0000-0000-000000000099", Session{
		SessionDate: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
	})
	if err != ErrNotFound {
		t.Fatalf("cross-tenant/missing client: got %v", err)
	}
}
