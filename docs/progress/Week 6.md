# Week 6 — Querying & Analytics

**Status**: Done (2026-09-04)

## Shipped

- `GET /v1/clients/{id}/sessions` — keyset history (headers only)
- `GET /v1/clients/{id}/sessions/{sessionId}` — one session graph (same shape as Week 5 201)
- `GET /v1/clients/{id}/exercises/{exerciseId}/progress` — max working-set kg per session, warm-ups excluded
- `internal/db/migrations/00004_week6_query_indexes.sql` — composite indexes; replaces `workout_session_exercises_exercise_id_idx` with `(exercise_id, session_id)`

## Decisions

- **Pagination**: keyset, not offset. Newest `session_date` first, then `id`. Cursor is `before_date` + `before_id` (both or neither). Default `limit=20`, max `100`. Repo fetches `limit+1`; JSON `next` is omitted on the last page. Sort is `(session_date, id)`, not uuidv7 alone (backfilled Mondays).
- **List payload**: `{id, session_date, notes}` only. Open a day with GET one session. Empty history is `"sessions": []`.
- **Progress**: one point per session `{session_id, session_date, max_weight}`. Two sessions on the same date stay two points. Never-logged / unknown `exercise_id` → `"points": []` (not 404). Warm-ups and cards with no working sets are omitted.
- **Tenancy**: missing or other-trainer client → **404**. Missing/forged token → **401**. Invalid path UUID → **400**. GET a session under the wrong `{id}` → **404**.
- **Indexes**: `(client_id, trainer_id, session_date DESC, id DESC)` on `workout_sessions` for the list. `(exercise_id, session_id)` on `workout_session_exercises` for progress (drops the old single-column `exercise_id` index; leftmost prefix still serves `exercise_id` lookups).
- **N+1**: GET one session is three queries (session, all cards, all sets `IN (...)`), assembled in Go.
- **Out of scope**: add-exercise / add-set on an existing session, edit/delete, offset pages, client login/RLS.

## Verify

```text
go run ./cmd/migrate up
# version 4: week 6 indexes

go test ./...
go test -race ./internal/session ./internal/server/session
go vet ./...

export AUTH_JWT_SECRET='at-least-32-bytes-of-local-secret!'
go run ./cmd/server

# TOKEN, CLIENT, EX from Week 5; create a few sessions first

curl -s localhost:8080/v1/clients/$CLIENT/sessions -H "Authorization: Bearer $TOKEN"
# 200 {"sessions":[...]} newest first

curl -s "localhost:8080/v1/clients/$CLIENT/sessions?limit=2" \
  -H "Authorization: Bearer $TOKEN"
# 200; if more remain, "next": {"before_date":"...","before_id":"..."}

curl -s "localhost:8080/v1/clients/$CLIENT/sessions?limit=2&before_date=$DATE&before_id=$ID" \
  -H "Authorization: Bearer $TOKEN"

curl -s localhost:8080/v1/clients/$CLIENT/sessions/$SESSION_ID \
  -H "Authorization: Bearer $TOKEN"
# 200 full graph

curl -s localhost:8080/v1/clients/$CLIENT/exercises/$EX/progress \
  -H "Authorization: Bearer $TOKEN"
# 200 {"points":[{"session_id":"...","session_date":"...","max_weight":80}]}
```

`go test ./internal/session` includes `TestProgressQueryUsesIndex`: ~5000 noise cards, `EXPLAIN` must name `workout_session_exercises_exercise_session_idx` and must not seq-scan `workout_session_exercises`.
