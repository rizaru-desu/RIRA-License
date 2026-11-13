# RiraLicense — Community Edition v1.0 Roadmap

> **Target Release:** RiraLicense Community Edition v1.0 (`GNU AGPL-3.0`)
> **Production Domain:** https://license.rira.web.id
> **Architecture:** Go + `chi` v5 API Server (`server/`), PostgreSQL 17, Svelte 5 Control Plane (`saas/`), Svelte 5 + Vite Landing Page (`landingpage/`).
> **Commit Rule:** Every task below represents **one atomic commit** following Conventional Commits.

---

## Current Repository State Summary

- **`server/` (Go 1.26 + `chi` v5):** Contains HTTP server bootstrap with graceful `SIGINT`/`SIGTERM` shutdown (`server/cmd/api/main.go`), `APP_PORT`/`APP_ENV` config loader (`server/internal/config/config.go`), `chi` router with standard middleware (`server/internal/router/router.go`), and health probes `GET /`, `GET /healthz`, and `GET /readyz` (`server/internal/handler/health.go`, `server/internal/service/health.go`). No database layer, migrations, domain models, or licensing endpoints exist yet.
- **`landingpage/` (Svelte 5 + Vite + Tailwind CSS 4):** Complete Neo-Brutalist marketing & documentation site with 9 sections (`Hero`, `PlatformSupport`, `Features`, `HowItWorks`, `DeveloperExperience`, `DeploymentOptions`, `FAQ`, `Community`, `FinalCTA`), containerization (`landingpage/Dockerfile`, `landingpage/nginx.conf`), edge routing (`docker-compose.yml`, `Caddyfile`), and Tailscale SSH CI/CD workflow (`.github/workflows/deploy.yml`) ready in the working tree.
- **`saas/` (Control Plane UI):** Initial Vite template from `780a6e5` was superseded when moving the landing page into `landingpage/`; the Svelte 5 control plane application in `saas/` has not been built yet.

---

## M0 — Landing Page Live (`https://license.rira.web.id`)

### Completed (In Repository / Working Tree)
- [x] **Project Legal & Governance Baseline** — AGPL-3.0 license, copyright/trademark notice, code of conduct, and root README are committed (`LICENSE`, `NOTICE`, `CODE_OF_CONDUCT.md`, `README.md` in `780a6e5`, `a515aff`, `3effcda`).
  `Commit: docs: translate README to English, add Code of Conduct (a515aff)`
- [x] **Landing Page Application (`landingpage/`)** — Svelte 5 + Tailwind CSS 4 Neo-Brutalist landing page with interactive REST/SDK/Health tabs, 7-step deployment guide, and Community v1.0 roadmap section (`landingpage/src/`).
  `Commit: feat(landingpage): move marketing site to landingpage/ and finalize v1.0 sections`
- [x] **Landing Page Container & Edge Proxy Stack** — Multi-stage Bun + Nginx container (`landingpage/Dockerfile`, `landingpage/nginx.conf`), internal `docker-compose.yml` (`web`, `caddy`, `cloudflared`), `Caddyfile`, `.env.example`, and `.dockerignore`.
  `Commit: chore(deploy): add Docker Compose, Caddy, and Cloudflare Tunnel setup for landing page`
- [x] **Tailscale SSH CI/CD Pipeline** — GitHub Actions workflow validating the Bun build and deploying to the private server over Tailscale SSH on port `7022` (`.github/workflows/deploy.yml`).
  `Commit: ci(deploy): add GitHub Actions workflow for Tailscale SSH deployment`

### Remaining in M0
- [ ] **Commit Landing Page & Deployment Infrastructure** — Stage the `saas/` cleanup, `landingpage/` source, `docker-compose.yml`, `Caddyfile`, `.env.example`, `.dockerignore`, and `.github/workflows/deploy.yml` so `main` matches the target directory layout.
  `Commit: feat(landingpage): add Svelte 5 landing page, Docker Compose stack, and Tailscale CI/CD`
- [ ] **Canonical URL, `robots.txt`, and `sitemap.xml`** — Add `<link rel="canonical" href="https://license.rira.web.id/" />` to `landingpage/index.html` and place static `robots.txt` and `sitemap.xml` inside `landingpage/public/`.
  `Commit: feat(landingpage): add canonical link tag, robots.txt, and sitemap.xml`
- [ ] **Production Tunnel Cutover & CI Verification** — Configure Cloudflare Tunnel public hostname (`license.rira.web.id` -> `http://caddy:80`), set GitHub `production` environment secrets, and pass the live HTTP 200 verification step in GitHub Actions.
  `Commit: docs(readme): document live production URL and landing page deployment architecture`

---

## M1 — API Foundation & Core Licensing Engine (`server/`)

### Completed (In Repository)
- [x] **Go + `chi` HTTP Server & Health Probes** — `cmd/api/main.go` with `slog`, graceful shutdown, `RequestID`/`RealIP`/`Logger`/`Recoverer`/`CleanPath`/`Timeout` middleware, and `GET /`, `GET /healthz`, `GET /readyz` (`server/internal/` in `780a6e5`).
  `Commit: feat: initial RiraLicense project — Go + chi API server & Svelte frontend (780a6e5)`

### Remaining in M1
- [ ] **Extend Server Config & PostgreSQL Connection Pool** — Add `DATABASE_URL`, `ADMIN_API_KEY`, and `ED25519_PRIVATE_KEY` to `server/internal/config/config.go`, connect to PostgreSQL using `pgxpool`, and verify DB connectivity inside `GET /readyz`.
  `Commit: feat(api): add PostgreSQL connection pool and database readiness check`
- [ ] **SQL Migration Runner & Core Schema Migrations** — Add embedded SQL migrations (`server/migrations/0001_init.sql`) creating `products`, `licenses`, `activations`, and `audit_logs` tables with indexes and foreign keys.
  `Commit: feat(api): add embedded SQL migrations for products, licenses, activations, and audit logs`
- [ ] **Domain Models & Standard JSON Error Envelope** — Define strongly-typed structs and enums (`LicenseType`: `PERPETUAL`/`SUBSCRIPTION`/`TRIAL`; `LicenseStatus`: `ACTIVE`/`TRIAL`/`SUSPENDED`/`REVOKED`) in `server/internal/domain/` and standard JSON error helpers in `server/internal/handler/`.
  `Commit: feat(api): define core licensing domain models and JSON error helpers`
- [ ] **Admin API Key & CORS Middleware** — Implement bearer/API-key authentication middleware for `/api/v1/admin/*` routes and configurable CORS middleware for control plane requests in `server/internal/router/`.
  `Commit: feat(api): add admin authentication and CORS middleware`
- [ ] **Audit Log Repository & Service** — Implement append-only audit event recording (`license.issued`, `license.validated`, `device.activated`, `license.revoked`, etc.) and paginated query filters in `server/internal/repo/audit.go` and `server/internal/service/audit.go`.
  `Commit: feat(api): add immutable audit log repository and service`
- [ ] **Product & Entitlements CRUD Endpoints** — Implement `POST /api/v1/admin/products`, `GET /api/v1/admin/products`, `GET /api/v1/admin/products/{id}`, `PATCH /api/v1/admin/products/{id}`, and `DELETE /api/v1/admin/products/{id}` with default policies and feature entitlements.
  `Commit: feat(api): add product and feature entitlement CRUD endpoints`
- [ ] **Cryptographic License Key Generator & License CRUD** — Implement formatted key generation (`RIRA-XXXX-XXXX-XXXX`) and `POST /api/v1/admin/licenses`, `GET /api/v1/admin/licenses`, `GET /api/v1/admin/licenses/{id}` supporting perpetual, subscription, and trial policies.
  `Commit: feat(api): add license key generation and license creation/listing endpoints`
- [ ] **License Lifecycle Transition Endpoints** — Add `POST /api/v1/admin/licenses/{id}/renew`, `POST /api/v1/admin/licenses/{id}/suspend`, `POST /api/v1/admin/licenses/{id}/revoke`, and entitlement update handlers that write atomic audit log entries.
  `Commit: feat(api): add license renew, suspend, and revoke lifecycle endpoints`
- [ ] **Node-Locked Device Activation & Deactivation Endpoints** — Implement `POST /api/v1/activate`, `POST /api/v1/deactivate`, and `GET /api/v1/admin/activations` enforcing `activationsMax` slot limits per license inside a PostgreSQL transaction.
  `Commit: feat(api): add node-locked device activation and deactivation endpoints`
- [ ] **Public License Validation Endpoint (`POST /api/v1/validate`)** — Implement `POST /api/v1/validate` matching the landing page contract (`productId`, `licenseKey`, `deviceFingerprint`, `deviceName` -> `valid`, `status`, `licenseType`, `deviceActivation`, `entitlements`, `expiresAt`).
  `Commit: feat(api): add POST /api/v1/validate endpoint matching v1.0 contract`
- [ ] **Dashboard Summary Metrics & Audit Log HTTP Handler** — Add `GET /api/v1/admin/stats` (active licenses, total products, active activations, validation rate) and `GET /api/v1/admin/audit-logs` for the control plane UI.
  `Commit: feat(api): add dashboard summary metrics and audit log query endpoints`
- [ ] **Unit & Integration Test Suite for Licensing Engine** — Add table-driven Go tests (`server/internal/service/*_test.go` and `server/internal/handler/*_test.go`) covering policy expiration, activation slot limits, and validation responses.
  `Commit: test(api): add unit and handler tests for license validation and activation`
- [ ] **Server Multi-Stage Dockerfile** — Create `server/Dockerfile` (`golang:1.26-alpine` build stage to minimal `alpine` runtime with `/healthz` healthcheck on port `7819`).
  `Commit: build(api): add multi-stage Dockerfile for Go API server`

---

## M2 — Control Plane UI (`saas/`)

- [ ] **Initialize Svelte 5 + Tailwind CSS 4 Control Plane App** — Scaffold `saas/` with Svelte 5 Runes, Vite, TypeScript, and the Neo-Brutalist design tokens (`cream`, `ink`, `yellow`, `cobalt`, `coral`, `shadow-brutal`) shared with `landingpage/`.
  `Commit: feat(saas): initialize Svelte 5 control plane with Neo-Brutalist design system`
- [ ] **Typed API Client & Admin Key Session Store** — Create `saas/src/lib/api/` client wrapper with auth header injection, error handling, and login/session guard for `/api/v1/admin/*` endpoints.
  `Commit: feat(saas): add typed API client and admin authentication session store`
- [ ] **Control Plane App Shell & Overview Dashboard** — Build the sidebar/header layout and overview dashboard displaying active licenses, total products, active device bindings, validation health status, and recent license events.
  `Commit: feat(saas): add control plane layout and overview metrics dashboard`
- [ ] **Products & Entitlements Management View** — Build the Products table, create/edit drawer, and feature-entitlement tag editor wired to `/api/v1/admin/products`.
  `Commit: feat(saas): add products management table and entitlement editor`
- [ ] **Licenses Lifecycle Management View** — Build the Licenses table with status/type filters, issue-license modal, one-click key copy, and renew/suspend/revoke actions wired to `/api/v1/admin/licenses`.
  `Commit: feat(saas): add license table, key issuance modal, and lifecycle controls`
- [ ] **Device Activations Management View** — Build the node-locked Activations table showing bound `deviceFingerprint`, `deviceName`, slot usage (`activationsUsed / activationsMax`), and manual slot revocation.
  `Commit: feat(saas): add device activations view with fingerprint slot management`
- [ ] **Audit Log Explorer View** — Build the filterable, paginated Audit Log viewer displaying event types, actor/IP metadata, license keys, and timestamps from `GET /api/v1/admin/audit-logs`.
  `Commit: feat(saas): add filterable audit log explorer view`
- [ ] **Control Plane Container (`saas/Dockerfile` & `saas/nginx.conf`)** — Add multi-stage Bun + Nginx build in `saas/` and wire `saas`, `server`, and `postgres:17-alpine` into `docker-compose.yml` and `Caddyfile`.
  `Commit: chore(deploy): containerize saas control plane and wire full stack in Docker Compose`

---

## M3 — Offline Cryptography, Webhooks, SDKs & v1.0 Release

- [ ] **Ed25519 Offline License Grant Signing** — Implement Ed25519 keypair loading and signed offline grant issuance (`POST /api/v1/admin/licenses/{id}/offline-grant` and signature attachment in `/api/v1/validate`) in `server/internal/crypto/`.
  `Commit: feat(api): add Ed25519 asymmetric signing for offline license grants`
- [ ] **Lifecycle Webhook Dispatcher & Delivery Retries** — Add webhook registration and asynchronous HMAC-signed HTTP delivery with exponential backoff for `license.*` and `device.*` events in `server/internal/service/webhook.go`.
  `Commit: feat(api): add HMAC-signed lifecycle webhook dispatcher with retries`
- [ ] **Official TypeScript / JavaScript SDK (`sdk/js/`)** — Create `@riralicense/sdk` implementing `new RiraLicense({ baseUrl, productId, publicKey })`, online `.validate()` / `.activate()`, and offline Ed25519 signature verification.
  `Commit: feat(sdk-js): add official TypeScript SDK with online validation and Ed25519 offline verification`
- [ ] **Official Go Client SDK (`sdk/go/`)** — Create `riralicense-go` client package providing idiomatic Go methods for online validation, device activation, and offline Ed25519 grant verification with unit tests.
  `Commit: feat(sdk-go): add official Go client SDK for online and offline license verification`
- [ ] **CI Pipeline for Go Server, Control Plane, and SDKs** — Add `.github/workflows/ci.yml` running `go vet`, `go test ./...`, `svelte-check`, and `bun run build` across `server/`, `saas/`, `landingpage/`, and `sdk/` on pull requests and pushes.
  `Commit: ci: add automated test and typecheck workflow for server, saas, and SDKs`
- [ ] **Self-Hosted Operations & API Reference Documentation** — Update `README.md` and `server/README.md` with full REST API endpoint schemas, PostgreSQL backup/restore commands, Ed25519 key generation instructions, and Docker Compose self-hosting guide.
  `Commit: docs: add comprehensive v1.0 API reference and self-hosted operations guide`
- [ ] **Community Edition v1.0.0 Release Checklist & Tag** — Update roadmap badges in `landingpage/src/lib/data/navigation.ts` and `features.ts` to `AVAILABLE`, add `CHANGELOG.md` for `v1.0.0`, and tag the release.
  `Commit: chore(release): prepare RiraLicense Community Edition v1.0.0`

