# RiraLicense

License server & API for RiraLab products — built with Go + chi router.

## ✨ Features

- REST API with [chi](https://github.com/go-chi/chi) v5
- Clean architecture: `handler` → `service` → (repo)
- Health check endpoints (`/healthz`, `/readyz`)
- Graceful shutdown (SIGINT / SIGTERM)
- Standard middleware: RequestID, RealIP, Logger, Recoverer, CleanPath, Timeout

## 📁 Project Structure

```
RiraLicense/
├── server/                  # Go backend
│   ├── cmd/api/             # Entry point
│   ├── internal/
│   │   ├── config/          # Configuration (port, env)
│   │   ├── handler/         # HTTP layer
│   │   ├── service/         # Business logic layer
│   │   └── router/          # Chi router + middleware
│   ├── go.mod / go.sum
│   └── README.md
├── saas/                    # Frontend (Svelte + Vite)
└── LICENSE                  # GNU AGPL-3.0
```

## 🚀 Getting Started

### Backend (Go)

**Requirements:** Go 1.22+

```sh
cd server
go run ./cmd/api
```

The server starts at **http://localhost:7819**.

#### Environment Variables

| Variable   | Default       | Description                  |
| ---------- | ------------- | ---------------------------- |
| `APP_PORT` | `7819`        | HTTP server port             |
| `APP_ENV`  | `development` | `development` / `production` |

#### Endpoints

| Method | Endpoint   | Description                 |
| ------ | ---------- | --------------------------- |
| `GET`  | `/`        | Liveness check              |
| `GET`  | `/healthz` | Liveness check              |
| `GET`  | `/readyz`  | Readiness + runtime info    |

Example `/readyz` response:

```json
{
  "status": "ok",
  "service": "riralicense-api",
  "env": "development",
  "uptime": "1m30s",
  "timestamp": "2026-11-11T23:44:44+07:00"
}
```

### Frontend (Svelte)

```sh
cd saas
bun install
bun run dev
```

## ⚖️ License & Ownership

> Copyright © 2026 [RiraLab — Rizaru Desu](https://github.com/rizarudev)

This project is licensed under the **GNU Affero General Public License v3.0
(AGPL-3.0)** — see [LICENSE](LICENSE) for the full text.

### Important Terms

1. **Ownership** — All source code is the property of and copyrighted by
   RiraLab — Rizaru Desu. No other party may claim ownership, originality,
   or authorship of this code.

2. **Forks & modifications** — Anyone is free to fork, modify, and use this
   project AS LONG AS they:
   - Keep the copyright notices and AGPL-3.0 license **intact** in all files;
   - Release their modified version under AGPL-3.0 as well (copyleft);
   - **Publish the source code** of any modified version running as a
     server/network service to its users (AGPL Section 13).

3. **Rebranding / renaming** — The name **"RiraLicense"**, its logo, brand
   identity, and the name **"RiraLab"** are trademarks of RiraLab and are
   **NOT licensed** under the AGPL-3.0 (per Section 7e). Forked/modified
   versions **must use their own name** and are **prohibited** from:
   - Using the "RiraLicense" or "RiraLab" name/brand;
   - Removing, obscuring, or replacing the original copyright notices;
   - Claiming or implying that a fork is an original work or an official
     RiraLab product.

4. **Violations** — Removing copyright attribution, rewriting the license in
   someone else's name, or rebranding and claiming ownership **violates
   copyright law** and the AGPL-3.0. Copyright remains with RiraLab —
   Rizaru Desu regardless of how the code is copied, altered, or distributed.

> 💡 **Note:** Copyright protects the *code*, trademarks protect the *name &
> brand*. The combination of both (AGPL + the trademark clause above) is what
> protects this project from rebranding/ownership cases by third parties.

## 🤝 Contributing

Contributions are welcome! Please read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
before participating.

---

Made with ❤️ by **RiraLab — Rizaru Desu**
