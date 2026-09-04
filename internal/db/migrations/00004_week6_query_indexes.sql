-- +goose Up

-- List/history keyset: newest first by session_date, then id.
CREATE INDEX workout_sessions_client_trainer_date_id_idx
    ON workout_sessions (client_id, trainer_id, session_date DESC, id DESC);

-- Progress: composite replaces the single-column exercise_id index from 00001.
CREATE INDEX workout_session_exercises_exercise_session_idx
    ON workout_session_exercises (exercise_id, session_id);
DROP INDEX workout_session_exercises_exercise_id_idx;

-- +goose Down

CREATE INDEX workout_session_exercises_exercise_id_idx
    ON workout_session_exercises (exercise_id);
DROP INDEX IF EXISTS workout_session_exercises_exercise_session_idx;
DROP INDEX IF EXISTS workout_sessions_client_trainer_date_id_idx;
