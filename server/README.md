# RiraLicense — Server (Go + chi)

Backend API untuk RiraLicense. Clean architecture: `handler` → `service` → (repo).

## Run

```sh
go run ./cmd/api
```

Server berjalan di **http://localhost:7819**.

| Variable   | Default       | Deskripsi            |
| ---------- | ------------- | -------------------- |
| `APP_PORT` | `7819`        | Port HTTP server     |
| `APP_ENV`  | `development` | `development` / `production` |

## Endpoints

| Method | Endpoint   | Deskripsi                |
| ------ | ---------- | ------------------------ |
| `GET`  | `/`        | Liveness                 |
| `GET`  | `/healthz` | Liveness                 |
| `GET`  | `/readyz`  | Readiness + info runtime |

## Struktur

```
server/
├── cmd/api/          # Entry point (main.go, graceful shutdown)
└── internal/
    ├── config/       # APP_PORT, APP_ENV
    ├── handler/      # HTTP layer
    ├── service/      # Business logic
    └── router/       # chi router + middleware
```

## Lisensi

AGPL-3.0 — Copyright © 2026 RiraLab — Rizaru Desu.
Lihat [LICENSE](../LICENSE) dan [NOTICE](../NOTICE) di root project.
