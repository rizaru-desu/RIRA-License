# RiraLicense

Server lisensi & API untuk produk RiraLab — dibangun dengan Go + chi router.

## ✨ Fitur

- REST API dengan [chi](https://github.com/go-chi/chi) v5
- Clean architecture: `handler` → `service` → (repo)
- Health check endpoints (`/healthz`, `/readyz`)
- Graceful shutdown (SIGINT / SIGTERM)
- Middleware standar: RequestID, RealIP, Logger, Recoverer, CleanPath, Timeout

## 📁 Struktur Project

```
RiraLicense/
├── server/                  # Backend Go
│   ├── cmd/api/             # Entry point
│   ├── internal/
│   │   ├── config/          # Konfigurasi (port, env)
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

Server berjalan di **http://localhost:7819**.

#### Environment Variables

| Variable   | Default       | Deskripsi            |
| ---------- | ------------- | -------------------- |
| `APP_PORT` | `7819`        | Port HTTP server     |
| `APP_ENV`  | `development` | `development` / `production` |

#### Endpoints

| Method | Endpoint   | Deskripsi                    |
| ------ | ---------- | ---------------------------- |
| `GET`  | `/`        | Liveness check               |
| `GET`  | `/healthz` | Liveness check               |
| `GET`  | `/readyz`  | Readiness + info runtime     |

Contoh response `/readyz`:

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

## ⚖️ Lisensi & Kepemilikan

> Copyright © 2026 [RiraLab — Rizaru Desu](https://github.com/rizarudev)

Project ini dilisensikan di bawah **GNU Affero General Public License v3.0 (AGPL-3.0)** —
lihat [LICENSE](LICENSE) untuk teks lengkap.

### Ketentuan Penting

1. **Kepemilikan** — Seluruh kode sumber adalah milik dan hak cipta RiraLab — Rizaru Desu.
   Tidak ada pihak lain yang boleh mengklaim kepemilikan, keaslian, atau kepengarangan
   atas kode ini.

2. **Fork & modifikasi** — Siapa pun bebas fork, memodifikasi, dan menggunakan project ini
   SELAMA:
   - Copyright notice dan lisensi AGPL-3.0 **dipertahankan utuh** di semua file;
   - Versi hasil modifikasi juga dirilis di bawah AGPL-3.0 (copyleft);
   - Source code dari versi yang dijalankan sebagai server/network service **wajib
     dipublikasikan** kepada penggunanya (Pasal 13 AGPL).

3. **Rebranding / penggantian nama** — Nama **"RiraLicense"**, logo, identitas brand,
   dan nama **"RiraLab"** merupakan merek milik RiraLab dan **TIDAK dilisensikan** di
   bawah AGPL-3.0 (sesuai Pasal 7e). Versi fork/hasil modifikasi **wajib menggunakan
   nama sendiri** dan **dilarang**:
   - menggunakan nama/brand RiraLicense atau RiraLab;
   - menghapus, menutupi, atau mengganti copyright notice asli;
   - mengklaim atau mengesankan bahwa hasil fork adalah karya asli atau produk
     resmi RiraLab.

4. **Pelanggaran** — Menghapus atribusi copyright, menulis ulang lisensi atas nama
   pihak lain, atau me-rebrand lalu mengklaim kepemilikan **melanggar hukum hak cipta**
   dan lisensi AGPL-3.0. Hak cipta tetap melekat pada RiraLab — Rizaru Desu terlepas
   dari bagaimana kodenya disalin, diubah, atau diedarkan.

> 💡 **Catatan:** Copyright melindungi *kode*, trademark melindungi *nama & brand*.
> Kombinasi keduanya (AGPL + klausul trademark di atas) adalah mekanisme yang
> melindungi project ini dari kasus rebranding/pemilikan kembali oleh pihak lain.

---

Dibuat dengan ❤️ oleh **RiraLab — Rizaru Desu**
