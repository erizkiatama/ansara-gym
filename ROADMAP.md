# Ansara Gym — Roadmap

## Project Overview

**Ansara Gym** is a trainer-client workout tracking backend. A trainer signs up, manages their own clients, and logs workout sessions for them — tracking exercises, sets, reps, and weight per set. Trainers can also track themselves by creating a client record for their own name.

This roadmap maps the MVP feature set to a 12-week Go Senior Backend learning path, per `.cursorrules`.

**Active week: 4** (Weeks 1–3 complete). Progress and decisions for finished weeks live in [`docs/progress/`](docs/progress/) (`Week 1.md`, `Week 2.md`, `Week 3.md`, …).

## Architecture Decisions (locked in during Phase 1)

- **Domain**: Trainer ↔ Client is a simple one-to-many relationship (`clients.trainer_id` FK). No history tracking for trainer switches in the MVP — deliberately deferred to feel the migration pain later if/when needed.
- **Tenancy**: Shared schema, `trainer_id`-scoped filtering at the repository layer for all queries. Postgres Row-Level Security (RLS) is explicitly **deferred** until a second access path exists (e.g. client-facing login), scheduled as a Week 11 hardening topic.
- **Auth**: JWT (stateless), trainer-only for the MVP. Clients have no login/account — they are records owned by a trainer.
- **Exercise catalog**: Global, shared catalog of standard exercises (e.g. "Bench Press", "Squat"). No per-trainer custom exercises in the MVP.
- **Workout data model**: Three-level hierarchy — `workout_session` → `workout_session_exercise` → `workout_set`. Weight and reps are tracked **per set** (not per exercise), since both can legitimately vary across sets (warm-ups, ramping, fatigue). Weight is stored in kg only (no `weight_unit` column — YAGNI until multi-unit support is actually needed). `rpe` (Rate of Perceived Exertion, 1–10 subjective effort scale) is included as an optional nullable field on `workout_set`.
- **Stack**: `chi` (routing/middleware) + `sqlx` over the `pgx/v5/stdlib` driver (Postgres-native performance with familiar `database/sql`/`sqlx` ergonomics) + `slog` (structured logging) + Viper (`config.json` + env).
- **Deployment**: Docker image, deployed to a cloud-managed platform (Fly.io/Render) that builds directly from the `Dockerfile` — full Docker learning without raw VM ops overhead (nginx, systemd, TLS).

## Core Entities

| Entity | Purpose | Key Fields |
|---|---|---|
| `trainers` | Authenticated users who own clients | `id`, `email`, `password_hash`, `name` |
| `clients` | People a trainer trains (no login) | `id`, `trainer_id` (FK), `name`, `is_self` (optional convenience flag) |
| `exercises` | Global exercise catalog | `id`, `name`, `category` |
| `workout_sessions` | One training session with a client | `id`, `client_id` (FK), `trainer_id` (captured at creation, protects history), `session_date`, `notes` |
| `workout_session_exercises` | An exercise performed within a session | `id`, `session_id` (FK), `exercise_id` (FK), `order_index`, `notes` |
| `workout_sets` | An individual set within an exercise entry | `id`, `session_exercise_id` (FK), `set_number`, `reps`, `weight` (kg), `rpe` (nullable), `is_warmup` |

## 12-Week Plan

### Week 1 — Project Foundations
- **Topic**: Go project layout (`/cmd`, `/internal`), config via Viper + `config.json` (env wins over the file; missing file is OK for production; env names match nested keys via `SetEnvKeyReplacer`, e.g. `DATABASE_HOST`), `chi` router skeleton, `slog` setup, DB connectivity via `sqlx` + `pgx/v5/stdlib`.
- **Feature**: Health check endpoint (`GET /healthz`), verified DB connection on startup.
- **Verify**: `go build ./...`, `go vet ./...`, manual `curl localhost:PORT/healthz`.
- **Progress**: [`docs/progress/Week 1.md`](docs/progress/Week%201.md)

### Week 2 — Schema Design & Migrations
- **Topic**: Migration tooling (`golang-migrate` or `goose`), index planning (`trainer_id`, `client_id`, `session_date`, FK columns), primary/foreign key constraints.
- **Feature**: Full initial schema migrated (all 6 core tables), seeded exercise catalog (~20-30 common exercises).
- **Verify**: Run migrations up/down cleanly, `\d+ <table>` in `psql` to confirm indexes/constraints.
- **Progress**: [`docs/progress/Week 2.md`](docs/progress/Week%202.md)

### Week 3 — Authentication

- **Topic**: Password hashing (bcrypt or argon2), JWT issuing/verification, middleware for protected routes, propagating trainer identity via `context.Context`.
- **Feature**: `POST /v1/auth/signup`, `POST /v1/auth/login`, auth middleware protecting all trainer routes.
- **Verify**: Manual token issuance/verification tests, confirm unauthenticated requests are rejected with 401.
- **Progress**: [`docs/progress/Week 3.md`](docs/progress/Week%203.md)

### Week 4 — Core CRUD (Clients)

- **Topic**: Input validation, consistent error-handling conventions, avoiding N+1 queries, repository pattern for `trainer_id` scoping.
- **Feature**: Full client CRUD (`POST/GET/PUT/DELETE /v1/clients`), always scoped to the authenticated trainer.
- **Verify**: Confirm a trainer cannot access/mutate another trainer's clients (manual cross-tenant test).

### Week 5 — Workout Logging

- **Topic**: Database transactions (`sqlx.Tx`), nested multi-row writes, partial-failure rollback handling.
- **Feature**: `POST /v1/clients/{id}/sessions` — create a session with nested exercises and sets in one atomic transaction.
- **Verify**: Force a mid-transaction failure (e.g. invalid exercise_id in one of many sets) and confirm full rollback — no partial rows persisted.

### Week 6 — Querying & Analytics

- **Topic**: `EXPLAIN ANALYZE`, composite indexes, pagination (keyset vs offset), aggregate queries.
- **Feature**: `GET /v1/clients/{id}/sessions` (paginated history), `GET /v1/clients/{id}/exercises/{exerciseId}/progress` (max weight over time, excluding warm-up sets).
- **Verify**: `EXPLAIN ANALYZE` on the progress query before/after adding the relevant composite index; confirm plan uses an index scan, not a sequential scan, at realistic data volume.

### Week 7 — Concurrency

- **Topic**: Goroutines, channels, `context` cancellation/timeouts, worker-pool pattern, avoiding goroutine leaks and race conditions.
- **Feature**: Async CSV export of a client's full workout history (kick off a background goroutine, poll or callback for completion, respect request cancellation).
- **Verify**: `go test -race ./...`, verify export goroutine terminates cleanly on client disconnect/context cancellation (no leaked goroutines — check with `pprof` goroutine profile).

### Week 8 — Caching

- **Topic**: Redis cache-aside pattern, TTL strategy, cache invalidation on writes.
- **Feature**: Redis-backed cache for the (read-heavy, rarely-changing) exercise catalog.
- **Verify**: Confirm cache hit reduces DB query count (log/metric check); confirm cache invalidates correctly if an exercise is ever updated.

### Week 9 — Worker Pools / Queues

- **Topic**: Background job processing, retry/backoff strategy, idempotency.
- **Feature**: Async job to generate a weekly progress report per client (e.g. summary of sessions, volume, PRs), processed by a worker pool off the request path.
- **Verify**: Simulate job failure and confirm retry behavior; confirm duplicate job submissions don't produce duplicate reports (idempotency key).

### Week 10 — Observability

- **Topic**: Structured `slog` logging with request IDs, `pprof` for CPU/memory profiling, basic metrics.
- **Feature**: Request-scoped structured logging middleware, `/debug/pprof/*` endpoints, `/metrics` (basic counters/histograms).
- **Verify**: `go tool pprof` against a running instance under synthetic load; confirm request logs include a correlation/request ID traceable across a single request's log lines.

### Week 11 — Testing & Hardening

- **Topic**: Table-driven unit tests, integration tests (testcontainers for Postgres), load testing (`k6` or `hey`) to validate connection pool sizing. Revisit the RLS decision here — if any client-facing/second access path work has started, this is where RLS pays for itself.
- **Feature**: Test suite covering repositories/handlers; a load test report validating the app under concurrent load without connection pool exhaustion.
- **Verify**: `go test ./... -race -cover`, load test results (latency/error rate under target concurrency), explicit decision recorded on whether RLS is introduced now or remains deferred.

### Week 12 — Docker & Deployment

- **Topic**: Multi-stage `Dockerfile`, `docker-compose` for local dev (app + Postgres + Redis), CI (GitHub Actions: build/test/lint), graceful shutdown on `SIGTERM`, deployment to Fly.io/Render. Viper already loads JSON + env; keep fail-fast invariants and precedence env > file in the image (no secrets in the committed example file).
- **Feature**: Deployed MVP, reachable over the internet, with CI running on every push. Local stack boots from a checked-in example config file; production uses platform env/secrets only.
- **Verify**: `docker build` succeeds and produces a minimal image; local `docker-compose up` runs the full stack; deployed health check responds `200`; CI pipeline green. Confirm a value set in the file is overridden when the same key is present in the environment.

## Deferred / Future Considerations (intentionally out of MVP scope)

- Trainer-switch history (`client_trainer_assignments` table) — revisit if/when a real need arises, understanding it will require a migration + backfill.
- Postgres RLS — revisit at Week 11 or when a client-facing API is added.
- Client-facing accounts/login.
- Multi-unit weight support (`weight_unit`).
- Per-trainer custom exercises (beyond the global catalog).
- `is_self` flag on clients for trainer self-tracking UX (currently works via a normal client record with no schema change needed).
