// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"compress/zlib"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waHistorySync"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestMessage_Padding(t *testing.T) {
	t.Run("Pad and unpad roundtrip", func(t *testing.T) {
		original := []byte("hello padding test")
		padded := padMessage(original)
		assert.Greater(t, len(padded), len(original))

		unpadded, err := unpadMessage(padded, 0)
		require.NoError(t, err)
		assert.Equal(t, original, unpadded)
	})

	t.Run("Version 3 skips unpadding", func(t *testing.T) {
		raw := []byte("raw message without padding")
		unpadded, err := unpadMessage(raw, 3)
		require.NoError(t, err)
		assert.Equal(t, raw, unpadded)
	})

	t.Run("Unpad empty plaintext returns error", func(t *testing.T) {
		_, err := unpadMessage([]byte{}, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plaintext is empty")
	})

	t.Run("Unpad invalid padding returns error", func(t *testing.T) {
		invalid := []byte{1, 2, 3, 4, 5, 2} // last byte 2, but preceding is 5 != 2
		_, err := unpadMessage(invalid, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plaintext doesn't have expected padding")
	})
}

func TestMessage_ParseMessageSource(t *testing.T) {
	devID := types.NewJID("12345", types.DefaultUserServer)
	devLID := types.NewJID("67890", types.HiddenUserServer)
	cli := NewClient(&store.Device{
		ID:  &devID,
		LID: devLID,
	}, waLog.Noop)

	t.Run("Empty own ID returns ErrNotLoggedIn", func(t *testing.T) {
		emptyCli := NewClient(&store.Device{}, waLog.Noop)
		_, err := emptyCli.parseMessageSource(&waBinary.Node{Tag: "message"}, false)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("Group message parsing", func(t *testing.T) {
		groupJID := types.NewJID("group1", types.GroupServer)
		participantJID := types.NewJID("user1", types.DefaultUserServer)
		lidJID := types.NewJID("67890", types.HiddenUserServer)
		node := &waBinary.Node{
			Tag: "message",
			Attrs: waBinary.Attrs{
				"from":            groupJID,
				"participant":    participantJID,
				"participant_lid": lidJID,
			},
		}

		source, err := cli.parseMessageSource(node, true)
		require.NoError(t, err)
		assert.True(t, source.IsGroup)
		assert.Equal(t, groupJID, source.Chat)
		assert.Equal(t, participantJID, source.Sender)
		assert.Equal(t, "67890", source.SenderAlt.User)
	})

	t.Run("Broadcast message parsing from me", func(t *testing.T) {
		bcastJID := types.NewJID("status", types.BroadcastServer)
		pnJID := types.NewJID("user3", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "message",
			Attrs: waBinary.Attrs{
				"from":        bcastJID,
				"participant": devID,
			},
			Content: []waBinary.Node{{
				Tag: "participants",
				Content: []waBinary.Node{
					{
						Tag:   "to",
						Attrs: waBinary.Attrs{"jid": types.NewJID("user2", types.DefaultUserServer)},
					},
					{
						Tag:   "to",
						Attrs: waBinary.Attrs{"jid": types.NewJID("user3", types.HiddenUserServer), "peer_recipient_pn": pnJID},
					},
				},
			}},
		}

		source, err := cli.parseMessageSource(node, true)
		require.NoError(t, err)
		assert.True(t, source.IsGroup)
		assert.True(t, source.IsFromMe)
		require.Len(t, source.BroadcastRecipients, 2)
	})

	t.Run("From me direct message", func(t *testing.T) {
		recipientJID := types.NewJID("peer-user", types.DefaultUserServer)
		peerLID := types.NewJID("peer-lid", types.HiddenUserServer)
		node := &waBinary.Node{
			Tag: "message",
			Attrs: waBinary.Attrs{
				"from":               devID,
				"recipient":          recipientJID,
				"peer_recipient_lid": peerLID,
			},
		}

		source, err := cli.parseMessageSource(node, false)
		require.NoError(t, err)
		assert.True(t, source.IsFromMe)
		assert.Equal(t, recipientJID, source.Chat)
		assert.Equal(t, "peer-lid", source.RecipientAlt.User)
	})

	t.Run("Incoming message from user", func(t *testing.T) {
		senderJID := types.NewJID("other-user", types.DefaultUserServer)
		senderLID := types.NewJID("other-lid", types.HiddenUserServer)
		node := &waBinary.Node{
			Tag: "message",
			Attrs: waBinary.Attrs{
				"from":       senderJID,
				"sender_lid": senderLID,
			},
		}

		source, err := cli.parseMessageSource(node, false)
		require.NoError(t, err)
		assert.False(t, source.IsFromMe)
		assert.Equal(t, senderJID, source.Chat)
		assert.Equal(t, senderJID, source.Sender)
		assert.Equal(t, "other-lid", source.SenderAlt.User)
	})
}

func TestMessage_ParseMessageInfo_And_MetaBot(t *testing.T) {
	devID := types.NewJID("12345", types.DefaultUserServer)
	cli := NewClient(&store.Device{ID: &devID}, waLog.Noop)

	t.Run("Parse bot info", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "message",
			Content: []waBinary.Node{{
				Tag: "bot",
				Attrs: waBinary.Attrs{
					"edit":                 "inner",
					"edit_target_id":       "target-123",
					"sender_timestamp_ms": "1700000000000",
					"client_thread_id":     "thread-abc",
				},
			}},
		}
		botInfo, err := cli.parseMsgBotInfo(node)
		require.NoError(t, err)
		assert.Equal(t, types.EditTypeInner, botInfo.EditType)
		assert.Equal(t, types.MessageID("target-123"), botInfo.EditTargetID)
		assert.Equal(t, "thread-abc", botInfo.ClientThreadID)
	})

	t.Run("Parse meta info", func(t *testing.T) {
		node := waBinary.Node{
			Tag: "message",
			Content: []waBinary.Node{{
				Tag: "meta",
				Attrs: waBinary.Attrs{
					"target_id":              "target-456",
					"target_sender_jid":      types.NewJID("sender", types.DefaultUserServer),
					"target_chat_jid":        types.NewJID("chat", types.DefaultUserServer),
					"deprecated_lid_session": "true",
					"thread_msg_id":          "thread-789",
				},
			}},
		}
		metaInfo, err := cli.parseMsgMetaInfo(node)
		require.NoError(t, err)
		assert.Equal(t, types.MessageID("target-456"), metaInfo.TargetID)
		assert.NotNil(t, metaInfo.DeprecatedLIDSession)
		assert.True(t, *metaInfo.DeprecatedLIDSession)
		assert.Equal(t, types.MessageID("thread-789"), metaInfo.ThreadMessageID)
	})

	t.Run("Parse full MessageInfo", func(t *testing.T) {
		sender := types.NewJID("sender-full", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "message",
			Attrs: waBinary.Attrs{
				"from":      sender,
				"id":        "full-msg-id",
				"server_id": "9999",
				"t":         "1700000000",
				"notify":    "Alice",
				"type":      "text",
				"category":  "peer",
			},
			Content: []waBinary.Node{
				{Tag: "multicast"},
			},
		}

		info, err := cli.parseMessageInfo(node)
		require.NoError(t, err)
		assert.Equal(t, types.MessageID("full-msg-id"), info.ID)
		assert.Equal(t, types.MessageServerID(9999), info.ServerID)
		assert.Equal(t, "Alice", info.PushName)
		assert.True(t, info.Multicast)
		assert.Equal(t, "peer", info.Category)
	})
}

func TestMessage_ProtocolMessages_And_HistorySync(t *testing.T) {
	ctx := context.Background()
	devID := types.NewJID("12345", types.DefaultUserServer)
	dev := &store.Device{
		ID:           &devID,
		LIDs:         newMockLIDStore(),
		MsgSecrets:   newMockCustomMsgSecretStore(),
		AppStateKeys: newMockCustomAppStateKeyStore(),
	}
	cli := NewClient(dev, waLog.Noop)

	t.Run("handleProtocolMessage with AppStateSyncKeyShare and peer receipt", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		mockCli.Store.AppStateKeys = newMockCustomAppStateKeyStore()
		mockCli.Store.AppState = &mockAppStateStore{}

		info := &types.MessageInfo{
			ID: "PROTO-1",
			MessageSource: types.MessageSource{
				Sender:   mockCli.getOwnID(),
				IsFromMe: true,
			},
			Category: "peer",
		}

		keyShare := &waE2E.AppStateSyncKeyShare{
			Keys: []*waE2E.AppStateSyncKey{{
				KeyID: &waE2E.AppStateSyncKeyId{KeyID: []byte("key-1")},
				KeyData: &waE2E.AppStateSyncKeyData{
					KeyData:   []byte("secret-data"),
					Timestamp: new(int64(1000)),
				},
			}},
		}

		protoMsg := &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				AppStateSyncKeyShare: keyShare,
			},
		}

		ok := mockCli.handleProtocolMessage(ctx, info, protoMsg)
		assert.True(t, ok)
	})

	t.Run("DownloadHistorySync compressed payload", func(t *testing.T) {
		syncObj := &waHistorySync.HistorySync{
			SyncType: waHistorySync.HistorySync_PUSH_NAME.Enum(),
			Pushnames: []*waHistorySync.Pushname{{
				ID:       new("123456@s.whatsapp.net"),
				Pushname: new("Test User"),
			}},
		}
		syncBytes, err := proto.Marshal(syncObj)
		require.NoError(t, err)

		var compressedBuf bytes.Buffer
		writer := zlib.NewWriter(&compressedBuf)
		_, err = writer.Write(syncBytes)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		notif := &waE2E.HistorySyncNotification{
			InitialHistBootstrapInlinePayload: compressedBuf.Bytes(),
		}

		downloaded, err := cli.DownloadHistorySync(ctx, notif, true)
		require.NoError(t, err)
		assert.Equal(t, waHistorySync.HistorySync_PUSH_NAME, downloaded.GetSyncType())
		require.Len(t, downloaded.GetPushnames(), 1)
		assert.Equal(t, "Test User", downloaded.GetPushnames()[0].GetPushname())
	})

	t.Run("SendProtocolMessageReceipt", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		// Empty ID returns nil immediately
		err := cli.SendProtocolMessageReceipt(ctx, "", types.ReceiptTypePeerMsg)
		require.NoError(t, err)

		// Non-empty ID sends node
		err = cli.SendProtocolMessageReceipt(ctx, "MSG-REC-1", types.ReceiptTypePeerMsg)
		require.NoError(t, err)
	})
}

func TestMessage_HandleDecryptedMessage_And_Plaintext(t *testing.T) {
	ctx := context.Background()
	devID := types.NewJID("12345", types.DefaultUserServer)
	dev := &store.Device{
		ID:         &devID,
		MsgSecrets: newMockCustomMsgSecretStore(),
	}
	cli := NewClient(dev, waLog.Noop)

	info := &types.MessageInfo{
		ID: "DEC-1",
		MessageSource: types.MessageSource{
			Chat:   types.NewJID("peer", types.DefaultUserServer),
			Sender: types.NewJID("peer", types.DefaultUserServer),
		},
	}
	msg := &waE2E.Message{
		Conversation: new("hello world conversation"),
	}

	var dispatched *events.Message
	cli.AddEventHandler(func(evt any) {
		if m, ok := evt.(*events.Message); ok {
			dispatched = m
		}
	})

	failed := cli.handleDecryptedMessage(ctx, info, msg, 0)
	assert.False(t, failed)
	require.NotNil(t, dispatched)
	assert.Equal(t, "hello world conversation", dispatched.Message.GetConversation())
}

type mockCustomAppStateKeyStore struct {
	keys map[string]store.AppStateSyncKey
}

func newMockCustomAppStateKeyStore() *mockCustomAppStateKeyStore {
	return &mockCustomAppStateKeyStore{keys: make(map[string]store.AppStateSyncKey)}
}

func (m *mockCustomAppStateKeyStore) PutAppStateSyncKey(_ context.Context, id []byte, val store.AppStateSyncKey) error {
	m.keys[string(id)] = val
	return nil
}

func (m *mockCustomAppStateKeyStore) GetAppStateSyncKey(_ context.Context, id []byte) (*store.AppStateSyncKey, error) {
	val, ok := m.keys[string(id)]
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (m *mockCustomAppStateKeyStore) GetLatestAppStateSyncKeyID(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (m *mockCustomAppStateKeyStore) GetAllAppStateSyncKeys(_ context.Context) ([]*store.AppStateSyncKey, error) {
	var res []*store.AppStateSyncKey
	for _, v := range m.keys {
		valCopy := v
		res = append(res, &valCopy)
	}
	return res, nil
}
