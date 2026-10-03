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

---

## ADR-006: Pure SemVer Versioning Starting at v0.26.10 with Automated Proxy Warmup

- **Status**: Accepted
- **Date**: 2026-10-01
- **Source**: CI workflow evolution (`.github/workflows/go.yml`) & Release tracking
- **Context**: Sebelumnya repositori memiliki tag legacy `v0.25.*` hingga `v0.26.9` dari upstream. Ketika rilis branch `main` PakaiWA dipersiapkan, penomoran versi baru harus berkesinambungan dan mematuhi standard Go module versioning tanpa memutus kompatibilitas down-stream dependency resolution. Selain itu, Go module proxy (`proxy.golang.org`) dan indexer dokumentasi (`pkg.go.dev`) membutuhkan request warmup otomatis setelah rilis dipublikasikan agar package langsung terindeks tanpa menunggu query organik.
- **Decision**:
  1. Melanjutkan penomoran SemVer secara linier dimulai dari `v0.26.10` untuk setiap rilis patch berikutnya.
  2. Mengotomatiskan auto-tagging patch release di GitHub Actions workflow (`.github/workflows/go.yml`) pada push ke branch `main`, dengan guard kondisi khusus (`filter` paths: `**/*.go`, `go.mod`, `go.sum`, `proto/**`, mengecualikan dokumen/workflow non-Go).
  3. Menambahkan step warmup otomatis setelah rilis yang memanggil endpoint `https://proxy.golang.org/github.com/PakaiWA/whatsmeow/@v/${NEW_TAG}.info` dan request reindex ke `https://pkg.go.dev/github.com/PakaiWA/whatsmeow@${NEW_TAG}`.
- **Consequences**:
  - Positif: Rilis selalu konsisten, tidak ada duplikasi tag, modul Go langsung tersedia dan terverifikasi di ekosistem Go proxy dan pkg.go.dev segera setelah rilis dibuat.
  - Negatif: Push yang hanya mengubah file non-Go (`*.md`, CI configs) tidak memicu pembuatan rilis baru (sesuai ekspektasi rilis library).

---

## ADR-007: Dual Copyright Attribution Under MPL-2.0

- **Status**: Accepted
- **Date**: 2026-10-01
- **Source**: Licensing compliance review (`LICENSE`, `README.md`)
- **Context**: `whatsmeow` dilisensikan di bawah Mozilla Public License 2.0 (MPL-2.0). Sebagai fork terkelola yang menambahkan adaptasi platform, modifikasi identitas, dan perbaikan keamanan, repositori wajib mempertahankan atribusi hak cipta pencipta awal sekaligus mencatatkan atribusi hak cipta bagi kontributor dan pengembang PakaiWA.
- **Decision**:
  1. Mempertahankan hak cipta orisinal: `Copyright (C) 2021-2026 Tulir Asokan`.
  2. Menambahkan hak cipta fork: `Copyright (C) 2025-2026 Kelvin Anggara, PakaiWA Developers and Contributors`.
  3. Menampilkan pernyataan dual copyright ini secara eksplisit pada file `LICENSE` dan bagian lisensi `README.md`.
- **Consequences**:
  - Positif: Memenuhi klausul legal MPL-2.0 secara transparan, menghargai kontribusi hulu (upstream) sekaligus menegaskan kepemilikan adaptasi hilir (downstream).
  - Negatif: Setiap perubahan teks lisensi atau header file di masa depan harus menjaga integritas atribusi ganda ini.

---

## ADR-008: Long-Term Runner Pinning to Ubuntu 24.04 in CI Workflows

- **Status**: Accepted
- **Date**: 2026-10-01
- **Source**: Infrastructure & CI standardization (`.github/workflows/go.yml`, `.github/workflows/label-pr.yml`)
- **Context**: Menggunakan runner alias mengambang seperti `ubuntu-latest` dapat memicu kegagalan build/test yang tidak terduga saat GitHub memigrasikan image default ke versi OS baru (misal perubahan toolchain GCC, glibc, Python, atau paket sistem).
- **Decision**:
  Mengunci semua job CI GitHub Actions (`build`, `test`, `release`, `pr-labeler`) ke image LTS spesifik: `runs-on: ubuntu-24.04`.
- **Consequences**:
  - Positif: Lingkungan runner deterministik, stabil, dan terhindar dari breaking changes mendadak akibat rolling update runner host GitHub.
  - Negatif: Perlu evaluasi berkala terjadwal saat image `ubuntu-24.04` mendekati siklus end-of-support (EOL).

---

## ADR-009: Standardization of CI Workflow Extensions (.yaml) and Application Path Push Triggers

- **Status**: Accepted
- **Date**: 2026-10-01
- **Source**: CI workflow modernization (`.github/workflows/go.yaml`, `.github/workflows/codeql.yaml`, `.github/workflows/stale.yaml`)
- **Context**: Sebelumnya workflow files menggunakan ekstensi gado-gado `.yml` dan push trigger pada branch `main` tidak memfilter path file. Akibatnya, setiap commit dokumen (seperti `README.md`, `ARCHITECTURE.md`, `DECISIONS.md`) berpotensi memicu pipeline rilis SemVer dan pembuatan tag baru yang tidak perlu untuk file non-aplikasi.
- **Decision**:
  1. Menstandardisasi semua file workflow GitHub Actions menggunakan ekstensi `.yaml` (`go.yaml`, `codeql.yaml`, `stale.yaml`).
  2. Menerapkan filter `paths` pada push trigger `main` di `.github/workflows/go.yaml` agar hanya berjalan jika terdapat perubahan pada file aplikasi:
     - `**/*.go`
     - `**/*.yaml`
     - `go.mod`
     - `go.sum`
     - `Makefile`
- **Consequences**:
  - Positif: Mencegah eksekusi build/test dan tagging release SemVer otomatis yang berlebihan saat hanya memperbarui dokumentasi atau file non-runtime. Konsistensi ekstensi file YAML di seluruh repositori.
  - Negatif: Perubahan konfigurasi workflow baru atau build script di luar filter di atas harus mendaftarkan ekstensinya ke `paths` jika ingin memicu CI release.
