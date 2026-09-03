# Deploying Learna

Production runs on three managed services, none of which you administer:

```
  Browser
     │
     ▼
  Netlify ───── learna-ui (Next.js 16)
     │            build-time: NEXT_PUBLIC_API_URL
     │ HTTPS / CORS
     ▼
  Render ────── learna-api (Go, Docker)
     │
     ▼
  Neon ──────── PostgreSQL 16
```

The two services are deployed **independently** from separate repos. They are
coupled by exactly two settings, and almost every deployment problem is one of
them being wrong:

| Setting | Lives on | Must equal |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | Netlify | the Render service URL |
| `CORS_ALLOWED_ORIGINS` | Render | the Netlify site origin |

For local development see [RUN.md](RUN.md).

---

## Before you start

- A **Neon** project. Reuse the development branch or, preferably, create a
  separate one — see [Use a separate Neon branch](#use-a-separate-neon-branch).
- Both repos pushed to GitHub.
- Two generated secrets, neither reused from development:

  ```bash
  openssl rand -base64 48   # JWT_SECRET
  openssl rand -base64 24   # SUPER_ADMIN_PASSWORD
  ```

---

## 1. API on Render

[`render.yaml`](../render.yaml) is a blueprint: Render reads it and creates the
service, so the only manual work is supplying secrets.

**Dashboard → New → Blueprint → select the `learna-api` repo.**

Render prompts for every variable marked `sync: false`:

| Variable | Value |
|---|---|
| `DATABASE_URL` | the Neon connection string, including `?sslmode=require&channel_binding=require` |
| `JWT_SECRET` | the 48-byte secret you generated |
| `CORS_ALLOWED_ORIGINS` | your Netlify origin, e.g. `https://learna.netlify.app` |
| `SUPER_ADMIN_EMAIL` | your admin address |
| `SUPER_ADMIN_PASSWORD` | the generated password |
| `CLOUDINARY_URL` | `cloudinary://key:secret@cloud` — leave empty to disable uploads |

You will not have the Netlify origin yet on a first deploy. Put a placeholder
in, deploy Netlify (step 2), then come back and correct it — changing it is a
restart, not a rebuild.

Everything else — ports, pool sizes, TTLs, rate limits — comes from the
blueprint.

### What the blueprint handles for you

**`PORT`.** Render injects it and routes to exactly that port. `config.Load`
reads `PORT` ahead of `SERVER_PORT` for this reason, and the blueprint
deliberately does not set either. Binding a port of our own choosing would make
the service unreachable and the deploy would fail its health check.

**`TRUSTED_PROXIES=*`.** Render terminates TLS at its edge and forwards over an
internal network, so the proxy is never loopback. Without this the app refuses
to believe `X-Forwarded-For`, every request reports the balancer's IP, and the
per-IP rate limiter throttles **all users as a single client** — 5 requests per
second across the entire site. The wildcard is safe here because the port is
only reachable through Render's edge, which overwrites the header.

**`healthCheckPath: /health`.** This reports database connectivity, so a
release that cannot reach Neon is rejected instead of going live broken.
`/live` deliberately ignores the database and is the wrong probe for a deploy
gate — it is for liveness restarts.

**Migrations.** `DB_AUTO_MIGRATE=true`, so they run on boot. Free instances
have no pre-deploy hook, and migrations are idempotent — a no-op once applied.
On a paid instance, prefer an explicit gate:

```yaml
plan: starter
preDeployCommand: /app/learna-api -migrate=up
envVars:
  - key: DB_AUTO_MIGRATE
    value: "false"
```

That runs migrations **and the seed** once per release rather than in every
booting replica, which matters as soon as you run more than one instance.

### The free plan sleeps

A free Render service spins down after ~15 minutes idle. The next request pays
a cold start of **roughly 30–60 seconds**, during which the UI shows its
loading state and a slow login looks like a hang. It is not a bug and there is
no code fix — `starter` at $7/month removes it.

---

## 2. UI on Netlify

`netlify.toml` in the `learna-ui` repo carries the whole configuration; that
repo also has its own `docs/DEPLOYMENT.md` with the UI-specific detail.

**Netlify → Add new site → Import an existing project → select the `learna-ui`
repo.** Build command and publish directory come from the file; accept them.

Then set the real URLs. Edit `netlify.toml` and replace the two placeholders
under `[context.production.environment]`:

```toml
NEXT_PUBLIC_API_URL  = "https://YOUR-SERVICE.onrender.com"
NEXT_PUBLIC_SITE_URL = "https://YOUR-SITE.netlify.app"
```

Commit and push — that triggers a rebuild.

> **`NEXT_PUBLIC_*` is compiled into the JavaScript bundle at build time.** It
> is not read at runtime. Changing the API URL requires a **redeploy**, and
> setting it in the Netlify UI without triggering a rebuild changes nothing.
> This is the single most common Netlify mistake with Next.js.

### What the config handles for you

**`output: "standalone"` is disabled on Netlify.** Netlify's Next runtime
packages the app itself and does not support standalone output, while the
Docker image requires it. `next.config.ts` switches on the `NETLIFY`
environment variable so both targets build from one config.

**`NPM_FLAGS = "--include=dev"`.** Netlify sets `NODE_ENV=production`, which
would otherwise skip devDependencies — and the build needs TypeScript, ESLint
and Tailwind.

**`NODE_VERSION = "22"`.** Next 16 requires Node 20 or newer.

---

## 3. Wire the two together

Once both are live:

1. Copy the Render URL → `NEXT_PUBLIC_API_URL` in `netlify.toml`, push.
2. Copy the Netlify origin → `CORS_ALLOWED_ORIGINS` on Render, save.

The origin must be **scheme + host with no trailing slash and no path**.
`https://learna.netlify.app/` and `learna.netlify.app` are both wrong, and both
fail as a browser CORS error rather than a server error.

`config.Load` rejects a wildcard origin when `APP_ENV=production`, so `*` is
not an escape hatch here.

### Verify

```bash
curl https://YOUR-SERVICE.onrender.com/health
# {"status":"ok","env":"production","services":{"database":"ok"}}

# CORS preflight must echo your origin back
curl -i -X OPTIONS https://YOUR-SERVICE.onrender.com/api/v1/auth/login \
  -H "Origin: https://YOUR-SITE.netlify.app" \
  -H "Access-Control-Request-Method: POST" | grep -i access-control-allow-origin
```

Then open the Netlify site and sign in with `SUPER_ADMIN_EMAIL` /
`SUPER_ADMIN_PASSWORD`. That exercises the whole chain: browser → Netlify →
CORS → Render → Neon.

---

## Deploy previews

Every Netlify preview gets a unique hostname, and `CORS_ALLOWED_ORIGINS` lists
exact origins — so **API calls from a preview are blocked by default**. Previews
are useful for reviewing layout, not for exercising authenticated flows.

To enable one, add its origin to the Render variable. Do not add a wildcard.

---

## Use a separate Neon branch

Development and production currently share one Neon branch. A local
`-migrate=down` would destroy production data.

In the Neon console, branch from `main`, copy the new connection string, and
put it in `DATABASE_URL` on Render. Then sync the schema and seed the admin:

```bash
DATABASE_URL="<the new branch URL>" APP_ENV=production \
  go run ./cmd/server -migrate=up
```

Branches are copy-on-write, so this is close to free.

---

## Rotating secrets

**`JWT_SECRET`** — change it on Render and restart. Every access token is
immediately invalid and every user is signed out. Refresh tokens live in the
database and survive, so clients recover on their next refresh.

**Super admin password** — use `PATCH /api/v1/me/password`. Editing
`SUPER_ADMIN_PASSWORD` does nothing on its own: the seed skips once a super
admin exists. To force it, set `SUPER_ADMIN_RESET_PASSWORD=true`, deploy once,
then set it back to `false` — leaving it true would reset the password on every
subsequent deploy.

**`DATABASE_URL`** — rotate in the Neon console, update Render, restart.

---

## Troubleshooting

**Deploy fails on the health check.** `/health` returned non-2xx, almost always
because `DATABASE_URL` is wrong or the Neon project is suspended. Check the
Render logs for `connect to database:`.

**Service starts then immediately exits.** `config.Load` reports every problem
at once and exits non-zero. Read the `invalid configuration:` block in the
logs — it names each offending variable. In production it additionally requires
`JWT_SECRET` of 32+ characters and rejects a wildcard CORS origin.

**CORS error in the browser, API healthy.** `CORS_ALLOWED_ORIGINS` does not
match the site origin exactly. Compare them character for character, including
scheme, and check for a trailing slash.

**UI calls the wrong API host.** The bundle was built with a stale
`NEXT_PUBLIC_API_URL`. Trigger a fresh Netlify deploy; a restart will not do
it. Confirm with browser DevTools → Network which host is actually called.

**First request after idle takes ~40 s.** Free-plan cold start. Expected.

**Rate limiting throttles everyone at once.** `TRUSTED_PROXIES` is not set to
`*` on Render, so every request is attributed to the load balancer.

**`501 Not Implemented`.** Expected. Only auth, profile, health and image
upload are built; every other module is registered but not yet implemented and
says so in its error message.
