-- +goose Up

CREATE TABLE trainers (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    name          text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT trainers_email_not_empty CHECK (email <> ''),
    CONSTRAINT trainers_password_hash_not_empty CHECK (password_hash <> ''),
    CONSTRAINT trainers_name_not_empty CHECK (name <> '')
);

CREATE TABLE clients (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    trainer_id uuid NOT NULL REFERENCES trainers (id) ON DELETE RESTRICT,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT clients_name_not_empty CHECK (name <> '')
);

CREATE INDEX clients_trainer_id_idx ON clients (trainer_id);

CREATE TABLE exercises (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL UNIQUE,
    category   text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT exercises_name_not_empty CHECK (name <> ''),
    CONSTRAINT exercises_category_not_empty CHECK (category <> '')
);

CREATE TABLE workout_sessions (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    client_id    uuid NOT NULL REFERENCES clients (id) ON DELETE CASCADE,
    trainer_id   uuid NOT NULL REFERENCES trainers (id) ON DELETE RESTRICT,
    session_date date NOT NULL,
    notes        text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workout_sessions_client_id_idx ON workout_sessions (client_id);
CREATE INDEX workout_sessions_trainer_id_idx ON workout_sessions (trainer_id);
CREATE INDEX workout_sessions_session_date_idx ON workout_sessions (session_date);

CREATE TABLE workout_session_exercises (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    session_id  uuid NOT NULL REFERENCES workout_sessions (id) ON DELETE CASCADE,
    exercise_id uuid NOT NULL REFERENCES exercises (id) ON DELETE RESTRICT,
    order_index integer NOT NULL,
    notes       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workout_session_exercises_order_nonnegative CHECK (order_index >= 0),
    CONSTRAINT workout_session_exercises_session_order_key UNIQUE (session_id, order_index)
);

CREATE INDEX workout_session_exercises_exercise_id_idx ON workout_session_exercises (exercise_id);

CREATE TABLE workout_sets (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    session_exercise_id uuid NOT NULL REFERENCES workout_session_exercises (id) ON DELETE CASCADE,
    set_number          integer NOT NULL,
    reps                integer NOT NULL,
    weight              numeric(6, 2) NOT NULL,
    rpe                 numeric(3, 1),
    is_warmup           boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workout_sets_set_number_positive CHECK (set_number > 0),
    CONSTRAINT workout_sets_reps_nonnegative CHECK (reps >= 0),
    CONSTRAINT workout_sets_weight_nonnegative CHECK (weight >= 0),
    CONSTRAINT workout_sets_rpe_range CHECK (rpe IS NULL OR (rpe >= 1 AND rpe <= 10)),
    CONSTRAINT workout_sets_session_exercise_set_key UNIQUE (session_exercise_id, set_number)
);

-- +goose Down

DROP TABLE IF EXISTS workout_sets;
DROP TABLE IF EXISTS workout_session_exercises;
DROP TABLE IF EXISTS workout_sessions;
DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS exercises;
DROP TABLE IF EXISTS trainers;
