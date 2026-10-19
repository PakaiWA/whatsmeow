// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"sync/atomic"
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

func TestReceipt_BuildBaseReceipt(t *testing.T) {
	node := &waBinary.Node{
		Attrs: waBinary.Attrs{
			"from":        "sender@s.whatsapp.net",
			"recipient":   "recip@s.whatsapp.net",
			"participant": "part@s.whatsapp.net",
		},
	}
	attrs := buildBaseReceipt("msg-123", node)
	assert.Equal(t, "msg-123", attrs["id"])
	assert.Equal(t, "sender@s.whatsapp.net", attrs["to"])
	assert.Equal(t, "recip@s.whatsapp.net", attrs["recipient"])
	assert.Equal(t, "part@s.whatsapp.net", attrs["participant"])
}

func TestReceipt_SetForceActiveDeliveryReceipts(t *testing.T) {
	var nilCli *Client
	nilCli.SetForceActiveDeliveryReceipts(true) // Should not panic

	cli := NewClient(&store.Device{}, waLog.Noop)
	assert.Equal(t, uint32(0), cli.sendActiveReceipts.Load())

	cli.SetForceActiveDeliveryReceipts(true)
	assert.Equal(t, uint32(2), cli.sendActiveReceipts.Load())

	cli.SetForceActiveDeliveryReceipts(false)
	assert.Equal(t, uint32(0), cli.sendActiveReceipts.Load())
}

func TestReceipt_ParseReceipt(t *testing.T) {
	cli := NewClient(&store.Device{
		ID: &types.JID{User: "me", Server: types.DefaultUserServer},
	}, waLog.Noop)

	userJID := types.NewJID("user", types.DefaultUserServer)
	groupJID := types.NewJID("group", types.GroupServer)

	t.Run("Single message receipt", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": userJID,
				"id":   "msg-1",
				"t":    "1700000000",
				"type": "read",
			},
		}
		receipt, participants, err := cli.parseReceipt(node)
		require.NoError(t, err)
		assert.Nil(t, participants)
		require.NotNil(t, receipt)
		assert.Equal(t, []types.MessageID{"msg-1"}, receipt.MessageIDs)
		assert.Equal(t, types.ReceiptTypeRead, receipt.Type)
		assert.Equal(t, int64(1700000000), receipt.Timestamp.Unix())
	})

	t.Run("List of message IDs receipt", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": userJID,
				"id":   "msg-main",
				"t":    "1700000000",
			},
			Content: []waBinary.Node{{
				Tag: "list",
				Content: []waBinary.Node{
					{Tag: "item", Attrs: waBinary.Attrs{"id": "msg-sub-1"}},
					{Tag: "item", Attrs: waBinary.Attrs{"id": "msg-sub-2"}},
					{Tag: "other", Attrs: waBinary.Attrs{"id": "ignore-me"}},
				},
			}},
		}
		receipt, participants, err := cli.parseReceipt(node)
		require.NoError(t, err)
		assert.Nil(t, participants)
		require.NotNil(t, receipt)
		assert.Equal(t, []types.MessageID{"msg-main", "msg-sub-1", "msg-sub-2"}, receipt.MessageIDs)
	})

	t.Run("Grouped receipt with participants", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": groupJID,
				"t":    "1700000000",
			},
			Content: []waBinary.Node{{
				Tag: "participants",
				Attrs: waBinary.Attrs{
					"key": "msg-group",
				},
				Content: []waBinary.Node{{
					Tag: "user",
					Attrs: waBinary.Attrs{
						"jid": types.NewJID("user1", types.DefaultUserServer),
						"t":   "1700000005",
					},
				}},
			}},
		}
		receipt, participants, err := cli.parseReceipt(node)
		require.NoError(t, err)
		require.Len(t, participants, 1)
		require.NotNil(t, receipt)
		assert.True(t, receipt.IsGroup)
	})

	t.Run("Grouped receipt missing participants tag", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": groupJID,
				"t":    "1700000000",
			},
		}
		receipt, participants, err := cli.parseReceipt(node)
		require.Error(t, err)
		assert.Nil(t, receipt)
		assert.Nil(t, participants)
		var elemErr *ElementMissingError
		assert.ErrorAs(t, err, &elemErr)
	})

	t.Run("Invalid attrs missing id", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": userJID,
				// missing "id"
			},
		}
		receipt, participants, err := cli.parseReceipt(node)
		require.Error(t, err)
		assert.Nil(t, receipt)
		assert.Nil(t, participants)
	})
}

func TestReceipt_HandleGroupedReceipt(t *testing.T) {
	cli := NewClient(&store.Device{}, waLog.Noop)
	var dispatched []events.Receipt
	cli.AddEventHandler(func(evt any) {
		if r, ok := evt.(*events.Receipt); ok {
			dispatched = append(dispatched, *r)
		}
	})

	partial := events.Receipt{
		MessageSource: types.MessageSource{
			Chat:    types.NewJID("group", types.GroupServer),
			IsGroup: true,
		},
	}
	user1JID := types.NewJID("user1", types.DefaultUserServer)
	participantsNode := &waBinary.Node{
		Tag:   "participants",
		Attrs: waBinary.Attrs{"key": "group-key-1"},
		Content: []waBinary.Node{
			{
				Tag: "user",
				Attrs: waBinary.Attrs{
					"jid": user1JID,
					"t":   "1700000010",
				},
			},
			{
				Tag: "ignored_tag",
			},
			{
				Tag: "user",
				Attrs: waBinary.Attrs{
					// Missing jid will cause ag.OK() to be false
					"t": "1700000020",
				},
			},
		},
	}

	cancelled := cli.handleGroupedReceipt(partial, participantsNode)
	assert.False(t, cancelled)
	require.Len(t, dispatched, 1)
	assert.Equal(t, []types.MessageID{"group-key-1"}, dispatched[0].MessageIDs)
	assert.Equal(t, user1JID, dispatched[0].Sender)
	assert.Equal(t, int64(1700000010), dispatched[0].Timestamp.Unix())
}

func TestReceipt_HandleReceipt(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{
		ID: &types.JID{User: "me", Server: types.DefaultUserServer},
	}, waLog.Noop)

	var dispatchedCount atomic.Int32
	cli.AddEventHandler(func(evt any) {
		if _, ok := evt.(*events.Receipt); ok {
			dispatchedCount.Add(1)
		}
	})

	userJID := types.NewJID("user", types.DefaultUserServer)

	t.Run("Parsing error triggers sendAck with NackParsingError", func(t *testing.T) {
		// Node with invalid from triggers parsing error
		node := &waBinary.Node{
			Tag:   "receipt",
			Attrs: waBinary.Attrs{"from": 12345},
		}
		cli.handleReceipt(ctx, node)
	})

	t.Run("Valid single receipt", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": userJID,
				"id":   "msg-123",
				"t":    "1700000000",
				"type": "read",
			},
		}
		cli.handleReceipt(ctx, node)
		assert.Equal(t, int32(1), dispatchedCount.Load())
	})

	t.Run("Retry receipt handling", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "receipt",
			Attrs: waBinary.Attrs{
				"from": userJID,
				"id":   "msg-retry",
				"t":    "1700000000",
				"type": string(types.ReceiptTypeRetry),
			},
		}
		cli.SynchronousAck = true
		cli.handleReceipt(ctx, node)
	})
}

func TestReceipt_MarkRead(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{
		ID: &types.JID{User: "me", Server: types.DefaultUserServer},
	}, waLog.Noop)

	userJID := types.NewJID("12345", types.DefaultUserServer)
	groupJID := types.NewJID("group", types.GroupServer)
	newsJID := types.NewJID("news", types.NewsletterServer)

	t.Run("Empty message IDs returns error", func(t *testing.T) {
		err := cli.MarkRead(ctx, []types.MessageID{}, time.Now(), userJID, types.EmptyJID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no message IDs specified")
	})

	t.Run("Multiple extra receipt types panics", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = cli.MarkRead(ctx, []types.MessageID{"msg-1"}, time.Now(), userJID, types.EmptyJID, types.ReceiptTypeRead, types.ReceiptTypePlayed)
		})
	})

	t.Run("MarkRead for NewsletterServer switches to ReadSelf", func(t *testing.T) {
		err := cli.MarkRead(ctx, []types.MessageID{"msg-1", "msg-2"}, time.Now(), newsJID, types.EmptyJID)
		// Will fail with ErrNotConnected since socket is nil, but exercises the logic
		require.ErrorIs(t, err, ErrNotConnected)
	})

	t.Run("MarkRead for group with participant", func(t *testing.T) {
		senderJID := types.NewJID("sender", types.DefaultUserServer)
		err := cli.MarkRead(ctx, []types.MessageID{"msg-1"}, time.Now(), groupJID, senderJID, types.ReceiptTypePlayed)
		require.ErrorIs(t, err, ErrNotConnected)
	})
}

func TestReceipt_SendMessageReceipt(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{
		ID: &types.JID{User: "me", Server: types.DefaultUserServer},
	}, waLog.Noop)

	node := &waBinary.Node{
		Attrs: waBinary.Attrs{
			"from": "sender@s.whatsapp.net",
		},
	}

	t.Run("Outgoing message", func(t *testing.T) {
		info := &types.MessageInfo{
			MessageSource: types.MessageSource{IsFromMe: true},
			ID:            "msg-out",
			Type:          "peer_msg",
		}
		cli.sendMessageReceipt(ctx, info, node)
	})

	t.Run("Incoming message inactive receipts", func(t *testing.T) {
		info := &types.MessageInfo{
			MessageSource: types.MessageSource{IsFromMe: false},
			ID:            "msg-in",
		}
		cli.sendMessageReceipt(ctx, info, node)
	})
}

func TestReceipt_AcksAndDeferred(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{
		ID: &types.JID{User: "me", Server: types.DefaultUserServer},
	}, waLog.Noop)

	node := &waBinary.Node{
		Tag: "message",
		Attrs: waBinary.Attrs{
			"from": types.NewJID("user", types.DefaultUserServer),
			"id":   "ack-test-1",
		},
	}

	// backgroundIfAsyncAck
	called := false
	cli.SynchronousAck = true
	cli.backgroundIfAsyncAck(func() {
		called = true
	})
	assert.True(t, called)

	// maybeDeferredAck synchronous
	ackFn := cli.maybeDeferredAck(ctx, node)
	cancelled := true
	ackFn(&cancelled) // Should not send ack if cancelled

	cancelled = false
	ackFn(&cancelled) // Should call sendAck

	// maybeDeferredAck asynchronous
	cli.SynchronousAck = false
	asyncAckFn := cli.maybeDeferredAck(ctx, node)
	asyncAckFn()

	// sendAck with error code and bot recipient
	botNode := &waBinary.Node{
		Tag: "message",
		Attrs: waBinary.Attrs{
			"from":        types.NewJID("user", types.DefaultUserServer),
			"id":          "ack-test-2",
			"recipient":   types.NewJID("bot", types.BotServer),
			"participant": types.NewJID("user2", types.DefaultUserServer),
			"type":        "read",
		},
	}
	cli.sendAck(ctx, botNode, NackSignalErrorOldCounter)
}
