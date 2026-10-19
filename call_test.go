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

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestCall_RejectCall(t *testing.T) {
	ctx := context.Background()
	callFrom := types.NewJID("628123456789", types.DefaultUserServer)
	callID := "call-test-id-123"

	t.Run("Not logged in", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		err := cli.RejectCall(ctx, callFrom, callID)
		assert.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("Success", func(t *testing.T) {
		device := &store.Device{
			ID: &types.JID{User: "12345", Server: types.DefaultUserServer},
		}
		cli := NewClient(device, waLog.Noop)

		var sentNode *waBinary.Node
		cli.sendNodeHook = func(_ context.Context, node waBinary.Node) error {
			sentNode = &node
			return nil
		}

		err := cli.RejectCall(ctx, callFrom, callID)
		require.NoError(t, err)
		require.NotNil(t, sentNode)
		assert.Equal(t, "call", sentNode.Tag)
		assert.Equal(t, types.NewJID("12345", types.DefaultUserServer), sentNode.Attrs["from"])
		assert.Equal(t, callFrom, sentNode.Attrs["to"])
		require.NotEmpty(t, sentNode.Attrs["id"])

		children := sentNode.GetChildren()
		require.Len(t, children, 1)
		child := children[0]
		assert.Equal(t, "reject", child.Tag)
		assert.Equal(t, callID, child.Attrs["call-id"])
		assert.Equal(t, callFrom, child.Attrs["call-creator"])
		assert.Equal(t, "0", child.Attrs["count"])
	})
}

func TestCall_HandleCallEvent(t *testing.T) {
	ctx := context.Background()
	fromJID := types.NewJID("628123456789", types.DefaultUserServer)
	callerLID := types.NewJID("100200300", types.HiddenUserServer)
	callerPN := types.NewJID("628999999", types.DefaultUserServer)
	groupJID := types.NewJID("group123", types.GroupServer)
	tsNow := time.Now().Truncate(time.Second)

	setupClientWithEventHandler := func() (*Client, chan any) {
		device := &store.Device{
			ID: &types.JID{User: "12345", Server: types.DefaultUserServer},
		}
		cli := NewClient(device, waLog.Noop)
		evtChan := make(chan any, 10)
		cli.AddEventHandler(func(evt any) {
			evtChan <- evt
		})
		return cli, evtChan
	}

	t.Run("Unknown call event when no children", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag:   "call",
			Attrs: waBinary.Attrs{"from": fromJID},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			unknown, ok := evt.(*events.UnknownCallEvent)
			require.True(t, ok)
			assert.Equal(t, node, unknown.Node)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("Unknown call event when multiple children", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag:   "call",
			Attrs: waBinary.Attrs{"from": fromJID},
			Content: []waBinary.Node{
				{Tag: "offer"},
				{Tag: "terminate"},
			},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			unknown, ok := evt.(*events.UnknownCallEvent)
			require.True(t, ok)
			assert.Equal(t, node, unknown.Node)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallOffer with normal creator and caller_lid", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from":     fromJID,
				"t":        tsNow.Unix(),
				"platform": "android",
				"version":  "2.24.1",
			},
			Content: []waBinary.Node{{
				Tag: "offer",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-offer-1",
					"group-jid":    groupJID,
					"caller_lid":   callerLID,
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			offer, ok := evt.(*events.CallOffer)
			require.True(t, ok)
			assert.Equal(t, fromJID, offer.CallCreator)
			assert.Equal(t, "call-offer-1", offer.CallID)
			assert.Equal(t, groupJID, offer.GroupJID)
			assert.Equal(t, callerLID, offer.CallCreatorAlt)
			assert.Equal(t, "android", offer.RemotePlatform)
			assert.Equal(t, "2.24.1", offer.RemoteVersion)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallOffer with hidden user server and caller_pn", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from": callerLID,
				"t":    tsNow.Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "offer",
				Attrs: waBinary.Attrs{
					"call-creator": callerLID,
					"call-id":      "call-offer-lid",
					"caller_pn":    callerPN,
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			offer, ok := evt.(*events.CallOffer)
			require.True(t, ok)
			assert.Equal(t, callerLID, offer.CallCreator)
			assert.Equal(t, callerPN, offer.CallCreatorAlt)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallOfferNotice", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from": fromJID,
				"t":    tsNow.Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "offer_notice",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-notice-1",
					"media":        "audio",
					"type":         "regular",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			notice, ok := evt.(*events.CallOfferNotice)
			require.True(t, ok)
			assert.Equal(t, "call-notice-1", notice.CallID)
			assert.Equal(t, "audio", notice.Media)
			assert.Equal(t, "regular", notice.Type)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallRelayLatency", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from": fromJID,
				"t":    tsNow.Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "relaylatency",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-relay-1",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			relay, ok := evt.(*events.CallRelayLatency)
			require.True(t, ok)
			assert.Equal(t, "call-relay-1", relay.CallID)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallAccept", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from":     fromJID,
				"t":        tsNow.Unix(),
				"platform": "ios",
				"version":  "2.24.2",
			},
			Content: []waBinary.Node{{
				Tag: "accept",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-accept-1",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			accept, ok := evt.(*events.CallAccept)
			require.True(t, ok)
			assert.Equal(t, "call-accept-1", accept.CallID)
			assert.Equal(t, "ios", accept.RemotePlatform)
			assert.Equal(t, "2.24.2", accept.RemoteVersion)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallPreAccept", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from":     fromJID,
				"t":        tsNow.Unix(),
				"platform": "ios",
				"version":  "2.24.2",
			},
			Content: []waBinary.Node{{
				Tag: "preaccept",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-preaccept-1",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			preAccept, ok := evt.(*events.CallPreAccept)
			require.True(t, ok)
			assert.Equal(t, "call-preaccept-1", preAccept.CallID)
			assert.Equal(t, "ios", preAccept.RemotePlatform)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallTransport", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from":     fromJID,
				"t":        tsNow.Unix(),
				"platform": "smba",
			},
			Content: []waBinary.Node{{
				Tag: "transport",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-transport-1",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			transport, ok := evt.(*events.CallTransport)
			require.True(t, ok)
			assert.Equal(t, "call-transport-1", transport.CallID)
			assert.Equal(t, "smba", transport.RemotePlatform)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallTerminate", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from": fromJID,
				"t":    tsNow.Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "terminate",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-term-1",
					"reason":       "timeout",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			term, ok := evt.(*events.CallTerminate)
			require.True(t, ok)
			assert.Equal(t, "call-term-1", term.CallID)
			assert.Equal(t, "timeout", term.Reason)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("CallReject", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from": fromJID,
				"t":    tsNow.Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "reject",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-reject-1",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			reject, ok := evt.(*events.CallReject)
			require.True(t, ok)
			assert.Equal(t, "call-reject-1", reject.CallID)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("Unknown child tag", func(t *testing.T) {
		cli, evtChan := setupClientWithEventHandler()
		node := &waBinary.Node{
			Tag: "call",
			Attrs: waBinary.Attrs{
				"from": fromJID,
				"t":    tsNow.Unix(),
			},
			Content: []waBinary.Node{{
				Tag: "something_custom",
				Attrs: waBinary.Attrs{
					"call-creator": fromJID,
					"call-id":      "call-custom-1",
				},
			}},
		}
		cli.handleCallEvent(ctx, node)

		select {
		case evt := <-evtChan:
			unknown, ok := evt.(*events.UnknownCallEvent)
			require.True(t, ok)
			assert.Equal(t, node, unknown.Node)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for event")
		}
	})
}
