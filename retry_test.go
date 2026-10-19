// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"golang.org/x/sync/semaphore"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waConsumerApplication"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waMsgApplication"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockSessionStore struct {
	sessions map[string][]byte
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string][]byte)}
}
func (m *mockSessionStore) GetSession(_ context.Context, address string) ([]byte, error) {
	return m.sessions[address], nil
}
func (m *mockSessionStore) HasSession(_ context.Context, address string) (bool, error) {
	_, ok := m.sessions[address]
	return ok, nil
}
func (m *mockSessionStore) GetManySessions(_ context.Context, addresses []string) (map[string][]byte, error) {
	res := make(map[string][]byte)
	for _, a := range addresses {
		if s, ok := m.sessions[a]; ok {
			res[a] = s
		}
	}
	return res, nil
}
func (m *mockSessionStore) PutSession(_ context.Context, address string, session []byte) error {
	m.sessions[address] = session
	return nil
}
func (m *mockSessionStore) PutManySessions(_ context.Context, sessions map[string][]byte) error {
	for k, v := range sessions {
		m.sessions[k] = v
	}
	return nil
}
func (m *mockSessionStore) DeleteAllSessions(_ context.Context, phone string) error {
	return nil
}
func (m *mockSessionStore) DeleteSession(_ context.Context, address string) error {
	delete(m.sessions, address)
	return nil
}
func (m *mockSessionStore) MigratePNToLID(_ context.Context, pn, lid types.JID) error {
	return nil
}

type mockPreKeyStore struct{}

func (m *mockPreKeyStore) GetOrGenPreKeys(_ context.Context, count uint32) ([]*keys.PreKey, error) {
	return nil, nil
}
func (m *mockPreKeyStore) GenOnePreKey(_ context.Context) (*keys.PreKey, error) {
	return keys.NewPreKey(1), nil
}
func (m *mockPreKeyStore) GetPreKey(_ context.Context, id uint32) (*keys.PreKey, error) {
	return nil, nil
}
func (m *mockPreKeyStore) RemovePreKey(_ context.Context, id uint32) error {
	return nil
}
func (m *mockPreKeyStore) MarkPreKeysAsUploaded(_ context.Context, upToID uint32) error {
	return nil
}
func (m *mockPreKeyStore) UploadedPreKeyCount(_ context.Context) (int, error) {
	return 0, nil
}

type mockEventBufferStore struct {
	outgoing      map[string][]byte
	format        map[string]string
	addErr        error
	getErr        error
	delOldErr     error
	deleteOldCall int
}

func newMockEventBufferStore() *mockEventBufferStore {
	return &mockEventBufferStore{
		outgoing: make(map[string][]byte),
		format:   make(map[string]string),
	}
}

func (m *mockEventBufferStore) GetBufferedEvent(_ context.Context, _ [32]byte) (*store.BufferedEvent, error) {
	return nil, nil
}
func (m *mockEventBufferStore) PutBufferedEvent(_ context.Context, _ [32]byte, _ []byte, _ time.Time) error {
	return nil
}
func (m *mockEventBufferStore) DoDecryptionTxn(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}
func (m *mockEventBufferStore) ClearBufferedEventPlaintext(_ context.Context, _ [32]byte) error {
	return nil
}
func (m *mockEventBufferStore) DeleteOldBufferedHashes(_ context.Context) error {
	return nil
}

func (m *mockEventBufferStore) GetOutgoingEvent(_ context.Context, chatJID, altChatJID types.JID, id types.MessageID) (string, []byte, error) {
	if m.getErr != nil {
		return "", nil, m.getErr
	}
	key1 := chatJID.String() + "/" + id
	if data, ok := m.outgoing[key1]; ok {
		return m.format[key1], data, nil
	}
	key2 := altChatJID.String() + "/" + id
	if data, ok := m.outgoing[key2]; ok {
		return m.format[key2], data, nil
	}
	return "", nil, nil
}

func (m *mockEventBufferStore) AddOutgoingEvent(_ context.Context, chatJID types.JID, id types.MessageID, format string, plaintext []byte) error {
	if m.addErr != nil {
		return m.addErr
	}
	key := chatJID.String() + "/" + id
	m.outgoing[key] = plaintext
	m.format[key] = format
	return nil
}

func (m *mockEventBufferStore) DeleteOldOutgoingEvents(_ context.Context) error {
	if m.delOldErr != nil {
		return m.delOldErr
	}
	m.deleteOldCall++
	return nil
}

func TestRecentMessage_IsEmpty(t *testing.T) {
	assert.True(t, RecentMessage{}.IsEmpty())
	assert.False(t, RecentMessage{wa: &waE2E.Message{}}.IsEmpty())
	assert.False(t, RecentMessage{fb: &waMsgApplication.MessageApplication{}}.IsEmpty())
}

func TestRetry_ParseRecentMessage(t *testing.T) {
	waMsg := &waE2E.Message{Conversation: proto.String("hello")}
	waBuf, err := proto.Marshal(waMsg)
	require.NoError(t, err)

	fbMsg := &waMsgApplication.MessageApplication{}
	fbBuf, err := proto.Marshal(fbMsg)
	require.NoError(t, err)

	t.Run("Parse WA message", func(t *testing.T) {
		rm, err := parseRecentMessage("wa", waBuf)
		require.NoError(t, err)
		assert.Equal(t, "hello", rm.wa.GetConversation())
	})

	t.Run("Parse FB message", func(t *testing.T) {
		rm, err := parseRecentMessage("fb", fbBuf)
		require.NoError(t, err)
		assert.NotNil(t, rm.fb)
	})

	t.Run("Unknown format", func(t *testing.T) {
		_, err := parseRecentMessage("unknown", waBuf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown format in retry store")
	})

	t.Run("Corrupted buffer", func(t *testing.T) {
		_, err := parseRecentMessage("wa", []byte{0xff, 0xff})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal payload")
	})
}

func TestRetry_AddAndGetRecentMessage(t *testing.T) {
	ctx := context.Background()
	evStore := newMockEventBufferStore()
	device := &store.Device{
		EventBuffer: evStore,
	}
	cli := NewClient(device, waLog.Noop)
	cli.UseRetryMessageStore = true

	chat := types.NewJID("12345", types.DefaultUserServer)
	msgID := "msg-123"
	waMsg := &waE2E.Message{Conversation: proto.String("test recent")}

	// Force clear old messages check by setting lastRetryStoreClear
	cli.lastRetryStoreClear = time.Now().Add(-24 * time.Hour)

	// Message and fbMessage both nil returns nil
	err := cli.addRecentMessage(ctx, chat, "nil-msg", nil, nil)
	require.NoError(t, err)

	err = cli.addRecentMessage(ctx, chat, msgID, waMsg, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, evStore.deleteOldCall)

	// In-memory lookup
	found := cli.getRecentMessage(chat, msgID)
	assert.False(t, found.IsEmpty())
	assert.Equal(t, "test recent", found.wa.GetConversation())

	// Ring buffer overwrite test
	for i := 0; i < recentMessagesSize+10; i++ {
		id := types.MessageID("id-" + string(rune(i)))
		_ = cli.addRecentMessage(ctx, chat, id, waMsg, nil)
	}
}

func TestRetry_GetMessageForRetry(t *testing.T) {
	ctx := context.Background()
	chatPN := types.NewJID("12345", types.DefaultUserServer)
	chatLID := types.NewJID("lid-12345", types.HiddenUserServer)
	sender := types.NewJID("67890", types.DefaultUserServer)

	lidStore := newMockLIDStore()
	lidStore.pnToLID[chatPN] = chatLID
	lidStore.lidToPN[chatLID] = chatPN

	evStore := newMockEventBufferStore()
	device := &store.Device{
		LIDs:        lidStore,
		EventBuffer: evStore,
	}
	cli := NewClient(device, waLog.Noop)

	receipt := &events.Receipt{
		MessageSource: types.MessageSource{
			Chat:   chatPN,
			Sender: sender,
		},
		MessageIDs: []types.MessageID{"msg-test"},
	}

	t.Run("Found in local cache directly", func(t *testing.T) {
		waMsg := &waE2E.Message{Conversation: proto.String("in cache")}
		_ = cli.addRecentMessage(ctx, chatPN, "msg-test", waMsg, nil)

		rm, err := cli.getMessageForRetry(ctx, receipt, "msg-test")
		require.NoError(t, err)
		require.NotNil(t, rm)
		assert.Equal(t, "in cache", rm.wa.GetConversation())
	})

	t.Run("Found via alternate chat JID", func(t *testing.T) {
		waMsg := &waE2E.Message{Conversation: proto.String("in alt cache")}
		_ = cli.addRecentMessage(ctx, chatLID, "msg-alt", waMsg, nil)

		rm, err := cli.getMessageForRetry(ctx, receipt, "msg-alt")
		require.NoError(t, err)
		require.NotNil(t, rm)
		assert.Equal(t, "in alt cache", rm.wa.GetConversation())
	})

	t.Run("Found in retry store", func(t *testing.T) {
		cli.UseRetryMessageStore = true
		waMsg := &waE2E.Message{Conversation: proto.String("in retry store")}
		buf, _ := proto.Marshal(waMsg)
		_ = evStore.AddOutgoingEvent(ctx, chatPN, "msg-store", "wa", buf)

		rm, err := cli.getMessageForRetry(ctx, receipt, "msg-store")
		require.NoError(t, err)
		require.NotNil(t, rm)
		assert.Equal(t, "in retry store", rm.wa.GetConversation())
	})

	t.Run("Found via GetMessageForRetry callback", func(t *testing.T) {
		cli.UseRetryMessageStore = false
		cli.GetMessageForRetry = func(s, c types.JID, id types.MessageID) *waE2E.Message {
			if id == "msg-cb" {
				return &waE2E.Message{Conversation: proto.String("from callback")}
			}
			return nil
		}

		rm, err := cli.getMessageForRetry(ctx, receipt, "msg-cb")
		require.NoError(t, err)
		require.NotNil(t, rm)
		assert.Equal(t, "from callback", rm.wa.GetConversation())

		// Not found returns nil, nil
		rmNotFound, err := cli.getMessageForRetry(ctx, receipt, "msg-none")
		require.NoError(t, err)
		assert.Nil(t, rmNotFound)
	})
}

func TestRetry_ShouldRecreateSession(t *testing.T) {
	ctx := context.Background()
	user := types.NewJID("12345", types.DefaultUserServer)
	identityKey := keys.NewKeyPair()
	sessStore := newMockSessionStore()
	device := &store.Device{
		IdentityKey:  identityKey,
		SignedPreKey: identityKey.CreateSignedPreKey(1),
		Sessions:     sessStore,
	}
	cli := NewClient(device, waLog.Noop)

	// No session exists in store -> recreate = true
	reason, recreate := cli.shouldRecreateSession(ctx, 0, user)
	assert.True(t, recreate)
	assert.Contains(t, reason, "we don't have a Signal session")

	// Store now contains a session
	_ = sessStore.PutSession(ctx, user.SignalAddress().String(), []byte("dummy-session"))
	reason, recreate = cli.shouldRecreateSession(ctx, 1, user)
	assert.False(t, recreate)
	assert.Empty(t, reason)

	reason, recreate = cli.shouldRecreateSession(ctx, 2, user)
	assert.False(t, recreate) // Within 1 hour
	assert.Empty(t, reason)

	// Over 1 hour
	cli.sessionRecreateHistory[user] = time.Now().Add(-2 * time.Hour)
	reason, recreate = cli.shouldRecreateSession(ctx, 2, user)
	assert.True(t, recreate)
	assert.Contains(t, reason, "over an hour")
}

func TestRetry_PhoneRerequestMethods(t *testing.T) {
	cli := NewClient(&store.Device{}, waLog.Noop)
	cli.AutomaticMessageRerequestFromPhone = true

	info := &types.MessageInfo{
		ID: "msg-phone-1",
	}

	// Immediate request (fails with ErrNotConnected without socket, but exercises logic)
	cli.immediateRequestMessageFromPhone(context.Background(), info)

	// Delayed request cancellation
	RequestFromPhoneDelay = 10 * time.Millisecond
	go cli.delayedRequestMessageFromPhone(info)
	time.Sleep(2 * time.Millisecond)
	cli.cancelDelayedRequestFromPhone("msg-phone-1")

	// Clear delayed requests
	go cli.delayedRequestMessageFromPhone(&types.MessageInfo{ID: "msg-phone-2"})
	time.Sleep(2 * time.Millisecond)
	cli.clearDelayedMessageRequests()

	// Delayed request timer expiration
	RequestFromPhoneDelay = 5 * time.Millisecond
	go cli.delayedRequestMessageFromPhone(&types.MessageInfo{ID: "msg-phone-3"})
	time.Sleep(15 * time.Millisecond)
}

type mockFailingPreKeyStore struct {
	mockPreKeyStore
}

func (m *mockFailingPreKeyStore) GenOnePreKey(_ context.Context) (*keys.PreKey, error) {
	return nil, assert.AnError
}

func TestRetry_SendRetryReceipt(t *testing.T) {
	ctx := context.Background()
	identityKey := keys.NewKeyPair()
	device := &store.Device{
		IdentityKey:    identityKey,
		SignedPreKey:   identityKey.CreateSignedPreKey(1),
		PreKeys:        &mockPreKeyStore{},
		RegistrationID: 1234,
	}
	cli := NewClient(device, waLog.Noop)
	userJID := types.NewJID("12345", types.DefaultUserServer)

	node := &waBinary.Node{
		Tag: "message",
		Attrs: waBinary.Attrs{
			"id":   "msg-retry-test",
			"from": userJID,
			"t":    "1700000000",
		},
		Content: []waBinary.Node{{
			Tag:   "enc",
			Attrs: waBinary.Attrs{"count": 0},
		}},
	}
	info := &types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:   userJID,
			Sender: userJID,
		},
		ID: "msg-retry-test",
	}

	// 1st retry
	cli.sendRetryReceipt(ctx, node, info, false)
	assert.Equal(t, 1, cli.messageRetries["msg-retry-test"])

	// 2nd retry includes identity & prekeys
	cli.sendRetryReceipt(ctx, node, info, false)
	assert.Equal(t, 2, cli.messageRetries["msg-retry-test"])

	// retryCountInMsg > 0 and peer_msg with SynchronousAck
	nodeWithCount := &waBinary.Node{
		Tag: "message",
		Attrs: waBinary.Attrs{
			"id":   "msg-retry-with-count",
			"from": userJID,
			"t":    "1700000000",
		},
		Content: []waBinary.Node{{
			Tag:   "enc",
			Attrs: waBinary.Attrs{"count": "2"},
		}},
	}
	infoWithCount := &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: userJID, Sender: userJID, IsFromMe: true},
		ID:            "msg-retry-with-count",
		Type:          "peer_msg",
	}
	cli.SynchronousAck = true
	cli.sendRetryReceipt(ctx, nodeWithCount, infoWithCount, true)
	assert.Equal(t, 3, cli.messageRetries["msg-retry-with-count"])

	// PreKeys GenOnePreKey failure
	cli.Store.PreKeys = &mockFailingPreKeyStore{}
	cli.sendRetryReceipt(ctx, node, info, true)

	// After 5 retries, drops
	cli.messageRetries["msg-retry-test"] = 5
	cli.sendRetryReceipt(ctx, node, info, false)
}

func TestRetry_HandleRetryReceipt_Errors(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{
		ID:   &types.JID{User: "me", Server: types.DefaultUserServer},
		LIDs: newMockLIDStore(),
	}, waLog.Noop)

	receipt := &events.Receipt{
		MessageSource: types.MessageSource{
			Chat: types.NewJID("user", types.DefaultUserServer),
		},
		MessageIDs: []types.MessageID{"msg-test"},
	}

	t.Run("Missing retry child node", func(t *testing.T) {
		node := &waBinary.Node{Tag: "receipt"}
		err := cli.handleRetryReceipt(ctx, receipt, node)
		require.Error(t, err)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
	})

	t.Run("Missing id in retry child", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Content: []waBinary.Node{{
				Tag:   "retry",
				Attrs: waBinary.Attrs{"count": 1},
			}},
		}
		err := cli.handleRetryReceipt(ctx, receipt, node)
		require.Error(t, err)
	})

	t.Run("Message not found", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    "msg-not-found",
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}
		err := cli.handleRetryReceipt(ctx, receipt, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "couldn't find message")
	})

	t.Run("Internal retry counter >= 10 drops request", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    "msg-counter-test",
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}
		waMsg := &waE2E.Message{Conversation: proto.String("counter test")}
		_ = cli.addRecentMessage(ctx, receipt.Chat, "msg-counter-test", waMsg, nil)

		// Set counter to 9 so next increment reaches 10
		cli.incomingRetryRequestCounter[incomingRetryKey{receipt.Sender, "msg-counter-test"}] = 9
		err := cli.handleRetryReceipt(ctx, receipt, node)
		require.NoError(t, err)
	})

	t.Run("PreRetryCallback cancel with IsFromMe WA message", func(t *testing.T) {
		fromMeReceipt := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:     receipt.Chat,
				Sender:   receipt.Sender,
				IsFromMe: true,
			},
			MessageIDs: []types.MessageID{"msg-fromme-wa"},
		}
		node := &waBinary.Node{
			Tag: "receipt",
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    "msg-fromme-wa",
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}
		waMsg := &waE2E.Message{Conversation: proto.String("from me wa")}
		_ = cli.addRecentMessage(ctx, receipt.Chat, "msg-fromme-wa", waMsg, nil)

		cli.PreRetryCallback = func(_ *events.Receipt, _ types.MessageID, _ int, _ *waE2E.Message) bool {
			return false
		}
		err := cli.handleRetryReceipt(ctx, fromMeReceipt, node)
		require.NoError(t, err)
	})

	t.Run("PreRetryCallback cancel with IsFromMe FB message", func(t *testing.T) {
		fromMeReceipt := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:     receipt.Chat,
				Sender:   receipt.Sender,
				IsFromMe: true,
			},
			MessageIDs: []types.MessageID{"msg-fromme-fb"},
		}
		node := &waBinary.Node{
			Tag: "receipt",
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    "msg-fromme-fb",
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}
		fbMsg := &waMsgApplication.MessageApplication{}
		_ = cli.addRecentMessage(ctx, receipt.Chat, "msg-fromme-fb", nil, fbMsg)

		cli.PreRetryCallback = func(_ *events.Receipt, _ types.MessageID, _ int, _ *waE2E.Message) bool {
			return false
		}
		err := cli.handleRetryReceipt(ctx, fromMeReceipt, node)
		require.NoError(t, err)
	})

	t.Run("TryHandleRetryReceipt catch error or cancel", func(t *testing.T) {
		node := &waBinary.Node{Tag: "receipt"}
		cli.tryHandleRetryReceipt(ctx, receipt, node)
	})

	t.Run("Full handleRetryReceipt WA message with connected socket and session", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		targetJID := types.NewJID("62812345678", types.DefaultUserServer)
		_, err := establishSignalSession(ctx, mockCli, targetJID)
		require.NoError(t, err)

		msgID := "msg-full-retry-wa"
		waMsg := &waE2E.Message{Conversation: proto.String("retry content")}
		_ = mockCli.addRecentMessage(ctx, targetJID, msgID, waMsg, nil)

		rcpt := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:   targetJID,
				Sender: targetJID,
			},
			MessageIDs: []types.MessageID{msgID},
		}
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": targetJID,
				"id":   msgID,
				"type": "retry",
			},
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    msgID,
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}

		err = mockCli.handleRetryReceipt(ctx, rcpt, node)
		require.NoError(t, err)
	})

	t.Run("Full handleRetryReceipt FB message with connected socket", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		targetJID := types.NewJID("62812345678", types.DefaultUserServer)
		_, err := establishSignalSession(ctx, mockCli, targetJID)
		require.NoError(t, err)

		msgID := "msg-full-retry-fb"
		consumerApp := &waConsumerApplication.ConsumerApplication{
			Payload: &waConsumerApplication.ConsumerApplication_Payload{
				Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{
					Content: &waConsumerApplication.ConsumerApplication_Content{
						Content: &waConsumerApplication.ConsumerApplication_Content_ImageMessage{
							ImageMessage: &waConsumerApplication.ConsumerApplication_ImageMessage{},
						},
					},
				},
			},
		}
		subProto := &waMsgApplication.MessageApplication_SubProtocolPayload_ConsumerMessage{}
		err = subProto.Set(consumerApp)
		require.NoError(t, err)

		fbMsg := &waMsgApplication.MessageApplication{
			Payload: &waMsgApplication.MessageApplication_Payload{
				Content: &waMsgApplication.MessageApplication_Payload_SubProtocol{
					SubProtocol: &waMsgApplication.MessageApplication_SubProtocolPayload{
						SubProtocol: subProto,
					},
				},
			},
			Metadata: &waMsgApplication.MessageApplication_Metadata{
				FrankingKey: []byte("01234567890123456789012345678901"),
			},
		}
		_ = mockCli.addRecentMessage(ctx, targetJID, msgID, nil, fbMsg)

		rcpt := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:   targetJID,
				Sender: targetJID,
			},
			MessageIDs: []types.MessageID{msgID},
		}
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": targetJID,
				"id":   msgID,
				"type": "retry",
			},
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    msgID,
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}

		err = mockCli.handleRetryReceipt(ctx, rcpt, node)
		require.NoError(t, err)
	})

	t.Run("Full handleRetryReceipt Group and FromMe", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		groupJID := types.NewJID("group123", types.GroupServer)
		senderJID := types.NewJID("62812345678", types.DefaultUserServer)
		_, err := establishSignalSession(ctx, mockCli, senderJID)
		require.NoError(t, err)

		msgID := "msg-group-retry"
		waMsg := &waE2E.Message{Conversation: proto.String("group message")}
		_ = mockCli.addRecentMessage(ctx, groupJID, msgID, waMsg, nil)

		rcpt := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:     groupJID,
				Sender:   senderJID,
				IsGroup:  true,
				IsFromMe: true,
			},
			MessageIDs: []types.MessageID{msgID},
		}
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from":        groupJID,
				"participant": senderJID,
				"id":          msgID,
				"type":        "retry",
			},
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    msgID,
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}

		err = mockCli.handleRetryReceipt(ctx, rcpt, node)
		require.NoError(t, err)

		// Also test FB message in group retry
		fbMsgID := "msg-group-fb"
		fbMsg := &waMsgApplication.MessageApplication{
			Metadata: &waMsgApplication.MessageApplication_Metadata{
				FrankingKey: []byte("01234567890123456789012345678901"),
			},
		}
		_ = mockCli.addRecentMessage(ctx, groupJID, fbMsgID, nil, fbMsg)
		rcptFB := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:     groupJID,
				Sender:   senderJID,
				IsGroup:  true,
				IsFromMe: false,
			},
			MessageIDs: []types.MessageID{fbMsgID},
		}
		nodeFB := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from":        groupJID,
				"participant": senderJID,
				"id":          fbMsgID,
				"type":        "retry",
			},
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    fbMsgID,
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}
		err = mockCli.handleRetryReceipt(ctx, rcptFB, nodeFB)
		require.NoError(t, err)

		// And FB message with IsFromMe
		fbFromMeID := "msg-fromme-fb-full"
		_ = mockCli.addRecentMessage(ctx, senderJID, fbFromMeID, nil, fbMsg)
		rcptFromMe := &events.Receipt{
			MessageSource: types.MessageSource{
				Chat:     senderJID,
				Sender:   senderJID,
				IsGroup:  false,
				IsFromMe: true,
			},
			MessageIDs: []types.MessageID{fbFromMeID},
		}
		nodeFromMe := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": senderJID,
				"id":   fbFromMeID,
				"type": "retry",
			},
			Content: []waBinary.Node{{
				Tag: "retry",
				Attrs: waBinary.Attrs{
					"id":    fbFromMeID,
					"count": "1",
					"t":     "1700000000",
				},
			}},
		}
		err = mockCli.handleRetryReceipt(ctx, rcptFromMe, nodeFromMe)
		require.NoError(t, err)
	})

	t.Run("HandleRetryReceipt invalid keys node", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		targetJID := types.NewJID("62812345678", types.DefaultUserServer)
		msgID := "msg-invalid-keys"
		_ = mockCli.addRecentMessage(ctx, targetJID, msgID, &waE2E.Message{Conversation: proto.String("text")}, nil)
		rcpt := &events.Receipt{
			MessageSource: types.MessageSource{Chat: targetJID, Sender: targetJID},
			MessageIDs:    []types.MessageID{msgID},
		}
		node := &waBinary.Node{
			Tag:   "receipt",
			Attrs: waBinary.Attrs{"from": targetJID, "id": msgID, "type": "retry"},
			Content: []waBinary.Node{
				{Tag: "retry", Attrs: waBinary.Attrs{"id": msgID, "count": "1", "t": "1700000000"}},
				{Tag: "keys", Content: []waBinary.Node{}},
			},
		}
		err := mockCli.handleRetryReceipt(ctx, rcpt, node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read prekey bundle")
	})

	t.Run("HandleRetryReceipt with recipient, edit, and LID migration", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		targetJID := types.NewJID("62812345678", types.DefaultUserServer)
		targetLID := types.NewJID("111111", types.DefaultUserServer)
		_, err := establishSignalSession(ctx, mockCli, targetJID)
		require.NoError(t, err)
		_, err = establishSignalSession(ctx, mockCli, targetLID)
		require.NoError(t, err)
		_ = mockCli.Store.LIDs.PutLIDMapping(ctx, targetJID, targetLID)

		msgID := "msg-edit-rcpt"
		_ = mockCli.addRecentMessage(ctx, targetJID, msgID, &waE2E.Message{Conversation: proto.String("text")}, nil)
		rcpt := &events.Receipt{
			MessageSource: types.MessageSource{Chat: targetJID, Sender: targetJID},
			MessageIDs:    []types.MessageID{msgID},
		}
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from":      targetJID,
				"id":        msgID,
				"type":      "retry",
				"recipient": targetJID,
				"edit":      "1",
			},
			Content: []waBinary.Node{
				{Tag: "retry", Attrs: waBinary.Attrs{"id": msgID, "count": "1", "t": "1700000000"}},
			},
		}
		err = mockCli.handleRetryReceipt(ctx, rcpt, node)
		require.NoError(t, err)
	})

	t.Run("TryHandleRetryReceipt semaphore and panic recovery", func(t *testing.T) {
		mockCli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		mockCli.retrySema = semaphore.NewWeighted(1)

		node := &waBinary.Node{Tag: "receipt"}
		// normal try
		mockCli.tryHandleRetryReceipt(ctx, receipt, node)

		// canceled context on Acquire
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		mockCli.tryHandleRetryReceipt(canceledCtx, receipt, node)

		// panic recovery
		mockCli.PreRetryCallback = func(_ *events.Receipt, _ types.MessageID, _ int, _ *waE2E.Message) bool {
			panic("simulated panic in retry callback")
		}
		// Put message so callback is reached
		_ = mockCli.addRecentMessage(ctx, receipt.Chat, "msg-panic", &waE2E.Message{Conversation: proto.String("hi")}, nil)
		panicRcpt := &events.Receipt{
			MessageSource: types.MessageSource{Chat: receipt.Chat, Sender: receipt.Sender},
			MessageIDs:    []types.MessageID{"msg-panic"},
		}
		panicNode := &waBinary.Node{
			Tag:   "receipt",
			Attrs: waBinary.Attrs{"from": receipt.Sender, "id": "msg-panic", "type": "retry"},
			Content: []waBinary.Node{
				{Tag: "retry", Attrs: waBinary.Attrs{"id": "msg-panic", "count": "1", "t": "1700000000"}},
			},
		}
		assert.NotPanics(t, func() {
			mockCli.tryHandleRetryReceipt(ctx, panicRcpt, panicNode)
		})

		// immediateRequestMessageFromPhone
		mockCli.immediateRequestMessageFromPhone(ctx, &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   receipt.Chat,
				Sender: receipt.Sender,
			},
			ID: "msg-phone-req",
		})
	})
}
