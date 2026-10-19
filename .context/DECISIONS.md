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

---

## ADR-010: Pure Unit Test Standard with In-Memory Mock Socket Architecture

- **Status**: Accepted
- **Date**: 2026-10-19
- **Source**: Unit test overhaul and Codecov integration (`*_test.go`, `.github/workflows/go.yaml`)
- **Context**: Sebelumnya banyak modul inti (`upload.go`, `send.go`, `request.go`, `retry.go`, `receipt.go`, `newsletter.go`, dsb.) belum memiliki unit test murni yang terisolasi tanpa koneksi jaringan live ke server WhatsApp. Pengujian berbasis end-to-end langsung ke WhatsApp Web rentan flakiness, rate limiting, dan tidak dapat dijalankan secara deterministik di CI.
- **Decision**:
  1. Menerapkan standar unit test murni (*zero network dependency*) di mana setiap modul inti harus memiliki cakupan pengujian pernyataan minimal ≥ 80% (statement coverage).
  2. Membangun abstraksi pengujian menggunakan in-memory mock socket (`mockTestSocket`) yang mereplikasi handshake Noise dan WebSocket frame loop tanpa I/O jaringan sungguhan.
  3. Memisahkan pengujian dengan simulasi response node (`waBinary.Node`) deterministik untuk skenario sukses, error IQ, error status code, dan context cancellation.
  4. Mengharuskan verifikasi thread-safety bebas data race di bawah flag `go test -race` dan upload coverage profile atomic ke Codecov.
- **Consequences**:
  - Positif: Test suite dapat dieksekusi cepat, 100% deterministik, bebas flakiness jaringan di CI, serta regresi protokol segera terdeteksi sedini mungkin.
  - Negatif: Setiap perubahan struktur node protobuf atau internal handling IQ mewajibkan penyesuaian payload fixture pada mock socket test.

---

## ADR-011: Exclusion of Test Files from SemVer Release Tagging in CI

- **Status**: Accepted
- **Date**: 2026-10-19
- **Source**: Developer request & CI workflow enhancement (`.github/workflows/go.yaml`)
- **Context**: Sebelumnya job `release` pada `.github/workflows/go.yaml` memicu pembuatan rilis SemVer baru (`vMAJOR.MINOR.PATCH`) setiap kali ada perubahan pada file `*.go`. Ketika developer hanya menambahkan atau memperbarui file pengujian (`*_test.go`), pipeline secara tidak perlu membuat rilis versi baru ke GitHub Releases dan mempublikasikan versi baru ke proxy Go.
- **Decision**:
  1. Menambahkan filter pengecualian `":(exclude)*_test.go"` pada perintah `git diff` dan `git log` di job release `.github/workflows/go.yaml`.
  2. Jika perubahan sejak tag SemVer sebelumnya murni hanya terdiri dari file `*_test.go`, script release akan mencetak pesan dan keluar (`exit 0`) tanpa memotong tag rilis baru.
- **Consequences**:
  - Positif: Mencegah rilis versi SemVer palsu/berlebihan (release tag bloat) saat hanya melakukan penambahan atau refactoring unit test.
  - Negatif: Jika sebuah rilis fitur sengaja dirilis bersamaan dengan unit test, setidaknya satu file non-test (`*.go` implementasi atau modul) harus terubah agar tag release terpicu.

---

## ADR-012: Defensive Nil-Check for Device Container in Save and Delete Operations

- **Status**: Accepted
- **Date**: 2026-10-19
- **Source**: CI regression failure analysis (CI job 117974312627 / commit `0cd2e75`) & commit `0631037`
- **Context**: Pada pengujian unit test dan mock socket harness, instance `store.Device` sering kali diinstansiasi secara in-memory tanpa underlying `DeviceContainer` database (`device.Container == nil`). Ketika event kegagalan koneksi atau logged-out terpicu secara asinkron dari antrean (`handlerQueueLoop`), method `device.Delete(ctx)` atau `device.Save(ctx)` dipanggil, mengakibatkan runtime panic SIGSEGV (nil pointer dereference) saat runtime goroutine scheduler (khususnya pada Go 1.27) mengeksekusi method tersebut.
- **Decision**:
  1. Menambahkan pengecekan defensif `if device == nil || device.Deleted` di awal pemanggilan `Save()` dan `Delete()`.
  2. Menambahkan pemeriksaan `if device.Container == nil { return nil }` di `Save()` dan `if device.Container != nil` sebelum memanggil `device.Container.DeleteDevice(ctx, device)` di `Delete()`.
  3. Memastikan pembersihan field in-memory (`device.ID = nil`, `device.Deleted = true`, `SetAllStores(&NoopStore{ErrDeviceDeleted})`) tetap berjalan aman meski tanpa storage container aktif.
- **Consequences**:
  - Positif: Mencegah race/panic crash fatal di lingkungan test maupun saat client diinisialisasi tanpa container database aktif, memastikan kestabilan lintas runtime Go 1.26 dan Go 1.27.
  - Negatif: Operasi `Save` dan `Delete` pada standalone `store.Device` (tanpa database backing) menjadi no-op silently tanpa mengembalikan error penyimpanan database.

---

## ADR-013: Codecov Flags and Architectural Components Configuration

- **Status**: Accepted
- **Date**: 2026-10-19
- **Source**: Developer request & Codecov configuration overhaul (`.codecov.yml`, `.github/workflows/go.yaml`)
- **Context**: Sebelumnya repositori hanya mengunggah satu coverage profile tanpa diferensiasi runtime matrix (Go 1.26 vs Go 1.27) dan tanpa isolasi domain arsitektur. Hal ini menyulitkan identifikasi modul mana yang mengalami penurunan coverage pada setiap PR, serta tidak memberikan pelacakan granular per-komponen di PR comment Codecov.
- **Decision**:
  1. **Flags**: Mengelompokkan laporan coverage berdasarkan versi Go di GitHub Actions matrix (`go-1.26` dan `go-1.27`) menggunakan parameter `flags: go-${{ matrix.go-version }}` pada `codecov/codecov-action@v5` dengan `carryforward: true`.
  2. **Components**: Mengelompokkan source code ke dalam 10 domain komponen arsitektur utama di `.codecov.yml`:
     - `core_client`: Core Client & Connection (`client.go`, `handshake.go`, `connectionevents.go`, `keepalive.go`, dll.)
     - `messaging_e2ee`: Messaging & Signal E2EE (`message.go`, `send.go`, `sendfb.go`, `armadillomessage.go`, `msgsecret.go`, dll.)
     - `media_transfer`: Media Transfer & CDN (`upload.go`, `download.go`, `download-to-file.go`, `mediaconn.go`, `mediaretry.go`)
     - `auth_pairing`: Authentication & Pairing (`pair.go`, `pair-code.go`, `pair-passkey.go`, `qrchan.go`, `prekeys.go`)
     - `chat_features`: Chat Features & State (`group.go`, `newsletter.go`, `user.go`, `business.go`, `appstate.go`, dll.)
     - `storage`: Storage & Persistence (`store/`)
     - `network_socket`: Socket & Noise Layer (`socket/`)
     - `binary_protocol`: WABinary Protocol (`binary/`)
     - `types_events`: Types & Events (`types/`)
     - `crypt_utilities`: Crypto & Log Utilities (`util/`, `appstate/lthash/`)
  3. **PR Comment Layout**: Mengaktifkan layout komprehensif `header, diff, flags, components, files` pada bot komentar PR Codecov.
  4. **Target Range**: Memperbarui visual threshold coverage range ke `70...90%` menyesuaikan realisasi statement coverage repositori yang telah melampaui 87%.
- **Consequences**:
  - Positif: Visibilitas coverage jauh lebih transparan dan granular di dashboard Codecov dan komentar PR, memudahkan pemantauan health per modul arsitektur.
  - Negatif: Setiap penambahan modul/direktori baru dianjurkan dipetakan ke komponen terkait di `.codecov.yml`.

---

## ADR-014: Implementation of Go Test Analytics and JUnit Reporting Pipeline

- **Status**: Accepted
- **Date**: 2026-10-19
- **Source**: Developer request & CI workflow enhancement (`.github/workflows/go.yaml`, `.gitignore`)
- **Context**: Sebelumnya pengujian di CI hanya mencetak output standar `go test -v` dan mengunggah `coverage.txt`. Tim tidak memiliki analitik performa eksekusi tes per suite (durasi eksekusi, identifikasi tes terlambat/lambat, deteksi *flaky tests*, dan pelacakan kegagalan historis) secara terpusat di dashboard Codecov.
- **Decision**:
  1. **JUnit Exporter**: Menggunakan `gotestsum` (`gotest.tools/gotestsum@latest`) di pipeline CI untuk membungkus eksekusi `go test` dengan argumen `--junitfile junit.xml --format testname -- -race -coverprofile=coverage.txt -covermode=atomic ./...`.
  2. **Codecov Test Analytics**: Menggunakan `codecov/codecov-action@v5` dengan parameter `report_type: test_results`, `files: junit.xml`, dan mapping flag matrix (`go-${{ matrix.go-version }}`) menggantikan action lama `test-results-action` yang sudah didepresiasi.
  3. **Repository Hygiene**: Menambahkan `junit.xml` ke `.gitignore` agar file artefak pengujian lokal tidak ter-commit ke git.
- **Consequences**:
  - Positif: CI menghasilkan telemetri lengkap per suite tes (durasi eksekusi, flaky tests, kegagalan granular) langsung di tab *Tests* Codecov dan GitHub Actions tanpa warning deprecation, menggunakan versi v5 terpadu.
  - Negatif: Memerlukan dependensi CLI tool `gotestsum` yang di-install saat setup runner CI.
