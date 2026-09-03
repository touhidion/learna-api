# learna-api

REST API for [Learna](../learna-ui), a self-hosted training portal. Go 1.24 +
Gin + PostgreSQL 16, with Cloudinary for file storage.

See [docs/learna-architecture.md](docs/learna-architecture.md) for the system
design and [docs/learna-features.md](docs/learna-features.md) for the feature
list this implements.

---

## Quick start

### With Docker (nothing but Docker required)

```bash
cp .env.example .env
# Set JWT_SECRET — compose refuses to start without it:
#   openssl rand -base64 48
docker compose up -d --build

curl localhost:8080/health
```

### Locally (Go 1.24+ and a PostgreSQL you already run)

```bash
cp .env.example .env      # set JWT_SECRET and your DB_* values
go mod tidy               # resolves dependencies and writes go.sum
go run ./cmd/server
```

`make setup` does both of those steps; `make help` lists every task.

On first boot the server applies its migrations and, if no super admin exists,
seeds one from `SUPER_ADMIN_EMAIL` / `SUPER_ADMIN_PASSWORD`.

---

## Layout

```
cmd/server/          entry point: config -> db -> migrations -> wiring -> listener
internal/
  config/            every env var, loaded and validated in one place
  database/          pgx pool, plus migrations embedded into the binary
  middleware/        request ID, logging, recovery, CORS, JWT auth, roles, rate limit
  models/            domain entities as stored in Postgres
  dto/request/       inbound payloads (a client cannot set what is not here)
  dto/response/      outbound payloads (password hashes cannot leak)
  repository/        the only layer that writes SQL
  services/          business rules; returns *utils.APIError
  handlers/          HTTP in, service call, render out
  router/            the complete route table, and the auth boundary
  utils/             errors, responses, JWT, bcrypt, slugs, pagination, validation
pkg/cloudinary/      SDK wrapper — the rest of the code never imports the SDK
```

The dependency direction is strictly `handler -> service -> repository`. A
handler never runs SQL; a repository never decides an HTTP status code.

---

## Configuration

Every setting is an environment variable, documented in
[.env.example](.env.example). `config.Load` validates them all at once and
reports every problem in a single error, so a bad deployment is fixed in one
pass rather than one restart at a time.

Production tightens three rules automatically: `JWT_SECRET` must be at least 32
characters, `DB_PASSWORD` becomes required, and a wildcard
`CORS_ALLOWED_ORIGINS` is rejected.

---

## Migrations

SQL lives in `internal/database/migrations/` and is embedded into the binary,
so a deployed image carries its own schema history. Migrations run on boot when
`DB_AUTO_MIGRATE=true`.

```bash
make migrate-up                      # apply pending
make migrate-down n=1                # roll back one
make migrate-version                 # current version, and whether it is dirty
make migrate-new name=add_quizzes    # scaffold the next pair
```

Each command is also available directly: `go run ./cmd/server -migrate=up`.

---

## Error format

Every error, from a validation failure to a panic, comes back in one shape:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "The request contains invalid fields.",
    "fields": [
      { "field": "email", "message": "Must be a valid email address." }
    ]
  }
}
```

`code` is stable and safe to branch on; `message` is for humans. Internal
failures are logged with their cause and returned as an opaque 500.

Every response carries an `X-Request-ID` header, echoed from the request when
one was supplied. It appears on every log line for that request.

---

## Implementation status

Phase 1 foundations are in place and the full route table is registered.

**Working**

- Configuration, structured logging, graceful shutdown
- Schema migrations for every Phase 1 table
- Middleware: request ID, access logs, panic recovery, CORS, JWT auth, role
  guards, per-IP rate limiting on `/auth`
- Auth end to end — signup, login, refresh-token rotation, logout, forgot and
  reset password, change password, profile read and update (features A1–A8,
  P1–P3)
- First-run super admin seed (A8)
- Cloudinary image upload (CL1)
- `GET /health` (database connectivity) and `GET /live`

**Registered, returning `501 Not Implemented`**

Courses (C1–C6, PC1–PC2), modules (M1–M4), lessons (L1–L5), attachments
(AT1–AT4), enrollment (E1–E4), progress (PR1–PR4), certificates (CT1–CT5),
admin user management (U1–U6), analytics (AN1–AN2).

Each stub reports which module and feature IDs it covers, so the route table
stays an accurate map of the work remaining. `internal/repository/stubs.go`
lists the repositories waiting to be filled in, and `user_repo.go` is the
pattern to follow.

---

## Development

```bash
make check    # fmt + vet + test
make test     # go test -race ./...
make lint     # golangci-lint, if installed
```
