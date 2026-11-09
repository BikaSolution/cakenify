# Cakenify

A small, self-hosted birthday reminder app.

Admins create user accounts; each user keeps their own list of birthdays and
chooses how they want to be notified. Every day at 10:00 Cakenify sends each
user a reminder for that day's birthdays, and on Sundays it also sends a
look-ahead digest for the coming week. Notifications go out over
[ntfy](https://ntfy.sh), behind a pluggable `Notifier` interface so other
channels can be added later.

## Features

- Username/password login with DB-backed sessions (no self-registration — an admin creates accounts)
- Per-user birthday list: name, day/month, optional birth year (for "turns N"), notes
- Daily 10:00 reminder and Sunday weekly digest, in a configurable timezone
- Restart-safe scheduler: each job records its last run date, so a restart around 10:00 never double-sends
- Per-user notification settings, with a "send test notification" button
- Server-rendered UI with htmx — no JavaScript build step, no CDN

## Stack

| Concern       | Choice |
|---------------|--------|
| Backend       | Go, stdlib `net/http` (1.22+ routing) and `html/template` |
| Frontend      | htmx + a small hand-written stylesheet, vendored and embedded in the binary |
| Database      | PostgreSQL 16 via `jackc/pgx/v5`; schema is embedded and migrated at startup |
| Passwords     | `golang.org/x/crypto/bcrypt` |
| Validation    | `go-playground/validator/v10` |
| Scheduling    | Small in-repo ticker-based `cronjob` package |
| Notifications | `Notifier` port + strategy registry; `ntfy` is the first implementation |
| Testing       | `testify` + `go.uber.org/mock` (gomock) |
| Deploy        | Docker Compose: app + Postgres + Cloudflare Tunnel (`cloudflared`) |

## Project layout

```
cmd/cakenify/            wiring only: config, DB pool + migration, notifier registry, scheduler, HTTP server
internal/cakenify/
  api/                   routes, middleware (auth, CSRF, rate limiting, security headers), handlers
  core/                  business logic (auth, birthdays, digest building) and the scheduler worker
  port.go types.go errors.go notify.go ntfynotifier.go
pkg/
  db/cakenifydb/         pgx-backed persistence, embedded migration
  cronjob/               ticker-based job runner
  ntfy/                  raw ntfy HTTP client
web/                     templates + static assets (embedded via web/embed.go)
```

Ports (`port.go`) define interfaces; `mock/` directories hold generated gomock
mocks (`go generate ./...`).

## Running it

### With Docker Compose

```sh
cp .env.example .env        # then fill in real values — .env is git-ignored
docker compose up -d --build
```

The app listens on `127.0.0.1:8080` on the host. Public traffic is expected to
arrive through the `cloudflared` service; point your Cloudflare Tunnel's
public hostname at `http://app:8080`.

On first boot with an empty `users` table, an admin account is created from
`CAKENIFY_ADMIN_USERNAME` / `CAKENIFY_ADMIN_PASSWORD`. Once any user exists
these are ignored, so you can clear them from `.env` afterwards.

> **Build note:** Postgres data is bind-mounted at `./pgdata`, which is owned
> by the container's postgres user and unreadable to you. Docker's legacy
> builder fails on unreadable directories in the build context even when
> they're in `.dockerignore`, so build with BuildKit (install the
> `docker-buildx` plugin).

### Locally

```sh
go test ./...
CAKENIFY_DATABASE_URL='postgres://user:pass@localhost:5432/cakenify?sslmode=disable' \
CAKENIFY_SECURE_COOKIES=false \
go run ./cmd/cakenify
```

## Configuration

All configuration is via environment variables (see [`.env.example`](.env.example)).

| Variable | Default | Purpose |
|----------|---------|---------|
| `CAKENIFY_PORT` | `8080` | HTTP listen port |
| `CAKENIFY_DATABASE_URL` | — | Full Postgres DSN; overrides the `CAKENIFY_DB_*` vars below |
| `CAKENIFY_DB_HOST` / `_PORT` / `_USER` / `_PASSWORD` / `_NAME` / `_SSLMODE` | `localhost` / `5432` / `cakenify` / `cakenify` / `cakenify` / `disable` | Postgres connection parts (password is URL-encoded safely). Docker Compose refuses to start without an explicit `CAKENIFY_DB_PASSWORD`. |
| `CAKENIFY_NTFY_BASE_URL` | `https://ntfy.bikasolutions.co.za` | ntfy server notifications are published to |
| `CAKENIFY_TIMEZONE` | `Africa/Johannesburg` | IANA timezone the 10:00 jobs run in (tzdata is embedded) |
| `CAKENIFY_SESSION_TTL` | `720h` | Login session lifetime |
| `CAKENIFY_SECURE_COOKIES` | `true` | Set `false` only for local plain-HTTP testing |
| `CAKENIFY_ADMIN_USERNAME` / `CAKENIFY_ADMIN_PASSWORD` | — | First-boot admin seed |
| `CLOUDFLARE_TUNNEL_TOKEN` | — | Token for the `cloudflared` service |

Each user configures their own ntfy topic and access token on the
**Notification settings** page. Saved tokens are never sent back to the browser.

## Routes

| Access | Routes |
|--------|--------|
| Public | `GET /login`, `POST /login` |
| Any signed-in user | `GET /` (dashboard), `GET/POST /birthdays`, `GET /birthdays/{id}/edit`, `PUT/DELETE /birthdays/{id}`, `GET/POST /settings/notifications`, `POST /settings/notifications/test`, `POST /logout` |
| Admin | `GET/POST /admin/users` |

## Notifications

```go
type Notifier interface {
    Strategy() string
    Send(ctx context.Context, cfg json.RawMessage, msg Message) error
}
```

At startup a registry of `Notifier`s is built, keyed by strategy name. The
scheduler loads each user's enabled notification settings, resolves the
matching `Notifier`, and calls `Send` with that setting's strategy-specific
JSON config. Adding a channel (email, Pushover, Slack, …) means adding one
adapter; the core logic doesn't change.

Message formats:

```
Title: Birthday today
Body:  David Agar, Jane Doe
```

```
Title: Birthdays this week
Body:
Tuesday: David Agar.
Friday: Jane Doe, Sam Peters.
```

Birthdays match on month and day; 29 February birthdays fire on 28 February
in non-leap years.

## Security

- bcrypt password hashing; login always runs bcrypt (against a dummy hash for unknown users) to avoid username enumeration by timing
- Login rate limiting per IP and per username, plus per-user rate limiting on test notifications (`CF-Connecting-IP` aware)
- Session-bound HMAC CSRF tokens on every state-changing request
- Strict security headers: CSP without `unsafe-inline`/`unsafe-eval`, `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy`, `Permissions-Policy`, HSTS when cookies are secure, `no-store` on pages
- Request bodies capped at 1 MiB; full `http.Server` timeouts
- Container: digest-pinned distroless base, runs as non-root (uid 65532), read-only root filesystem, all capabilities dropped, `no-new-privileges`, host port bound to loopback only

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
