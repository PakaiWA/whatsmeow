// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waWeb"
	"github.com/PakaiWA/whatsmeow/socket"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type dummySOCKS5Dialer struct{}

func (d *dummySOCKS5Dialer) Dial(network, addr string) (net.Conn, error) {
	return nil, errors.New("dummy dial")
}

func (d *dummySOCKS5Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return nil, errors.New("dummy dial context")
}

func TestClient_ProxiesAndTransports(t *testing.T) {
	cli := NewClient(&store.Device{}, waLog.Noop)

	t.Run("SetProxyAddress empty string resets proxy", func(t *testing.T) {
		err := cli.SetProxyAddress("")
		require.NoError(t, err)
	})

	t.Run("SetProxyAddress http URL", func(t *testing.T) {
		err := cli.SetProxyAddress("http://127.0.0.1:8080", SetProxyOptions{NoWebsocket: true})
		require.NoError(t, err)
	})

	t.Run("SetProxyAddress https URL", func(t *testing.T) {
		err := cli.SetProxyAddress("https://127.0.0.1:8443", SetProxyOptions{NoMedia: true})
		require.NoError(t, err)
	})

	t.Run("SetProxyAddress socks5 URL", func(t *testing.T) {
		err := cli.SetProxyAddress("socks5://127.0.0.1:1080", SetProxyOptions{OnlyLogin: true})
		require.NoError(t, err)
	})

	t.Run("SetProxyAddress unsupported scheme", func(t *testing.T) {
		err := cli.SetProxyAddress("ftp://127.0.0.1:21")
		assert.ErrorContains(t, err, "unsupported proxy scheme")
	})

	t.Run("SetProxyAddress invalid URL", func(t *testing.T) {
		err := cli.SetProxyAddress("http://invalid url:80")
		assert.Error(t, err)
	})

	t.Run("SetSOCKSProxy", func(t *testing.T) {
		cli.SetSOCKSProxy(&dummySOCKS5Dialer{})
	})

	t.Run("SetHTTPClients overrides", func(t *testing.T) {
		customMedia := &http.Client{Timeout: 10 * time.Second}
		customWS := &http.Client{Timeout: 20 * time.Second}
		customPre := &http.Client{Timeout: 30 * time.Second}

		cli.SetMediaHTTPClient(customMedia)
		cli.SetWebsocketHTTPClient(customWS)
		cli.SetPreLoginHTTPClient(customPre)

		assert.Equal(t, customMedia, cli.mediaHTTP)
		assert.Equal(t, customWS, cli.websocketHTTP)
		assert.Equal(t, customPre, cli.preLoginHTTP)
	})

	t.Run("SetMaxParallelRetryReceiptHandling", func(t *testing.T) {
		cli.SetMaxParallelRetryReceiptHandling(5)
		assert.NotNil(t, cli.retrySema)

		cli.SetMaxParallelRetryReceiptHandling(0)
		assert.Nil(t, cli.retrySema)

		cli.SetMaxParallelRetryReceiptHandling(-1)
		assert.Nil(t, cli.retrySema)
	})
}

func TestClient_EventHandlers(t *testing.T) {
	cli := NewClient(&store.Device{}, waLog.Noop)

	var count1, count2 int
	id1 := cli.AddEventHandler(func(evt any) {
		count1++
	})
	id2 := cli.AddEventHandlerWithSuccessStatus(func(evt any) bool {
		count2++
		return true
	})
	assert.NotZero(t, id2)

	cli.dispatchEvent(&events.Connected{})
	assert.Equal(t, 1, count1)
	assert.Equal(t, 1, count2)

	// Remove middle or first
	assert.True(t, cli.RemoveEventHandler(id1))
	cli.dispatchEvent(&events.Connected{})
	assert.Equal(t, 1, count1)
	assert.Equal(t, 2, count2)

	// Add new and test removing non-zero index
	id3 := cli.AddEventHandler(func(evt any) { count1++ })
	id4 := cli.AddEventHandler(func(evt any) { count1++ })
	assert.True(t, cli.RemoveEventHandler(id3))
	assert.True(t, cli.RemoveEventHandler(id4))

	// Remove non-existent
	assert.False(t, cli.RemoveEventHandler(99999))

	// Remove all
	cli.RemoveEventHandlers()
	cli.dispatchEvent(&events.Connected{})
	assert.Equal(t, 2, count2)

	// Panic recovery inside dispatchEvent
	cli.AddEventHandler(func(evt any) {
		panic("handler panic test")
	})
	assert.NotPanics(t, func() {
		cli.dispatchEvent(&events.Connected{})
	})
}

func TestClient_IsRetryableConnectError(t *testing.T) {
	assert.True(t, isRetryableConnectError(socket.ErrDialFailed))
	assert.True(t, isRetryableConnectError(&net.OpError{Op: "dial", Err: errors.New("network unreachable")}))
	assert.True(t, isRetryableConnectError(socket.ErrWithStatusCode{StatusCode: 500}))
	assert.True(t, isRetryableConnectError(socket.ErrWithStatusCode{StatusCode: 503}))
	assert.True(t, isRetryableConnectError(socket.ErrWithStatusCode{StatusCode: 408}))
	assert.False(t, isRetryableConnectError(socket.ErrWithStatusCode{StatusCode: 404}))
	assert.False(t, isRetryableConnectError(errors.New("random error")))
}

func TestClient_ConnectContext_And_AutoReconnect(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client returns ErrClientIsNil", func(t *testing.T) {
		var nilCli *Client
		assert.ErrorIs(t, nilCli.Connect(), ErrClientIsNil)
		assert.ErrorIs(t, nilCli.ConnectContext(ctx), ErrClientIsNil)
		assert.False(t, nilCli.IsConnected())
		assert.False(t, nilCli.IsLoggedIn())
		assert.False(t, nilCli.WaitForConnection(time.Millisecond))
		nilCli.Disconnect()
		nilCli.ResetConnection()
		nilCli.sendUnifiedSession()
	})

	t.Run("InitialAutoReconnect triggers reconnect loop in background", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "12345", Server: types.DefaultUserServer},
		}, waLog.Noop)
		cli.InitialAutoReconnect = true
		cli.EnableAutoReconnect = true
		// Mock websocketHTTP to fail with retryable dial error
		cli.websocketHTTP = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return nil, socket.ErrDialFailed
				},
			},
		}

		err := cli.ConnectContext(cancelCtx)
		require.NoError(t, err)
		cli.Disconnect()
	})

	t.Run("AutoReconnect loop exits on expectedDisconnect", func(t *testing.T) {
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "12345", Server: types.DefaultUserServer},
		}, waLog.Noop)
		cli.EnableAutoReconnect = true
		cli.expectDisconnect()
		cli.AutoReconnectErrors = 1
		cli.websocketHTTP = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return nil, socket.ErrDialFailed
				},
			},
		}

		// Run autoReconnect, should exit immediately
		cli.autoReconnect(ctx)
	})

	t.Run("AutoReconnect loop exits on cancelled context", func(t *testing.T) {
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "12345", Server: types.DefaultUserServer},
		}, waLog.Noop)
		cli.EnableAutoReconnect = true
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		cli.autoReconnect(cancelCtx)
	})

	t.Run("AutoReconnect loop with AutoReconnectHook", func(t *testing.T) {
		cli := NewClient(&store.Device{
			ID: &types.JID{User: "12345", Server: types.DefaultUserServer},
		}, waLog.Noop)
		cli.EnableAutoReconnect = true
		cli.AutoReconnectErrors = 0
		cli.AutoReconnectHook = func(err error) bool {
			return false // stop reconnecting
		}

		// connect will fail because websocket cannot connect to dummy URL
		cli.websocketHTTP = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return nil, errors.New("dial failed")
				},
			},
		}

		cli.autoReconnect(ctx)
	})
}

func TestClient_ConnectionLifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("GetOwnID and GetOwnLID", func(t *testing.T) {
		jid := types.NewJID("628123", types.DefaultUserServer)
		lid := types.NewJID("1001", types.HiddenUserServer)
		cli := NewClient(&store.Device{ID: &jid, LID: lid}, waLog.Noop)

		assert.Equal(t, jid, cli.getOwnID())
		assert.Equal(t, lid, cli.getOwnLID())
		assert.Equal(t, "", cli.getUserAgent())

		cli.UserAgent = "whatsmeow-custom"
		assert.Equal(t, "whatsmeow-custom", cli.getUserAgent())

		cli.MessengerConfig = &MessengerConfig{UserAgent: "custom-ua"}
		assert.Equal(t, "custom-ua", cli.getUserAgent())
	})

	t.Run("WaitForConnection returns false on timeout", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		assert.False(t, cli.WaitForConnection(10*time.Millisecond))
	})

	t.Run("WaitForConnection returns false on expectedDisconnect", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.expectDisconnect()
		assert.False(t, cli.WaitForConnection(100*time.Millisecond))
	})

	t.Run("WaitForConnection returns true when connected and logged in", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		assert.True(t, cli.WaitForConnection(100*time.Millisecond))
	})

	t.Run("ResetConnection forces auto reconnect", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		cli.ResetConnection()
		assert.True(t, cli.forceAutoReconnect.Load())
	})

	t.Run("Logout validations", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)

		// Not logged in error
		assert.ErrorIs(t, cli.Logout(ctx), ErrNotLoggedIn)

		// Messenger credentials error
		cli.Store.ID = &types.JID{User: "123", Server: types.DefaultUserServer}
		cli.MessengerConfig = &MessengerConfig{}
		assert.ErrorContains(t, cli.Logout(ctx), "can't logout with Messenger credentials")
		cli.MessengerConfig = nil

		// SendIQ error during logout
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return nil, errors.New("iq fail")
		}
		assert.ErrorContains(t, cli.Logout(ctx), "error sending logout request")

		// Successful logout deletes store
		mockContainer := &mockCustomContainer{}
		cli.Store.Container = mockContainer
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}
		err := cli.Logout(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, mockContainer.deleteCalls)
	})

	t.Run("OnDisconnect handling", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		disconnectedChan := make(chan *events.Disconnected, 1)
		cli.AddEventHandler(func(evt any) {
			if d, ok := evt.(*events.Disconnected); ok {
				disconnectedChan <- d
			}
		})

		cli.socketLock.RLock()
		sock := cli.socket
		cli.socketLock.RUnlock()

		// Remote disconnect emits event
		cli.onDisconnect(ctx, sock, true)

		select {
		case disconnectedEvt := <-disconnectedChan:
			require.NotNil(t, disconnectedEvt)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for Disconnected event")
		}
	})
}

func TestClient_FrameHandling_And_QueueLoop(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	queue := make(chan *waBinary.Node, 10)
	frameHandler := cli.makeFrameHandler(queue)
	require.NotNil(t, frameHandler)

	t.Run("handleFrame invalid data", func(t *testing.T) {
		cli.handleFrame(ctx, []byte("invalid-data"), queue)
		assert.Empty(t, queue)
	})

	t.Run("handleFrame xmlstreamend frame", func(t *testing.T) {
		packed, err := waBinary.Marshal(waBinary.Node{Tag: "xmlstreamend"})
		require.NoError(t, err)
		cli.handleFrame(ctx, packed, queue)
		assert.Empty(t, queue)
	})

	t.Run("handleFrame routed to nodeHandlers queue", func(t *testing.T) {
		packed, err := waBinary.Marshal(waBinary.Node{Tag: "message"})
		require.NoError(t, err)
		cli.handleFrame(ctx, packed, queue)
		assert.Len(t, queue, 1)
		node := <-queue
		assert.Equal(t, "message", node.Tag)
	})

	t.Run("handlerQueueLoop processes node and handles cancellation", func(t *testing.T) {
		q := make(chan *waBinary.Node, 5)
		closeWait := make(chan struct{})

		processedChan := make(chan struct{}, 1)
		cli.nodeHandlers["test_tag"] = func(_ context.Context, _ *waBinary.Node) {
			select {
			case processedChan <- struct{}{}:
			default:
			}
		}

		connCtx, cancel := context.WithCancel(context.Background())
		go cli.handlerQueueLoop(ctx, connCtx, q, closeWait)

		q <- &waBinary.Node{Tag: "test_tag"}

		// Wait for node to be processed
		select {
		case <-processedChan:
		case <-time.After(1 * time.Second):
			t.Fatal("node was not processed in time")
		}

		// Cancel context and verify closeWait closes
		cancel()
		select {
		case <-closeWait:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("handlerQueueLoop did not close closeWait channel")
		}
	})

	t.Run("handlerQueueLoop handles stream:error after context cancellation", func(t *testing.T) {
		q := make(chan *waBinary.Node, 5)
		closeWait := make(chan struct{})

		q <- &waBinary.Node{Tag: "stream:error", Attrs: waBinary.Attrs{"code": "503"}}

		connCtx, cancel := context.WithCancel(context.Background())
		cancel() // already cancelled

		go cli.handlerQueueLoop(ctx, connCtx, q, closeWait)

		select {
		case <-closeWait:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("handlerQueueLoop did not drain stream:error")
		}
	})
}

func TestClient_ParseWebMessage(t *testing.T) {
	cli := NewClient(&store.Device{
		ID:  &types.JID{User: "11111", Server: types.DefaultUserServer},
		LID: types.NewJID("22222", types.HiddenUserServer),
	}, waLog.Noop)

	t.Run("Missing chat JID parses from remote JID", func(t *testing.T) {
		msg := &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String("62812345@s.whatsapp.net"),
				ID:        proto.String("msg-1"),
			},
			Message: &waE2E.Message{
				Conversation: proto.String("Hello"),
			},
		}

		evt, err := cli.ParseWebMessage(types.EmptyJID, msg)
		require.NoError(t, err)
		assert.Equal(t, "msg-1", evt.Info.ID)
		assert.Equal(t, "62812345@s.whatsapp.net", evt.Info.Chat.String())
		assert.Equal(t, "Hello", evt.Message.GetConversation())
	})

	t.Run("Invalid remote JID returns error", func(t *testing.T) {
		msg := &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String("user.part.extra@s.whatsapp.net"),
				ID:        proto.String("msg-err"),
			},
		}
		_, err := cli.ParseWebMessage(types.EmptyJID, msg)
		assert.ErrorContains(t, err, "failed to parse remote JID")
	})

	t.Run("IsFromMe with OriginalSelfAuthorUserJIDString", func(t *testing.T) {
		chat := types.NewJID("62812345", types.DefaultUserServer)
		msg := &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				FromMe: proto.Bool(true),
				ID:     proto.String("from-me-1"),
			},
			OriginalSelfAuthorUserJIDString: proto.String("6289999@s.whatsapp.net"),
			Message:                         &waE2E.Message{},
		}

		evt, err := cli.ParseWebMessage(chat, msg)
		require.NoError(t, err)
		assert.True(t, evt.Info.IsFromMe)
		assert.Equal(t, "6289999@s.whatsapp.net", evt.Info.Sender.String())
	})

	t.Run("IsFromMe fallback to own JID or LID", func(t *testing.T) {
		chat := types.NewJID("62812345", types.DefaultUserServer)
		msg := &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				FromMe: proto.Bool(true),
				ID:     proto.String("from-me-2"),
			},
			Message: &waE2E.Message{},
		}

		evt, err := cli.ParseWebMessage(chat, msg)
		require.NoError(t, err)
		assert.Equal(t, "11111@s.whatsapp.net", evt.Info.Sender.String())

		// Chat is HiddenUserServer
		hiddenChat := types.NewJID("33333", types.HiddenUserServer)
		evtLID, err := cli.ParseWebMessage(hiddenChat, msg)
		require.NoError(t, err)
		assert.Equal(t, "22222@lid", evtLID.Info.Sender.String())
	})

	t.Run("Group message with participant", func(t *testing.T) {
		groupChat := types.NewJID("group-1", types.GroupServer)
		participant := "6287777@s.whatsapp.net"
		msg := &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				FromMe: proto.Bool(false),
				ID:     proto.String("grp-1"),
			},
			Participant: &participant,
			Message:     &waE2E.Message{},
		}

		evt, err := cli.ParseWebMessage(groupChat, msg)
		require.NoError(t, err)
		assert.True(t, evt.Info.IsGroup)
		assert.Equal(t, participant, evt.Info.Sender.String())
	})

	t.Run("Message edit unwrapping", func(t *testing.T) {
		chat := types.NewJID("62812345", types.DefaultUserServer)
		msg := &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				FromMe: proto.Bool(false),
				ID:     proto.String("orig-id"),
			},
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
					Key: &waCommon.MessageKey{
						ID: proto.String("edited-target-id"),
					},
					EditedMessage: &waE2E.Message{
						Conversation: proto.String("Edited text"),
					},
				},
			},
		}

		evt, err := cli.ParseWebMessage(chat, msg)
		require.NoError(t, err)
		assert.Equal(t, "edited-target-id", evt.Info.ID)
		assert.Equal(t, "Edited text", evt.Message.GetConversation())
	})
}
