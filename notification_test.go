// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockIdentityStore struct {
	sync.Mutex
	deletedUsers []string
}

func (m *mockIdentityStore) PutIdentity(_ context.Context, _ string, _ [32]byte) error { return nil }
func (m *mockIdentityStore) IsTrustedIdentity(_ context.Context, _ string, _ [32]byte) (bool, error) {
	return true, nil
}
func (m *mockIdentityStore) SaveIdentity(_ context.Context, _ string, _ [32]byte) (bool, error) {
	return true, nil
}
func (m *mockIdentityStore) DeleteAllIdentities(_ context.Context, user string) error {
	m.Lock()
	defer m.Unlock()
	m.deletedUsers = append(m.deletedUsers, user)
	return nil
}
func (m *mockIdentityStore) DeleteIdentity(_ context.Context, _ string) error { return nil }

type mockAppStateStore struct {
	versionErr error
}

func (m *mockAppStateStore) PutAppStateVersion(_ context.Context, _ string, _ uint64, _ [128]byte) error {
	return nil
}

func (m *mockAppStateStore) GetAppStateVersion(_ context.Context, _ string) (uint64, [128]byte, error) {
	if m.versionErr != nil {
		return 0, [128]byte{}, m.versionErr
	}
	return 0, [128]byte{}, nil
}

func (m *mockAppStateStore) DeleteAppStateVersion(_ context.Context, _ string) error { return nil }
func (m *mockAppStateStore) PutAppStateMutationMACs(_ context.Context, _ string, _ uint64, _ []store.AppStateMutationMAC) error {
	return nil
}
func (m *mockAppStateStore) DeleteAppStateMutationMACs(_ context.Context, _ string, _ [][]byte) error {
	return nil
}
func (m *mockAppStateStore) GetAppStateMutationMAC(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, nil
}

func TestNotification_ParseNewsletterMessages(t *testing.T) {
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	validMsgProto, _ := proto.Marshal(&waE2E.Message{
		Conversation: proto.String("Hello Channel"),
	})

	node := &waBinary.Node{
		Tag: "messages",
		Content: []waBinary.Node{
			{Tag: "not_message"}, // Should be ignored
			{
				Tag: "message",
				Attrs: waBinary.Attrs{
					"server_id": "1001",
					"id":        "msg-1001",
					"type":      "text",
					"t":         "1700000000",
				},
				Content: []waBinary.Node{
					{Tag: "plaintext", Content: validMsgProto},
					{Tag: "views_count", Attrs: waBinary.Attrs{"count": "150"}},
					{Tag: "forwards_count", Attrs: waBinary.Attrs{"count": "12"}},
					{
						Tag: "reactions",
						Content: []waBinary.Node{
							{Tag: "reaction", Attrs: waBinary.Attrs{"code": "🔥", "count": "8"}},
							{Tag: "reaction", Attrs: waBinary.Attrs{"code": "👏", "count": "5"}},
						},
					},
				},
			},
			{
				Tag: "message",
				Attrs: waBinary.Attrs{
					"server_id": "1002",
					"id":        "msg-1002",
					"type":      "text",
					"t":         "1700000001",
				},
				Content: []waBinary.Node{
					{Tag: "plaintext", Content: []byte("invalid-proto-bytes")},
				},
			},
		},
	}

	msgs := cli.parseNewsletterMessages(node)
	require.Len(t, msgs, 2)

	assert.Equal(t, types.MessageServerID(1001), msgs[0].MessageServerID)
	assert.Equal(t, types.MessageID("msg-1001"), msgs[0].MessageID)
	assert.Equal(t, 150, msgs[0].ViewsCount)
	assert.Equal(t, 12, msgs[0].ForwardsCount)
	assert.Equal(t, 8, msgs[0].ReactionCounts["🔥"])
	assert.Equal(t, 5, msgs[0].ReactionCounts["👏"])
	require.NotNil(t, msgs[0].Message)
	assert.Equal(t, "Hello Channel", msgs[0].Message.GetConversation())

	// Invalid proto unmarshaling leaves Message as nil
	assert.Equal(t, types.MessageServerID(1002), msgs[1].MessageServerID)
	assert.Nil(t, msgs[1].Message)
}

func TestNotification_HandleNewsletterNotification(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	channelJID := types.NewJID("12345", types.NewsletterServer)
	var dispatchedEvt *events.NewsletterLiveUpdate
	cli.AddEventHandler(func(evt any) {
		if e, ok := evt.(*events.NewsletterLiveUpdate); ok {
			dispatchedEvt = e
		}
	})

	node := &waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"from": channelJID,
			"t":    "1700000000",
		},
		Content: []waBinary.Node{{
			Tag: "live_updates",
			Content: []waBinary.Node{{
				Tag: "messages",
				Content: []waBinary.Node{{
					Tag:   "message",
					Attrs: waBinary.Attrs{"server_id": "500", "id": "m500", "type": "text", "t": "1700000000"},
				}},
			}},
		}},
	}

	cli.handleNewsletterNotification(ctx, node)
	require.NotNil(t, dispatchedEvt)
	assert.Equal(t, channelJID, dispatchedEvt.JID)
	assert.Equal(t, time.Unix(1700000000, 0), dispatchedEvt.Time)
	require.Len(t, dispatchedEvt.Messages, 1)
	assert.Equal(t, types.MessageServerID(500), dispatchedEvt.Messages[0].MessageServerID)

	// Fallback when messages tag is omitted
	dispatchedEvt = nil
	nodeWithoutMessagesTag := &waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"from": channelJID,
			"t":    "1700000000",
		},
		Content: []waBinary.Node{{
			Tag: "live_updates",
			Content: []waBinary.Node{{
				Tag:   "message",
				Attrs: waBinary.Attrs{"server_id": "501", "id": "m501", "type": "text", "t": "1700000000"},
			}},
		}},
	}
	cli.handleNewsletterNotification(ctx, nodeWithoutMessagesTag)
	require.NotNil(t, dispatchedEvt)
	require.Len(t, dispatchedEvt.Messages, 1)
	assert.Equal(t, types.MessageServerID(501), dispatchedEvt.Messages[0].MessageServerID)
}

func TestNotification_HandleMexNotification(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	var received []any
	cli.AddEventHandler(func(evt any) {
		received = append(received, evt)
	})

	t.Run("Non-update and non-bytes content ignored", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{
				{Tag: "other"},
				{Tag: "update", Content: "not-bytes"},
				{Tag: "update", Content: []byte("invalid-json{")},
			},
		}
		cli.handleMexNotification(ctx, node)
		assert.Empty(t, received)
	})

	t.Run("Valid events dispatching", func(t *testing.T) {
		received = nil
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"t": "1700000000"},
			Content: []waBinary.Node{
				{
					Tag:     "update",
					Attrs:   waBinary.Attrs{"op_name": "join_op"},
					Content: []byte(`{"data":{"xwa2_notify_newsletter_on_join":{"id":"111@newsletter"}}}`),
				},
				{
					Tag:     "update",
					Attrs:   waBinary.Attrs{"op_name": "leave_op"},
					Content: []byte(`{"data":{"xwa2_notify_newsletter_on_leave":{"id":"222@newsletter"}}}`),
				},
				{
					Tag:     "update",
					Attrs:   waBinary.Attrs{"op_name": "mute_op"},
					Content: []byte(`{"data":{"xwa2_notify_newsletter_on_mute_change":{"id":"333@newsletter"}}}`),
				},
				{
					Tag:     "update",
					Attrs:   waBinary.Attrs{"op_name": "milestone_op"},
					Content: []byte(`{"data":{"xwa2_notify_newsletter_milestone":{"newsletter_id":"444@newsletter"}}}`),
				},
				{
					Tag:     "update",
					Attrs:   waBinary.Attrs{"op_name": "timelock_op"},
					Content: []byte(`{"data":{"xwa2_notify_account_reachout_timelock":{"enforcement_type":"temporary","is_active":true}}}`),
				},
			},
		}
		cli.handleMexNotification(ctx, node)
		require.Len(t, received, 5)

		join, ok := received[0].(*events.NewsletterJoin)
		require.True(t, ok)
		assert.Equal(t, types.NewJID("111", types.NewsletterServer), join.ID)
		assert.Equal(t, "join_op", join.Mex.OpName)

		leave, ok := received[1].(*events.NewsletterLeave)
		require.True(t, ok)
		assert.Equal(t, types.NewJID("222", types.NewsletterServer), leave.ID)

		mute, ok := received[2].(*events.NewsletterMuteChange)
		require.True(t, ok)
		assert.Equal(t, types.NewJID("333", types.NewsletterServer), mute.ID)

		milestone, ok := received[3].(*events.NewsletterMilestone)
		require.True(t, ok)
		assert.Equal(t, types.NewJID("444", types.NewsletterServer), milestone.ID)

		timelock, ok := received[4].(*events.NotifyAccountReachoutTimelock)
		require.True(t, ok)
		assert.Equal(t, "temporary", timelock.EnforcementType)
		assert.True(t, timelock.IsActive)
	})
}

func TestNotification_HandleStatusNotification(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	var aboutEvt *events.UserAbout
	cli.AddEventHandler(func(evt any) {
		if e, ok := evt.(*events.UserAbout); ok {
			aboutEvt = e
		}
	})

	userJID := types.NewJID("62811111", types.DefaultUserServer)

	t.Run("Missing set child", func(t *testing.T) {
		aboutEvt = nil
		node := &waBinary.Node{Tag: "notification", Attrs: waBinary.Attrs{"from": userJID}}
		cli.handleStatusNotification(ctx, node)
		assert.Nil(t, aboutEvt)
	})

	t.Run("Set child has unexpected content", func(t *testing.T) {
		aboutEvt = nil
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": userJID},
			Content: []waBinary.Node{
				{Tag: "set", Content: 12345},
			},
		}
		cli.handleStatusNotification(ctx, node)
		assert.Nil(t, aboutEvt)
	})

	t.Run("Success dispatch", func(t *testing.T) {
		aboutEvt = nil
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": userJID, "t": "1700000000"},
			Content: []waBinary.Node{
				{Tag: "set", Content: []byte("Available")},
			},
		}
		cli.handleStatusNotification(ctx, node)
		require.NotNil(t, aboutEvt)
		assert.Equal(t, userJID, aboutEvt.JID)
		assert.Equal(t, "Available", aboutEvt.Status)
		assert.Equal(t, time.Unix(1700000000, 0), aboutEvt.Timestamp)
	})
}

func TestNotification_HandlePictureNotification(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	var picEvts []*events.Picture
	cli.AddEventHandler(func(evt any) {
		if e, ok := evt.(*events.Picture); ok {
			picEvts = append(picEvts, e)
		}
	})

	userJID := types.NewJID("62811111", types.DefaultUserServer)
	authorJID := types.NewJID("62822222", types.DefaultUserServer)

	node := &waBinary.Node{
		Tag:   "notification",
		Attrs: waBinary.Attrs{"t": "1700000000"},
		Content: []waBinary.Node{
			{Tag: "delete", Attrs: waBinary.Attrs{"jid": userJID, "author": authorJID}},
			{Tag: "add", Attrs: waBinary.Attrs{"jid": userJID, "id": "pic-add-1"}},
			{Tag: "set", Attrs: waBinary.Attrs{"jid": userJID, "id": "pic-set-1"}},
			{Tag: "ignored_tag"},
			{Tag: "add"}, // Missing jid -> invalid attrs
		},
	}

	cli.handlePictureNotification(ctx, node)
	require.Len(t, picEvts, 3)

	assert.True(t, picEvts[0].Remove)
	assert.Equal(t, userJID, picEvts[0].JID)
	assert.Equal(t, authorJID, picEvts[0].Author)

	assert.False(t, picEvts[1].Remove)
	assert.Equal(t, "pic-add-1", picEvts[1].PictureID)

	assert.False(t, picEvts[2].Remove)
	assert.Equal(t, "pic-set-1", picEvts[2].PictureID)
}

func TestNotification_HandleBlocklist(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	var blockEvt *events.Blocklist
	cli.AddEventHandler(func(evt any) {
		if e, ok := evt.(*events.Blocklist); ok {
			blockEvt = e
		}
	})

	user1 := types.NewJID("628111", types.DefaultUserServer)
	user2 := types.NewJID("628222", types.DefaultUserServer)

	node := &waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"action":     "modify",
			"dhash":      "hash-new",
			"prev_dhash": "hash-prev",
		},
		Content: []waBinary.Node{
			{Tag: "item", Attrs: waBinary.Attrs{"jid": user1, "action": "block"}},
			{Tag: "item", Attrs: waBinary.Attrs{"jid": user2, "action": "unblock"}},
			{Tag: "item"}, // Missing attrs
		},
	}

	cli.handleBlocklist(ctx, node)
	require.NotNil(t, blockEvt)
	assert.Equal(t, events.BlocklistAction("modify"), blockEvt.Action)
	assert.Equal(t, "hash-new", blockEvt.DHash)
	assert.Equal(t, "hash-prev", blockEvt.PrevDHash)
	require.Len(t, blockEvt.Changes, 2)
	assert.Equal(t, user1, blockEvt.Changes[0].JID)
	assert.Equal(t, events.BlocklistChangeAction("block"), blockEvt.Changes[0].Action)
	assert.Equal(t, user2, blockEvt.Changes[1].JID)
	assert.Equal(t, events.BlocklistChangeAction("unblock"), blockEvt.Changes[1].Action)
}

func TestNotification_HandleDeviceNotification(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{
		ID:   &types.JID{User: "me", Server: types.DefaultUserServer},
		LIDs: newMockLIDStore(),
	}
	cli := NewClient(device, waLog.Noop)

	target := types.NewJID("62812345", types.DefaultUserServer)
	dev1 := types.NewJID("62812345", types.DefaultUserServer)
	dev1.Device = 1
	dev2 := types.NewJID("62812345", types.DefaultUserServer)
	dev2.Device = 2

	t.Run("Not in cache ignores", func(t *testing.T) {
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": target},
		}
		cli.handleDeviceNotification(ctx, node)
		_, exists := cli.userDevicesCache[target]
		assert.False(t, exists)
	})

	t.Run("Add device matching hash", func(t *testing.T) {
		cli.userDevicesCache[target] = deviceCache{devices: []types.JID{dev1}}
		expectedHash := participantListHashV2([]types.JID{dev1, dev2})

		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": target},
			Content: []waBinary.Node{
				{
					Tag:   "add",
					Attrs: waBinary.Attrs{"device_hash": expectedHash},
					Content: []waBinary.Node{
						{Tag: "device", Attrs: waBinary.Attrs{"jid": dev2}},
					},
				},
			},
		}
		cli.handleDeviceNotification(ctx, node)
		cached := cli.userDevicesCache[target]
		assert.ElementsMatch(t, []types.JID{dev1, dev2}, cached.devices)
	})

	t.Run("Remove device matching hash", func(t *testing.T) {
		cli.userDevicesCache[target] = deviceCache{devices: []types.JID{dev1, dev2}}
		expectedHash := participantListHashV2([]types.JID{dev1})

		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": target},
			Content: []waBinary.Node{
				{
					Tag:   "remove",
					Attrs: waBinary.Attrs{"device_hash": expectedHash},
					Content: []waBinary.Node{
						{Tag: "device", Attrs: waBinary.Attrs{"jid": dev2}},
					},
				},
			},
		}
		cli.handleDeviceNotification(ctx, node)
		cached := cli.userDevicesCache[target]
		assert.ElementsMatch(t, []types.JID{dev1}, cached.devices)
	})

	t.Run("Hash mismatch clears cache", func(t *testing.T) {
		cli.userDevicesCache[target] = deviceCache{devices: []types.JID{dev1}}
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": target},
			Content: []waBinary.Node{
				{
					Tag:   "add",
					Attrs: waBinary.Attrs{"device_hash": "wrong-hash"},
					Content: []waBinary.Node{
						{Tag: "device", Attrs: waBinary.Attrs{"jid": dev2}},
					},
				},
			},
		}
		cli.handleDeviceNotification(ctx, node)
		_, exists := cli.userDevicesCache[target]
		assert.False(t, exists)
	})

	t.Run("Update tag clears cache", func(t *testing.T) {
		cli.userDevicesCache[target] = deviceCache{devices: []types.JID{dev1}}
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": target},
			Content: []waBinary.Node{
				{Tag: "update"},
			},
		}
		cli.handleDeviceNotification(ctx, node)
		_, exists := cli.userDevicesCache[target]
		assert.False(t, exists)
	})

	t.Run("With LID mapping and device_lid_hash", func(t *testing.T) {
		targetLID := types.NewJID("888888", types.HiddenUserServer)
		devLID1 := types.NewJID("888888", types.HiddenUserServer)
		devLID1.Device = 1
		devLID2 := types.NewJID("888888", types.HiddenUserServer)
		devLID2.Device = 2

		cli.userDevicesCache[target] = deviceCache{devices: []types.JID{dev1}}
		cli.userDevicesCache[targetLID] = deviceCache{devices: []types.JID{devLID1}}

		expectedPNHash := participantListHashV2([]types.JID{dev1, dev2})
		expectedLIDHash := participantListHashV2([]types.JID{devLID1, devLID2})

		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": target, "lid": targetLID},
			Content: []waBinary.Node{
				{
					Tag: "add",
					Attrs: waBinary.Attrs{
						"device_hash":     expectedPNHash,
						"device_lid_hash": expectedLIDHash,
					},
					Content: []waBinary.Node{
						{Tag: "device", Attrs: waBinary.Attrs{"jid": dev2, "lid": devLID2}},
					},
				},
			},
		}
		cli.handleDeviceNotification(ctx, node)
		assert.ElementsMatch(t, []types.JID{dev1, dev2}, cli.userDevicesCache[target].devices)
		assert.ElementsMatch(t, []types.JID{devLID1, devLID2}, cli.userDevicesCache[targetLID].devices)
	})
}

func TestNotification_HandleFBDeviceNotification(t *testing.T) {
	ctx := context.Background()
	device := &store.Device{ID: nil}
	cli := NewClient(device, waLog.Noop)

	target := types.NewJID("fbuser", types.MessengerServer)
	node := &waBinary.Node{
		Tag:   "notification",
		Attrs: waBinary.Attrs{"from": target},
		Content: []waBinary.Node{{
			Tag: "devices",
			Content: []waBinary.Node{
				{Tag: "device", Attrs: waBinary.Attrs{"id": "0"}},
				{Tag: "device", Attrs: waBinary.Attrs{"id": "1"}},
			},
		}},
	}

	cli.handleFBDeviceNotification(ctx, node)
	cached, ok := cli.userDevicesCache[target]
	require.True(t, ok)
	require.Len(t, cached.devices, 2)
}

func TestNotification_HandleOwnDevicesNotification(t *testing.T) {
	ctx := context.Background()
	ownID := types.NewJID("62812345", types.DefaultUserServer)
	ownLID := types.NewJID("111111", types.HiddenUserServer)

	device := &store.Device{
		ID:  &ownID,
		LID: ownLID,
	}
	cli := NewClient(device, waLog.Noop)

	t.Run("OwnID empty ignores", func(t *testing.T) {
		emptyCli := NewClient(&store.Device{ID: nil}, waLog.Noop)
		emptyCli.handleOwnDevicesNotification(ctx, &waBinary.Node{}, ownID)
	})

	t.Run("Unexpected sender ignores", func(t *testing.T) {
		unexpected := types.NewJID("999999", types.DefaultUserServer)
		cli.handleOwnDevicesNotification(ctx, &waBinary.Node{}, unexpected)
	})

	t.Run("Matches expected hash updates both ownID and ownLID", func(t *testing.T) {
		dev0 := ownID
		dev0.Device = 0
		dev1 := ownID
		dev1.Device = 1
		expectedHash := participantListHashV2([]types.JID{dev0, dev1})

		node := &waBinary.Node{
			Tag:   "devices",
			Attrs: waBinary.Attrs{"dhash": expectedHash},
			Content: []waBinary.Node{
				{Tag: "device", Attrs: waBinary.Attrs{"jid": dev0}},
				{Tag: "device", Attrs: waBinary.Attrs{"jid": dev1}},
			},
		}

		cli.handleOwnDevicesNotification(ctx, node, ownID)
		cachedID, okID := cli.userDevicesCache[ownID]
		cachedLID, okLID := cli.userDevicesCache[ownLID]
		require.True(t, okID)
		require.True(t, okLID)
		assert.Equal(t, expectedHash, cachedID.dhash)
		assert.Len(t, cachedID.devices, 2)
		assert.Len(t, cachedLID.devices, 2)
		assert.Equal(t, ownLID.User, cachedLID.devices[0].User)
	})

	t.Run("Hash mismatch clears caches", func(t *testing.T) {
		node := &waBinary.Node{
			Tag:   "devices",
			Attrs: waBinary.Attrs{"dhash": "wrong-dhash"},
			Content: []waBinary.Node{
				{Tag: "device", Attrs: waBinary.Attrs{"jid": ownID}},
			},
		}
		cli.handleOwnDevicesNotification(ctx, node, ownID)
		_, existsID := cli.userDevicesCache[ownID]
		_, existsLID := cli.userDevicesCache[ownLID]
		assert.False(t, existsID)
		assert.False(t, existsLID)
	})
}

func TestNotification_HandleAccountSyncNotification(t *testing.T) {
	ctx := context.Background()
	ownID := types.NewJID("62812345", types.DefaultUserServer)
	device := &store.Device{ID: &ownID}
	cli := NewClient(device, waLog.Noop)

	var picDispatched bool
	cli.AddEventHandler(func(evt any) {
		if _, ok := evt.(*events.Picture); ok {
			picDispatched = true
		}
	})

	node := &waBinary.Node{
		Tag:   "notification",
		Attrs: waBinary.Attrs{"from": ownID, "t": "1700000000"},
		Content: []waBinary.Node{
			{Tag: "picture"},
			{Tag: "privacy", Content: []waBinary.Node{
				{Tag: "category", Attrs: waBinary.Attrs{"name": "last", "value": "all"}},
			}},
			{Tag: "devices", Attrs: waBinary.Attrs{"dhash": "hash"}},
			{Tag: "blocklist", Attrs: waBinary.Attrs{"dhash": "bl-hash"}},
			{Tag: "unhandled_item"},
		},
	}

	cli.handleAccountSyncNotification(ctx, node)
	assert.True(t, picDispatched)
}

func TestNotification_HandlePrivacyTokenNotification(t *testing.T) {
	ctx := context.Background()
	ownID := types.NewJID("me", types.DefaultUserServer)
	pStore := newMockPrivacyTokenStore()
	device := &store.Device{
		ID:            &ownID,
		PrivacyTokens: pStore,
		LIDs:          newMockLIDStore(),
	}
	cli := NewClient(device, waLog.Noop)
	sender := types.NewJID("sender123", types.DefaultUserServer)
	senderLID := types.NewJID("senderLID", types.HiddenUserServer)

	t.Run("OwnID empty", func(t *testing.T) {
		emptyCli := NewClient(&store.Device{ID: nil}, waLog.Noop)
		emptyCli.handlePrivacyTokenNotification(ctx, &waBinary.Node{})
	})

	t.Run("Missing tokens tag", func(t *testing.T) {
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": sender},
		}
		cli.handlePrivacyTokenNotification(ctx, node)
	})

	t.Run("Missing from attribute", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "notification",
			Content: []waBinary.Node{
				{Tag: "tokens"},
			},
		}
		cli.handlePrivacyTokenNotification(ctx, node)
	})

	t.Run("Valid token saved", func(t *testing.T) {
		tokenBytes := []byte("privacy-token-secret")
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"from":       sender,
				"sender_lid": senderLID,
			},
			Content: []waBinary.Node{{
				Tag: "tokens",
				Content: []waBinary.Node{
					{Tag: "wrong_tag"},
					{Tag: "token", Attrs: waBinary.Attrs{"type": "other"}},
					{Tag: "token", Attrs: waBinary.Attrs{"type": "trusted_contact"}, Content: "not-bytes"},
					{
						Tag:     "token",
						Attrs:   waBinary.Attrs{"type": "trusted_contact", "t": "1700000000"},
						Content: tokenBytes,
					},
				},
			}},
		}

		cli.handlePrivacyTokenNotification(ctx, node)
		tok, err := pStore.GetPrivacyToken(ctx, senderLID)
		require.NoError(t, err)
		require.NotNil(t, tok)
		assert.Equal(t, tokenBytes, tok.Token)
		assert.Equal(t, time.Unix(1700000000, 0), tok.Timestamp)
	})
}

func TestNotification_HandleEncryptNotification(t *testing.T) {
	ctx := context.Background()
	pStore := newMockPrivacyTokenStore()
	identStore := &mockIdentityStore{}
	sessStore := newMockSessionStore()
	device := &store.Device{
		ID:            &types.JID{User: "me", Server: types.DefaultUserServer},
		Identities:    identStore,
		Sessions:      sessStore,
		PrivacyTokens: pStore,
		LIDs:          newMockLIDStore(),
	}
	cli := NewClient(device, waLog.Noop)

	t.Run("ServerJID prekey count", func(t *testing.T) {
		// Valid count but >= MinPreKeyCount -> no uploadPreKeys
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{
				{Tag: "count", Attrs: waBinary.Attrs{"value": "100"}},
			},
		}
		cli.handleEncryptNotification(ctx, node)

		// Invalid count attribute
		nodeInvalid := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": types.ServerJID},
			Content: []waBinary.Node{
				{Tag: "count"},
			},
		}
		cli.handleEncryptNotification(ctx, nodeInvalid)
	})

	t.Run("Identity change notification", func(t *testing.T) {
		sender := types.NewJID("628111222", types.DefaultUserServer)
		var identityEvt *events.IdentityChange
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.IdentityChange); ok {
				identityEvt = e
			}
		})

		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": sender, "t": "1700000000"},
			Content: []waBinary.Node{
				{Tag: "identity"},
			},
		}

		cli.handleEncryptNotification(ctx, node)
		require.NotNil(t, identityEvt)
		assert.Equal(t, sender, identityEvt.JID)
		assert.Equal(t, time.Unix(1700000000, 0), identityEvt.Timestamp)
	})

	t.Run("Unknown encryption notification", func(t *testing.T) {
		sender := types.NewJID("628111222", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag:   "notification",
			Attrs: waBinary.Attrs{"from": sender},
		}
		cli.handleEncryptNotification(ctx, node)
	})
}

func TestNotification_HandleAppStateNotification(t *testing.T) {
	ctx := context.Background()
	appStore := &mockAppStateStore{}
	device := &store.Device{
		ID:       &types.JID{User: "me", Server: types.DefaultUserServer},
		AppState: appStore,
	}
	cli := NewClient(device, waLog.Noop)

	node := &waBinary.Node{
		Tag: "notification",
		Content: []waBinary.Node{
			{Tag: "collection", Attrs: waBinary.Attrs{"name": "regular_high", "version": "1"}},
		},
	}

	// 1. ErrNotConnected branch
	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return nil, ErrNotConnected
	}
	cli.handleAppStateNotification(ctx, node)

	// 2. Generic error branch
	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return nil, errors.New("generic sync error")
	}
	cli.handleAppStateNotification(ctx, node)
}

func TestNotification_HandleNotification_Router(t *testing.T) {
	ctx := context.Background()
	ownID := types.NewJID("62812345", types.DefaultUserServer)
	device := &store.Device{
		ID:            &ownID,
		LIDs:          newMockLIDStore(),
		PrivacyTokens: newMockPrivacyTokenStore(),
		Identities:    &mockIdentityStore{},
		Sessions:      newMockSessionStore(),
		Contacts:      newMockContactStore(),
		AppState:      &mockAppStateStore{},
	}
	cli := NewClient(device, waLog.Noop)
	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return nil, ErrNotConnected
	}

	// Test each router case
	notificationTypes := []struct {
		typ     string
		content []waBinary.Node
	}{
		{"encrypt", []waBinary.Node{{Tag: "count", Attrs: waBinary.Attrs{"value": "100"}}}},
		{"server_sync", []waBinary.Node{{Tag: "collection", Attrs: waBinary.Attrs{"name": "critical_block", "version": "1"}}}},
		{"account_sync", []waBinary.Node{{Tag: "picture"}}},
		{"devices", nil},
		{"fbid:devices", []waBinary.Node{{Tag: "devices"}}},
		{"w:gp2", nil}, // parseGroupNotification returns error -> logs error
		{"picture", []waBinary.Node{{Tag: "delete", Attrs: waBinary.Attrs{"jid": ownID}}}},
		{"mediaretry", nil},
		{"privacy_token", []waBinary.Node{{Tag: "tokens"}}},
		{"link_code_companion_reg", nil},
		{"newsletter", []waBinary.Node{{Tag: "live_updates"}}},
		{"mex", nil},
		{"status", []waBinary.Node{{Tag: "set", Content: []byte("Hello")}}},
		{"passkey_prologue_request", nil},
		{"crsc_continuation", nil},
		{"companion_reg_refresh", []waBinary.Node{{Tag: "companion_reg_refresh"}}},
		{"companion_reg_refresh", []waBinary.Node{{Tag: "pair-device-rotate-qr"}}},
		{"companion_reg_refresh", nil}, // Unrecognized
		{"unknown_type", nil},
	}

	for _, tc := range notificationTypes {
		node := &waBinary.Node{
			Tag: "notification",
			Attrs: waBinary.Attrs{
				"type": tc.typ,
				"from": ownID,
				"t":    "1700000000",
			},
			Content: tc.content,
		}
		cli.handleNotification(ctx, node)
	}

	// Missing type attribute returns early
	cli.handleNotification(ctx, &waBinary.Node{Tag: "notification"})

	// Give spawned background goroutines time to finish
	time.Sleep(50 * time.Millisecond)
}
