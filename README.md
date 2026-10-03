# whatsmeow
[![Go Reference](https://pkg.go.dev/badge/github.com/PakaiWA/whatsmeow.svg)](https://pkg.go.dev/github.com/PakaiWA/whatsmeow)

`whatsmeow` is a Go library for the WhatsApp web multidevice API. It provides persistent WebSocket connectivity, Noise Protocol handshake, Signal Protocol end-to-end encryption (E2EE), session persistence, media handling, and event dispatching.

Forked and maintained by **PakaiWA** with platform-specific adaptations, device identity customization, and security hardening.

---

## Installation

```bash
go get -u github.com/PakaiWA/whatsmeow
```

> **Requirements**: Go 1.26 or later.

---

## Quick Start Example

Here is a simple example demonstrating how to initialize a SQLite session store, connect the client, handle QR code authentication, and listen for incoming messages:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"

	"github.com/PakaiWA/whatsmeow"
	"github.com/PakaiWA/whatsmeow/store/sqlstore"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func eventHandler(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Printf("Received a message from %s: %s\n", v.Info.Sender.String(), v.Message.GetConversation())
	}
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	ctx := context.Background()

	// Initialize SQL store container (SQLite / PostgreSQL)
	container, err := sqlstore.New(ctx, "sqlite3", "file:store.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	// Fetch first session device or create a new one if none exists
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		// New login: listen on QR channel
		qrChan, _ := client.GetQRChannel(ctx)
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR code in WhatsApp Web:")
				fmt.Println(evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in: connect directly
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		fmt.Println("Connected successfully!")
	}

	// Graceful shutdown handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
```

---

## Features

### Core Capabilities
* **Messaging**: Send and receive messages across 1-on-1 chats and groups (text, media, documents, voice notes, stickers, reactions, and view-once messages).
* **End-to-End Encryption (E2EE)**: Full Signal Protocol cryptographic suite (Double Ratchet, Pre-Keys, Axolotl Sender Keys).
* **Group Management**: Group creation, metadata updates, participant management (add, remove, promote, demote), disappearing messages timer, invite links, and join request approval.
* **Newsletters / Channels**: Interact with WhatsApp channels (retrieve metadata, follow/unfollow, live reactions).
* **Rich Responses**: Unmarshaling and parsing support for interactive messages and formatted text entities (`types/richresponse`).
* **Pairing Modes**: Authentication via QR Code scanning, Phone Pairing Code (8-character code without camera scanning), and Passkeys.
* **Persistent Storage**: Robust multi-session SQL storage supporting SQLite and PostgreSQL (`store/sqlstore`) with automated schema migrations.
* **App State Synchronization**: Encrypted app state reconciliation for contacts and chat mutations (mute, pin, archive) using Homomorphic Hashing (LtHash).
* **Auto-Reconnect & Retry**: Automated network reconnection backoff and retry receipt dispatching on message decryption failure.

### Protocol Boundaries & Limitations
* **Broadcast Lists**: Sending broadcast list messages is not supported by the WhatsApp Web multidevice protocol itself.
* **VoIP Calling**: Only incoming/outgoing call notifications and events are parsed (real-time voice/video audio stream negotiation is not supported).

---

## Documentation & Architecture

Comprehensive architectural guides and codebase maps are available in the repository:
* [`ARCHITECTURE.md`](ARCHITECTURE.md) — Technical architecture, message ingress/egress data flow, concurrency patterns, and telemetry.
* [`CODEBASE_MAP.md`](CODEBASE_MAP.md) — Sub-package navigation map, dependencies, and component responsibilities.
* [`PROJECT_CONTEXT.md`](PROJECT_CONTEXT.md) — PakaiWA system boundaries, domain models, and engineering conventions.
* [`DECISIONS.md`](DECISIONS.md) — Architecture Decision Records (ADRs).
* [`SECURITY.md`](SECURITY.md) — Security policy and vulnerability disclosure procedures.

Complete API reference documentation is hosted on [pkg.go.dev](https://pkg.go.dev/github.com/PakaiWA/whatsmeow).

---

## Discussion

Matrix room: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)

For questions about the WhatsApp protocol (like how to send a specific type of
message), you can also use the [WhatsApp protocol Q&A] section on GitHub
discussions.

[WhatsApp protocol Q&A]: https://github.com/tulir/whatsmeow/discussions/categories/whatsapp-protocol-q-a

---

## License

This project is licensed under the **Mozilla Public License 2.0 (MPL-2.0)**.
- Copyright (c) 2021-2026 Tulir Asokan and whatsmeow contributors
- Copyright (c) 2025-2026 Kelvin Anggara, PakaiWA Developers and Contributors

See the [LICENSE](LICENSE) file for the full license text.
