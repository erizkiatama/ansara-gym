# Week 5 — Workout Logging

**Status**: Done (2026-09-04)

## Shipped

- `internal/session` — `Insert(ctx, trainerID, clientID, graph)` owns `sqlx.Tx` (Begin / Commit / `defer Rollback`). Writes `workout_sessions`, `workout_session_exercises`, and `workout_sets` in one transaction. No exercise/set repos; no `internal/service`.
- `internal/server/session` — `POST /v1/clients/{id}/sessions` on the same `RequireTrainer` group as `/v1/clients`. Handler flow matches clients: `respond.Bind` → trim onto a `store` graph → `validateSession` → repo takes the struct.
- `internal/utils` — shared `ValidID` (8-4-4-4-12 hex UUID). `isHex` stays unexported. Client and session handlers both call it.

## Decisions

- **Tx owner**: session repo `Insert`, not the handler. Unexported helpers on the same `Repo` take `*sqlx.Tx`. Sunday’s create and Monday’s “add a set” are different HTTP requests; a Tx cannot span the week.
- **Body**: nested `{session_date, notes, exercises:[{exercise_id, order_index, notes, sets:[...]}]}`. Matches the tables. `session_date` is `YYYY-MM-DD`. `trainer_id` from JWT, not the JSON. `client_id` from the URL.
- **Empty graph**: allowed. Empty `exercises` = calendar slot (Mon/Wed/Fri). Exercise with empty/omitted `sets` = planned movement. Exercise with sets = first set logged immediately. Mixed in one POST is fine.
- **201**: full graph with ids at all three levels. No `created_at` / `updated_at`, no `trainer_id` / `client_id` on the wire. Empty slices are `[]`, not `null`. Notes `omitempty`.
- **Unknown `exercise_id`**: **400** `{"error":"unknown exercise_id"}`. Catalog is global, not tenant-scoped. Checked with `SELECT id FROM exercises WHERE id IN ($1, …)` inside the Tx (no SQLSTATE sniffing). Malformed UUID is **400** `invalid exercise_id`.
- **Tenancy**: `INSERT … SELECT FROM clients WHERE id = $1 AND trainer_id = $2`. No row → `ErrNotFound` → **404** `not found` (cross-tenant client id looks missing). Missing/forged token → **401**.
- **Validation**: duplicate `order_index` / `set_number` → 400 in Go (DB unique is the backstop). `order_index >= 0`, `set_number > 0`, `reps >= 0`, `weight` 0–9999.99, `rpe` omitted/null or 1–10.
- **Out of scope**: GET session list/history (Week 6), nested GET, add-exercise / add-set on an existing session, edit/delete, pagination, client login/RLS, body-weight, custom exercises.

## Verify

```text
go test ./...
go test -race ./...
go vet ./...
go build ./...

export AUTH_JWT_SECRET='at-least-32-bytes-of-local-secret!'
go run ./cmd/server

TOKEN=$(curl -s -X POST localhost:8080/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"week5@example.com","password":"correct-horse","name":"Ada"}' | jq -r .token)
CLIENT=$(curl -s -X POST localhost:8080/v1/clients \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Rina","notes":"ACL on left knee"}' | jq -r .id)
EX=$(psql -d ansara_gym -tA -c "SELECT id FROM exercises WHERE name = 'Bench Press';")

curl -s localhost:8080/v1/clients/$CLIENT/sessions
# 401 without a token — route is POST-only; unauthenticated POST:
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/v1/clients/$CLIENT/sessions \
  -H 'Content-Type: application/json' \
  -d '{"session_date":"2026-09-07"}'
# 401

# Calendar slot
curl -s -X POST localhost:8080/v1/clients/$CLIENT/sessions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"session_date":"2026-09-07"}'
# 201, exercises: []

# Planned movement (no sets)
curl -s -X POST localhost:8080/v1/clients/$CLIENT/sessions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"session_date":"2026-09-07","exercises":[{"exercise_id":"'"$EX"'","order_index":0}]}'
# 201, sets: []

# First set with the card
curl -s -X POST localhost:8080/v1/clients/$CLIENT/sessions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"session_date":"2026-09-07","exercises":[{"exercise_id":"'"$EX"'","order_index":0,"sets":[{"set_number":1,"reps":8,"weight":80,"rpe":7,"is_warmup":false}]}]}'
# 201, set id present

# Unknown exercise_id: 400, no session row
curl -s -X POST localhost:8080/v1/clients/$CLIENT/sessions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"session_date":"2026-09-07","exercises":[{"exercise_id":"00000000-0000-0000-0000-000000000001","order_index":0}]}'
# 400 {"error":"unknown exercise_id"}
```

`go test ./internal/session` includes `TestInsertUnknownExerciseRollsBack` against local Postgres: Bench Press + a fake uuid → `ErrUnknownExercise`, session count unchanged, no orphan exercise rows.

Handler tests cover unauthenticated 401, cross-tenant client 404, empty session, exercises without sets, mixed graph, unknown exercise 400.
