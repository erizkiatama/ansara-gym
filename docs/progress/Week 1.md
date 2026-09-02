# Week 1 — Project Foundations

**Status**: Done (2026-08-29)

## Shipped

- `cmd/server` — thin `main` (load config, open DB, listen)
- `internal/config` — Viper JSON + env, fail-fast `validate`
- `internal/db` — `sqlx.ConnectContext` + pgx stdlib, pool caps from config
- `internal/server` — Chi + `GET /healthz` (DB ping)
- `config.example.json` committed; `config.json` gitignored

## Decisions

- **Layout**: `cmd/` is binaries only (`package main`). Shared code lives in `internal/`. No `pkg/` (this module is not a public library). HTTP handlers stay in `internal/server`, not a separate `internal/handlers` or `internal/api`.
- **Layers**: Handler may call the repository directly. Add `internal/service` only when there is a real use case (multi-table orchestration, or a second entrypoint sharing the same rules). Empty pass-through services are out of scope.
- **Boot**: Fail-fast if Postgres is unreachable at startup. Single `/healthz` (no live/ready split yet); the handler still pings the DB so a dropped connection after boot is visible (`503`).
- **Config**: Nested `App` / `Database` / `Log` structs. Discrete DB fields (not `DATABASE_URL`). Pool settings (`max_open_conns`, lifetimes) live in config, not hardcoded in `db.Open`. Missing `DATABASE_HOST` / `USER` / `NAME` and invalid values fail at startup. Secrets later via env (`DATABASE_PASSWORD`, etc.).
- **Viper**: `config.json` as the file (not `.env`). `AutomaticEnv` + `SetEnvKeyReplacer(".", "_")` so `database.host` → `DATABASE_HOST` — no `BindEnv` list. Precedence: **env > file**. `godotenv` / Cobra are not used. Cobra stays off the 12-week path unless we collapse multiple `cmd/` binaries into one.
- **Chi `RealIP`**: Removed (deprecated, IP spoofing). No `ClientIPFrom*` until something actually needs the client IP (logs, rate limits, Fly proxy).
- **Local Postgres**: Homebrew instance, database `ansara_gym`, role `erizkiatama`. The role has **no password** (`pg_authid.rolpassword` is null). `pg_hba.conf` uses **`trust`** for Unix sockets and `127.0.0.1` / `::1`, so TCP `localhost` also does not ask for a password. Empty `"password": ""` in local JSON is correct. Production must inject `DATABASE_PASSWORD` (Fly/Render will not be `trust`).
- **No docker-compose in Week 1**: Postgres was already running locally.
- **Go**: Week 1 started on `1.26.6`. Module is now `go 1.27` (see `go.mod`).
- **Deferred to later weeks**: graceful shutdown (12), request-ID logging (10), tests (11), schema/migrations (2).
