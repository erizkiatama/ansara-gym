# Week 3 — Authentication

**Status**: Done (2026-09-02)

## Shipped

- `internal/auth` — Argon2id PHC hash/compare, HS256 JWT issue/verify, unexported context key for `trainer_id`
- `internal/trainer` — `Insert` / `GetByEmail` (handler calls repo directly)
- `internal/server/auth` — `POST /v1/auth/signup`, `POST /v1/auth/login`, `GET /v1/auth/me`, `RequireTrainer` middleware
- `internal/server/respond` — JSON encode/decode (leaf package; avoids import cycle with domain handlers)
- `cmd/server` — `cfg.Auth.Validate()` (min 32-byte `AUTH_JWT_SECRET`); `cmd/migrate` does not require it

## Decisions

- **Hasher**: Argon2id (`golang.org/x/crypto/argon2`), not bcrypt. OWASP baseline in code: 19 MiB, `t=2`, `p=1`, 16-byte salt, 32-byte key. Stored as a PHC string so params travel with the hash. No pepper; no config knobs for cost. Dummy Argon2id compare on unknown email so login timing is closer to a real miss.
- **JWT**: HS256 access token only, 24h TTL. Claims: `sub` (trainer UUID), `iat`, `exp`. Refresh tokens deferred (would be stateful; Phase 1 locked stateless JWT). `Authorization: Bearer`. Forged/missing → `401` with `{"error":"unauthorized"}`.
- **Secret / TTL**: `auth.jwt_secret` / `auth.jwt_ttl` → `AUTH_JWT_SECRET` / `AUTH_JWT_TTL`. Env wins. Example file keeps `"jwt_secret": ""`. Empty or short secret → **fail-fast in `cmd/server` only**. TTL defaults to 24h if unset.
- **Routes**: URL version `/v1` for application APIs. `/healthz` stays unversioned (probes). `/v1/auth/signup` and `/v1/auth/login` public; `GET /v1/auth/me` is the protected identity probe (Bearer, loads name/email from DB) so 401 is testable before Week 4 `/v1/clients`. Chi `middleware.Logger` writes a stdlib access line per request (status included). Structured per-request `slog` stays Week 10.
- **Duplicate email**: `INSERT … ON CONFLICT (email) DO NOTHING RETURNING …`; no row → `ErrEmailTaken` → `409`. Not SQLSTATE `23505` sniffing. `$1` placeholders (Postgres-native); no `?` + `Rebind`.
- **Layout**: HTTP grouped by domain under `internal/server/auth` (`auth.go`, `types.go`, `validation.go`). `TrainerStore` interface lives on the consumer. Persistence row stays `internal/trainer.Trainer` (includes `password_hash`). JSON DTOs stay unexported in the HTTP package. Crypto stays `internal/auth` (aliased `authkit` where both would be named `auth`).
- **Out of scope**: refresh/revoke, rate limits on `/v1/auth/*`, RLS.

## Verify

```text
export AUTH_JWT_SECRET='at-least-32-bytes-of-local-secret!'
go run ./cmd/server
# without AUTH_JWT_SECRET: process exits, AUTH_JWT_SECRET must be at least 32 bytes

curl -s localhost:8080/healthz
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/v1/auth/me
# 401

curl -s -X POST localhost:8080/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"trainer@example.com","password":"correct-horse","name":"Ada"}'
# 201, token + trainer (no password_hash)

curl -s -X POST localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"trainer@example.com","password":"correct-horse"}'
# 200, token

curl -s localhost:8080/v1/auth/me -H "Authorization: Bearer $TOKEN"
# 200 {"id":"...","email":"...","name":"..."}

curl -s localhost:8080/v1/auth/me -H 'Authorization: Bearer forged.token.here'
# 401
```

`go test ./...`, `go vet ./...`, `go build ./...` clean. Hand-checked on `:8081` (existing process already bound `:8080`): missing/forged `GET /v1/auth/me` → 401; signup 201; login 200; wrong password / unknown email → 401; duplicate signup → 409. Test trainer row deleted after.
