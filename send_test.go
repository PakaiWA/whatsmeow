// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestSend_GenerateIDs(t *testing.T) {
	id1 := GenerateMessageID()
	id2 := GenerateMessageID()
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	fbID := GenerateFacebookMessageID()
	assert.NotZero(t, fbID)

	cli := NewClient(&store.Device{}, waLog.Noop)
	cliID := cli.GenerateMessageID()
	assert.NotEmpty(t, cliID)

	cli.MessengerConfig = &MessengerConfig{}
	cliFBID := cli.GenerateMessageID()
	assert.NotEmpty(t, cliFBID)
}

func TestSend_MessageDebugTimings(t *testing.T) {
	timings := MessageDebugTimings{
		LIDFetch:        5 * time.Millisecond,
		Queue:           10 * time.Millisecond,
		GetParticipants: 20 * time.Millisecond,
		GetDevices:      30 * time.Millisecond,
		GroupEncrypt:    40 * time.Millisecond,
		PeerEncrypt:     50 * time.Millisecond,
		Send:            60 * time.Millisecond,
		Resp:            70 * time.Millisecond,
		Retry:           80 * time.Millisecond,
	}

	logger := zerolog.Nop()
	evt := logger.Info()
	timings.MarshalZerologObject(evt)
	// Verified it does not panic and marshals all durations
}

func TestSend_Builders(t *testing.T) {
	chat := types.NewJID("12345", types.DefaultUserServer)
	sender := types.NewJID("67890", types.DefaultUserServer)
	group := types.NewJID("mygroup", types.GroupServer)
	ownJID := types.NewJID("me", types.DefaultUserServer)

	cli := NewClient(&store.Device{
		ID: &ownJID,
	}, waLog.Noop)

	t.Run("BuildMessageKey", func(t *testing.T) {
		keyDM := cli.BuildMessageKey(chat, types.EmptyJID, "msg-1")
		assert.Equal(t, "12345@s.whatsapp.net", keyDM.GetRemoteJID())
		assert.Equal(t, "msg-1", keyDM.GetID())
		assert.True(t, keyDM.GetFromMe())

		keyGroup := cli.BuildMessageKey(group, sender, "msg-2")
		assert.Equal(t, "mygroup@g.us", keyGroup.GetRemoteJID())
		assert.Equal(t, "67890@s.whatsapp.net", keyGroup.GetParticipant())
		assert.False(t, keyGroup.GetFromMe())
	})

	t.Run("BuildRevoke", func(t *testing.T) {
		msg := cli.BuildRevoke(chat, sender, "msg-123")
		require.NotNil(t, msg.GetProtocolMessage())
		assert.Equal(t, waE2E.ProtocolMessage_REVOKE, msg.GetProtocolMessage().GetType())
		assert.Equal(t, "msg-123", msg.GetProtocolMessage().GetKey().GetID())
	})

	t.Run("BuildReaction", func(t *testing.T) {
		msg := cli.BuildReaction(chat, sender, "msg-123", "❤️")
		require.NotNil(t, msg.GetReactionMessage())
		assert.Equal(t, "❤️", msg.GetReactionMessage().GetText())
		assert.Equal(t, "msg-123", msg.GetReactionMessage().GetKey().GetID())
	})

	t.Run("BuildUnavailableMessageRequest", func(t *testing.T) {
		msg := cli.BuildUnavailableMessageRequest(chat, sender, "msg-unavail")
		require.NotNil(t, msg.GetProtocolMessage())
		assert.Equal(t, waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE, msg.GetProtocolMessage().GetType())
		pReq := msg.GetProtocolMessage().GetPeerDataOperationRequestMessage()
		require.NotNil(t, pReq)
		require.Len(t, pReq.GetPlaceholderMessageResendRequest(), 1)
		assert.Equal(t, "msg-unavail", pReq.GetPlaceholderMessageResendRequest()[0].GetMessageKey().GetID())
	})

	t.Run("BuildHistorySyncRequest", func(t *testing.T) {
		info := &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   chat,
				Sender: sender,
			},
			ID:        "msg-hist",
			Timestamp: time.Unix(1700000000, 0),
		}
		msg := cli.BuildHistorySyncRequest(info, 50)
		require.NotNil(t, msg.GetProtocolMessage())
		assert.Equal(t, waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE, msg.GetProtocolMessage().GetType())
		pReq := msg.GetProtocolMessage().GetPeerDataOperationRequestMessage()
		require.NotNil(t, pReq)
		req := pReq.GetHistorySyncOnDemandRequest()
		require.NotNil(t, req)
		assert.Equal(t, int32(50), req.GetOnDemandMsgCount())
		assert.Equal(t, "msg-hist", req.GetOldestMsgID())
	})

	t.Run("BuildEdit", func(t *testing.T) {
		newMsg := &waE2E.Message{Conversation: proto.String("updated text")}
		editMsg := cli.BuildEdit(chat, "msg-orig", newMsg)
		require.NotNil(t, editMsg.GetEditedMessage())
		assert.Equal(t, waE2E.ProtocolMessage_MESSAGE_EDIT, editMsg.GetEditedMessage().GetMessage().GetProtocolMessage().GetType())
		assert.Equal(t, "updated text", editMsg.GetEditedMessage().GetMessage().GetProtocolMessage().GetEditedMessage().GetConversation())
	})
}

func TestSend_DisappearingTimer(t *testing.T) {
	t.Run("ParseDisappearingTimerString", func(t *testing.T) {
		d, ok := ParseDisappearingTimerString("24h")
		assert.True(t, ok)
		assert.Equal(t, 24*time.Hour, d)

		d, ok = ParseDisappearingTimerString("7d")
		assert.True(t, ok)
		assert.Equal(t, 7*24*time.Hour, d)

		d, ok = ParseDisappearingTimerString("90d")
		assert.True(t, ok)
		assert.Equal(t, 90*24*time.Hour, d)

		d, ok = ParseDisappearingTimerString("off")
		assert.True(t, ok)
		assert.Equal(t, time.Duration(0), d)

		_, ok = ParseDisappearingTimerString("invalid-timer")
		assert.False(t, ok)
	})

	t.Run("SetDisappearingTimer", func(t *testing.T) {
		ctx := context.Background()
		chatGroup := types.NewJID("group", types.GroupServer)
		chatNews := types.NewJID("news", types.NewsletterServer)

		cli := NewClient(&store.Device{
			ID:   &types.JID{User: "me", Server: types.DefaultUserServer},
			LIDs: newMockLIDStore(),
		}, waLog.Noop)

		// Group chat with sendIQHook
		var lastIQ infoQuery
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			lastIQ = query
			return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}}, nil
		}

		err := cli.SetDisappearingTimer(ctx, chatGroup, 24*time.Hour, time.Now())
		require.NoError(t, err)
		assert.Equal(t, chatGroup, lastIQ.To)

		// Group chat turn off (timer = 0)
		err = cli.SetDisappearingTimer(ctx, chatGroup, 0, time.Now())
		require.NoError(t, err)

		// Unsupported server (Newsletter)
		err = cli.SetDisappearingTimer(ctx, chatNews, 24*time.Hour, time.Now())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can't set disappearing time in a newsletter chat")
	})
}

func TestSend_MessageTypesAndAttributes(t *testing.T) {
	t.Run("getTypeFromMessage", func(t *testing.T) {
		assert.Equal(t, "text", getTypeFromMessage(&waE2E.Message{Conversation: proto.String("hi")}))
		assert.Equal(t, "text", getTypeFromMessage(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hi")}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Title: proto.String("URL Title")}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{ContactMessage: &waE2E.ContactMessage{}}))
		assert.Equal(t, "poll", getTypeFromMessage(&waE2E.Message{PollCreationMessage: &waE2E.PollCreationMessage{}}))
		assert.Equal(t, "poll", getTypeFromMessage(&waE2E.Message{PollUpdateMessage: &waE2E.PollUpdateMessage{}}))
		assert.Equal(t, "reaction", getTypeFromMessage(&waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{}}))
		assert.Equal(t, "text", getTypeFromMessage(&waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{}}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{}}))
		assert.Equal(t, "reaction", getTypeFromMessage(&waE2E.Message{EncReactionMessage: &waE2E.EncReactionMessage{}}))
	})

	t.Run("getMediaTypeFromMessage", func(t *testing.T) {
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}))
		assert.Equal(t, "sticker", getMediaTypeFromMessage(&waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}))
		assert.Equal(t, "document", getMediaTypeFromMessage(&waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}}))
		assert.Equal(t, "audio", getMediaTypeFromMessage(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(false)}}))
		assert.Equal(t, "ptt", getMediaTypeFromMessage(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true)}}))
		assert.Equal(t, "video", getMediaTypeFromMessage(&waE2E.Message{VideoMessage: &waE2E.VideoMessage{GifPlayback: proto.Bool(false)}}))
		assert.Equal(t, "gif", getMediaTypeFromMessage(&waE2E.Message{VideoMessage: &waE2E.VideoMessage{GifPlayback: proto.Bool(true)}}))
		assert.Equal(t, "url", getMediaTypeFromMessage(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Title: proto.String("Web Link")}}))
		assert.Equal(t, "vcard", getMediaTypeFromMessage(&waE2E.Message{ContactMessage: &waE2E.ContactMessage{}}))
		assert.Equal(t, "contact_array", getMediaTypeFromMessage(&waE2E.Message{ContactsArrayMessage: &waE2E.ContactsArrayMessage{}}))
		assert.Equal(t, "list", getMediaTypeFromMessage(&waE2E.Message{ListMessage: &waE2E.ListMessage{}}))
		assert.Equal(t, "list_response", getMediaTypeFromMessage(&waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{}}))
		assert.Equal(t, "buttons_response", getMediaTypeFromMessage(&waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{}}))
		assert.Equal(t, "order", getMediaTypeFromMessage(&waE2E.Message{OrderMessage: &waE2E.OrderMessage{}}))
		assert.Equal(t, "product", getMediaTypeFromMessage(&waE2E.Message{ProductMessage: &waE2E.ProductMessage{}}))
		assert.Equal(t, "native_flow_response", getMediaTypeFromMessage(&waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{}}))
		assert.Empty(t, getMediaTypeFromMessage(&waE2E.Message{Conversation: proto.String("text")}))
	})

	t.Run("getButtonTypeFromMessage", func(t *testing.T) {
		assert.Equal(t, "buttons", getButtonTypeFromMessage(&waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{}}))
		assert.Equal(t, "buttons_response", getButtonTypeFromMessage(&waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{}}))
		assert.Equal(t, "list", getButtonTypeFromMessage(&waE2E.Message{ListMessage: &waE2E.ListMessage{}}))
		assert.Equal(t, "list_response", getButtonTypeFromMessage(&waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{}}))
		assert.Equal(t, "interactive_response", getButtonTypeFromMessage(&waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{}}))
		assert.Empty(t, getButtonTypeFromMessage(&waE2E.Message{Conversation: proto.String("text")}))
	})

	t.Run("getButtonAttributes", func(t *testing.T) {
		listMsg := &waE2E.Message{
			ListMessage: &waE2E.ListMessage{
				ListType: waE2E.ListMessage_SINGLE_SELECT.Enum(),
			},
		}
		attrs := getButtonAttributes(listMsg)
		assert.Equal(t, "2", attrs["v"])
		assert.Equal(t, "single_select", attrs["type"])

		tmplMsg := &waE2E.Message{
			TemplateMessage: &waE2E.TemplateMessage{},
		}
		assert.Empty(t, getButtonAttributes(tmplMsg))
		assert.Empty(t, getButtonAttributes(&waE2E.Message{Conversation: proto.String("none")}))
	})

	t.Run("getTypeFromMessage wrappers", func(t *testing.T) {
		inner := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
		fp := &waE2E.FutureProofMessage{Message: inner}
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{ViewOnceMessage: fp}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{ViewOnceMessageV2: fp}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{ViewOnceMessageV2Extension: fp}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{LottieStickerMessage: fp}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{EphemeralMessage: fp}))
		assert.Equal(t, "media", getTypeFromMessage(&waE2E.Message{DocumentWithCaptionMessage: fp}))
	})

	t.Run("getMediaTypeFromMessage wrappers", func(t *testing.T) {
		inner := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
		fp := &waE2E.FutureProofMessage{Message: inner}
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{ViewOnceMessage: fp}))
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{ViewOnceMessageV2: fp}))
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{ViewOnceMessageV2Extension: fp}))
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{LottieStickerMessage: fp}))
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{EphemeralMessage: fp}))
		assert.Equal(t, "image", getMediaTypeFromMessage(&waE2E.Message{DocumentWithCaptionMessage: fp}))
	})

	t.Run("getButtonTypeFromMessage wrappers", func(t *testing.T) {
		inner := &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{}}
		fp := &waE2E.FutureProofMessage{Message: inner}
		assert.Equal(t, "buttons", getButtonTypeFromMessage(&waE2E.Message{ViewOnceMessage: fp}))
		assert.Equal(t, "buttons", getButtonTypeFromMessage(&waE2E.Message{ViewOnceMessageV2: fp}))
		assert.Equal(t, "buttons", getButtonTypeFromMessage(&waE2E.Message{EphemeralMessage: fp}))
	})

	t.Run("getButtonAttributes wrappers", func(t *testing.T) {
		inner := &waE2E.Message{ListMessage: &waE2E.ListMessage{ListType: waE2E.ListMessage_SINGLE_SELECT.Enum()}}
		fp := &waE2E.FutureProofMessage{Message: inner}
		assert.NotEmpty(t, getButtonAttributes(&waE2E.Message{ViewOnceMessage: fp}))
		assert.NotEmpty(t, getButtonAttributes(&waE2E.Message{ViewOnceMessageV2: fp}))
		assert.NotEmpty(t, getButtonAttributes(&waE2E.Message{EphemeralMessage: fp}))
		assert.Empty(t, getButtonAttributes(&waE2E.Message{TemplateMessage: &waE2E.TemplateMessage{}}))
	})

	t.Run("getEditAttribute", func(t *testing.T) {
		assert.Equal(t, types.EditAttributeMessageEdit, getEditAttribute(&waE2E.Message{EditedMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Type:          waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
					Key:           &waCommon.MessageKey{ID: proto.String("1")},
					EditedMessage: &waE2E.Message{Conversation: proto.String("edited")},
				},
			},
		}}))

		revokeMsg := &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type: waE2E.ProtocolMessage_REVOKE.Enum(),
				Key:  &waCommon.MessageKey{FromMe: proto.Bool(true)},
			},
		}
		assert.Equal(t, types.EditAttributeSenderRevoke, getEditAttribute(revokeMsg))

		adminRevokeMsg := &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type: waE2E.ProtocolMessage_REVOKE.Enum(),
				Key:  &waCommon.MessageKey{FromMe: proto.Bool(false)},
			},
		}
		assert.Equal(t, types.EditAttributeAdminRevoke, getEditAttribute(adminRevokeMsg))

		removeReactionMsg := &waE2E.Message{
			ReactionMessage: &waE2E.ReactionMessage{Text: proto.String("")},
		}
		assert.Equal(t, types.EditAttributeSenderRevoke, getEditAttribute(removeReactionMsg))

		pinMsg := &waE2E.Message{
			PinInChatMessage: &waE2E.PinInChatMessage{},
		}
		assert.Equal(t, types.EditAttributePinInChat, getEditAttribute(pinMsg))
	})
}

func TestSend_MarshalMessage(t *testing.T) {
	newsJID := types.NewJID("news", types.NewsletterServer)
	pt, dsm, err := marshalMessage(newsJID, nil)
	require.NoError(t, err)
	assert.Nil(t, pt)
	assert.Nil(t, dsm)

	msg := &waE2E.Message{Conversation: proto.String("hello")}
	toUser := types.NewJID("12345", types.DefaultUserServer)
	toGroup := types.NewJID("group", types.GroupServer)

	plaintext, dsm, err := marshalMessage(toUser, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, plaintext)
	assert.NotEmpty(t, dsm)

	plaintextGroup, dsmGroup, err := marshalMessage(toGroup, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, plaintextGroup)
	assert.Nil(t, dsmGroup)
}

func TestSend_ParticipantListHashV2(t *testing.T) {
	jids := []types.JID{
		types.NewJID("100", types.DefaultUserServer),
		types.NewJID("200", types.DefaultUserServer),
	}
	h1 := participantListHashV2(jids)
	assert.NotEmpty(t, h1)

	// Same JIDs in different order produce same hash
	jidsRev := []types.JID{jids[1], jids[0]}
	h2 := participantListHashV2(jidsRev)
	assert.Equal(t, h1, h2)
}

func TestSend_SendMessage_Validations(t *testing.T) {
	ctx := context.Background()
	user := types.NewJID("12345", types.DefaultUserServer)
	msg := &waE2E.Message{Conversation: proto.String("valid")}

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.SendMessage(ctx, user, msg)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Multiple extra params", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.SendMessage(ctx, user, msg, SendRequestExtra{}, SendRequestExtra{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only one extra parameter")
	})

	t.Run("Recipient AD JID without Peer", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		adJID := types.NewADJID("12345", 1, 2)
		_, err := cli.SendMessage(ctx, adJID, msg)
		require.ErrorIs(t, err, ErrRecipientADJID)
	})

	t.Run("Not logged in", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.SendMessage(ctx, user, msg)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("Unknown server returns ErrUnknownServer", func(t *testing.T) {
		unknownJID := types.NewJID("user", "random_server")
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "me", Server: types.DefaultUserServer},
		}, waLog.Noop)
		_, err := cli.SendMessage(ctx, unknownJID, msg)
		require.ErrorIs(t, err, ErrUnknownServer)
	})

	t.Run("SendPeerMessage not logged in", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.SendPeerMessage(ctx, msg)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("RevokeMessage not logged in", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.RevokeMessage(ctx, user, "msg-123")
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("Invalid inline bot JID", func(t *testing.T) {
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "me", Server: types.DefaultUserServer},
		}, waLog.Noop)
		cli.isLoggedIn.Store(true)
		_, err := cli.SendMessage(ctx, user, msg, SendRequestExtra{InlineBotJID: types.NewJID("notbot", types.DefaultUserServer)})
		require.ErrorIs(t, err, ErrInvalidInlineBotID)
	})
}

func TestSend_MakeDeviceIdentityNode(t *testing.T) {
	identityKey := keys.NewKeyPair()
	device := &store.Device{
		IdentityKey: identityKey,
	}
	cli := NewClient(device, waLog.Noop)
	node := cli.makeDeviceIdentityNode()
	assert.Equal(t, "device-identity", node.Tag)
}

func TestSend_ConnectedFull(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	targetJID := types.NewJID("62812345678", types.DefaultUserServer)
	targetLID := types.NewJID("111111", types.HiddenUserServer)
	_, err := establishSignalSession(ctx, cli, targetLID)
	require.NoError(t, err)
	_, err = establishSignalSession(ctx, cli, targetJID)
	require.NoError(t, err)
	_, err = establishSignalSession(ctx, cli, cli.getOwnID())
	require.NoError(t, err)

	_ = cli.Store.LIDs.PutLIDMapping(ctx, targetLID, targetJID)
	_ = cli.Store.LIDs.PutLIDMapping(ctx, targetLID, cli.getOwnID().ToNonAD())
	cli.userDevicesCache[targetLID] = deviceCache{devices: []types.JID{targetLID}}
	cli.userDevicesCache[targetJID] = deviceCache{devices: []types.JID{targetJID}}
	cli.userDevicesCache[cli.getOwnID().ToNonAD()] = deviceCache{devices: []types.JID{cli.getOwnID()}}
	cli.userDevicesCache[cli.getOwnLID().ToNonAD()] = deviceCache{devices: []types.JID{cli.getOwnLID()}}

	var customFulfillLock sync.Mutex
	var customFulfill func(id string) *waBinary.Node

	stopFulfill := make(chan struct{})
	defer close(stopFulfill)
	go func() {
		for {
			select {
			case <-stopFulfill:
				return
			case <-time.After(5 * time.Millisecond):
				cli.responseWaitersLock.Lock()
				for id := range cli.responseWaiters {
					cli.responseWaitersLock.Unlock()

					customFulfillLock.Lock()
					fn := customFulfill
					customFulfillLock.Unlock()

					if fn != nil {
						node := fn(id)
						if node != nil {
							cli.receiveResponse(context.Background(), node)
						}
					} else {
						cli.receiveResponse(context.Background(), &waBinary.Node{
							Tag:   "ack",
							Attrs: waBinary.Attrs{"id": id, "class": "receipt", "type": "result"},
						})
					}
					cli.responseWaitersLock.Lock()
				}
				cli.responseWaitersLock.Unlock()
			}
		}
	}()

	t.Run("Send DM message with response ack", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		msg := &waE2E.Message{Conversation: proto.String("hello DM")}
		resp, err := cli.SendMessage(tCtx, targetLID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send Group message", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		groupJID := types.NewJID("1234567-group", types.GroupServer)
		cli.groupCacheLock.Lock()
		cli.groupCache[groupJID] = &groupMetaCache{
			Members: []types.JID{cli.getOwnID(), targetLID},
		}
		cli.groupCacheLock.Unlock()
		msg := &waE2E.Message{Conversation: proto.String("hello group")}
		resp, err := cli.SendMessage(tCtx, groupJID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send group message addressing mode LID", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		lidGroup := types.NewJID("lid-group", types.GroupServer)
		cli.groupCacheLock.Lock()
		cli.groupCache[lidGroup] = &groupMetaCache{
			AddressingMode: types.AddressingModeLID,
			Members:        []types.JID{cli.getOwnID(), targetLID},
		}
		cli.groupCacheLock.Unlock()
		resp, err := cli.SendMessage(tCtx, lidGroup, &waE2E.Message{Conversation: proto.String("lid group")})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send group message community announcement", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		annGroup := types.NewJID("ann-group", types.GroupServer)
		cli.groupCacheLock.Lock()
		cli.groupCache[annGroup] = &groupMetaCache{
			CommunityAnnouncementGroup: true,
			Members:                    []types.JID{cli.getOwnID(), targetLID},
		}
		cli.groupCacheLock.Unlock()
		resp, err := cli.SendMessage(tCtx, annGroup, &waE2E.Message{Conversation: proto.String("ann group")}, SendRequestExtra{
			Meta: &types.MsgMetaInfo{},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send message with inline bot", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		botJID := types.NewJID("123", types.BotServer)
		_, err := establishSignalSession(ctx, cli, botJID)
		require.NoError(t, err)
		cli.userDevicesCache[botJID] = deviceCache{devices: []types.JID{botJID}}
		resp, err := cli.SendMessage(tCtx, targetLID, &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hi bot")},
		}, SendRequestExtra{InlineBotJID: botJID})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send message to bot directly", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		botDirect := types.NewJID("musebot", types.BotServer)
		_, err := establishSignalSession(ctx, cli, botDirect)
		require.NoError(t, err)
		cli.userDevicesCache[botDirect] = deviceCache{devices: []types.JID{botDirect}}
		resp, err := cli.SendMessage(tCtx, botDirect, &waE2E.Message{
			Conversation: proto.String("hello direct bot"),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send message with buttons", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		btnMsg := &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{}}
		resp, err := cli.SendMessage(tCtx, targetLID, btnMsg)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send message reaction", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		reactMsg := &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Text: proto.String("👍")}}
		resp, err := cli.SendMessage(tCtx, targetLID, reactMsg)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send message with message secret", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		msg := &waE2E.Message{
			Conversation: proto.String("secret message"),
			MessageContextInfo: &waE2E.MessageContextInfo{
				MessageSecret: []byte("12345678901234567890123456789012"),
			},
		}
		resp, err := cli.SendMessage(tCtx, targetLID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send message with Meta and AdditionalNodes", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		depLID := true
		msg := &waE2E.Message{Conversation: proto.String("with meta")}
		resp, err := cli.SendMessage(tCtx, targetLID, msg, SendRequestExtra{
			Timeout: 5 * time.Second,
			Meta: &types.MsgMetaInfo{
				DeprecatedLIDSession:   &depLID,
				ThreadMessageID:        "thread-123",
				ThreadMessageSenderJID: targetLID,
			},
			AdditionalNodes: &[]waBinary.Node{{Tag: "custom"}},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send group message no permission", func(t *testing.T) {
		roGroup := types.NewJID("rogroup", types.GroupServer)
		cli.groupCacheLock.Lock()
		cli.groupCache[roGroup] = &groupMetaCache{
			ReadOnly: true,
			Admins:   []types.JID{},
			Members:  []types.JID{cli.getOwnID(), targetLID},
		}
		cli.groupCacheLock.Unlock()
		msg := &waE2E.Message{Conversation: proto.String("cannot send")}
		_, err := cli.SendMessage(ctx, roGroup, msg, SendRequestExtra{CheckPermissions: true})
		require.ErrorIs(t, err, ErrNoPermissionInGroup)
	})

	t.Run("Send message server returns error", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		customFulfillLock.Lock()
		customFulfill = func(id string) *waBinary.Node {
			return &waBinary.Node{
				Tag:   "ack",
				Attrs: waBinary.Attrs{"id": id, "error": "500"},
			}
		}
		customFulfillLock.Unlock()
		defer func() {
			customFulfillLock.Lock()
			customFulfill = nil
			customFulfillLock.Unlock()
		}()
		msg := &waE2E.Message{Conversation: proto.String("error msg")}
		_, err := cli.SendMessage(tCtx, targetLID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.ErrorIs(t, err, ErrServerReturnedError)
	})

	t.Run("Send message phash mismatch invalidates cache", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		phashLID := types.NewJID("777777", types.HiddenUserServer)
		_, _ = establishSignalSession(ctx, cli, phashLID)
		cli.userDevicesCache[phashLID] = deviceCache{devices: []types.JID{phashLID}}
		customFulfillLock.Lock()
		customFulfill = func(id string) *waBinary.Node {
			return &waBinary.Node{
				Tag:   "ack",
				Attrs: waBinary.Attrs{"id": id, "class": "receipt", "type": "result", "phash": "different-phash"},
			}
		}
		customFulfillLock.Unlock()
		defer func() {
			customFulfillLock.Lock()
			customFulfill = nil
			customFulfillLock.Unlock()
		}()
		msg := &waE2E.Message{Conversation: proto.String("phash mismatch")}
		resp, err := cli.SendMessage(tCtx, phashLID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send Newsletter message", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		newsletterJID := types.NewJID("123456789", types.NewsletterServer)
		msg := &waE2E.Message{Conversation: proto.String("newsletter news")}
		resp, err := cli.SendMessage(tCtx, newsletterJID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send PeerMessage", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		peerMsg := cli.BuildUnavailableMessageRequest(targetJID, targetJID, "msg-unavailable")
		resp, err := cli.SendPeerMessage(tCtx, peerMsg)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("SetDisappearingTimer", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		err := cli.SetDisappearingTimer(tCtx, targetLID, 24*time.Hour, time.Now())
		require.NoError(t, err)
	})

	t.Run("PreparePeerMessageNode directly", func(t *testing.T) {
		node, err := cli.preparePeerMessageNode(ctx, targetLID, "msg-prep-peer", &waE2E.Message{
			Conversation: proto.String("direct peer"),
		}, &MessageDebugTimings{})
		require.NoError(t, err)
		assert.Equal(t, "message", node.Tag)
		assert.Equal(t, "peer", node.Attrs["category"])
	})

	t.Run("EncryptMessageForDevice with prekey bundle", func(t *testing.T) {
		bundle, err := setupSignalBundle(targetLID)
		require.NoError(t, err)
		node, isPreKey, err := cli.encryptMessageForDevice(ctx, []byte("test-encrypt"), targetLID, bundle, waBinary.Attrs{}, nil)
		require.NoError(t, err)
		assert.True(t, isPreKey)
		assert.Equal(t, "enc", node.Tag)
		assert.Equal(t, "pkmsg", node.Attrs["type"])
	})

	t.Run("EncryptMessageForDevices batch", func(t *testing.T) {
		nodes, _, err := cli.encryptMessageForDevices(ctx, []types.JID{targetLID}, "msg-batch", []byte("msg-pt"), []byte("dsm-pt"), waBinary.Attrs{})
		require.NoError(t, err)
		assert.Len(t, nodes, 1)
	})

	t.Run("GetMessageContent variations", func(t *testing.T) {
		baseNode := waBinary.Node{Tag: "enc"}

		// Poll creation
		pollCreateMsg := &waE2E.Message{
			PollCreationMessage: &waE2E.PollCreationMessage{},
		}
		content := cli.getMessageContent(baseNode, pollCreateMsg, waBinary.Attrs{"type": "poll"}, true, nodeExtraParams{})
		assert.Len(t, content, 3) // base + identity + meta(polltype=creation)

		// Poll vote
		pollVoteMsg := &waE2E.Message{
			PollUpdateMessage: &waE2E.PollUpdateMessage{},
		}
		content = cli.getMessageContent(baseNode, pollVoteMsg, waBinary.Attrs{"type": "poll"}, false, nodeExtraParams{})
		assert.Len(t, content, 2) // base + meta(polltype=vote)

		// Bot node, meta node, and additional nodes
		botNode := waBinary.Node{Tag: "bot"}
		metaNode := waBinary.Node{Tag: "meta"}
		addNodes := []waBinary.Node{{Tag: "extra"}}
		content = cli.getMessageContent(baseNode, &waE2E.Message{}, waBinary.Attrs{}, false, nodeExtraParams{
			botNode:         &botNode,
			metaNode:        &metaNode,
			additionalNodes: &addNodes,
		})
		assert.Len(t, content, 4)
	})

	t.Run("Send message timeout", func(t *testing.T) {
		customFulfillLock.Lock()
		customFulfill = func(id string) *waBinary.Node {
			return nil
		}
		customFulfillLock.Unlock()
		defer func() {
			customFulfillLock.Lock()
			customFulfill = nil
			customFulfillLock.Unlock()
		}()
		msg := &waE2E.Message{Conversation: proto.String("timeout msg")}
		_, err := cli.SendMessage(ctx, targetLID, msg, SendRequestExtra{Timeout: 10 * time.Millisecond})
		require.ErrorIs(t, err, ErrMessageTimedOut)
	})

	t.Run("Send message context cancel", func(t *testing.T) {
		cCtx, cancel := context.WithCancel(ctx)
		cancel()
		msg := &waE2E.Message{Conversation: proto.String("cancel msg")}
		_, err := cli.SendMessage(cCtx, targetLID, msg, SendRequestExtra{Timeout: 5 * time.Second})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestSend_DisconnectNode(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	targetLID := types.NewJID("111111", types.HiddenUserServer)
	_, err := establishSignalSession(ctx, cli, targetLID)
	require.NoError(t, err)
	cli.userDevicesCache[targetLID] = deviceCache{devices: []types.JID{targetLID}}

	go func() {
		for i := 0; i < 400; i++ {
			time.Sleep(10 * time.Millisecond)
			cli.responseWaitersLock.Lock()
			for id := range cli.responseWaiters {
				cli.responseWaitersLock.Unlock()
				cli.receiveResponse(context.Background(), &waBinary.Node{
					Tag:   "stream:error",
					Attrs: waBinary.Attrs{"id": id, "code": "401"},
				})
				return
			}
			cli.responseWaitersLock.Unlock()
		}
	}()

	msg := &waE2E.Message{Conversation: proto.String("disconnect msg")}
	_, err = cli.SendMessage(ctx, targetLID, msg, SendRequestExtra{Timeout: 5 * time.Second})
	require.Error(t, err)
}

func TestSend_GetEditAttribute(t *testing.T) {
	assert.Equal(t, types.EditAttributeEmpty, getEditAttribute(&waE2E.Message{}))

	msgEdit := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type:          waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
			Key:           &waCommon.MessageKey{},
			EditedMessage: &waE2E.Message{Conversation: proto.String("edited")},
		},
	}
	assert.Equal(t, types.EditAttributeMessageEdit, getEditAttribute(msgEdit))

	msgReactionRemove := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{Text: proto.String(RemoveReactionText)},
	}
	assert.Equal(t, types.EditAttributeSenderRevoke, getEditAttribute(msgReactionRemove))

	msgKeepUndo := &waE2E.Message{
		KeepInChatMessage: &waE2E.KeepInChatMessage{
			Key:      &waCommon.MessageKey{FromMe: proto.Bool(true)},
			KeepType: waE2E.KeepType_UNDO_KEEP_FOR_ALL.Enum(),
		},
	}
	assert.Equal(t, types.EditAttributeSenderRevoke, getEditAttribute(msgKeepUndo))

	msgPin := &waE2E.Message{
		PinInChatMessage: &waE2E.PinInChatMessage{},
	}
	assert.Equal(t, types.EditAttributePinInChat, getEditAttribute(msgPin))

	msgSenderRevoke := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
			Key:  &waCommon.MessageKey{FromMe: proto.Bool(true)},
		},
	}
	assert.Equal(t, types.EditAttributeSenderRevoke, getEditAttribute(msgSenderRevoke))

	msgAdminRevoke := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
			Key:  &waCommon.MessageKey{FromMe: proto.Bool(false)},
		},
	}
	assert.Equal(t, types.EditAttributeAdminRevoke, getEditAttribute(msgAdminRevoke))

	nestedEdit := &waE2E.Message{
		EditedMessage: &waE2E.FutureProofMessage{
			Message: msgEdit,
		},
	}
	assert.Equal(t, types.EditAttributeMessageEdit, getEditAttribute(nestedEdit))
}

func TestSend_PreparePeerMessageNode_Variations(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	targetLID := types.NewJID("111111", types.HiddenUserServer)
	_, err := establishSignalSession(ctx, cli, targetLID)
	require.NoError(t, err)

	syncKeyReq := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST.Enum(),
		},
	}
	node, err := cli.preparePeerMessageNode(ctx, targetLID, "sync-req", syncKeyReq, &MessageDebugTimings{})
	require.NoError(t, err)
	assert.Equal(t, "high", node.Attrs["push_priority"])

	historySyncReq := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			PeerDataOperationRequestMessage: &waE2E.PeerDataOperationRequestMessage{
				PeerDataOperationRequestType: waE2E.PeerDataOperationRequestType_HISTORY_SYNC_ON_DEMAND.Enum(),
			},
		},
	}
	node, err = cli.preparePeerMessageNode(ctx, targetLID, "history-req", historySyncReq, &MessageDebugTimings{})
	require.NoError(t, err)
	assert.Equal(t, "high_force", node.Attrs["push_priority"])
	assert.Equal(t, "1", node.Attrs["privacy_sensitive"])
}

func TestSend_EncryptMessageForDevice_Errors(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	unknownJID := types.NewJID("999999", types.HiddenUserServer)
	_, _, err := cli.encryptMessageForDevice(ctx, []byte("plaintext"), unknownJID, nil, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoSession)
}
