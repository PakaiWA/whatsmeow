// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/util/random"

	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/gcmutil"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockCustomChatSettingsStore struct {
	wasaSecretID types.MessageID
	err          error
}

func (m *mockCustomChatSettingsStore) PutMutedUntil(_ context.Context, _ types.JID, _ time.Time) error {
	return nil
}
func (m *mockCustomChatSettingsStore) PutPinned(_ context.Context, _ types.JID, _ bool) error {
	return nil
}
func (m *mockCustomChatSettingsStore) PutArchived(_ context.Context, _ types.JID, _ bool) error {
	return nil
}
func (m *mockCustomChatSettingsStore) PutWASARootSecretID(_ context.Context, _ types.JID, id types.MessageID) error {
	m.wasaSecretID = id
	return nil
}
func (m *mockCustomChatSettingsStore) GetWASARootSecretID(_ context.Context, _ types.JID) (types.MessageID, error) {
	return m.wasaSecretID, m.err
}
func (m *mockCustomChatSettingsStore) GetChatSettings(_ context.Context, _ types.JID) (types.LocalChatSettings, error) {
	return types.LocalChatSettings{}, nil
}

type mockCustomMsgSecretStore struct {
	secrets map[string][]byte
	senders map[string]types.JID
	getErr  error
}

func newMockCustomMsgSecretStore() *mockCustomMsgSecretStore {
	return &mockCustomMsgSecretStore{
		secrets: make(map[string][]byte),
		senders: make(map[string]types.JID),
	}
}

func (m *mockCustomMsgSecretStore) PutMessageSecrets(_ context.Context, inserts []store.MessageSecretInsert) error {
	for _, ins := range inserts {
		key := ins.Chat.String() + ins.Sender.String() + string(ins.ID)
		m.secrets[key] = ins.Secret
		m.senders[key] = ins.Sender
	}
	return nil
}

func (m *mockCustomMsgSecretStore) PutMessageSecret(_ context.Context, chat, sender types.JID, id types.MessageID, secret []byte) error {
	key := chat.String() + sender.String() + string(id)
	m.secrets[key] = secret
	m.senders[key] = sender
	return nil
}

func (m *mockCustomMsgSecretStore) GetMessageSecret(_ context.Context, chat, sender types.JID, id types.MessageID) ([]byte, types.JID, error) {
	if m.getErr != nil {
		return nil, types.EmptyJID, m.getErr
	}
	key := chat.String() + sender.String() + string(id)
	sec, ok := m.secrets[key]
	if !ok {
		return nil, types.EmptyJID, nil
	}
	return sec, m.senders[key], nil
}

func TestMsgSecret_Helpers(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	botHKDF := applyBotMessageHKDF(secret)
	assert.Len(t, botHKDF, 32)

	pollOptions := []string{"Option A", "Option B"}
	hashes := HashPollOptions(pollOptions)
	require.Len(t, hashes, 2)
	assert.Len(t, hashes[0], 32)
	assert.Len(t, hashes[1], 32)

	creationMsg := (&Client{}).BuildPollCreation("Which color?", pollOptions, 1)
	require.NotNil(t, creationMsg.GetPollCreationMessage())
	assert.Equal(t, "Which color?", creationMsg.GetPollCreationMessage().GetName())
	assert.Len(t, creationMsg.GetPollCreationMessage().GetOptions(), 2)
	assert.Equal(t, uint32(1), creationMsg.GetPollCreationMessage().GetSelectableOptionsCount())
	assert.NotEmpty(t, creationMsg.GetMessageContextInfo().GetMessageSecret())

	creationMsgInvalidSelectable := (&Client{}).BuildPollCreation("Which color?", pollOptions, 5)
	assert.Equal(t, uint32(0), creationMsgInvalidSelectable.GetPollCreationMessage().GetSelectableOptionsCount())
}

func TestMsgSecret_GetOrigSenderFromKey_EdgeCases(t *testing.T) {
	senderJID := types.NewJID("sender1", types.DefaultUserServer)
	chatDirect := types.NewJID("direct1", types.DefaultUserServer)
	chatGroup := types.NewJID("group1", types.GroupServer)

	t.Run("fromMe is true returns msg.Info.Sender", func(t *testing.T) {
		msg := &events.Message{
			Info: types.MessageInfo{MessageSource: types.MessageSource{Sender: senderJID}},
		}
		key := &waCommon.MessageKey{FromMe: new(true)}
		res, err := getOrigSenderFromKey(msg, key)
		require.NoError(t, err)
		assert.Equal(t, senderJID, res)
	})

	t.Run("Direct chat parses RemoteJID", func(t *testing.T) {
		msg := &events.Message{
			Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chatDirect}},
		}
		remoteJIDStr := types.NewJID("peer1", types.DefaultUserServer).String()
		key := &waCommon.MessageKey{
			FromMe:    new(false),
			RemoteJID: new(remoteJIDStr),
		}
		res, err := getOrigSenderFromKey(msg, key)
		require.NoError(t, err)
		assert.Equal(t, "peer1", res.User)
	})

	t.Run("Group chat parses Participant", func(t *testing.T) {
		msg := &events.Message{
			Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chatGroup}},
		}
		participantStr := types.NewJID("member1", types.DefaultUserServer).String()
		key := &waCommon.MessageKey{
			FromMe:      new(false),
			Participant: new(participantStr),
		}
		res, err := getOrigSenderFromKey(msg, key)
		require.NoError(t, err)
		assert.Equal(t, "member1", res.User)
	})

	t.Run("Group chat participant with unexpected server fails", func(t *testing.T) {
		msg := &events.Message{
			Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chatGroup}},
		}
		broadcastStr := types.NewJID("bcast", types.BroadcastServer).String()
		key := &waCommon.MessageKey{
			FromMe:      new(false),
			Participant: new(broadcastStr),
		}
		_, err := getOrigSenderFromKey(msg, key)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected server")
	})
}

func TestMsgSecret_EncryptAndDecryptMsgSecret(t *testing.T) {
	ctx := context.Background()
	devID := types.NewJID("ownuser", types.DefaultUserServer)
	devLID := types.NewJID("ownlid", types.HiddenUserServer)
	dev := &store.Device{
		ID:  &devID,
		LID: devLID,
	}
	secretStore := newMockCustomMsgSecretStore()
	dev.MsgSecrets = secretStore
	cli := NewClient(dev, waLog.Noop)

	chatJID := types.NewJID("group1", types.GroupServer)
	origSender := types.NewJID("user1", types.DefaultUserServer)
	origMsgID := types.MessageID("ORIG-MSG-1")
	baseSecret := []byte("01234567890123456789012345678901")

	// Save original message secret
	err := secretStore.PutMessageSecret(ctx, chatJID, origSender, origMsgID, baseSecret)
	require.NoError(t, err)

	plaintext := []byte("secret payload data")
	ciphertext, iv, err := cli.encryptMsgSecret(ctx, devID, chatJID, origSender, origMsgID, EncSecretReaction, plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.Len(t, iv, 12)

	// Decrypt
	encObj := &waE2E.EncReactionMessage{
		TargetMessageKey: &waCommon.MessageKey{
			RemoteJID:   new(chatJID.String()),
			Participant: new(origSender.String()),
			ID:          new(string(origMsgID)),
			FromMe:      new(false),
		},
		EncPayload: ciphertext,
		EncIV:      iv,
	}
	evtMsg := &events.Message{
		Info: types.MessageInfo{
			ID: "REACTION-1",
			MessageSource: types.MessageSource{
				Chat:   chatJID,
				Sender: devID,
			},
		},
	}

	decrypted, err := cli.decryptMsgSecret(ctx, evtMsg, EncSecretReaction, encObj, encObj.TargetMessageKey)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestMsgSecret_DecryptMsgSecret_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.decryptMsgSecret(ctx, &events.Message{}, EncSecretReaction, nil, nil)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Secret not found", func(t *testing.T) {
		dev := &store.Device{
			ID:         &types.JID{User: "u", Server: types.DefaultUserServer},
			MsgSecrets: newMockCustomMsgSecretStore(),
		}
		cli := NewClient(dev, waLog.Noop)
		evtMsg := &events.Message{
			Info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Chat:   types.NewJID("u2", types.DefaultUserServer),
					Sender: types.NewJID("u2", types.DefaultUserServer),
				},
			},
		}
		key := &waCommon.MessageKey{
			RemoteJID: new(types.NewJID("u2", types.DefaultUserServer).String()),
			ID:        new("not-found-id"),
			FromMe:    new(false),
		}
		_, err := cli.decryptMsgSecret(ctx, evtMsg, EncSecretReaction, &waE2E.EncReactionMessage{}, key)
		require.ErrorIs(t, err, ErrOriginalMessageSecretNotFound)
	})

	t.Run("Store returns error", func(t *testing.T) {
		mockStore := newMockCustomMsgSecretStore()
		mockStore.getErr = errors.New("db error")
		dev := &store.Device{
			ID:         &types.JID{User: "u", Server: types.DefaultUserServer},
			MsgSecrets: mockStore,
		}
		cli := NewClient(dev, waLog.Noop)
		evtMsg := &events.Message{
			Info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Chat:   types.NewJID("u2", types.DefaultUserServer),
					Sender: types.NewJID("u2", types.DefaultUserServer),
				},
			},
		}
		key := &waCommon.MessageKey{
			RemoteJID: new(types.NewJID("u2", types.DefaultUserServer).String()),
			ID:        new("id-1"),
			FromMe:    new(false),
		}
		_, err := cli.decryptMsgSecret(ctx, evtMsg, EncSecretReaction, &waE2E.EncReactionMessage{}, key)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get original message secret key")
	})
}

func TestMsgSecret_Poll_Reaction_Comment_E2E(t *testing.T) {
	ctx := context.Background()
	devID := types.NewJID("ownuser", types.DefaultUserServer)
	devLID := types.NewJID("ownlid", types.HiddenUserServer)
	dev := &store.Device{
		ID:  &devID,
		LID: devLID,
	}
	secretStore := newMockCustomMsgSecretStore()
	dev.MsgSecrets = secretStore
	cli := NewClient(dev, waLog.Noop)

	chatJID := types.NewJID("user2", types.DefaultUserServer)
	origID := types.MessageID("POLL-1")
	baseSecret := []byte("01234567890123456789012345678901")
	err := secretStore.PutMessageSecret(ctx, chatJID, chatJID, origID, baseSecret)
	require.NoError(t, err)

	pollInfo := &types.MessageInfo{
		ID: origID,
		MessageSource: types.MessageSource{
			Chat:   chatJID,
			Sender: chatJID,
		},
	}

	// 1. Poll Vote
	voteMsg, err := cli.BuildPollVote(ctx, pollInfo, []string{"Option 1"})
	require.NoError(t, err)
	require.NotNil(t, voteMsg.GetPollUpdateMessage())

	decryptedVote, err := cli.DecryptPollVote(ctx, &events.Message{
		Info: types.MessageInfo{
			ID: "VOTE-1",
			MessageSource: types.MessageSource{
				Chat:   chatJID,
				Sender: devID,
			},
		},
		Message: voteMsg,
	})
	require.NoError(t, err)
	require.Len(t, decryptedVote.GetSelectedOptions(), 1)
	assert.Equal(t, HashPollOptions([]string{"Option 1"})[0], decryptedVote.GetSelectedOptions()[0])

	// 2. Reaction
	reactionProto := &waE2E.ReactionMessage{
		Text: new("👍"),
		Key: &waCommon.MessageKey{
			RemoteJID: new(chatJID.String()),
			ID:        new(string(origID)),
			FromMe:    new(false),
		},
	}
	encReaction, err := cli.EncryptReaction(ctx, pollInfo, reactionProto)
	require.NoError(t, err)

	decryptedReaction, err := cli.DecryptReaction(ctx, &events.Message{
		Info: types.MessageInfo{
			ID: "REACT-1",
			MessageSource: types.MessageSource{
				Chat:   chatJID,
				Sender: devLID,
			},
		},
		Message: &waE2E.Message{EncReactionMessage: encReaction},
	})
	require.NoError(t, err)
	assert.Equal(t, "👍", decryptedReaction.GetText())

	// 3. Comment
	commentProto := &waE2E.Message{Conversation: new("my comment")}
	encCommentMsg, err := cli.EncryptComment(ctx, pollInfo, commentProto)
	require.NoError(t, err)

	decryptedComment, err := cli.DecryptComment(ctx, &events.Message{
		Info: types.MessageInfo{
			ID: "COMMENT-1",
			MessageSource: types.MessageSource{
				Chat:   chatJID,
				Sender: devLID,
			},
		},
		Message: encCommentMsg,
	})
	require.NoError(t, err)
	assert.Equal(t, "my comment", decryptedComment.GetConversation())
}

func TestMsgSecret_DecryptSecretEncryptedMessage(t *testing.T) {
	ctx := context.Background()
	devID := types.NewJID("ownuser", types.DefaultUserServer)
	devLID := types.NewJID("ownlid", types.HiddenUserServer)
	dev := &store.Device{
		ID:  &devID,
		LID: devLID,
	}
	secretStore := newMockCustomMsgSecretStore()
	dev.MsgSecrets = secretStore
	cli := NewClient(dev, waLog.Noop)

	chatJID := types.NewJID("user2", types.DefaultUserServer)
	origID := types.MessageID("EVENT-1")
	baseSecret := []byte("01234567890123456789012345678901")
	_ = secretStore.PutMessageSecret(ctx, chatJID, chatJID, origID, baseSecret)

	innerEditedMsg := &waE2E.Message{Conversation: new("edited message text")}
	innerBytes, err := proto.Marshal(innerEditedMsg)
	require.NoError(t, err)

	ciphertext, iv, err := cli.encryptMsgSecret(ctx, devID, chatJID, chatJID, origID, EncSecretMessageEdit, innerBytes)
	require.NoError(t, err)

	secMsg := &waE2E.SecretEncryptedMessage{
		TargetMessageKey: &waCommon.MessageKey{
			RemoteJID: new(chatJID.String()),
			ID:        new(string(origID)),
			FromMe:    new(false),
		},
		SecretEncType: waE2E.SecretEncryptedMessage_MESSAGE_EDIT.Enum(),
		EncPayload:    ciphertext,
		EncIV:         iv,
	}

	t.Run("Valid SecretEncryptedMessage", func(t *testing.T) {
		evt := &events.Message{
			Info: types.MessageInfo{
				ID: "EDIT-1",
				MessageSource: types.MessageSource{
					Chat:   chatJID,
					Sender: devID,
				},
			},
			Message: &waE2E.Message{SecretEncryptedMessage: secMsg},
		}

		decrypted, err := cli.DecryptSecretEncryptedMessage(ctx, evt)
		require.NoError(t, err)
		assert.Equal(t, "edited message text", decrypted.GetConversation())
	})

	t.Run("Unsupported SecretEncType", func(t *testing.T) {
		badSecMsg := proto.Clone(secMsg).(*waE2E.SecretEncryptedMessage)
		badSecMsg.SecretEncType = waE2E.SecretEncryptedMessage_UNKNOWN.Enum()

		evt := &events.Message{
			Info:    types.MessageInfo{},
			Message: &waE2E.Message{SecretEncryptedMessage: badSecMsg},
		}
		_, err := cli.DecryptSecretEncryptedMessage(ctx, evt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported secret enc type")
	})

	t.Run("Not secret encrypted message", func(t *testing.T) {
		evt := &events.Message{Message: &waE2E.Message{}}
		_, err := cli.DecryptSecretEncryptedMessage(ctx, evt)
		require.ErrorIs(t, err, ErrNotSecretEncryptedMessage)
	})
}

func TestMsgSecret_WASA_And_Bot(t *testing.T) {
	ctx := context.Background()
	botJID := types.NewJID("13135550002", types.DefaultUserServer)
	devID := types.NewJID("ownuser", types.DefaultUserServer)
	devLID := types.NewJID("ownlid", types.HiddenUserServer)

	dev := &store.Device{
		ID:           &devID,
		LID:          devLID,
		MsgSecrets:   newMockCustomMsgSecretStore(),
		ChatSettings: &mockCustomChatSettingsStore{wasaSecretID: "WASA-ROOT-1"},
	}
	cli := NewClient(dev, waLog.Noop)

	t.Run("ProtocolMessage REQUEST_WELCOME_MESSAGE marshals directly", func(t *testing.T) {
		msg := &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type: waE2E.ProtocolMessage_REQUEST_WELCOME_MESSAGE.Enum(),
			},
		}
		bytes, err := cli.encryptWASAMessage(ctx, botJID, "REQ-1", msg)
		require.NoError(t, err)
		assert.NotEmpty(t, bytes)
	})

	t.Run("WASA message encryption and bot decryption", func(t *testing.T) {
		rootSecret := []byte("01234567890123456789012345678901")
		_ = dev.MsgSecrets.PutMessageSecret(ctx, botJID, devLID, "WASA-ROOT-1", rootSecret)

		textMsg := &waE2E.Message{
			Conversation: new("hello ai bot"),
		}
		encBytes, err := cli.encryptWASAMessage(ctx, botJID, "MSG-BOT-1", textMsg)
		require.NoError(t, err)
		assert.NotEmpty(t, encBytes)

		// Test decryptBotMessage
		info := &types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: botJID,
			},
		}
		secretMsgPayload := []byte("bot response text")
		botSecretKey, botAddData := generateMsgSecretKey("", info.Sender, "MSG-BOT-2", devLID, applyBotMessageHKDF(rootSecret))
		botIV := random.Bytes(12)
		botCiphertext, err := gcmutil.Encrypt(botSecretKey, botIV, secretMsgPayload, botAddData)
		require.NoError(t, err)

		msMsg := &waE2E.MessageSecretMessage{
			EncPayload: botCiphertext,
			EncIV:      botIV,
		}
		decrypted, err := cli.decryptBotMessage(ctx, rootSecret, msMsg, "MSG-BOT-2", devLID, info)
		require.NoError(t, err)
		assert.Equal(t, secretMsgPayload, decrypted)
	})

	t.Run("encryptWASAMessage validation errors", func(t *testing.T) {
		dummyMsg := &waE2E.Message{Conversation: new("test")}

		// nil ChatSettings
		cliNoChatSettings := NewClient(&store.Device{ID: &devID, LID: devLID}, waLog.Noop)
		_, err := cliNoChatSettings.encryptWASAMessage(ctx, botJID, "ID-1", dummyMsg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chat settings store is required")

		// empty root ID
		cliEmptyRoot := NewClient(&store.Device{
			ID:           &devID,
			LID:          devLID,
			ChatSettings: &mockCustomChatSettingsStore{wasaSecretID: ""},
		}, waLog.Noop)
		_, err = cliEmptyRoot.encryptWASAMessage(ctx, botJID, "ID-1", dummyMsg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active WASA root secret")

		// empty own LID
		cliEmptyLID := NewClient(&store.Device{
			ID:           &devID,
			ChatSettings: &mockCustomChatSettingsStore{wasaSecretID: "ROOT-1"},
		}, waLog.Noop)
		_, err = cliEmptyLID.encryptWASAMessage(ctx, botJID, "ID-1", dummyMsg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "own LID not available")

		// invalid secret length
		shortStore := newMockCustomMsgSecretStore()
		_ = shortStore.PutMessageSecret(ctx, botJID, devLID, "ROOT-SHORT", []byte("short"))
		cliShortSecret := NewClient(&store.Device{
			ID:           &devID,
			LID:          devLID,
			MsgSecrets:   shortStore,
			ChatSettings: &mockCustomChatSettingsStore{wasaSecretID: "ROOT-SHORT"},
		}, waLog.Noop)
		_, err = cliShortSecret.encryptWASAMessage(ctx, botJID, "ID-1", dummyMsg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid WASA root secret")

		// document with caption message
		docStore := newMockCustomMsgSecretStore()
		_ = docStore.PutMessageSecret(ctx, botJID, devLID, "ROOT-DOC", make([]byte, 32))
		cliDoc := NewClient(&store.Device{
			ID:           &devID,
			LID:          devLID,
			MsgSecrets:   docStore,
			ChatSettings: &mockCustomChatSettingsStore{wasaSecretID: "ROOT-DOC"},
		}, waLog.Noop)
		docMsg := &waE2E.Message{
			DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{
					DocumentMessage: &waE2E.DocumentMessage{Title: new("document title")},
				},
			},
		}
		bytes, err := cliDoc.encryptWASAMessage(ctx, botJID, "ID-DOC", docMsg)
		require.NoError(t, err)
		assert.NotEmpty(t, bytes)
	})
}

func TestMsgSecret_DecryptMsgSecret_OrigSenderHack(t *testing.T) {
	ctx := context.Background()
	chatJID := types.NewJID("group1", types.GroupServer)
	origSenderKey := types.NewJID("user-pn", types.DefaultUserServer)
	storedOrigSender := types.NewJID("user-lid", types.HiddenUserServer)
	voterJID := types.NewJID("voter", types.DefaultUserServer)

	secretStore := newMockCustomMsgSecretStore()
	dev := &store.Device{
		ID:         &voterJID,
		MsgSecrets: secretStore,
	}
	cli := NewClient(dev, waLog.Noop)

	origMsgID := types.MessageID("POLL-1")
	baseSecret := []byte("01234567890123456789012345678901")

	// Store message secret under origSenderKey, but specify storedOrigSender in senders map
	mapKey := chatJID.String() + origSenderKey.String() + string(origMsgID)
	secretStore.secrets[mapKey] = baseSecret
	secretStore.senders[mapKey] = storedOrigSender

	// Encrypt using storedOrigSender
	plaintext := []byte("encrypted poll vote payload")
	secretKey, addData := generateMsgSecretKey(EncSecretPollVote, voterJID, origMsgID, storedOrigSender, baseSecret)
	iv := random.Bytes(12)
	ciphertext, err := gcmutil.Encrypt(secretKey, iv, plaintext, addData)
	require.NoError(t, err)

	encObj := &waE2E.PollEncValue{
		EncPayload: ciphertext,
		EncIV:      iv,
	}
	evtMsg := &events.Message{
		Info: types.MessageInfo{
			ID: "VOTE-1",
			MessageSource: types.MessageSource{
				Chat:   chatJID,
				Sender: voterJID,
			},
		},
	}
	key := &waCommon.MessageKey{
		RemoteJID:   new(chatJID.String()),
		Participant: new(origSenderKey.String()),
		ID:          new(string(origMsgID)),
		FromMe:      new(false),
	}

	decrypted, err := cli.decryptMsgSecret(ctx, evtMsg, EncSecretPollVote, encObj, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
