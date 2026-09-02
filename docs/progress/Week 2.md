# Week 2 — Schema Design & Migrations

**Status**: Done (2026-09-02)

## Shipped

- `internal/db/migrations/00001_init_schema.sql` — 6 core tables, PKs/FKs/indexes/checks
- `internal/db/migrations/00002_seed_exercises.sql` — 30 catalog rows, `ON CONFLICT (name) DO NOTHING`, no-op down
- `internal/db/migrate.go` — goose `Provider` over `embed.FS` (SQL only, no Go migrations)
- `cmd/migrate` — thin binary: `up`, `down`, `reset`, `status`. Same Viper config + `db.Open` as `cmd/server`. Migrations are **not** run on HTTP boot.

## Decisions

- **Tool**: goose v3 (`github.com/pressly/goose/v3`), not golang-migrate. SQL files with `-- +goose Up` / `-- +goose Down`. Sequential versions (`00001`, `00002`). Library-in-binary rather than a global CLI, matching `cmd/` = binaries only.
- **Apply path**: explicit `cmd/migrate`. Auto-migrate on `cmd/server` boot is deferred (multi-instance race, mixes schema ownership with request serving).
- **Seed vs schema**: catalog is data, not reversible schema. Version 2 inserts idempotently; its down is empty (`empty: true`). Destroying the catalog is `down` of version 1 (`DROP TABLE`).
- **IDs**: `uuid PRIMARY KEY DEFAULT uuidv7()`. Postgres 18 provides `uuidv7()` in core (no `pgcrypto`). Time-ordered UUIDs avoid random v4 B-tree page splits while remaining unguessable-enough for API ids. `bigint` identity was rejected: public ids in URLs/JWTs later.
- **`is_self`**: not a column (roadmap deferred — self-tracking is a normal client row).
- **`updated_at`**: `timestamptz NOT NULL DEFAULT now()` on all 6 tables. Default covers INSERT only — UPDATE statements must set `updated_at = now()` (no trigger yet). Rewritten in `00001` (not a follow-up migration) while this schema is still unpublished.
- **ON DELETE**: `RESTRICT` on trainer FKs and `exercises` (history + catalog). `CASCADE` on session → session_exercise → set (composition).
- **Indexes**: btree on every FK that is not already the leftmost column of a UNIQUE constraint (`session_id` and `session_exercise_id` are covered). Extra btree on `workout_sessions.session_date`. Composite `(client_id, session_date DESC)` deferred to Week 6 (`EXPLAIN ANALYZE`).
- **Weight / RPE**: `numeric(6,2)` kg, `numeric(3,1)` nullable RPE with `CHECK (rpe IS NULL OR (rpe >= 1 AND rpe <= 10))`. Failed sets allowed (`reps >= 0`).
- **Locking**: goose session/advisory lock not enabled. Add before multiple migrate processes can run (Week 12 deploy).

## Verify

```text
go run ./cmd/migrate up
go run ./cmd/migrate down    # seed no-op; 30 exercises remain
go run ./cmd/migrate down    # drops 6 tables; goose_db_version remains
go run ./cmd/migrate up      # 30 exercises again
psql -d ansara_gym -c '\d+ trainers'
```

`go build ./...` and `go vet ./...` clean. Local DB left at version 2 after the up/down cycle.
