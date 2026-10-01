# Codebase Navigation Map

## Root Package (`github.com/PakaiWA/whatsmeow`)
- **Responsibility**: Implementasi client WhatsApp Web Multidevice API, orkestrator koneksi, pertukaran pesan E2EE, panggilan grup, newsletter, media download/upload, pairing, dan retry protocol.
- **Entry / Key Files**:
  - `client.go` — Definisi `Client` struct, loop orkestrator koneksi, event handler registration, dan lifecycle hooks.
  - `handshake.go` — WhatsApp Noise XX handshake dan autentikasi socket session.
  - `message.go` — Parsing pesan masuk, penanganan frame pesan, dan dispatch event pesan.
  - `send.go` & `sendfb.go` — Serialisasi, enkripsi libsignal (signal protocol), pengiriman pesan individual dan group (termasuk fallback).
  - `download.go` & `download-to-file.go` — Download media WhatsApp terenkripsi (CBC/GCM/HKDF).
  - `upload.go` — Upload media terenkripsi ke WhatsApp media server.
  - `group.go` — Pengelolaan grup WhatsApp (create, update, invite, participant management, ephemeral settings).
  - `newsletter.go` — Interaksi channel / saluran WhatsApp (newsletter metadata, follow, unfollow).
  - `pair.go`, `pair-code.go`, `pair-passkey.go`, `qrchan.go` — Mekanisme login (QR code scanning, phone pairing code, passkey authentication).
- **Dependencies**: `github.com/coder/websocket`, `go.mau.fi/libsignal`, `google.golang.org/protobuf`, internal sub-packages (`store`, `socket`, `binary`, `types`, `proto`, `appstate`, `util`).
- **Consumers**: Library consumers, bot applications, integrasi webhook, microservices perpesanan WhatsApp.
- **External Integrations**: WhatsApp WebSocket endpoints (`wss://web.whatsapp.com/ws/chat`), CDN media WhatsApp.
- **Key Notes**: Fork dari `tulir/whatsmeow` dengan kustomisasi identitas modul `github.com/PakaiWA/whatsmeow` dan security hardening.

---

## Sub-package: `store` & `store/sqlstore`
- **Responsibility**: Abstraksi persistence layer untuk sesi perangkat, identity keys, pre-keys, sender keys, contact lists, LID mappings, dan app state sync keys.
- **Entry / Key Files**:
  - `store/store.go` — Definisi interface `Device`, `SessionStore`, `IdentityStore`, `PreKeyStore`, `SenderKeyStore`.
  - `store/sqlstore/container.go` — Multi-session SQL database container wrapper.
  - `store/sqlstore/upgrades/` — Database migration scripts untuk SQLite dan PostgreSQL.
- **Dependencies**: `database/sql`, `go.mau.fi/util/dbutil`, driver SQLite/Postgres.
- **Consumers**: `whatsmeow.Client`, handler handshake, message decrypter/encrypter.
- **External Integrations**: SQLite / PostgreSQL database.
- **Key Notes**: Mendukung multi-device session dalam satu database tunggal.

---

## Sub-package: `socket`
- **Responsibility**: Transport layer WebSocket tingkat rendah dan Noise Protocol framework (Noise_XX_25519_AESGCM_SHA256).
- **Entry / Key Files**:
  - `socket/framesocket.go` — Pembungkusan frame raw WebSocket, buffering frame baca/tulis.
  - `socket/noisehandshake.go` — State machine Noise Handshake kriptografi.
  - `socket/noisesocket.go` — Socket terenkripsi post-handshake dengan AEAD AES-GCM cipher.
- **Dependencies**: `github.com/coder/websocket`, `golang.org/x/crypto`.
- **Consumers**: `handshake.go`, `client.go`.
- **External Integrations**: WhatsApp WebSocket gateway.
- **Key Notes**: Mengelola framing binary 3-byte length prefix.

---

## Sub-package: `binary` & `binary/token`
- **Responsibility**: Protokol serialisasi data WhatsApp proprietary WABinary (binary XML tree: tag, attributes, children/payload).
- **Entry / Key Files**:
  - `binary/node.go` — Representasi struktur `Node` (Tag, Attrs, Content).
  - `binary/encoder.go` & `binary/decoder.go` — Encoder & decoder streaming WABinary ke/dari byte slice.
  - `binary/attrs.go` — Utility getter aman untuk parsing atribut XML node.
  - `binary/token/token.go` — Tabel dictionary token kompresi string WhatsApp (SingleByteToken & DoubleByteToken).
- **Dependencies**: `github.com/PakaiWA/whatsmeow/types`.
- **Consumers**: `client.go`, `handshake.go`, `message.go`, `group.go`.
- **Key Notes**: Menghemat bandwidth koneksi dengan merepresentasikan string berulang sebagai indeks 1-2 byte token.

---

## Sub-package: `proto`
- **Responsibility**: Definisi protobuf dan Go generated code untuk payload WhatsApp (E2E Message, CompanionReg, Adv, HistorySync, SyncAction, dsb.).
- **Entry / Key Files**:
  - `proto/waE2E/WAWebProtobufsE2E.proto` & `.pb.go` — Skema pesan chat E2E WhatsApp lengkap.
  - `proto/convert.sh` — Script otomasi kompilasi protoc dan pemformatan goimports.
- **Dependencies**: `google.golang.org/protobuf`.
- **Consumers**: Seluruh enkripsi/dekripsi payload pesan dan sinkronisasi state.
- **Key Notes**: Dilengkapi hook otomatis di pre-commit untuk sinkronisasi generator protobuf.

---

## Sub-package: `appstate`
- **Responsibility**: Sinkronisasi mutasi state aplikasi WhatsApp (kontak, mute chat, pin status, archive, quick replies).
- **Entry / Key Files**:
  - `appstate/processor.go` — State machine pemroses mutasi dan patch.
  - `appstate/hash.go` & `appstate/lthash/` — Verifikasi integritas state menggunakan Homomorphic Hashing (LtHash).
- **Dependencies**: `github.com/PakaiWA/whatsmeow/store`, `github.com/PakaiWA/whatsmeow/binary`.
- **Consumers**: `client.go`.
- **Key Notes**: Mengimplementasikan rekonsiliasi state terenkripsi antar-perangkat tertaut.

---

## Sub-package: `types` & `types/events`
- **Responsibility**: Model domain kuat, JID parser/formatter (User, Group, Broadcast, Newsletter, Hidden User/LID), serta event structs.
- **Entry / Key Files**:
  - `types/jid.go` — Definisi `JID` struct dan konversi server type (`@s.whatsapp.net`, `@g.us`, `@lid`, `@newsletter`).
  - `types/events/events.go` — Event definisi yang dikirimkan ke `EventHandler` consumer (`Message`, `Receipt`, `GroupInfo`, `Connected`, `LoggedOut`).
- **Dependencies**: Tidak ada dependensi internal berat.
- **Consumers**: Seluruh codebase dan consumer library eksternal.
- **Key Notes**: Membedakan JID berbasis Phone Number (PN) dan Privacy LID.

---

## Sub-package: `util`
- **Responsibility**: Helper kriptografi dan utility internal.
- **Entry / Key Files**:
  - `util/keys/keypair.go` — Curve25519 keypair generation & serialization.
  - `util/log/log.go` — Abstraksi logger (`waLog.Logger`, zerolog adapter).
  - `util/hkdfutil/`, `util/cbcutil/`, `util/gcmutil/` — Primitif kripto untuk media & signal handshake.
- **Dependencies**: `golang.org/x/crypto`, `github.com/rs/zerolog`.
- **Consumers**: `client.go`, `download.go`, `handshake.go`.
- **Key Notes**: Modular dan independen dari logika protokol utama.
