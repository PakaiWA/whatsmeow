# Project Context

## Project Purpose
`github.com/PakaiWA/whatsmeow` adalah library inti Go untuk protokol WhatsApp Web Multidevice yang di-fork dan dimodifikasi khusus untuk memenuhi kebutuhan integrasi internal pada arsitektur platform microservice **PakaiWA**. Library ini menyediakan konektivitas WebSocket persisten, enkripsi/dekripsi Signal Protocol end-to-end (E2EE), manajemen sesi, transfer media terenkripsi, serta event dispatching perpesanan WhatsApp.

---

## System Boundary
- **Dalam Ruang Lingkup Sistem**:
  - Negosiasi koneksi WebSocket terenkripsi Noise Protocol ke WhatsApp server gateway (`wss://web.whatsapp.com/ws/chat`).
  - Implementasi Signal Protocol untuk E2EE enkripsi pesan chat 1-on-1 dan group chat (Sender Keys).
  - Serialisasi/deserialisasi payload WABinary (binary XML) dan Protobuf WhatsApp.
  - Penyimpanan sesi (keys, tokens, LID mapping, devices) ke persistent storage SQL (PostgreSQL di produksi).
  - Mekanisme autentikasi login (QR code, phone pairing code, passkey).
- **Di Luar Ruang Lingkup Sistem**:
  - Business logic aplikasi akhir (bot automation, webhook router, CRM dispatch, message template parser).
  - WhatsApp VoIP calling streaming (hanya parsing event notifikasi call).
  - Broadcast list messages (tidak didukung oleh WhatsApp Web protocol itu sendiri).

---

## Main Actors
- **Internal Microservice Workers (PakaiWA)**: Konsumen utama library yang memanggil method client untuk mengirim/menerima pesan dan mendengarkan events.
- **WhatsApp Web Gateway**: Server eksternal WhatsApp yang menerima koneksi WebSocket dan merutekan frame data.
- **Database Administrator / Ops**: Mengelola cluster PostgreSQL untuk persistence session credentials.
- **Developer / Maintainer**: Menjaga integritas fork, menyelaraskan update saat ada protokol breaking change dari upstream.

---

## Important Domain Concepts & Glossary
- **JID (Jabber ID)**: Pengenal unik pada ekosistem WhatsApp:
  - Phone Number JID: `<phone>@s.whatsapp.net`
  - Privacy LID (Local Identity): `<lid>@lid` (identitas privasi berbasis akun WhatsApp baru)
  - Group JID: `<group_id>@g.us`
  - Newsletter JID: `<channel_id>@newsletter`
- **Signal Protocol (libsignal)**: Standar kriptografi Double Ratchet, Pre-Keys, dan Axolotl sender keys untuk keamanan percakapan E2EE.
- **WABinary**: Format serialisasi biner proprietary WhatsApp yang memetakan string umum ke tabel token 1-2 byte untuk kompresi data jaringan.
- **Noise Handshake**: Protokol pertukaran kunci kriptografi awal (`Noise_XX_25519_AESGCM_SHA256`) sebelum session socket diamankan dengan cipher AES-GCM.
- **App State & LtHash**: Sinkronisasi terenkripsi riwayat kontak, setting chat (archive, mute, pin) menggunakan Homomorphic Hashing.

---

## External Systems
- **WhatsApp Web Gateway**: `wss://web.whatsapp.com/ws/chat`
- **WhatsApp Media CDN**: Endpoint download/upload file media WhatsApp terenkripsi (gambar, audio, dokumen, video).
- **PostgreSQL**: Database session penyimpanan utama di lingkungan produksi.
- **Upstream Repository**: `github.com/tulir/whatsmeow` (sumber update upstream).

---

## Runtime Environment & Constraints
- **Runtime**: Go 1.26+ (direkomendasikan Go 1.27).
- **Architecture**: Darwin / Linux container (amd64 / arm64).
- **Concurrency Model**: Goroutine-heavy, event-driven streaming sockets dengan channel buffering.
- **Persistent Storage**: PostgreSQL (wajib connection pooling yang dikonfigurasi dengan baik di microservices).

---

## Coding Conventions & Standards
- **Module Path**: Wajib menggunakan `github.com/PakaiWA/whatsmeow`.
- **Pre-commit Pipeline**:
  - `proto-convert`: Regenerasi `.pb.go` dari file `.proto` di dalam `proto/`.
  - `go-imports-repo`: Pengurutan import otomatis dengan `-local github.com/PakaiWA/whatsmeow -w`.
  - `staticcheck` & `go-vet`: Pengecekan kualitas kode dan static analysis.
- **Device Fingerprint**: Metadata `DeviceProps OS` wajib menggunakan identitas platform `PakaiWA`.

---

## Known Limitations
- WhatsApp Web protocol memiliki batasan bawaan seperti ketidakmampuan mengirim pesan broadcast list.
- Penyelarasan Phone Number (PN) ke LID masih terus berkembang mengikuti adaptasi WhatsApp Web server.
