# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/lang/id/).


## [v0.26.9-131114](https://github.com/PakaiWA/whatsmeow/tree/v0.26.9-131114) - 2026-09-13 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.9-131101...v0.26.9-131114)

### Summary
- **CI/CD Optimization**: Memperbaiki pembersihan remote tags pada hari yang sama sebelum tag baru dibuat untuk mencegah tabrakan rilis harian.
- **Dependency Upgrades**: Bump `golang.org/x/crypto` (0.56.0 -> 0.57.0), `golang.org/x/net` (0.58.0 -> 0.59.0), dan `golang.org/x/sync` (0.22.0 -> 0.23.0) beserta pembersihan checksum di `go.sum`.
- **Diff Stats**: 3 files diubah, +32 insertions, -21 deletions.

<details><summary><b>Commit Log (3 commits)</b></summary>

- ci(github): delete previous same-day tags before pushing new tag ([`8685972`](https://github.com/PakaiWA/whatsmeow/commit/8685972))
- build(deps): bump golang.org/x/{crypto,net} and tidy checksums ([`dce9cd2`](https://github.com/PakaiWA/whatsmeow/commit/dce9cd2))
- build(deps): bump golang.org/x/sync from 0.22.0 to 0.23.0 ([`a7f02cf`](https://github.com/PakaiWA/whatsmeow/commit/a7f02cf))

</details>


---

## [v0.26.9-131101](https://github.com/PakaiWA/whatsmeow/tree/v0.26.9-131101) - 2026-09-13 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.9-71253...v0.26.9-131101)

### Summary
- **Upstream Sync & Protocol Updates**: Sinkronisasi perubahan dari `tulir/whatsmeow`.
- **Protobuf Update**: Pembaruan definisi protobuf ke versi v1047068806, menambahkan dukungan Labyrinth WebAssembly (`WAWebLabyrinthWaWasm`), virtual device ID derivation, dan token rotasi.
- **Send & Messaging Enhancements**: `SendResponse` kini menyertakan `Chat` JID tujuan secara eksplisit; `GetQRChannel` mendukung penanganan rotasi rahasia ADV (`RotateADVSecret`).
- **Group API**: Metode topic ganda didepresiasi.
- **Companion Registration**: Menambahkan dukungan awal untuk `companion_reg_refresh`.
- **Diff Stats**: 15 files diubah, +1,272 insertions, -533 deletions.

<details><summary><b>Commit Log (4 commits)</b></summary>

- send: add chat JID in SendResponse (#1258) ([`b25a56d`](https://github.com/PakaiWA/whatsmeow/commit/b25a56d))
- pair: add untested support for companion_reg_refresh ([`9ec8f76`](https://github.com/PakaiWA/whatsmeow/commit/9ec8f76))
- proto: update to v1047068806 ([`30593f2`](https://github.com/PakaiWA/whatsmeow/commit/30593f2))
- group: deprecate duplicate topic method ([`57796d3`](https://github.com/PakaiWA/whatsmeow/commit/57796d3))

</details>


---

## [v0.26.9-71253](https://github.com/PakaiWA/whatsmeow/tree/v0.26.9-71253) - 2026-09-07 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.9-71205...v0.26.9-71253)

### Summary
- **Release Metadata Tag**: Tag pembaruan workflow/metadata rilis identik dengan basis `v0.26.9-71205` tanpa perubahan kode tambahan.

<details><summary><b>Commit Log (2 commits)</b></summary>

- revert(store): restore database table names to whatsmeow prefix ([`3bb7b98`](https://github.com/PakaiWA/whatsmeow/commit/3bb7b98))
- refactor(store): rename sqlstore tables and test package to pakaiwa ([`c38252b`](https://github.com/PakaiWA/whatsmeow/commit/c38252b))

</details>


---

## [v0.26.9-71205](https://github.com/PakaiWA/whatsmeow/tree/v0.26.9-71205) - 2026-09-07 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.9-71158...v0.26.9-71205)

### Summary
- **CI Workflow Tuning**: Menghapus step redundant pada build workflow `.github/workflows/go.yml`.
- **Diff Stats**: 1 file diubah, +1 insertion, -6 deletions.

<details><summary><b>Commit Log (1 commits)</b></summary>

- ci(github): remove protoc setup and bump cache action version ([`e6798eb`](https://github.com/PakaiWA/whatsmeow/commit/e6798eb))

</details>


---

## [v0.26.9-71158](https://github.com/PakaiWA/whatsmeow/tree/v0.26.9-71158) - 2026-09-07 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-110917...v0.26.9-71158)

### Summary
- **Upstream Sync**: Sinkronisasi masif upstream ke versi WhatsApp Web v1046691727 & v1045649367.
- **Architecture & Project Context**: Penambahan dokumentasi bootstrap arsitektur repositori (`ARCHITECTURE.md`, `CODEBASE_MAP.md`, `DECISIONS.md`, `PROJECT_CONTEXT.md`, `TODO.md`).
- **Protobuf Automation Tooling**: Pengoptimalan `proto/convert.sh` dengan flag strict bash (`set -euo pipefail`), pengecekan dependency tools (`protoc`, `goimports`), dan kemampuan eksekusi dari direktori mana pun.
- **CodeQL & CI Pipeline**: Integrasi concurrency grouping, path filtering, tool caching (`goimports`, `staticcheck`, `protoc-gen-go-grpc`).
- **Client & Noise Protocol**: Perbaikan penanganan stream error sebelum proses reconnect, perbaikan context frame socket Noise, dan proteksi decoupling handler queue antar koneksi.
- **Diff Stats**: 95 files diubah, +2,675 insertions, -1,314 deletions.

<details><summary><b>Commit Log (31 commits)</b></summary>

- ci: remove pre-commit action and cache from build workflow ([`fff9eef`](https://github.com/PakaiWA/whatsmeow/commit/fff9eef))
- chore: remove proto-convert from pre-commit config ([`1fc9ee0`](https://github.com/PakaiWA/whatsmeow/commit/1fc9ee0))
- ci: install protoc-gen-go-grpc and make grpc flag conditional in convert.sh ([`8c4fcff`](https://github.com/PakaiWA/whatsmeow/commit/8c4fcff))
- ci: optimize tagging job with shallow checkout and atomic remote tag check ([`3b4d4e6`](https://github.com/PakaiWA/whatsmeow/commit/3b4d4e6))
- build(deps): bump golang.org/x/crypto to 0.56.0 and update dependencies ([`b9c1a7f`](https://github.com/PakaiWA/whatsmeow/commit/b9c1a7f))
- ci: optimize codeql workflow with concurrency, path filters, and go setup caching ([`f8841a0`](https://github.com/PakaiWA/whatsmeow/commit/f8841a0))
- ci: streamline workflow steps and guard branch executions ([`37daf5b`](https://github.com/PakaiWA/whatsmeow/commit/37daf5b))
- ci: optimize workflow with caching for protoc, tools, and pre-commit ([`ef04089`](https://github.com/PakaiWA/whatsmeow/commit/ef04089))
- ci: install protobuf-compiler and protoc-gen-go in build workflow ([`5414a15`](https://github.com/PakaiWA/whatsmeow/commit/5414a15))
- docs: bootstrap project memory and architecture documentation ([`173359c`](https://github.com/PakaiWA/whatsmeow/commit/173359c))
- refactor(proto): optimize convert.sh with strict bash flags and dependency checks ([`ea66a42`](https://github.com/PakaiWA/whatsmeow/commit/ea66a42))
- ci: add proto-convert hook to pre-commit config ([`41859c1`](https://github.com/PakaiWA/whatsmeow/commit/41859c1))
- fix(proto): allow convert.sh to be executed from any working directory ([`ad8ce23`](https://github.com/PakaiWA/whatsmeow/commit/ad8ce23))
- sync/upstream-20260906 ([`e87f6e0`](https://github.com/PakaiWA/whatsmeow/commit/e87f6e0))
- client: ensure stream error is handled before reconnecting ([`28bfe53`](https://github.com/PakaiWA/whatsmeow/commit/28bfe53))
- client: guess correct own ID in ParseWebMessage ([`0ca8346`](https://github.com/PakaiWA/whatsmeow/commit/0ca8346))
- group: make delete reason optional ([`bdd4e83`](https://github.com/PakaiWA/whatsmeow/commit/bdd4e83))
- proto: update to v1046691727 ([`de26b4a`](https://github.com/PakaiWA/whatsmeow/commit/de26b4a))
- download: also ignore unencrypted media keys for thumbnails ([`1f6240e`](https://github.com/PakaiWA/whatsmeow/commit/1f6240e))
- client,socket: use correct context for noise socket frames ([`0fadda7`](https://github.com/PakaiWA/whatsmeow/commit/0fadda7))
- ci: enable staticcheck ([`33cfac5`](https://github.com/PakaiWA/whatsmeow/commit/33cfac5))
- ci: disable goimports on go 1.26 ([`d1cc3c0`](https://github.com/PakaiWA/whatsmeow/commit/d1cc3c0))
- dependencies: bump minimum Go version to 1.26 ([`4650ea9`](https://github.com/PakaiWA/whatsmeow/commit/4650ea9))
- download: ignore unencrypted media keys ([`72f22e6`](https://github.com/PakaiWA/whatsmeow/commit/72f22e6))
- client: don't reuse handler queue between connections ([`4fa3462`](https://github.com/PakaiWA/whatsmeow/commit/4fa3462))
- proto: update to v1045649367 ([`6eefbff`](https://github.com/PakaiWA/whatsmeow/commit/6eefbff))
- dependencies: update ([`fb386f1`](https://github.com/PakaiWA/whatsmeow/commit/fb386f1))
- proto: update to v1045305987 ([`b06ae6e`](https://github.com/PakaiWA/whatsmeow/commit/b06ae6e))
- group: remove create_key ([`0dcf1f5`](https://github.com/PakaiWA/whatsmeow/commit/0dcf1f5))
- user: update imports ([`197e617`](https://github.com/PakaiWA/whatsmeow/commit/197e617))
- user: switch UpdateBlocklist to use LIDs (#1137) ([`8d023aa`](https://github.com/PakaiWA/whatsmeow/commit/8d023aa))

</details>


---

## [v0.26.8-110917](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-110917) - 2026-08-11 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-100916...v0.26.8-110917)

### Summary
- **Protobuf v1044834443**: Pembaruan masif protobuf `WAWebProtobufsE2E` (+1,900 baris) dan `WAWebProtobufsDeviceCapabilities`.
- **Sync Actions**: Penambahan field baru pada protokol sinkronisasi `WAWebProtobufSyncAction`.
- **Diff Stats**: 11 files diubah, +2,509 insertions, -2,148 deletions.

<details><summary><b>Commit Log (1 commits)</b></summary>

- proto: update to v1044834443 ([`a23afe3`](https://github.com/PakaiWA/whatsmeow/commit/a23afe3))

</details>


---

## [v0.26.8-100916](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-100916) - 2026-08-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-100741...v0.26.8-100916)

### Summary
- **Security & Dependabot**: Penambahan konfigurasi Dependabot `.github/dependabot.yml` untuk GitHub Actions dan Go modules.
- **Client OS Identifier**: Mengubah `DeviceProps` OS menjadi `PakaiWA`.
- **Crypto CBC Safety**: Penanganan bounds checking pada validasi padding `cbcutil/cbc.go` untuk mencegah potensi integer overflow/out-of-bounds.
- **Diff Stats**: 3 files diubah, +23 insertions, -7 deletions.

<details><summary><b>Commit Log (4 commits)</b></summary>

- Create dependabot.yml ([`f813c30`](https://github.com/PakaiWA/whatsmeow/commit/f813c30))
- chore: update DeviceProps OS to PakaiWA ([`62d3c0f`](https://github.com/PakaiWA/whatsmeow/commit/62d3c0f))
- Potential fix for code scanning alert no. 34: Size computation for allocation may overflow ([`ecd32cb`](https://github.com/PakaiWA/whatsmeow/commit/ecd32cb))
- Potential fix for code scanning alert no. 33: Size computation for allocation may overflow ([`f4e54e6`](https://github.com/PakaiWA/whatsmeow/commit/f4e54e6))

</details>


---

## [v0.26.8-100741](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-100741) - 2026-08-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-100739...v0.26.8-100741)

### Summary
- **Tag Rilis Metadata**: Tag rilis cepat untuk memastikan pipeline build terpicu.


*No code commits between tags.*


---

## [v0.26.8-100739](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-100739) - 2026-08-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-100735...v0.26.8-100739)

### Summary
- **Crypto Fix**: Penyempurnaan pemeriksaan panjang blok ciphertext pada `util/cbcutil/cbc.go`.
- **Diff Stats**: 1 file diubah, +6 insertions.

<details><summary><b>Commit Log (1 commits)</b></summary>

- Potential fix for code scanning alert no. 33: Size computation for allocation may overflow ([`d5f7c8a`](https://github.com/PakaiWA/whatsmeow/commit/d5f7c8a))

</details>


---

## [v0.26.8-100735](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-100735) - 2026-08-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-100718...v0.26.8-100735)

### Summary
- **Crypto Fix**: Koreksi slice index boundary di `util/cbcutil/cbc.go`.
- **Diff Stats**: 1 file diubah, +4 insertions, -4 deletions.

<details><summary><b>Commit Log (1 commits)</b></summary>

- Potential fix for code scanning alert no. 32: Size computation for allocation may overflow ([`b64acf8`](https://github.com/PakaiWA/whatsmeow/commit/b64acf8))

</details>


---

## [v0.26.8-100718](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-100718) - 2026-08-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.8-92307...v0.26.8-100718)

### Summary
- **Crypto Fix**: Penyesuaian pemeriksaan padding di `util/cbcutil/cbc.go`.
- **Diff Stats**: 1 file diubah, +1 insertion, -1 deletion.

<details><summary><b>Commit Log (1 commits)</b></summary>

- Potential fix for code scanning alert no. 22: Size computation for allocation may overflow ([`179c021`](https://github.com/PakaiWA/whatsmeow/commit/179c021))

</details>


---

## [v0.26.8-92307](https://github.com/PakaiWA/whatsmeow/tree/v0.26.8-92307) - 2026-08-09 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.4-101927...v0.26.8-92307)

### Summary
- **Passkey Support**: Penambahan implementasi tipe autentikasi passkey di `types/passkey.go`.
- **Sticker Pack Metadata**: Penambahan modul dan parsing metadata stiker WhatsApp di `types/sticker.go`.
- **User Blocklist via LID**: Pembaruan `UpdateBlocklist` untuk mendukung LID (Linked IDs) disamping Phone Number.
- **Noise Handshake Optimization**: Peningkatan keandalan handshake Noise protocol dan logging terstruktur Zerolog.
- **Upstream Sync**: Sinkronisasi berkala dari upstream `tulir/whatsmeow`.
- **Diff Stats**: 141 files diubah, +14,029 insertions, -4,871 deletions.

<details><summary><b>Commit Log (136 commits)</b></summary>

- remove duplicate codwQL ([`f328b4a`](https://github.com/PakaiWA/whatsmeow/commit/f328b4a))
- Potential fix for code scanning alert no. 28: Size computation for allocation may overflow ([`a12a4d1`](https://github.com/PakaiWA/whatsmeow/commit/a12a4d1))
- remove duplicate codwQL ([`18263e4`](https://github.com/PakaiWA/whatsmeow/commit/18263e4))
- Potential fix for code scanning alert no. 28: Size computation for allocation may overflow ([`cb213d2`](https://github.com/PakaiWA/whatsmeow/commit/cb213d2))
- Potential fix for code scanning alert no. 27: Incorrect conversion between integer types ([`8cf1ab3`](https://github.com/PakaiWA/whatsmeow/commit/8cf1ab3))
- Potential fix for code scanning alert no. 26: Incorrect conversion between integer types ([`1118aa1`](https://github.com/PakaiWA/whatsmeow/commit/1118aa1))
- Potential fix for code scanning alert no. 24: Size computation for allocation may overflow ([`8af8a0d`](https://github.com/PakaiWA/whatsmeow/commit/8af8a0d))
- Potential fix for code scanning alert no. 23: Size computation for allocation may overflow ([`b35d90a`](https://github.com/PakaiWA/whatsmeow/commit/b35d90a))
- Potential fix for code scanning alert no. 22: Size computation for allocation may overflow ([`810c7ca`](https://github.com/PakaiWA/whatsmeow/commit/810c7ca))
- Potential fix for code scanning alert no. 20: Size computation for allocation may overflow ([`ea80c56`](https://github.com/PakaiWA/whatsmeow/commit/ea80c56))
- Potential fix for code scanning alert no. 17: Incorrect conversion between integer types ([`cd98601`](https://github.com/PakaiWA/whatsmeow/commit/cd98601))
- Potential fix for code scanning alert no. 3: Size computation for allocation may overflow ([`6796e51`](https://github.com/PakaiWA/whatsmeow/commit/6796e51))
- Update CodeQL workflow schedule and checkout action version ([`5da98c0`](https://github.com/PakaiWA/whatsmeow/commit/5da98c0))
- Potential fix for code scanning alert no. 16: Incorrect conversion between integer types ([`a5b660a`](https://github.com/PakaiWA/whatsmeow/commit/a5b660a))
- Potential fix for code scanning alert no. 15: Incorrect conversion between integer types ([`558ea7a`](https://github.com/PakaiWA/whatsmeow/commit/558ea7a))
- Potential fix for code scanning alert no. 14: Incorrect conversion between integer types ([`fc49b08`](https://github.com/PakaiWA/whatsmeow/commit/fc49b08))
- Potential fix for code scanning alert no. 13: Incorrect conversion between integer types ([`d97e7d9`](https://github.com/PakaiWA/whatsmeow/commit/d97e7d9))
- Potential fix for code scanning alert no. 12: Incorrect conversion between integer types ([`ce2e259`](https://github.com/PakaiWA/whatsmeow/commit/ce2e259))
- Potential fix for code scanning alert no. 11: Incorrect conversion between integer types ([`3d857d4`](https://github.com/PakaiWA/whatsmeow/commit/3d857d4))
- Potential fix for code scanning alert no. 10: Incorrect conversion between integer types ([`3f5e40c`](https://github.com/PakaiWA/whatsmeow/commit/3f5e40c))
- Potential fix for code scanning alert no. 9: Incorrect conversion between integer types ([`8f6843e`](https://github.com/PakaiWA/whatsmeow/commit/8f6843e))
- Potential fix for code scanning alert no. 8: Incorrect conversion between integer types ([`4b43fbf`](https://github.com/PakaiWA/whatsmeow/commit/4b43fbf))
- Potential fix for pull request finding 'CodeQL / Size computation for allocation may overflow' ([`a5c38bf`](https://github.com/PakaiWA/whatsmeow/commit/a5c38bf))
- Potential fix for pull request finding 'CodeQL / Size computation for allocation may overflow' ([`5a23773`](https://github.com/PakaiWA/whatsmeow/commit/5a23773))
- Potential fix for code scanning alert no. 7: Incorrect conversion between integer types ([`efbefdc`](https://github.com/PakaiWA/whatsmeow/commit/efbefdc))
- Potential fix for code scanning alert no. 6: Size computation for allocation may overflow ([`b6794b5`](https://github.com/PakaiWA/whatsmeow/commit/b6794b5))
- Potential fix for code scanning alert no. 5: Size computation for allocation may overflow ([`26c96f7`](https://github.com/PakaiWA/whatsmeow/commit/26c96f7))
- Potential fix for code scanning alert no. 4: Size computation for allocation may overflow ([`23847e3`](https://github.com/PakaiWA/whatsmeow/commit/23847e3))
- Potential fix for code scanning alert no. 2: Size computation for allocation may overflow ([`c418d5e`](https://github.com/PakaiWA/whatsmeow/commit/c418d5e))
- ci(github): bump peter-evans/create-pull-request from v6 to v8.1.1 in go.yml ([`48914fa`](https://github.com/PakaiWA/whatsmeow/commit/48914fa))
- ci(github): bump actions/checkout from v4 to v7.0.1 in workflow files ([`9b1446a`](https://github.com/PakaiWA/whatsmeow/commit/9b1446a))
- style(proto): format generated protobuf files with goimports ([`e0cd794`](https://github.com/PakaiWA/whatsmeow/commit/e0cd794))
- chore: clean up unused entries in go.sum ([`0b14e8f`](https://github.com/PakaiWA/whatsmeow/commit/0b14e8f))
- chore(deps): update indirect dependencies in go.mod and go.sum ([`5efb96e`](https://github.com/PakaiWA/whatsmeow/commit/5efb96e))
- style: fix formatting in client_test.go and trailing newline in WAWebProtobufsAICommon.proto ([`57fefb0`](https://github.com/PakaiWA/whatsmeow/commit/57fefb0))
- fix(proto): resolve syntax error in WAWebProtobufsAICommon.proto and regenerate pb.go files ([`dd84120`](https://github.com/PakaiWA/whatsmeow/commit/dd84120))
- group: fix setting member_add_mode in CreateGroup (#1220) ([`e277b76`](https://github.com/PakaiWA/whatsmeow/commit/e277b76))
- group: update group creation parameters ([`39b719b`](https://github.com/PakaiWA/whatsmeow/commit/39b719b))
- user: switch SetStatusMessage to use new mex query ([`662b012`](https://github.com/PakaiWA/whatsmeow/commit/662b012))
- message: prefer storing history sync privacy tokens under LIDs ([`ad9cc5c`](https://github.com/PakaiWA/whatsmeow/commit/ad9cc5c))
- client: allow configuring socket headers ([`07e92ec`](https://github.com/PakaiWA/whatsmeow/commit/07e92ec))
- historysync,store: save companion meta nonce and send it when deleting history syncs ([`6723e55`](https://github.com/PakaiWA/whatsmeow/commit/6723e55))
- sqlstore/lidmap: keep device ID when falling back to the database in GetManyLIDsForPNs (#1216) ([`098bcf7`](https://github.com/PakaiWA/whatsmeow/commit/098bcf7))
- presence: remove from attribute in chat state sending ([`1494ba7`](https://github.com/PakaiWA/whatsmeow/commit/1494ba7))
- proto: update to v1044440921 ([`e229058`](https://github.com/PakaiWA/whatsmeow/commit/e229058))
- business: add support for fetching order details (#967) ([`f547f09`](https://github.com/PakaiWA/whatsmeow/commit/f547f09))
- proto: update to v1044142122 ([`662ad1d`](https://github.com/PakaiWA/whatsmeow/commit/662ad1d))
- proto: update to v1043636855 ([`e9a033b`](https://github.com/PakaiWA/whatsmeow/commit/e9a033b))
- types/jid: add IsZero method and match behavior in String ([`8e740c1`](https://github.com/PakaiWA/whatsmeow/commit/8e740c1))
- receipt: remove unused context parameter ([`d7b3e15`](https://github.com/PakaiWA/whatsmeow/commit/d7b3e15))
- events: add original notification to history sync downloaded events ([`9ec44c3`](https://github.com/PakaiWA/whatsmeow/commit/9ec44c3))
- group: include phone number for lid group creation ([`8b4a8ba`](https://github.com/PakaiWA/whatsmeow/commit/8b4a8ba))
- group: fix parsing addressing mode for JoinedGroup event ([`1142895`](https://github.com/PakaiWA/whatsmeow/commit/1142895))
- receipt: only ack retry receipts after handling ([`bbe00cd`](https://github.com/PakaiWA/whatsmeow/commit/bbe00cd))
- client: handle <status> nodes ([`a238105`](https://github.com/PakaiWA/whatsmeow/commit/a238105))
- proto: update to v1043423438 ([`fac667d`](https://github.com/PakaiWA/whatsmeow/commit/fac667d))
- dependencies: update go-util ([`dcfd3eb`](https://github.com/PakaiWA/whatsmeow/commit/dcfd3eb))
- dependencies: update ([`85d9908`](https://github.com/PakaiWA/whatsmeow/commit/85d9908))
- user: fix parsing not on whatsapp responses ([`d8960d9`](https://github.com/PakaiWA/whatsmeow/commit/d8960d9))
- call: remove tctoken in reject requests ([`592721a`](https://github.com/PakaiWA/whatsmeow/commit/592721a))
- pair: parse client props ([`37a86e6`](https://github.com/PakaiWA/whatsmeow/commit/37a86e6))
- user: update IsOnWhatsApp query ([`6e4c341`](https://github.com/PakaiWA/whatsmeow/commit/6e4c341))
- send: always use LID for DMs ([`4f8f64e`](https://github.com/PakaiWA/whatsmeow/commit/4f8f64e))
- proto: update to v1043045417 ([`ebde598`](https://github.com/PakaiWA/whatsmeow/commit/ebde598))
- msgsecret: add log to see if orig sender hack is used ([`73fe735`](https://github.com/PakaiWA/whatsmeow/commit/73fe735))
- pair: add support for passkeys ([`b572e5b`](https://github.com/PakaiWA/whatsmeow/commit/b572e5b))
- proto: update to v1042386815 ([`37913c8`](https://github.com/PakaiWA/whatsmeow/commit/37913c8))
- client: ignore connect success before pairing is completed ([`5f04eac`](https://github.com/PakaiWA/whatsmeow/commit/5f04eac))
- tctoken: return error in ensureTCToken if not logged in ([`c50f490`](https://github.com/PakaiWA/whatsmeow/commit/c50f490))
- proto: update to v1041871181 ([`e94c58a`](https://github.com/PakaiWA/whatsmeow/commit/e94c58a))
- notification: handle timelock mex notifications ([`c833019`](https://github.com/PakaiWA/whatsmeow/commit/c833019))
- call,group,user: add tctokens to more requests ([`00ce57f`](https://github.com/PakaiWA/whatsmeow/commit/00ce57f))
- cstoken: remove unnecessary checks ([`5eeb02b`](https://github.com/PakaiWA/whatsmeow/commit/5eeb02b))
- dependencies: update ([`eaa388b`](https://github.com/PakaiWA/whatsmeow/commit/eaa388b))
- docs: update sqlstore.New example to include context parameter (#1133) ([`0899323`](https://github.com/PakaiWA/whatsmeow/commit/0899323))
- all: replace interface{} with any ([`ae27897`](https://github.com/PakaiWA/whatsmeow/commit/ae27897))
- binary/xml: replace XMLString with standard String method ([`0ff5082`](https://github.com/PakaiWA/whatsmeow/commit/0ff5082))
- send: ignore own device in all cases ([`3706a7d`](https://github.com/PakaiWA/whatsmeow/commit/3706a7d))
- client: fix parsing own web message info in LID chats ([`4e62216`](https://github.com/PakaiWA/whatsmeow/commit/4e62216))
- sqlstore: close container in New if upgrade fails ([`88b76e5`](https://github.com/PakaiWA/whatsmeow/commit/88b76e5))
- proto: update to v1040847988 ([`c6a4b70`](https://github.com/PakaiWA/whatsmeow/commit/c6a4b70))
- socket: use more atomic in FrameSocket ([`b10b707`](https://github.com/PakaiWA/whatsmeow/commit/b10b707))
- download: remove file length checks and always check plaintext hash ([`d707fc2`](https://github.com/PakaiWA/whatsmeow/commit/d707fc2))
- download: add flag to indicate whether hash should be required ([`52afebe`](https://github.com/PakaiWA/whatsmeow/commit/52afebe))
- group: make group creation time optional (#1136) ([`db52b2c`](https://github.com/PakaiWA/whatsmeow/commit/db52b2c))
- qrchan: update to atomic bool and listen for all disconnects ([`256f4d7`](https://github.com/PakaiWA/whatsmeow/commit/256f4d7))
- sqlstore: switch to rowiters for scanning many rows ([`4216bd2`](https://github.com/PakaiWA/whatsmeow/commit/4216bd2))
- download: check unencrypted file checksums ([`6a7ac99`](https://github.com/PakaiWA/whatsmeow/commit/6a7ac99))
- message: handle historical message secrets and privacy tokens in LID chats ([`595ceb0`](https://github.com/PakaiWA/whatsmeow/commit/595ceb0))
- download: remove support for legacy urls ([`9ff5508`](https://github.com/PakaiWA/whatsmeow/commit/9ff5508))
- message: ensure all nodes get acked ([`a7ea563`](https://github.com/PakaiWA/whatsmeow/commit/a7ea563))
- client: close node handler channel instead of sending struct ([`c404e9a`](https://github.com/PakaiWA/whatsmeow/commit/c404e9a))
- proto: update to v1040390703 ([`f87a9c5`](https://github.com/PakaiWA/whatsmeow/commit/f87a9c5))
- dependencies: update ([`fb137f8`](https://github.com/PakaiWA/whatsmeow/commit/fb137f8))
- sqlstore/lidmap: delete old map entries when LID or phone number changes ([`563bcaa`](https://github.com/PakaiWA/whatsmeow/commit/563bcaa))
- update: bump user agent for fetching latest version ([`933deb5`](https://github.com/PakaiWA/whatsmeow/commit/933deb5))
- proto: update to v1040098269 ([`3e7fbac`](https://github.com/PakaiWA/whatsmeow/commit/3e7fbac))
- user,group,newsletter: strip query params when parsing links ([`7c10721`](https://github.com/PakaiWA/whatsmeow/commit/7c10721))
- dependencies: update ([`8d37001`](https://github.com/PakaiWA/whatsmeow/commit/8d37001))
- proto: update to v1039406452 ([`c551a40`](https://github.com/PakaiWA/whatsmeow/commit/c551a40))
- msgsecret: include params when failing to decrypt ([`9b5c655`](https://github.com/PakaiWA/whatsmeow/commit/9b5c655))
- pair-code: fix platform ID type ([`eb05d94`](https://github.com/PakaiWA/whatsmeow/commit/eb05d94))
- pair: add macOS pair client constant ([`81f8702`](https://github.com/PakaiWA/whatsmeow/commit/81f8702))
- pair: update PairClientType to a string ([`59cfb6c`](https://github.com/PakaiWA/whatsmeow/commit/59cfb6c))
- pair: update QR code format ([`876de1e`](https://github.com/PakaiWA/whatsmeow/commit/876de1e))
- notification: fix handling own device list changes ([`3b5b4fe`](https://github.com/PakaiWA/whatsmeow/commit/3b5b4fe))
- types/sticker: use pointer for sticker pack items ([`6a7198d`](https://github.com/PakaiWA/whatsmeow/commit/6a7198d))
- store/clientpayload: remove redundant init function ([`8d5b5f4`](https://github.com/PakaiWA/whatsmeow/commit/8d5b5f4))
- store/clientpayload: fix SetWAVersion ([`a763037`](https://github.com/PakaiWA/whatsmeow/commit/a763037))
- proto: update to v1038839325 ([`03911c6`](https://github.com/PakaiWA/whatsmeow/commit/03911c6))
- msgsecret: fix incorrect variable ([`1c97bf4`](https://github.com/PakaiWA/whatsmeow/commit/1c97bf4))
- download: add method for fetching sticker packs ([`6dd3d24`](https://github.com/PakaiWA/whatsmeow/commit/6dd3d24))
- send: add edit attribute to pin in chat messages (#1106) ([`51dcc5e`](https://github.com/PakaiWA/whatsmeow/commit/51dcc5e))
- msgsecret: add hack for trying both senders for decryption ([`e46d104`](https://github.com/PakaiWA/whatsmeow/commit/e46d104))
- msgsecret: pick vote encryption sender based on poll sender ([`5d16908`](https://github.com/PakaiWA/whatsmeow/commit/5d16908))
- client: remove temporary migration for own lid mapping ([`b7ea3e4`](https://github.com/PakaiWA/whatsmeow/commit/b7ea3e4))
- msgsecret: add more secret encrypted message types ([`fcbcaad`](https://github.com/PakaiWA/whatsmeow/commit/fcbcaad))
- proto: update to v1038709472 ([`6661da3`](https://github.com/PakaiWA/whatsmeow/commit/6661da3))
- client: add cstoken support (#1102) ([`82d579d`](https://github.com/PakaiWA/whatsmeow/commit/82d579d))
- .github: link to general contributing guidelines ([`7514259`](https://github.com/PakaiWA/whatsmeow/commit/7514259))
- client: add option to manually acknowledge history syncs (#1100) ([`60c0488`](https://github.com/PakaiWA/whatsmeow/commit/60c0488))
- client: implement full tctoken lifecycle (#1081) ([`8528b5e`](https://github.com/PakaiWA/whatsmeow/commit/8528b5e))
- proto: update to v1038187123 ([`74a8496`](https://github.com/PakaiWA/whatsmeow/commit/74a8496))
- pair-code: check received ephemeral key length ([`76c09e2`](https://github.com/PakaiWA/whatsmeow/commit/76c09e2))
- appstate,handshake,pair: use constant-time byte comparisons ([`c26a4c5`](https://github.com/PakaiWA/whatsmeow/commit/c26a4c5))
- handshake: check noise certificate validity ([`75186f8`](https://github.com/PakaiWA/whatsmeow/commit/75186f8))
- util/cbc: fix ciphertext length check ([`bc3d5b3`](https://github.com/PakaiWA/whatsmeow/commit/bc3d5b3))
- message: add extra data to ciphertext hash in buffer ([`74a1ddb`](https://github.com/PakaiWA/whatsmeow/commit/74a1ddb))
- clientpayload: bump version number to v1037753511 ([`5b88861`](https://github.com/PakaiWA/whatsmeow/commit/5b88861))
- retry: add optional semaphore for retry receipts ([`a5594bf`](https://github.com/PakaiWA/whatsmeow/commit/a5594bf))
- receipt: catch panics in retry receipt goroutine ([`415df31`](https://github.com/PakaiWA/whatsmeow/commit/415df31))
- dependencies: update ([`3ff20cd`](https://github.com/PakaiWA/whatsmeow/commit/3ff20cd))
- appstate: send sync error event more consistently ([`d4ffc1d`](https://github.com/PakaiWA/whatsmeow/commit/d4ffc1d))
- send: match attrs native uses for on demand history sync ([`015cd40`](https://github.com/PakaiWA/whatsmeow/commit/015cd40))
- proto: update to v1037076227 ([`b95d922`](https://github.com/PakaiWA/whatsmeow/commit/b95d922))
- Potential fix for code scanning alert no. 19: Incorrect conversion between integer types ([`538e589`](https://github.com/PakaiWA/whatsmeow/commit/538e589))

</details>


---

## [v0.26.4-101927](https://github.com/PakaiWA/whatsmeow/tree/v0.26.4-101927) - 2026-04-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.4-101917...v0.26.4-101927)

### Summary
- **Go Mod Tidy & Checksums**: Sinkronisasi dependency checksums di `go.mod` dan `go.sum`.
- **Diff Stats**: 2 files diubah, +33 insertions, -43 deletions.

<details><summary><b>Commit Log (1 commits)</b></summary>

- build: update dependencies ([`5e14fcd`](https://github.com/PakaiWA/whatsmeow/commit/5e14fcd))

</details>


---

## [v0.26.4-101917](https://github.com/PakaiWA/whatsmeow/tree/v0.26.4-101917) - 2026-04-10 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.2-240615...v0.26.4-101917)

### Summary
- **Database Schema Upgrades**: Migrasi SQLStore menambahkan tabel `12-privacy-token-sender-timestamp.sql` dan `13-retry-buffer.sql` untuk buffer retry pengiriman pesan.
- **Media Upload Enhancements**: Penambahan fungsi penanganan upload media terisolasi dan context propagation.
- **CI CodeQL Integration**: Penambahan pipeline scanning keamanan CodeQL GitHub Actions (`.github/workflows/codeql.yml`).
- **Protobuf Updates**: Pembaruan protobuf web layer.
- **Diff Stats**: 103 files diubah, +12,067 insertions, -6,807 deletions.

<details><summary><b>Commit Log (26 commits)</b></summary>

- Potential fix for code scanning alert no. 1: Workflow does not contain permissions ([`9301bd1`](https://github.com/PakaiWA/whatsmeow/commit/9301bd1))
- chore: update protobuf files from upstream ([`0cc7c08`](https://github.com/PakaiWA/whatsmeow/commit/0cc7c08))
- chore: fix import path migration in CI workflow ([`3e12a9e`](https://github.com/PakaiWA/whatsmeow/commit/3e12a9e))
- upload: add media delete method ([`02ec817`](https://github.com/PakaiWA/whatsmeow/commit/02ec817))
- store: add new default client payload fields ([`7e66fe5`](https://github.com/PakaiWA/whatsmeow/commit/7e66fe5))
- send: update on-demand history sync parameters ([`5465030`](https://github.com/PakaiWA/whatsmeow/commit/5465030))
- proto: update to v1035920091 ([`dbc4f26`](https://github.com/PakaiWA/whatsmeow/commit/dbc4f26))
- store: prevent misuse of deleted devices ([`ce4daa5`](https://github.com/PakaiWA/whatsmeow/commit/ce4daa5))
- proto: update to v1035682492 ([`234105b`](https://github.com/PakaiWA/whatsmeow/commit/234105b))
- proto: update to v1034566421 ([`fc65416`](https://github.com/PakaiWA/whatsmeow/commit/fc65416))
- proto: update to v1034187832 ([`c9652e4`](https://github.com/PakaiWA/whatsmeow/commit/c9652e4))
- client: auto-reconnect on any dial errors ([`f9fcedb`](https://github.com/PakaiWA/whatsmeow/commit/f9fcedb))
- Add CodeQL analysis workflow configuration ([`3f58c5d`](https://github.com/PakaiWA/whatsmeow/commit/3f58c5d))
- build(deps): bump filippo.io/edwards25519 from 1.1.0 to 1.1.1 ([`866748e`](https://github.com/PakaiWA/whatsmeow/commit/866748e))
- proto: add decoders for raven messages ([`7ae702b`](https://github.com/PakaiWA/whatsmeow/commit/7ae702b))
- proto: update to v1033703022 ([`dcaf8a7`](https://github.com/PakaiWA/whatsmeow/commit/dcaf8a7))
- privacysettings: add support for new privacy settings (#1075) ([`9cbe80f`](https://github.com/PakaiWA/whatsmeow/commit/9cbe80f))
- retry: add option to store outgoing messages in db ([`e4d82a0`](https://github.com/PakaiWA/whatsmeow/commit/e4d82a0))
- store/sqlstore: add sender timestamp for privacy tokens ([`52d69f8`](https://github.com/PakaiWA/whatsmeow/commit/52d69f8))
- socket: don't set dial options in wasm ([`0283654`](https://github.com/PakaiWA/whatsmeow/commit/0283654))
- dependencies: update ([`34b971e`](https://github.com/PakaiWA/whatsmeow/commit/34b971e))
- proto: update to v1033516464 ([`19d88b9`](https://github.com/PakaiWA/whatsmeow/commit/19d88b9))
- main: bump minimum Go version to 1.25 ([`6bd7aa0`](https://github.com/PakaiWA/whatsmeow/commit/6bd7aa0))
- reportingtoken: fix jid order ([`7b33f62`](https://github.com/PakaiWA/whatsmeow/commit/7b33f62))
- proto: update to v1033287297 ([`ac62b96`](https://github.com/PakaiWA/whatsmeow/commit/ac62b96))
- presence: send unified session id with online status ([`8e7b838`](https://github.com/PakaiWA/whatsmeow/commit/8e7b838))

</details>


---

## [v0.26.2-240615](https://github.com/PakaiWA/whatsmeow/tree/v0.26.2-240615) - 2026-02-24 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.2-90726...v0.26.2-240615)

### Summary
- **Security Policy**: Penambahan file kebijakan pelaporan celah keamanan `SECURITY.md`.
- **CI Tuning**: Penyesuaian flag caching pada workflow build Go.
- **Diff Stats**: 2 files diubah, +22 insertions, -1 deletion.

<details><summary><b>Commit Log (2 commits)</b></summary>

- Create SECURITY.md ([`ace0b04`](https://github.com/PakaiWA/whatsmeow/commit/ace0b04))
- Mon Feb  9 07:27:38 WIB 2026 ([`4e8d0c5`](https://github.com/PakaiWA/whatsmeow/commit/4e8d0c5))

</details>


---

## [v0.26.2-90726](https://github.com/PakaiWA/whatsmeow/tree/v0.26.2-90726) - 2026-02-09 (https://github.com/PakaiWA/whatsmeow/compare/v0.26.1-181628...v0.26.2-90726)

### Summary
- **AppState Synchronization Overhaul**: Perombakan modul sinkronisasi appstate (`appstate/decode.go`, `appstate/encode.go`, `appstate/hash.go`, `appstate/recovery.go`).
- **Message Retry Resilience**: Implementasi mekanisme retry pengiriman pesan dengan backoff logic di `retry.go`.
- **SQLStore Storage Extension**: Penambahan method penyimpanan appstate snapshot dan mutation hash di `store/sqlstore/store.go`.
- **Diff Stats**: 21 files diubah, +1,191 insertions, -434 deletions.

<details><summary><b>Commit Log (33 commits)</b></summary>

- chore: replace go.mau.fi/whatsmeow with github.com/PakaiWA/whatsmeow ([`37ae7b2`](https://github.com/PakaiWA/whatsmeow/commit/37ae7b2))
- Update recovery.go ([`1ea0e85`](https://github.com/PakaiWA/whatsmeow/commit/1ea0e85))
- Mon Feb  9 07:22:19 WIB 2026 ([`4423b00`](https://github.com/PakaiWA/whatsmeow/commit/4423b00))
- Mon Feb  9 07:22:03 WIB 2026 ([`ab668e2`](https://github.com/PakaiWA/whatsmeow/commit/ab668e2))
- improve module name when create new device ([`cafb34c`](https://github.com/PakaiWA/whatsmeow/commit/cafb34c))
- improve module name when create new device ([`c135859`](https://github.com/PakaiWA/whatsmeow/commit/c135859))
- Update go.yml ([`4d13d0d`](https://github.com/PakaiWA/whatsmeow/commit/4d13d0d))
- chore: replace go.mau.fi/whatsmeow with github.com/PakaiWA/whatsmeow ([`3917d4c`](https://github.com/PakaiWA/whatsmeow/commit/3917d4c))
- Enhance GitHub Actions workflow for import path replacement ([`0ade264`](https://github.com/PakaiWA/whatsmeow/commit/0ade264))
- proto: update to v1032642223 ([`7787ab9`](https://github.com/PakaiWA/whatsmeow/commit/7787ab9))
- appstate/keys: fix typo ([`b621394`](https://github.com/PakaiWA/whatsmeow/commit/b621394))
- appstate: include version in sync complete event ([`4dbbef8`](https://github.com/PakaiWA/whatsmeow/commit/4dbbef8))
- appstate: add event for sync errors ([`583635a`](https://github.com/PakaiWA/whatsmeow/commit/583635a))
- appstate: remove lid hack ([`6250ed5`](https://github.com/PakaiWA/whatsmeow/commit/6250ed5))
- appstate: add support for handling recovery events ([`b6f3348`](https://github.com/PakaiWA/whatsmeow/commit/b6f3348))
- message: fix return value of placeholder resend handler ([`c786fa2`](https://github.com/PakaiWA/whatsmeow/commit/c786fa2))
- appstate: add delete media field to clear/delete chat events ([`f834b5a`](https://github.com/PakaiWA/whatsmeow/commit/f834b5a))
- appstate: add constants for all indexes ([`a36dbd9`](https://github.com/PakaiWA/whatsmeow/commit/a36dbd9))
- appstate: fix delete chat builder ([`8338520`](https://github.com/PakaiWA/whatsmeow/commit/8338520))
- retry: check cache for other JID type in DMs ([`89be06b`](https://github.com/PakaiWA/whatsmeow/commit/89be06b))
- send: don't use LID for peer messages ([`cb6d9da`](https://github.com/PakaiWA/whatsmeow/commit/cb6d9da))
- send: fix SendPeerMessage and use it ([`513ae1b`](https://github.com/PakaiWA/whatsmeow/commit/513ae1b))
- appstate: add syncd recovery request builder ([`1ba7eba`](https://github.com/PakaiWA/whatsmeow/commit/1ba7eba))
- appstate: disable hacky fixes by default ([`83db047`](https://github.com/PakaiWA/whatsmeow/commit/83db047))
- appstate: check all mutations in snapshot for LID hack ([`37568b9`](https://github.com/PakaiWA/whatsmeow/commit/37568b9))
- appstate: only log warnings on error ([`c717857`](https://github.com/PakaiWA/whatsmeow/commit/c717857))
- appstate: expand LID hack to snapshots ([`29d8055`](https://github.com/PakaiWA/whatsmeow/commit/29d8055))
- appstate: adjust error message when snapshot fails to verify ([`8e1e589`](https://github.com/PakaiWA/whatsmeow/commit/8e1e589))
- appstate: remove both index MACs for LID hack ([`baf2967`](https://github.com/PakaiWA/whatsmeow/commit/baf2967))
- appstate: add evil LID migration hack ([`d7b64e5`](https://github.com/PakaiWA/whatsmeow/commit/d7b64e5))
- appstate: refactor patch validation into separate method ([`af2f0e4`](https://github.com/PakaiWA/whatsmeow/commit/af2f0e4))
- appstate: refactor decodeMutation into separate method ([`05d18fb`](https://github.com/PakaiWA/whatsmeow/commit/05d18fb))
- appstate: fix some edge case bugs and add assertions ([`84f938d`](https://github.com/PakaiWA/whatsmeow/commit/84f938d))

</details>


---

## [v0.26.1-181628](https://github.com/PakaiWA/whatsmeow/tree/v0.26.1-181628) - 2026-01-18 (https://github.com/PakaiWA/whatsmeow/compare/v0.25.12-261638...v0.26.1-181628)

### Summary
- **Instamadillo Protobufs Integration**: Penambahan definisi paket protobuf instamadillo (`InstamadilloAddMessage`, `InstamadilloCoreTypeActionLog`, dll.) untuk integrasi pesan multi-protokol.
- **Noise Socket Hardening**: Penyempurnaan handling frame socket noise enkripsi.
- **Diff Stats**: 63 files diubah, +350 insertions, -274 deletions.

<details><summary><b>Commit Log (11 commits)</b></summary>

- fix tagging ([`b2fd402`](https://github.com/PakaiWA/whatsmeow/commit/b2fd402))
- fix tagging ([`8a83f1a`](https://github.com/PakaiWA/whatsmeow/commit/8a83f1a))
- fix tagging ([`edc82f8`](https://github.com/PakaiWA/whatsmeow/commit/edc82f8))
- upgrade proto compiler ([`e2b3c16`](https://github.com/PakaiWA/whatsmeow/commit/e2b3c16))
- sync Sun Jan 18 09:24:07 WIB 2026 ([`ee85f53`](https://github.com/PakaiWA/whatsmeow/commit/ee85f53))
- proto: update to v1032094433 ([`06f4737`](https://github.com/PakaiWA/whatsmeow/commit/06f4737))
- dependencies: update ([`1e73899`](https://github.com/PakaiWA/whatsmeow/commit/1e73899))
- client: fix ResetConnection ([`20f0273`](https://github.com/PakaiWA/whatsmeow/commit/20f0273))
- client: add method for reconnecting socket ([`117540e`](https://github.com/PakaiWA/whatsmeow/commit/117540e))
- proto: update to v1031716404 ([`ccfa04f`](https://github.com/PakaiWA/whatsmeow/commit/ccfa04f))
- message: automatically stop history sync loop ([`ad53517`](https://github.com/PakaiWA/whatsmeow/commit/ad53517))

</details>


---

## [v0.25.12-261638](https://github.com/PakaiWA/whatsmeow/tree/v0.25.12-261638) - 2025-12-26 (https://github.com/PakaiWA/whatsmeow/compare/v0.25.12-141822...v0.25.12-261638)

### Summary
- **Stale Workflow & Pre-commit Configuration**: Penambahan workflow `.github/workflows/stale.yml` dan konfigurasi linter `.pre-commit-config.yaml`.
- **Binary Encoding & Protobuf Wa6**: Penyempurnaan binary XML encoder WhatsApp dan protobuf protokol WA6.
- **Diff Stats**: 14 files diubah, +103 insertions, -57 deletions.

<details><summary><b>Commit Log (4 commits)</b></summary>

- update: bump user agent version used for GetLatestVersion ([`11cf47c`](https://github.com/PakaiWA/whatsmeow/commit/11cf47c))
- ci,dependencies: update ([`56a8e44`](https://github.com/PakaiWA/whatsmeow/commit/56a8e44))
- proto: update to v1031080782 ([`b553db2`](https://github.com/PakaiWA/whatsmeow/commit/b553db2))
- binary/encoder: add missing case for writing AD JIDs ([`fc91dda`](https://github.com/PakaiWA/whatsmeow/commit/fc91dda))

</details>


---

## [v0.25.12-141822](https://github.com/PakaiWA/whatsmeow/tree/v0.25.12-141822) - 2025-12-14 (https://github.com/PakaiWA/whatsmeow/compare/v0.25.12-52235...v0.25.12-141822)

### Summary
- **Group Attribute Enhancement**: Penambahan atribut penanganan deskripsi dan status grup pada `group.go` dan `types/group.go`.
- **Diff Stats**: 2 files diubah, +2 insertions.

<details><summary><b>Commit Log (1 commits)</b></summary>

- group: parse size attribute (#1029) ([`fd6170a`](https://github.com/PakaiWA/whatsmeow/commit/fd6170a))

</details>


---

## [v0.25.12-52235](https://github.com/PakaiWA/whatsmeow/tree/v0.25.12-52235) - 2025-12-05 (https://github.com/PakaiWA/whatsmeow/compare/v0.25.10-302033...v0.25.12-52235)

### Summary
- **HTTP Client Isolation**: Penghapusan reuse pointer HTTP client bersama untuk mencegah race condition pada koneksi paralel.
- **Prekey Bundle Initialization**: Pengiriman 812 prekeys pada inisialisasi sesi awal sesuai spesifikasi terbaru WhatsApp.
- **History Sync Receipts**: Perbaikan handling receipt acknowledgment pada payload sinkronisasi histori chat.
- **Device Cache Synchronization**: Sinkronisasi otomatis device cache pada kegagalan hash participant (`phash`).
- **Protobuf v1030403648 & v1029948479**: Pembaruan masif protobuf upstream dan penghapusan getter legacy.
- **Diff Stats**: 101 files diubah, +7,203 insertions, -3,554 deletions.

<details><summary><b>Commit Log (23 commits)</b></summary>

- sync Fri Dec  5 22:33:56 WIB 2025 ([`fd90e48`](https://github.com/PakaiWA/whatsmeow/commit/fd90e48))
- sync Fri Dec  5 22:24:41 WIB 2025 ([`d6c8f75`](https://github.com/PakaiWA/whatsmeow/commit/d6c8f75))
- sync Fri Dec  5 22:22:35 WIB 2025 ([`26299d6`](https://github.com/PakaiWA/whatsmeow/commit/26299d6))
- sync Fri Dec  5 22:17:02 WIB 2025 ([`d797699`](https://github.com/PakaiWA/whatsmeow/commit/d797699))
- client: Retry on WA 5xx errors (#1023) ([`3643699`](https://github.com/PakaiWA/whatsmeow/commit/3643699))
- group: parse suspended flag (#1021) ([`fc1c13d`](https://github.com/PakaiWA/whatsmeow/commit/fc1c13d))
- group: include phone number attribute when adding LID participants ([`b8b6014`](https://github.com/PakaiWA/whatsmeow/commit/b8b6014))
- appstate: apply patches if send fails with conflict ([`d5bb536`](https://github.com/PakaiWA/whatsmeow/commit/d5bb536))
- proto: update to v1030403648 ([`22e2ea1`](https://github.com/PakaiWA/whatsmeow/commit/22e2ea1))
- notification: handle device list updates ([`b9ac3d5`](https://github.com/PakaiWA/whatsmeow/commit/b9ac3d5))
- client: don't reuse http client pointer for different purposes ([`071293c`](https://github.com/PakaiWA/whatsmeow/commit/071293c))
- dependencies: update ([`3aca430`](https://github.com/PakaiWA/whatsmeow/commit/3aca430))
- sqlstore: query both identifier types for privacy tokens ([`7159d90`](https://github.com/PakaiWA/whatsmeow/commit/7159d90))
- proto: remove remaining legacy getter proxies ([`201d873`](https://github.com/PakaiWA/whatsmeow/commit/201d873))
- proto: update to v1029948479 ([`bdfdf4a`](https://github.com/PakaiWA/whatsmeow/commit/bdfdf4a))
- user: add missing nil check to GetProfilePictureInfo ([`a121e2b`](https://github.com/PakaiWA/whatsmeow/commit/a121e2b))
- send: stop using deprecated GetUserDevicesContext ([`74100a9`](https://github.com/PakaiWA/whatsmeow/commit/74100a9))
- proto: update to v1029399661 ([`720bd0b`](https://github.com/PakaiWA/whatsmeow/commit/720bd0b))
- message: add support for initialHistBootstrapInlinePayload ([`6cfa7f7`](https://github.com/PakaiWA/whatsmeow/commit/6cfa7f7))
- connectionevents: add stub handler for dirty event ([`a8a4c7e`](https://github.com/PakaiWA/whatsmeow/commit/a8a4c7e))
- store/clientpayload: update default values ([`ac4867a`](https://github.com/PakaiWA/whatsmeow/commit/ac4867a))
- message: fix history sync receipts ([`a2f58c9`](https://github.com/PakaiWA/whatsmeow/commit/a2f58c9))
- prekeys: upload 812 prekeys in initial bundle ([`1e8afd5`](https://github.com/PakaiWA/whatsmeow/commit/1e8afd5))

</details>


---

## [v0.25.10-302033](https://github.com/PakaiWA/whatsmeow/tree/v0.25.10-302033) - 2025-10-30

### Summary
- **Baseline Release**: Titik awal tracking rilis repositori PakaiWA/whatsmeow.
- **Core Features**:
  - WebSocket engine berbasis `coder/websocket` dengan dukungan context di seluruh layer API.
  - Dukungan protokol WhatsApp Multi-Device (Noise Handshake, AppState Sync, E2E Signal protocol).
  - SQLStore modular untuk PostgreSQL, SQLite, dan SQL Server.
  - Dukungan sinkronisasi profil bisnis, push name, dan peer message metadata.

<details><summary><b>Commit Log (8 commits)</b></summary>

- Thu Oct 30 20:32:07 WIB 2025 ([`8bd61ed`](https://github.com/PakaiWA/whatsmeow/commit/8bd61ed))
- Thu Oct 30 17:05:42 WIB 2025 ([`a5684f6`](https://github.com/PakaiWA/whatsmeow/commit/a5684f6))
- Thu Oct 30 17:02:57 WIB 2025 ([`2899559`](https://github.com/PakaiWA/whatsmeow/commit/2899559))
- Thu Oct 30 16:55:54 WIB 2025 ([`784d087`](https://github.com/PakaiWA/whatsmeow/commit/784d087))
- Thu Oct 30 16:46:15 WIB 2025 ([`cf206ec`](https://github.com/PakaiWA/whatsmeow/commit/cf206ec))
- Thu Oct 30 16:44:32 WIB 2025 ([`1aa4614`](https://github.com/PakaiWA/whatsmeow/commit/1aa4614))
- send: clear device cache for DMs on phash mismatch ([`ad7a618`](https://github.com/PakaiWA/whatsmeow/commit/ad7a618))
- user: synchronize updating push/business names in store ([`ba0d4eb`](https://github.com/PakaiWA/whatsmeow/commit/ba0d4eb))

</details>


---
