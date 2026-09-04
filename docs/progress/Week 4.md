# Week 4 — Core CRUD (Clients)

**Status**: Done (2026-09-03)

## Shipped

- `internal/db/migrations/00003_clients_notes.sql` — nullable `clients.notes` (do not rewrite `00001`)
- `internal/client` — `Insert` / `List` / `Get` / `Update` / `Delete`, every query `WHERE trainer_id = $1`
- `internal/server/client` — `POST/GET /v1/clients`, `GET/PUT/DELETE /v1/clients/{id}` on the same `RequireTrainer` group as `/v1/auth/me` (not under `/auth`)
- `respond.Bind` — decode JSON or write `400 {"error":"invalid json"}`. Auth signup/login use the same helper.

## Decisions

- **List**: uncapped `{ "clients": [ ... ] }`. One `SELECT … WHERE trainer_id = $1 ORDER BY id` — no session embed (Week 5), no pagination (Week 6 is session history). Empty roster is `[]`, not `null`.
- **JSON**: `{id,name,notes}` only. `created_at` / `updated_at` stay on the DB row; PUT still sets `updated_at = now()`.
- **Update verb**: `PUT`, not `PATCH`. The UI is one edit form that loads `GET` and saves `name` + `notes` together. Body matches create: `{"name":"...","notes":"..."}`. Empty `name` after trim → 400 `name is required`. Empty `notes` after trim → `NULL`. Omitting `"notes"` unmarshals to `""` (clears notes).
- **Notes**: free-text trainer memo (e.g. "ACL on left knee"), not a medical-record model. Same nullability as `workout_sessions.notes`. Distinct from per-session notes.
- **Handler flow**: Bind DTO → `strings.TrimSpace` onto a `store` struct → `validateX` → repo takes the struct. Consumer interface is `Repository`. Persistence package imported as `store` when the HTTP package name would collide.
- **Tenancy**: `trainer_id` from `internal/auth` context, passed into every repo call. Cross-tenant get/update/delete → `404 {"error":"not found"}`. Missing/forged token → `401`. Malformed path id → `400 {"error":"invalid id"}`.
- **DELETE**: `204` empty body. `ON DELETE CASCADE` from `clients` → sessions is already in `00001` — hard-delete, no soft-delete.
- **Out of scope**: pagination, nested sessions, client login/RLS, body-weight, `is_self`.

## Verify

```text
go run ./cmd/migrate up
# version 3: clients.notes

export AUTH_JWT_SECRET='at-least-32-bytes-of-local-secret!'
go run ./cmd/server

TOKEN_A=$(curl -s -X POST localhost:8080/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"correct-horse","name":"A"}' | jq -r .token)
TOKEN_B=$(curl -s -X POST localhost:8080/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"b@example.com","password":"correct-horse","name":"B"}' | jq -r .token)

curl -s localhost:8080/v1/clients
# 401

CLIENT_A=$(curl -s -X POST localhost:8080/v1/clients \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"name":"Rina","notes":"ACL on left knee"}' | jq -r .id)
CLIENT_B=$(curl -s -X POST localhost:8080/v1/clients \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' \
  -d '{"name":"Budi","notes":"wrist is weaker than other people"}' | jq -r .id)

curl -s localhost:8080/v1/clients -H "Authorization: Bearer $TOKEN_A"
# 200, only Rina

curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/v1/clients/$CLIENT_B \
  -H "Authorization: Bearer $TOKEN_A"
# 404
curl -s -o /dev/null -w '%{http_code}\n' -X PUT localhost:8080/v1/clients/$CLIENT_B \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"name":"Nope","notes":""}'
# 404
curl -s -o /dev/null -w '%{http_code}\n' -X DELETE localhost:8080/v1/clients/$CLIENT_B \
  -H "Authorization: Bearer $TOKEN_A"
# 404
```

`go test ./...`, `go vet ./...`, `go build ./...` clean. Handler tests cover empty list envelope, trainer-scoped list, cross-tenant 404 on get/put/delete, PUT clear-notes, unauthenticated 401.
