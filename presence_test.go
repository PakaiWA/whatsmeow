// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestPresence_HandleChatState(t *testing.T) {
	ctx := context.Background()
	devID := types.NewJID("111222", types.DefaultUserServer)
	cli := NewClient(&store.Device{
		ID:   &devID,
		LIDs: newMockLIDStore(),
	}, waLog.Noop)

	t.Run("Parse message source error", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "chatstate",
			// Missing 'from' attribute causes parseMessageSource to fail
		}
		cli.handleChatState(ctx, node)
	})

	t.Run("Unexpected children count", func(t *testing.T) {
		sender := types.NewJID("123456", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag:   "chatstate",
			Attrs: waBinary.Attrs{"from": sender},
			Content: []waBinary.Node{
				{Tag: "composing"},
				{Tag: "paused"},
			},
		}
		cli.handleChatState(ctx, node)
	})

	t.Run("Valid chat state composing with media", func(t *testing.T) {
		sender := types.NewJID("123456", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag:   "chatstate",
			Attrs: waBinary.Attrs{"from": sender},
			Content: []waBinary.Node{{
				Tag:   "composing",
				Attrs: waBinary.Attrs{"media": "audio"},
			}},
		}

		var dispatched *events.ChatPresence
		cli.AddEventHandler(func(evt any) {
			if cp, ok := evt.(*events.ChatPresence); ok {
				dispatched = cp
			}
		})

		cli.handleChatState(ctx, node)
		require.NotNil(t, dispatched)
		assert.Equal(t, types.ChatPresenceComposing, dispatched.State)
		assert.Equal(t, types.ChatPresenceMediaAudio, dispatched.Media)
		assert.Equal(t, sender, dispatched.Chat)
	})

	t.Run("Unrecognized chat state", func(t *testing.T) {
		sender := types.NewJID("123456", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag:   "chatstate",
			Attrs: waBinary.Attrs{"from": sender},
			Content: []waBinary.Node{{
				Tag: "unknown_state",
			}},
		}
		cli.handleChatState(ctx, node)
	})
}

func TestPresence_HandlePresence(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("Presence unavailable with last seen timestamp", func(t *testing.T) {
		sender := types.NewJID("123456", types.DefaultUserServer)
		nowUnix := time.Now().Unix()
		node := &waBinary.Node{
			Tag: "presence",
			Attrs: waBinary.Attrs{
				"from": sender,
				"type": "unavailable",
				"last": strconv.FormatInt(nowUnix, 10),
			},
		}

		var dispatched *events.Presence
		cli.AddEventHandler(func(evt any) {
			if p, ok := evt.(*events.Presence); ok {
				dispatched = p
			}
		})

		cli.handlePresence(ctx, node)
		require.NotNil(t, dispatched)
		assert.Equal(t, sender, dispatched.From)
		assert.True(t, dispatched.Unavailable)
		assert.Equal(t, nowUnix, dispatched.LastSeen.Unix())
	})

	t.Run("Presence available with deny last seen and unrecognized type", func(t *testing.T) {
		sender := types.NewJID("654321", types.DefaultUserServer)
		node := &waBinary.Node{
			Tag: "presence",
			Attrs: waBinary.Attrs{
				"from": sender,
				"type": "custom_presence",
				"last": "deny",
			},
		}

		var dispatched *events.Presence
		cli.AddEventHandler(func(evt any) {
			if p, ok := evt.(*events.Presence); ok {
				dispatched = p
			}
		})

		cli.handlePresence(ctx, node)
		require.NotNil(t, dispatched)
		assert.Equal(t, sender, dispatched.From)
		assert.False(t, dispatched.Unavailable)
		assert.True(t, dispatched.LastSeen.IsZero())
	})

	t.Run("AttrGetter invalid from JID", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "presence",
			Attrs: waBinary.Attrs{
				"from": 12345, // invalid type for JID
			},
		}
		cli.handlePresence(ctx, node)
	})
}

func TestPresence_SendPresence(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		err := nilCli.SendPresence(ctx, types.PresenceAvailable)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("No push name without MessengerConfig", func(t *testing.T) {
		cli := NewClient(&store.Device{PushName: ""}, waLog.Noop)
		err := cli.SendPresence(ctx, types.PresenceAvailable)
		require.ErrorIs(t, err, ErrNoPushName)
	})

	t.Run("Available presence sends node and updates active receipts", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.Store.PushName = "MyTestPushName"

		err := cli.SendPresence(ctx, types.PresenceAvailable)
		require.NoError(t, err)
		assert.Equal(t, uint32(1), cli.sendActiveReceipts.Load())
	})

	t.Run("Unavailable presence sends node and resets active receipts", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.Store.PushName = "MyTestPushName"
		cli.sendActiveReceipts.Store(1)

		err := cli.SendPresence(ctx, types.PresenceUnavailable)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), cli.sendActiveReceipts.Load())
	})

	t.Run("MessengerConfig sends presence without PushName", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.Store.PushName = ""
		cli.MessengerConfig = &MessengerConfig{}

		err := cli.SendPresence(ctx, types.PresenceUnavailable)
		require.NoError(t, err)
	})
}

func TestPresence_SubscribePresence(t *testing.T) {
	ctx := context.Background()
	targetJID := types.NewJID("123456", types.DefaultUserServer)

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		err := nilCli.SubscribePresence(ctx, targetJID)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("ensureTCToken returns error", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.Store.PrivacyTokens = &mockErrPrivacyTokenStore{err: errors.New("db error")}
		err := cli.SubscribePresence(ctx, targetJID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get privacy token")
	})

	t.Run("No privacy token with ErrorOnSubscribePresenceWithoutToken set", func(t *testing.T) {
		devID := types.NewJID("99999", types.DefaultUserServer)
		cli := NewClient(&store.Device{
			ID:   &devID,
			LIDs: newMockLIDStore(),
		}, waLog.Noop)
		cli.Store.PrivacyTokens = newMockPrivacyTokenStore()
		cli.ErrorOnSubscribePresenceWithoutToken = true
		err := cli.SubscribePresence(ctx, targetJID)
		require.ErrorIs(t, err, ErrNoPrivacyToken)
	})

	t.Run("Subscribe presence success with token and connected socket", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		// Save a privacy token in store
		tokenBytes := []byte("privacy-token-data")
		_ = cli.Store.PrivacyTokens.PutPrivacyTokens(ctx, store.PrivacyToken{
			User:      targetJID,
			Token:     tokenBytes,
			Timestamp: time.Now(),
		})

		err := cli.SubscribePresence(ctx, targetJID)
		require.NoError(t, err)
	})

	t.Run("Subscribe presence success without token and ErrorOnSubscribePresenceWithoutToken false", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.ErrorOnSubscribePresenceWithoutToken = false

		err := cli.SubscribePresence(ctx, targetJID)
		require.NoError(t, err)
	})
}

func TestPresence_SendChatPresence(t *testing.T) {
	ctx := context.Background()
	targetJID := types.NewJID("123456", types.DefaultUserServer)

	t.Run("Send chat presence composing with audio media", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		err := cli.SendChatPresence(ctx, targetJID, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
		require.NoError(t, err)
	})

	t.Run("Send chat presence paused", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		err := cli.SendChatPresence(ctx, targetJID, types.ChatPresencePaused, "")
		require.NoError(t, err)
	})
}

type mockErrPrivacyTokenStore struct {
	err error
}

func (m *mockErrPrivacyTokenStore) GetPrivacyToken(_ context.Context, _ types.JID) (*store.PrivacyToken, error) {
	return nil, m.err
}

func (m *mockErrPrivacyTokenStore) PutPrivacyTokens(_ context.Context, _ ...store.PrivacyToken) error {
	return m.err
}

func (m *mockErrPrivacyTokenStore) DeleteExpiredPrivacyTokens(_ context.Context, _ time.Time) (int64, error) {
	return 0, m.err
}
