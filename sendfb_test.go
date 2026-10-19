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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waArmadilloApplication"
	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waConsumerApplication"
	"github.com/PakaiWA/whatsmeow/proto/waMsgApplication"
	"github.com/PakaiWA/whatsmeow/proto/waMsgTransport"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestSendFB_GetAttrsFromFBMessage(t *testing.T) {
	t.Run("Armadillo application", func(t *testing.T) {
		arm := &waArmadilloApplication.Armadillo{}
		attrs := getAttrsFromFBMessage(arm)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "document", attrs.MediaType)
	})

	t.Run("Unsupported / default message", func(t *testing.T) {
		attrs := getAttrsFromFBMessage(nil)
		assert.Equal(t, "text", attrs.Type)
	})
}

func TestSendFB_GetAttrsFromFBConsumerMessage(t *testing.T) {
	wrapContent := func(c *waConsumerApplication.ConsumerApplication_Content) *waConsumerApplication.ConsumerApplication {
		return &waConsumerApplication.ConsumerApplication{
			Payload: &waConsumerApplication.ConsumerApplication_Payload{
				Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{
					Content: c,
				},
			},
		}
	}

	wrapAppContent := func(ac *waConsumerApplication.ConsumerApplication_ApplicationData) *waConsumerApplication.ConsumerApplication {
		return &waConsumerApplication.ConsumerApplication{
			Payload: &waConsumerApplication.ConsumerApplication_Payload{
				Payload: &waConsumerApplication.ConsumerApplication_Payload_ApplicationData{
					ApplicationData: ac,
				},
			},
		}
	}

	t.Run("Text messages", func(t *testing.T) {
		msg := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_MessageText{
				MessageText: &waCommon.MessageText{
					Text: proto.String("hi"),
				},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(msg)
		assert.Equal(t, "text", attrs.Type)

		extMsg := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ExtendedTextMessage{
				ExtendedTextMessage: &waConsumerApplication.ConsumerApplication_ExtendedTextMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(extMsg)
		assert.Equal(t, "text", attrs.Type)
	})

	t.Run("Image and Sticker", func(t *testing.T) {
		imgMsg := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ImageMessage{
				ImageMessage: &waConsumerApplication.ConsumerApplication_ImageMessage{},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(imgMsg)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "image", attrs.MediaType)

		stkMsg := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_StickerMessage{
				StickerMessage: &waConsumerApplication.ConsumerApplication_StickerMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(stkMsg)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "sticker", attrs.MediaType)
	})

	t.Run("ViewOnce Message (Image and Video)", func(t *testing.T) {
		voImg := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ViewOnceMessage{
				ViewOnceMessage: &waConsumerApplication.ConsumerApplication_ViewOnceMessage{
					ViewOnceContent: &waConsumerApplication.ConsumerApplication_ViewOnceMessage_ImageMessage{
						ImageMessage: &waConsumerApplication.ConsumerApplication_ImageMessage{},
					},
				},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(voImg)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "image", attrs.MediaType)

		voVid := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ViewOnceMessage{
				ViewOnceMessage: &waConsumerApplication.ConsumerApplication_ViewOnceMessage{
					ViewOnceContent: &waConsumerApplication.ConsumerApplication_ViewOnceMessage_VideoMessage{
						VideoMessage: &waConsumerApplication.ConsumerApplication_VideoMessage{},
					},
				},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(voVid)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "video", attrs.MediaType)
	})

	t.Run("Document, Audio (PTT and Normal), Video", func(t *testing.T) {
		doc := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_DocumentMessage{
				DocumentMessage: &waConsumerApplication.ConsumerApplication_DocumentMessage{},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(doc)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "document", attrs.MediaType)

		audioNormal := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_AudioMessage{
				AudioMessage: &waConsumerApplication.ConsumerApplication_AudioMessage{
					PTT: proto.Bool(false),
				},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(audioNormal)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "audio", attrs.MediaType)

		audioPTT := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_AudioMessage{
				AudioMessage: &waConsumerApplication.ConsumerApplication_AudioMessage{
					PTT: proto.Bool(true),
				},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(audioPTT)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "ptt", attrs.MediaType)

		vid := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_VideoMessage{
				VideoMessage: &waConsumerApplication.ConsumerApplication_VideoMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(vid)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "video", attrs.MediaType)
	})

	t.Run("Locations and Contacts", func(t *testing.T) {
		loc := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_LocationMessage{
				LocationMessage: &waConsumerApplication.ConsumerApplication_LocationMessage{},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(loc)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "location", attrs.MediaType)

		liveLoc := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_LiveLocationMessage{
				LiveLocationMessage: &waConsumerApplication.ConsumerApplication_LiveLocationMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(liveLoc)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "location", attrs.MediaType)

		contact := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ContactMessage{
				ContactMessage: &waConsumerApplication.ConsumerApplication_ContactMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(contact)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "vcard", attrs.MediaType)

		contactArr := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ContactsArrayMessage{
				ContactsArrayMessage: &waConsumerApplication.ConsumerApplication_ContactsArrayMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(contactArr)
		assert.Equal(t, "media", attrs.Type)
		assert.Equal(t, "contact_array", attrs.MediaType)
	})

	t.Run("Polls, Reactions, Edits", func(t *testing.T) {
		pollCreate := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_PollCreationMessage{
				PollCreationMessage: &waConsumerApplication.ConsumerApplication_PollCreationMessage{},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(pollCreate)
		assert.Equal(t, "poll", attrs.Type)
		assert.Equal(t, "creation", attrs.PollType)

		pollVote := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_PollUpdateMessage{
				PollUpdateMessage: &waConsumerApplication.ConsumerApplication_PollUpdateMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(pollVote)
		assert.Equal(t, "poll", attrs.Type)
		assert.Equal(t, "vote", attrs.PollType)
		assert.Equal(t, events.DecryptFailHide, attrs.DecryptFail)

		reaction := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_ReactionMessage{
				ReactionMessage: &waConsumerApplication.ConsumerApplication_ReactionMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(reaction)
		assert.Equal(t, "reaction", attrs.Type)
		assert.Equal(t, events.DecryptFailHide, attrs.DecryptFail)

		edit := wrapContent(&waConsumerApplication.ConsumerApplication_Content{
			Content: &waConsumerApplication.ConsumerApplication_Content_EditMessage{
				EditMessage: &waConsumerApplication.ConsumerApplication_EditMessage{},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(edit)
		assert.Equal(t, types.EditAttributeMessageEdit, attrs.Edit)
		assert.Equal(t, events.DecryptFailHide, attrs.DecryptFail)
	})

	t.Run("Revokes", func(t *testing.T) {
		senderRevoke := wrapAppContent(&waConsumerApplication.ConsumerApplication_ApplicationData{
			ApplicationContent: &waConsumerApplication.ConsumerApplication_ApplicationData_Revoke{
				Revoke: &waConsumerApplication.ConsumerApplication_RevokeMessage{
					Key: &waCommon.MessageKey{FromMe: proto.Bool(true)},
				},
			},
		})
		attrs := getAttrsFromFBConsumerMessage(senderRevoke)
		assert.Equal(t, types.EditAttributeSenderRevoke, attrs.Edit)
		assert.Equal(t, events.DecryptFailHide, attrs.DecryptFail)

		adminRevoke := wrapAppContent(&waConsumerApplication.ConsumerApplication_ApplicationData{
			ApplicationContent: &waConsumerApplication.ConsumerApplication_ApplicationData_Revoke{
				Revoke: &waConsumerApplication.ConsumerApplication_RevokeMessage{
					Key: &waCommon.MessageKey{FromMe: proto.Bool(false)},
				},
			},
		})
		attrs = getAttrsFromFBConsumerMessage(adminRevoke)
		assert.Equal(t, types.EditAttributeAdminRevoke, attrs.Edit)
		assert.Equal(t, events.DecryptFailHide, attrs.DecryptFail)
	})
}

func TestSendFB_SendFBMessage_Validations(t *testing.T) {
	ctx := context.Background()
	user := types.NewJID("12345", types.DefaultUserServer)
	consumerMsg := &waConsumerApplication.ConsumerApplication{}

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.SendFBMessage(ctx, user, consumerMsg, nil)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Multiple extra params", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.SendFBMessage(ctx, user, consumerMsg, nil, SendRequestExtra{}, SendRequestExtra{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only one extra parameter")
	})

	t.Run("Unsupported message type", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.SendFBMessage(ctx, user, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported message type")
	})

	t.Run("Recipient AD JID without Peer", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		adJID := types.NewADJID("12345", 1, 2)
		_, err := cli.SendFBMessage(ctx, adJID, consumerMsg, nil)
		require.ErrorIs(t, err, ErrRecipientADJID)
	})

	t.Run("Not logged in", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		_, err := cli.SendFBMessage(ctx, user, consumerMsg, nil)
		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("Unknown server", func(t *testing.T) {
		unknownJID := types.NewJID("user", "random_server")
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "me", Server: types.DefaultUserServer},
		}, waLog.Noop)
		_, err := cli.SendFBMessage(ctx, unknownJID, consumerMsg, nil)
		require.ErrorIs(t, err, ErrUnknownServer)
	})

	t.Run("Armadillo message packaging", func(t *testing.T) {
		arm := &waArmadilloApplication.Armadillo{}
		unknownJID := types.NewJID("user", "random_server")
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "me", Server: types.DefaultUserServer},
		}, waLog.Noop)
		_, err := cli.SendFBMessage(ctx, unknownJID, arm, nil)
		require.ErrorIs(t, err, ErrUnknownServer)
	})

	t.Run("Peer message not supported", func(t *testing.T) {
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "me", Server: types.DefaultUserServer},
		}, waLog.Noop)
		cli.isLoggedIn.Store(true)
		_, err := cli.SendFBMessage(ctx, user, consumerMsg, nil, SendRequestExtra{Peer: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "peer messages to fb are not yet supported")
	})
}

func TestSendFB_ConnectedFull(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	targetLID := types.NewJID("111111", types.HiddenUserServer)
	targetJID := types.NewJID("62812345678", types.DefaultUserServer)
	_, err := establishSignalSession(ctx, cli, targetLID)
	require.NoError(t, err)
	_, err = establishSignalSession(ctx, cli, targetJID)
	require.NoError(t, err)
	_, err = establishSignalSession(ctx, cli, cli.getOwnID())
	require.NoError(t, err)

	_ = cli.Store.LIDs.PutLIDMapping(ctx, targetLID, targetJID)
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

	consumerMsg := &waConsumerApplication.ConsumerApplication{
		Payload: &waConsumerApplication.ConsumerApplication_Payload{
			Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{
				Content: &waConsumerApplication.ConsumerApplication_Content{
					Content: &waConsumerApplication.ConsumerApplication_Content_MessageText{
						MessageText: &waCommon.MessageText{Text: proto.String("hi consumer")},
					},
				},
			},
		},
	}

	t.Run("Send DM ConsumerApplication", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		resp, err := cli.SendFBMessage(tCtx, targetJID, consumerMsg, nil, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send Group Armadillo", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		groupJID := types.NewJID("1234567-group", types.GroupServer)
		cli.groupCacheLock.Lock()
		cli.groupCache[groupJID] = &groupMetaCache{
			Members: []types.JID{cli.getOwnID(), targetLID},
		}
		cli.groupCacheLock.Unlock()
		arm := &waArmadilloApplication.Armadillo{}
		resp, err := cli.SendFBMessage(tCtx, groupJID, arm, nil, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send with custom metadata", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		meta := &waMsgApplication.MessageApplication_Metadata{}
		resp, err := cli.SendFBMessage(tCtx, targetJID, consumerMsg, meta, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Server returned error", func(t *testing.T) {
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
		_, err := cli.SendFBMessage(tCtx, targetJID, consumerMsg, nil, SendRequestExtra{Timeout: 5 * time.Second})
		require.ErrorIs(t, err, ErrServerReturnedError)
	})

	t.Run("PHash mismatch invalidates group and user cache", func(t *testing.T) {
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		phashJID := types.NewJID("phashuser", types.DefaultUserServer)
		_, _ = establishSignalSession(ctx, cli, phashJID)
		cli.userDevicesCache[phashJID] = deviceCache{devices: []types.JID{phashJID}}

		customFulfillLock.Lock()
		customFulfill = func(id string) *waBinary.Node {
			return &waBinary.Node{
				Tag:   "ack",
				Attrs: waBinary.Attrs{"id": id, "class": "receipt", "type": "result", "phash": "diff-phash"},
			}
		}
		customFulfillLock.Unlock()
		defer func() {
			customFulfillLock.Lock()
			customFulfill = nil
			customFulfillLock.Unlock()
		}()
		resp, err := cli.SendFBMessage(tCtx, phashJID, consumerMsg, nil, SendRequestExtra{Timeout: 5 * time.Second})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("Send timeout", func(t *testing.T) {
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
		_, err := cli.SendFBMessage(ctx, targetJID, consumerMsg, nil, SendRequestExtra{Timeout: 10 * time.Millisecond})
		require.ErrorIs(t, err, ErrMessageTimedOut)
	})

	t.Run("Send context cancel", func(t *testing.T) {
		cCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := cli.SendFBMessage(cCtx, targetJID, consumerMsg, nil, SendRequestExtra{Timeout: 5 * time.Second})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestSendFB_DisconnectNode(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	targetJID := types.NewJID("62812345678", types.DefaultUserServer)
	_, err := establishSignalSession(ctx, cli, targetJID)
	require.NoError(t, err)
	cli.userDevicesCache[targetJID] = deviceCache{devices: []types.JID{targetJID}}
	cli.userDevicesCache[cli.getOwnID().ToNonAD()] = deviceCache{devices: []types.JID{cli.getOwnID()}}

	go func() {
		for i := 0; i < 400; i++ {
			time.Sleep(10 * time.Millisecond)
			cli.responseWaitersLock.Lock()
			for id, waiter := range cli.responseWaiters {
				delete(cli.responseWaiters, id)
				cli.responseWaitersLock.Unlock()
				waiter <- &waBinary.Node{
					Tag:   "stream:error",
					Attrs: waBinary.Attrs{"id": id, "code": "401"},
				}
				return
			}
			cli.responseWaitersLock.Unlock()
		}
	}()

	consumerMsg := &waConsumerApplication.ConsumerApplication{}
	_, err = cli.SendFBMessage(ctx, targetJID, consumerMsg, nil, SendRequestExtra{Timeout: 5 * time.Second})
	require.Error(t, err)
}

func TestSendFB_InternalEncryptionAndNodes(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	targetLID := types.NewJID("111111", types.HiddenUserServer)
	bundle, err := setupSignalBundle(targetLID)
	require.NoError(t, err)

	payload := &waMsgTransport.MessageTransport_Payload{}

	// encryptMessageForDeviceV3 with bundle
	node, err := cli.encryptMessageForDeviceV3(ctx, payload, nil, nil, targetLID, bundle, waBinary.Attrs{"extra": "1"})
	require.NoError(t, err)
	assert.Equal(t, "enc", node.Tag)
	assert.Equal(t, "pkmsg", node.Attrs["type"])
	assert.Equal(t, "1", node.Attrs["extra"])

	// encryptMessageForDeviceV3 with existing session
	node, err = cli.encryptMessageForDeviceV3(ctx, payload, nil, nil, targetLID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "pkmsg", node.Attrs["type"])

	// encryptMessageForDeviceV3 with no session
	unknownJID := types.NewJID("999999", types.HiddenUserServer)
	_, err = cli.encryptMessageForDeviceV3(ctx, payload, nil, nil, unknownJID, nil, nil)
	require.ErrorIs(t, err, ErrNoSession)

	// encryptMessageForDeviceAndWrapV3
	wrapped, err := cli.encryptMessageForDeviceAndWrapV3(ctx, payload, nil, nil, targetLID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "to", wrapped.Tag)
	assert.Equal(t, targetLID, wrapped.Attrs["jid"])

	// prepareMessageNodeV3 with PollType, DecryptFail, and Edit
	cli.userDevicesCache[targetLID] = deviceCache{devices: []types.JID{targetLID}}
	msgAttrs := messageAttrs{
		Type:        "poll",
		MediaType:   "document",
		Edit:        types.EditAttributeMessageEdit,
		DecryptFail: events.DecryptFailHide,
		PollType:    "creation",
	}
	prepNode, devices, err := cli.prepareMessageNodeV3(ctx, targetLID, cli.getOwnID(), "msg-prep", payload, nil, msgAttrs, []byte("tag"), []types.JID{targetLID}, &MessageDebugTimings{})
	require.NoError(t, err)
	assert.NotEmpty(t, devices)
	assert.Equal(t, "message", prepNode.Tag)
	assert.Equal(t, types.MessageID("msg-prep"), prepNode.Attrs["id"])
	assert.Equal(t, string(types.EditAttributeMessageEdit), prepNode.Attrs["edit"])
}
