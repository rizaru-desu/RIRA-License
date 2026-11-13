# RiraLicense

**Your Software. Your Licenses. Your Control.**

[![GitHub stars](https://img.shields.io/github/stars/rizaru-desu/RIRA-License?style=flat-square)](https://github.com/rizaru-desu/RIRA-License)
[![License: GNU AGPL-3.0](https://img.shields.io/github/license/rizaru-desu/RIRA-License?style=flat-square)](LICENSE)
[![Deploy Landing Page](https://github.com/rizaru-desu/RIRA-License/actions/workflows/deploy.yml/badge.svg)](https://github.com/rizaru-desu/RIRA-License/actions/workflows/deploy.yml)

**Live demo:** [https://license.rira.web.id](https://license.rira.web.id)

[![RiraLicense Landing Page Preview](https://license.rira.web.id/og-image.png)](https://license.rira.web.id)

## About

RiraLicense is an open-source, self-hosted software licensing infrastructure platform by **RiraLab**. It is built for independent developers, software vendors, and teams who want full ownership over license issuance, validation, device activations, and audit trails for desktop applications, SaaS products, and APIs—without third-party cloud lock-in.

> **Project Status:** RiraLicense Community Edition v1.0 is actively in development (see [ROADMAP.md](ROADMAP.md)). The landing page and Go HTTP server foundation are live in the repository today; the PostgreSQL licensing engine and Svelte 5 control plane are currently being built.

## Features

| Area | Status | What's Included |
| :--- | :--- | :--- |
| **Landing Page & Docs (`landingpage/`)** | Live | Svelte 5 + Vite site at [license.rira.web.id](https://license.rira.web.id) with REST API contract preview, 7-step self-hosted deployment guide, and Community v1.0 roadmap |
| **Go + `chi` Server Foundation (`server/`)** | Available | HTTP bootstrap with graceful `SIGINT`/`SIGTERM` shutdown, standard middleware (`RequestID`, `RealIP`, `Logger`, `Recoverer`, `CleanPath`, `Timeout`), and `/`, `/healthz`, `/readyz` probes |
| **Self-Hosted Deploy Stack** | Available | Pre-built GHCR image (`linux/arm64`), `docker-compose.yml` with `web` + `caddy` reverse proxy, and Tailscale SSH CI/CD |
| **Licensing Engine & REST API (`server/`)** | In Progress (v1.0) | Product & entitlement CRUD, perpetual/subscription/trial keys, `POST /api/v1/validate`, node-locked device activations, immutable audit log, and Ed25519 offline verification |
| **Control Plane UI (`saas/`)** | In Progress (v1.0) | Svelte 5 admin console for managing products, licenses, device activations, and audit log events |

## Quick Start

### Deploy the Landing Page Stack (Docker Compose)

```bash
git clone https://github.com/rizaru-desu/RIRA-License.git
cd RIRA-License
cp .env.example .env
# Edit .env if you need a custom loopback port (default: CADDY_HTTP_PORT=8080)
docker compose up -d
```

See the full deployment walkthrough on the landing page: [Self-Hosted Deployment Guide](https://license.rira.web.id/#deployment).

### Run the Go API Server Locally

Requires Go 1.22+ (module targets Go 1.26):

```bash
cd server
go run ./cmd/api
```

Verify the readiness probe at `http://localhost:7819/readyz`:

```bash
curl http://localhost:7819/readyz
```

## Architecture & Project Structure

```text
RIRA-License/
├── landingpage/              # Svelte 5 + Vite landing page, Dockerfile, and Nginx SPA config
├── saas/                     # Svelte 5 control plane UI (scheduled for M2 in ROADMAP.md)
├── server/                   # Go + chi v5 API server (cmd/api/, internal/{config,handler,router,service})
├── .github/workflows/        # GitHub Actions CI/CD pipeline (GHCR build + Tailscale SSH deploy)
├── docker-compose.yml        # Production compose stack (web + caddy)
├── Caddyfile                 # Internal HTTP reverse proxy configuration (TLS terminated at Cloudflare)
├── .env.example              # Environment variable template (CADDY_HTTP_PORT)
├── ROADMAP.md                # Community Edition v1.0 milestone plan
├── LICENSE                   # GNU AGPL-3.0 license text
├── NOTICE                    # Copyright and trademark attribution notice
└── CODE_OF_CONDUCT.md        # Community participation guidelines
```

## Tech Stack

- **Backend:** Go, [`chi` v5](https://github.com/go-chi/chi), PostgreSQL (v1.0 licensing storage)
- **Frontend:** Svelte 5, Vite, TypeScript, Tailwind CSS 4
- **Infrastructure:** Docker Compose, Nginx, Caddy, Cloudflare Tunnel (`cloudflared`)
- **CI/CD:** GitHub Actions, GitHub Container Registry (GHCR), Tailscale

## Contributing

Contributions, bug reports, and architecture feedback are welcome. Please review [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) and check [ROADMAP.md](ROADMAP.md) for current v1.0 milestones before opening a pull request.

## License & Ownership

> Copyright © 2026 [RiraLab — Rizaru Desu](https://github.com/rizarudev)

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)** — see [LICENSE](LICENSE) and [NOTICE](NOTICE) for full terms.

- **Copyleft & Network Use:** Forks and modified versions must preserve copyright notices, remain under AGPL-3.0, and make source code available to users interacting over a network (AGPL-3.0 Section 13).
- **Trademarks (Section 7e):** The names **"RiraLicense"** and **"RiraLab"**, along with their logos and brand identity, are trademarks of RiraLab and are not licensed under AGPL-3.0. Forks must use their own distinct name and branding.

## Links

- **Live Landing Page:** [https://license.rira.web.id](https://license.rira.web.id)
- **GitHub Discussions:** [github.com/rizaru-desu/RIRA-License/discussions](https://github.com/rizaru-desu/RIRA-License/discussions)
- **Issue Tracker:** [github.com/rizaru-desu/RIRA-License/issues](https://github.com/rizaru-desu/RIRA-License/issues)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
