# Running Learna locally

> Deploying instead? See **[DEPLOYMENT.md](DEPLOYMENT.md)** for Render + Netlify.

Two repos, one Neon database.

| | dev | prod |
|---|---|---|
| API | `8081` | `8082` |
| UI | `3100` | `3200` |

Which env file loads is decided by `APP_ENV` — `development` reads
`.env.development`, `production` reads `.env.production`. Both already hold the
Neon `DATABASE_URL`.

> `make` is not installed on this machine, so the raw commands come first. If
> you install it (`winget install ezwinports.make`), every step below has a
> shorter `make` equivalent shown beside it.

---

## First time

```bash
cd e:/go/learna-api && go mod tidy      # writes go.sum
cd e:/go/learna-ui  && npm install
```

Both `.env.development` and `.env.production` already exist and are gitignored.
Nothing to copy.

---

## 1. Sync the database to Neon

Migrations are embedded in the binary, so this needs no separate tool.

```bash
cd e:/go/learna-api

# apply everything pending                     (make db-sync)
APP_ENV=development go run ./cmd/server -migrate=up

# check where the schema stands                (make db-status)
APP_ENV=development go run ./cmd/server -migrate=version
```

Expected: `version=1 dirty=false`.

**`-migrate=up` also seeds.** Schema and baseline data move together, because a
freshly synced database is unusable without an account to sign in as. The
seeder lives in [internal/seed/seed.go](../internal/seed/seed.go) and currently
creates one row: the super admin, from `SUPER_ADMIN_EMAIL` /
`SUPER_ADMIN_PASSWORD` in your env file.

It is idempotent — safe to run on every sync forever:

| Database state | What the seed does |
|---|---|
| No super admin | Creates it, logs `action=created` |
| A super admin exists | Leaves it alone, logs `action=skipped` |
| `SUPER_ADMIN_EMAIL`/`_PASSWORD` unset | Does nothing, logs `action=disabled` |

Skips are logged at `debug`, so add `LOG_LEVEL=debug` to see them.

### Resetting the super admin password

Because the guard is "does *any* super admin exist", editing
`SUPER_ADMIN_PASSWORD` and re-syncing normally changes nothing. To force the
stored password to match your env file, opt in for one run:

```bash
SUPER_ADMIN_RESET_PASSWORD=true APP_ENV=development \
  go run ./cmd/server -migrate=up
```

That updates the password **and revokes every existing session** for the
account. It is off by default (`SUPER_ADMIN_RESET_PASSWORD=false`) so a deploy
cannot silently undo a password rotated through `PATCH /api/v1/me/password`.

`dirty=true` means a migration failed halfway. Fix the schema by hand, then
clear the flag with `-migrate=force -n=<version>`. Nothing else will run until
you do.

In **development** the server also migrates automatically on boot
(`DB_AUTO_MIGRATE=true`), so step 1 is optional day to day. Production has it
off deliberately — there, run the command above with `APP_ENV=production`
(`make db-sync-prod`) as an explicit deploy step.

---

## 2. Run

Two terminals.

**API** — `http://localhost:8081`

```bash
cd e:/go/learna-api
APP_ENV=development go run ./cmd/server      # make dev
```

**UI** — `http://localhost:3100`

```bash
cd e:/go/learna-ui
npm run dev
```

The port is pinned in the npm script, so no flag is needed.

---

## 3. Check it works

```bash
curl localhost:8081/health
# {"status":"ok","env":"development","services":{"database":"ok"}}

curl -X POST localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@learna.local","password":"ChangeMe123!"}'
```

Then open <http://localhost:3100> and sign in with the same credentials.

The super admin is **already seeded in Neon**. Editing `SUPER_ADMIN_PASSWORD`
alone changes nothing, because the seed skips once a super admin exists — use
`PATCH /api/v1/me/password`, or the `SUPER_ADMIN_RESET_PASSWORD` route above.

---

## Production mode locally

```bash
cd e:/go/learna-api
APP_ENV=production go run ./cmd/server        # make prod  -> :8082
```

```bash
cd e:/go/learna-ui
npm run build && npm start                    # -> :3200
```

`NEXT_PUBLIC_*` is baked into the bundle at build time, so the UI must be
**rebuilt** — not just restarted — after changing an API URL.

---

## Docker

The database is Neon, so no database container starts by default.

```bash
cd e:/go/learna-api
docker compose up -d --build     # API on :8081, reads .env.development
docker compose logs -f api
```

Need an offline database instead? It is opt-in:

```bash
docker compose --profile local-db up -d
# then point DATABASE_URL at it:
#   postgresql://learna:learna@postgres:5432/learna?sslmode=disable
```

---

## Troubleshooting

**`failed SASL auth` / `password authentication failed`** — a local Postgres is
answering instead of Neon. `DATABASE_URL` is unset or empty, so the config fell
back to `DB_HOST=localhost`. Confirm the startup log names the right server:

```
msg="database connected" target=ep-quiet-waterfall-...neon.tech/neondb
```

**CORS error in the browser** — the UI origin must match the API's allowlist
exactly. Dev pairs `3100` with `8081`; prod pairs `3200` with `8082`. Mixing
them (UI on 3100 against the API on 8082) is rejected by design.

**UI still calling the old port** — a stale `.env.local` overrides both
`.env.development` and `.env.production`. There should not be one; delete it.

**`bind: Only one usage of each socket address...`** — a previous run is still
holding the port.

The process is **not** called `learna-api`. `go run` compiles to a temp binary
named `server.exe`, and the UI runs as `node.exe`, so searching the task list
for the project name finds nothing. Go by port instead.

PowerShell:

```powershell
# what is holding it
Get-NetTCPConnection -State Listen -LocalPort 8081 |
  Select-Object LocalPort, OwningProcess

# stop it
Stop-Process -Id <pid> -Force

# or clear every Learna port at once
Get-NetTCPConnection -State Listen -LocalPort 8081,8082,3100,3200 -EA SilentlyContinue |
  ForEach-Object { Stop-Process -Id $_.OwningProcess -Force }
```

Git Bash:

```bash
netstat -ano | grep LISTENING | grep -E ':(8081|8082|3100|3200)\s'
taskkill //F //PID <pid>
```

Ctrl+C in the terminal that started it is the clean way; the above is for a run
that was backgrounded or whose terminal is gone.

**`501 Not Implemented`** — expected. Only auth, profile, health and image
upload are built; every other module is registered but not yet implemented, and
says so in the error message.
