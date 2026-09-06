# Architecture Decision Records (ADR)

## ADR-001: Independent Module Namespace under github.com/PakaiWA/whatsmeow

- **Status**: Accepted
- **Date**: 2026-02-09
- **Source**: Repository commit history (`37ae7b2`, `1a8f20e`) & Developer confirmation
- **Context**: Upstream menggunakan module path `go.mau.fi/whatsmeow`. Untuk integrasi stabil pada platform microservice internal PakaiWA dan menghindari ketergantungan upstream yang tak terkontrol, modul dialihkan ke namespace internal.
- **Decision**: Mengubah `go.mod` dan seluruh file import Go ke namespace `github.com/PakaiWA/whatsmeow`. Menjaga konsistensi import melalui pre-commit hook `go-imports-repo` dan workflow CI.
- **Consequences**:
  - Positif: Kontrol penuh atas rilis, patch internal, dan isolasi dependensi mikroservis.
  - Negatif: Setiap merge dari upstream memerlukan proses rekonsiliasi import path.

---

## ADR-002: Custom DeviceProps OS Fingerprint (PakaiWA Identity)

- **Status**: Accepted
- **Date**: 2026-08-10
- **Source**: Developer confirmation & Commit `60c6e68`
- **Context**: WhatsApp mencatat metadata platform client saat inisiasi companion registration handshake.
- **Decision**: Menetapkan nilai identitas `DeviceProps OS` ke nama platform internal `PakaiWA`. Nilai ini wajib dipertahankan setiap kali melakukan sinkronisasi dengan upstream.
- **Consequences**:
  - Positif: Identitas session perangkat tercatat rapi di WhatsApp Web sebagai platform resmi internal PakaiWA.
  - Negatif: Wajib dicek saat ada pembaruan skema `DeviceProps` di protobuf upstream.

---

## ADR-003: Production Session Persistence on PostgreSQL

- **Status**: Accepted
- **Date**: 2026-09-06
- **Source**: Developer interview
- **Context**: Sistem WhatsApp multi-device membutuhkan penyimpanan sesi (identity keys, sender keys, pre-keys, app state) yang andal, scalable, dan mendukung high availability pada deployment container/Kubernetes microservice.
- **Decision**: Menetapkan PostgreSQL sebagai standar persistence layer utama untuk lingkungan produksi (via `store/sqlstore`). SQLite tetap didukung sebagai opsi development atau ephemeral testing.
- **Consequences**:
  - Positif: Mendukung concurrent access dari cluster service, ACID compliant, dan backup database terpusat.
  - Negatif: Memerlukan koneksi database terkelola dan tuning connection pool di lingkungan cloud/K8s.

---

## ADR-004: Event-Driven Upstream Synchronization Strategy

- **Status**: Accepted
- **Date**: 2026-09-06
- **Source**: Developer interview
- **Context**: Upstream `tulir/whatsmeow` aktif melakukan commit protobuf dan fitur minor, namun sinkronisasi tanpa filter dapat menimbulkan regresi pada integrasi internal.
- **Decision**: Sinkronisasi dari upstream dilakukan secara *event-driven*, dipicu terutama saat terjadi breaking change pada WhatsApp Web protocol atau bug krusial pada layer socket/dekripsi. CodeQL alerts dievaluasi secara selektif dan tidak selalu di-autofix otomatis.
- **Consequences**:
  - Positif: Menjaga stabilitas platform microservice PakaiWA dan meminimalkan churn rilis yang tidak perlu.
  - Negatif: Pembaruan fitur opsional upstream tidak langsung tersedia sebelum dilakukan sinkronisasi manual.

---

## ADR-005: Pre-Commit Automated Protobuf Compilation and Formatting Pipeline

- **Status**: Accepted
- **Date**: 2026-09-06
- **Source**: Codebase implementation (`.pre-commit-config.yaml`, `proto/convert.sh`) & Developer request
- **Context**: Perubahan skema `.proto` sering menghasilkan inkonsistensi formatting import Go pada file `.pb.go` jika tidak dijalankan bersamaan dengan toolchain formatter.
- **Decision**:
  1. Menambahkan local hook `proto-convert` di `.pre-commit-config.yaml` sebelum `go-imports-repo`.
  2. Mengoptimalkan `proto/convert.sh` dengan mode `set -euo pipefail`, validasi `protoc` & `protoc-gen-go`, serta pemformatan otomatis via `goimports -local github.com/PakaiWA/whatsmeow -w .`.
- **Consequences**:
  - Positif: Setiap perubahan protobuf langsung terkompilasi dan terformat seragam sebelum kode di-push.
  - Negatif: Developer wajib memiliki binary `protoc` dan `protoc-gen-go` di local environment saat mengubah skema proto.
