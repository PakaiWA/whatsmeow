# Project TODO & Technical Debt

## 1. Immediate Tasks
- [ ] `.pre-commit-config.yaml:37`: Aktifkan atau selesaikan hook `zerolog-ban-msgf` dan `zerolog-use-stringer` setelah memastikan format logging seragam.
- [ ] `request.go:227`: Refactor error timeout handling pada `sendIQ` yang masih ditandai `FIXME this error isn't technically correct`.
- [ ] `client.go:630`: Pastikan penutupan dan drain `handlerQueue` saat socket teardown bersih sebelum reconnect.

## 2. Existing Code Annotations (TODO / FIXME)
- `msgsecret.go:73`: `// TODO this is wrong if the message key used @s.whatsapp.net, but the new event is from @lid`
- `types/user.go:202`: `// TODO is this just a timestamp?` (field `DHash`)
- `user.go:170`: `// TODO check output result?`
- `user.go:430`: `// TODO parse bot_fields`
- `user.go:816`: `// TODO take identities here too?`
- `user.go:818`: `// TODO do something with the icdc blob?`
- `user.go:830`: `// TODO include dhash for users`
- `tctoken.go:170`: `// TODO replace with an UPDATE call instead of get+put`
- `message.go:142`: `// TODO IsFromMe?`
- `message.go:276`: `// TODO edits have an additional <meta msg_edit_t="..." original_msg_t="..."/> node`
- `message.go:421`: `// TODO this probably isn't supposed to ack`
- `download.go:161`: `// TODO more proper check for unencrypted media? (also Download and DownloadToFile)`
- `download.go:276`: `// TODO omit hash for unencrypted media?`
- `appstate.go:298`: `// TODO what's index 2 here?`
- `appstate.go:521`: `// TODO create new key instead of reusing the primary client's keys`
- `send.go:214`: `// TODO somehow deduplicate this with the code in sendNewsletter?`
- `send.go:306`: `// TODO this is fairly hacky, is there a proper way to determine which identity the message is sent with?`
- `send.go:1200`: `// TODO this is a very hacky hack for announcement group messages, why is it pn anyway?`
- `send.go:1303`: `// TODO query LID from server for missing entries`
- `group.go:57`: `// TODO member_share_group_history_mode`
- `group.go:767`: `// TODO confirm field name (group.TopicSetByPN)`
- `retry.go:281`: `// TODO pre-retry callback for fb`

## 3. Technical Debt & Structural Improvements
- [ ] **LID vs Phone Number Normalization**: Penyempurnaan mapping otomatis antara Phone Number (PN) dan Privacy LID yang mulai diwajibkan WhatsApp pada update protokol terbaru (`send.go`, `user.go`).
- [ ] **Media Deduplication**: Konsolidasi fungsi duplikasi antara `download.go` dan `download-to-file.go` serta logika upload newsletter di `send.go`.
- [ ] **Upstream Synchronization Cadence**: Mengotomatiskan pengecekan berkala terhadap repo upstream `tulir/whatsmeow` dengan mempertahankan branding dan spesifikasi keamanan `PakaiWA`.
