# RiraLicense — Server (Go + chi)

Backend API for RiraLicense. Modular monolith architecture (`handler` → `auth` / `service` → `db` / `sqlc`).

## Run Locally

```sh
cd server
cp .env.example .env   # fill in DATABASE_URL, APP_URL, SESSION_SECRET, TOTP_ENCRYPTION_KEY
set -a && source .env && set +a
go run ./cmd/api
```

Server listens on **http://localhost:7819** by default. Migrations in `server/migrations/` run automatically on startup.

## Environment Variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `APP_PORT` | No | `7819` | HTTP server port |
| `APP_ENV` | No | `development` | `development` or `production` |
| `APP_URL` | **Yes** | — | Public base URL of the SaaS app (e.g. `http://localhost:5173`). Used to build email links and OAuth callback URLs. |
| `DATABASE_URL` | **Yes** | — | PostgreSQL connection string (`postgres://...`) |
| `DB_MAX_CONNS` | No | `20` | pgxpool max connections |
| `DB_MIN_CONNS` | No | `2` | pgxpool min connections |
| `DB_MAX_CONN_LIFETIME` | No | `1h` | pgxpool max connection lifetime |
| `DB_QUERY_TIMEOUT` | No | `5s` | Database startup/query timeout |
| `HTTP_TIMEOUT` | No | `15s` | Per-request context timeout |
| `SESSION_SECRET` | **Yes** | — | At least 32 bytes (raw or 64+ hex chars) used to HMAC-sign OAuth state and 2FA challenge cookies |
| `TOTP_ENCRYPTION_KEY` | **Yes** | — | Exact 32-byte AES-256-GCM key (64 hex chars or base64) for encrypting TOTP secrets at rest |
| `COOKIE_SECURE` | No | auto | Defaults to `true` when `APP_URL` uses `https://` or `APP_ENV=production` |
| `CORS_ALLOWED_ORIGINS` | No | `APP_URL` | Comma-separated allowed CORS origins |
| `SMTP_HOST` | No | — | SMTP host. When unset in `development`, email links/OTPs are logged to server stdout marked `DEV-ONLY`. |
| `SMTP_PORT` | No | `587` | SMTP port |
| `SMTP_USER` | No | — | SMTP username (optional for local relays) |
| `SMTP_PASS` | No | — | SMTP password |
| `SMTP_FROM` | No | `no-reply@riralicense.local` | Sender address |
| `SMTP_TLS` | No | `starttls` | `none`, `starttls`, or `tls` |
| `GITHUB_CLIENT_ID` | No | — | Optional GitHub OAuth client ID (disabled if unset) |
| `GITHUB_CLIENT_SECRET` | No | — | Optional GitHub OAuth client secret |
| `GOOGLE_CLIENT_ID` | No | — | Optional Google OAuth client ID (disabled if unset) |
| `GOOGLE_CLIENT_SECRET` | No | — | Optional Google OAuth client secret |

Generate keys safely:

```sh
openssl rand -hex 32   # use separate outputs for SESSION_SECRET and TOTP_ENCRYPTION_KEY
```

## OAuth App Registration

OAuth providers are optional. When credentials are missing, that provider's UI button is hidden and `/api/v1/auth/oauth/<provider>` returns `404`.

- **GitHub:** Open **Settings > Developer settings > OAuth Apps > New OAuth App**.
  - Set **Authorization callback URL** to: `<APP_URL>/api/v1/auth/oauth/github/callback`
- **Google:** Open **Google Cloud Console > APIs & Services > Credentials > Create Credentials > OAuth client ID** (Web application).
  - Set **Authorized redirect URI** to: `<APP_URL>/api/v1/auth/oauth/google/callback`

## Auth Endpoints (`/api/v1/auth`)

- `POST /register`, `GET|POST /verify-email`, `POST /resend-verification`
- `POST /login`, `POST /logout`, `POST /logout-all`, `GET /me`
- `POST /forgot-password`, `POST /reset-password`
- `POST /email/change`, `GET|POST /email/confirm`, `POST /password/change`
- `POST /2fa/totp/setup`, `POST /2fa/totp/verify`, `POST /2fa/totp/disable`
- `POST /2fa/email/send`, `POST /2fa/email/verify`, `POST /2fa/backup-codes/regenerate`
- `GET /providers`, `GET /oauth/{provider}`, `GET /oauth/{provider}/callback`

## Code Generation (`sqlc`)

```sh
cd server
sqlc generate -f sqlc/sqlc.yaml
```

## License

AGPL-3.0 — Copyright © 2026 RiraLab — Rizaru Desu.
See [LICENSE](../LICENSE) and [NOTICE](../NOTICE).
