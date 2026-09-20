# Security Policy

PakaiWA mengutamakan keamanan komunikasi data dan proteksi integritas protokol perpesanan. Kebijakan ini menjelaskan versi yang didukung serta prosedur pelaporan kerentanan keamanan (*vulnerability disclosure*).

---

## Supported Versions

Kami secara aktif memelihara dan merilis patch keamanan untuk versi-versi berikut:

| Version | Supported          | Catatan                                                       |
| :------ | :----------------- | :------------------------------------------------------------ |
| `v0.1.x`| :white_check_mark: | Versi SemVer aktif saat ini (didukung pembaruan dan patch)    |
| `main`  | :white_check_mark: | Branch utama (selalu menerima patch keamanan dan sync terbaru)|
| `< v0.1`| :x:                | Tag rilis format lama berbasis timestamp (tidak didukung)     |

Pengguna dan layanan internal sangat disarankan untuk selalu menggunakan rilis SemVer terbaru atau mengikuti branch `main`.

---

## Scope & Security Boundary

### Ruang Lingkup Keamanan:
- **Kriptografi & Signal Protocol**: Implementasi Double Ratchet, Sender Keys, derivasi kunci Noise Handshake, CBC, dan AES-GCM.
- **Session & Key Storage**: Proteksi kunci privat sesi, token autentikasi, identitas LID, dan kerahasiaan database SQLStore.
- **Memory Safety & Injection**: Buffer overflow/underflow, SQL injection pada driver SQLStore, desensitisasi kredensial pada logging Zerolog.
- **Remote Code Execution (RCE) / Denial of Service (DoS)**: Parsing malformed WABinary, Protobuf decoding crash, dan manipulasi socket frame.

### Di Luar Ruang Lingkup Keamanan:
- Penangguhan atau pemblokiran nomor akun oleh server WhatsApp akibat pelanggaran ketentuan layanan (*ToS abuse* atau *rate limit* WhatsApp).
- Kebocoran kredensial akibat kelalaian pada konfigurasi server/database pengguna sendiri.

---

## Reporting a Vulnerability

Jika Anda menemukan potensi kerentanan keamanan atau celah privasi:

> [!IMPORTANT]
> **JANGAN MEMBUAT PUBLIC ISSUE ATAU PULL REQUEST TERBUKA** untuk melaporkan celah keamanan.

Silakan laporkan kerentanan melalui salah satu saluran rahasia berikut:

1. **GitHub Private Vulnerability Reporting (Disarankan)**:
   - Kunjungi tab [Security Advisories](https://github.com/PakaiWA/whatsmeow/security/advisories/new) pada repositori ini.
   - Klik **"Report a vulnerability"** untuk membuka laporan tertutup langsung ke maintainer.

2. **Email Tim Keamanan**:
   - Kirimkan detail temuan ke `security@pakaiwa.com` atau kontak maintainer terkait.

### Informasi yang Dibutuhkan dalam Laporan:
- Ringkasan kerentanan dan potensi dampak (*impact*).
- Langkah-langkah reproduksi (*Proof of Concept* / PoC) atau contoh pesan/kode pemicu.
- Versi pustaka atau commit hash yang terpengaruh.
- Rekomendasi mitigasi atau patch (jika ada).

---

## Response Process & SLA

- **Konfirmasi Awal**: Tim maintainer akan mengonfirmasi penerimaan laporan dalam kurun waktu **1x24 jam** hingga maksimal **48 jam**.
- **Investigasi & Validasi**: Evaluasi tingkat keparahan (CVSS) dan penyusunan mitigasi dilakukan dalam **3–5 hari kerja**.
- **Penyelesaian & Rilis Patch**: Setelah perbaikan teruji, patch keamanan akan dirilis ke branch `main` dan versi rilis SemVer baru dibuat melalui automated GitHub Release.
- **Pengungkapan Publik (*Coordinated Disclosure*)**: Publikasi detail kerentanan (CVE / Advisory) akan dikoordinasikan bersama pelapor setelah perbaikan tersedia bagi seluruh pengguna.
