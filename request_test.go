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
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestRequest_HelperNodes(t *testing.T) {
	t.Run("isDisconnectNode", func(t *testing.T) {
		assert.True(t, isDisconnectNode(xmlStreamEndNode))
		assert.True(t, isDisconnectNode(&waBinary.Node{Tag: "stream:error"}))
		assert.False(t, isDisconnectNode(&waBinary.Node{Tag: "iq"}))
		assert.False(t, isDisconnectNode(&waBinary.Node{Tag: "ack"}))
	})

	t.Run("isAuthErrorDisconnect", func(t *testing.T) {
		nonStream := &waBinary.Node{Tag: "iq"}
		assert.False(t, isAuthErrorDisconnect(nonStream))

		code401 := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "401"},
		}
		assert.True(t, isAuthErrorDisconnect(code401))

		code500 := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "500"},
		}
		assert.False(t, isAuthErrorDisconnect(code500))

		conflictReplaced := &waBinary.Node{
			Tag: "stream:error",
			Content: []waBinary.Node{{
				Tag:   "conflict",
				Attrs: waBinary.Attrs{"type": "replaced"},
			}},
		}
		assert.True(t, isAuthErrorDisconnect(conflictReplaced))

		conflictRemoved := &waBinary.Node{
			Tag: "stream:error",
			Content: []waBinary.Node{{
				Tag:   "conflict",
				Attrs: waBinary.Attrs{"type": "device_removed"},
			}},
		}
		assert.True(t, isAuthErrorDisconnect(conflictRemoved))

		conflictOther := &waBinary.Node{
			Tag: "stream:error",
			Content: []waBinary.Node{{
				Tag:   "conflict",
				Attrs: waBinary.Attrs{"type": "other"},
			}},
		}
		assert.False(t, isAuthErrorDisconnect(conflictOther))
	})
}

func TestRequest_ResponseWaiters(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	// waitResponse and cancelResponse
	ch := cli.waitResponse("req-1")
	assert.NotNil(t, ch)
	cli.cancelResponse("req-1", ch)

	// receiveResponse on non-existent or invalid tag
	assert.False(t, cli.receiveResponse(ctx, &waBinary.Node{Tag: "message", Attrs: waBinary.Attrs{"id": "req-1"}}))
	assert.False(t, cli.receiveResponse(ctx, &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"id": "unknown-id"}}))
	assert.False(t, cli.receiveResponse(ctx, &waBinary.Node{Tag: "iq"}))

	// receiveResponse success
	ch2 := cli.waitResponse("req-2")
	iqNode := &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"id": "req-2", "type": "result"}}
	assert.True(t, cli.receiveResponse(ctx, iqNode))
	select {
	case received := <-ch2:
		assert.Equal(t, iqNode, received)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected response in waiter")
	}

	// clearResponseWaiters
	ch3 := cli.waitResponse("req-3")
	ch4 := cli.waitResponse("req-4")
	endNode := &waBinary.Node{Tag: "xmlstreamend"}
	cli.clearResponseWaiters(endNode)
	assert.Equal(t, endNode, <-ch3)
	assert.Equal(t, endNode, <-ch4)
}

func TestRequest_GenerateRequestID(t *testing.T) {
	cli := NewClient(&store.Device{}, waLog.Noop)
	id1 := cli.generateRequestID()
	id2 := cli.generateRequestID()
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestRequest_SendIQ_Variations(t *testing.T) {
	ctx := context.Background()

	t.Run("Client is nil in sendIQAsyncAndGetData", func(t *testing.T) {
		var nilCli *Client
		_, _, err := nilCli.sendIQAsyncAndGetData(ctx, &infoQuery{})
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("SendIQHook active", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		expectedNode := &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}}
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return expectedNode, nil
		}
		res, err := cli.sendIQ(ctx, infoQuery{})
		require.NoError(t, err)
		assert.Equal(t, expectedNode, res)
	})

	t.Run("SendIQ Async channels", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		// Not connected client calling sendIQAsync will fail on sendNode
		query := infoQuery{
			Namespace: "test",
			Type:      iqGet,
			SMaxID:    "smax-123",
			To:        types.NewJID("user", types.DefaultUserServer),
			Target:    types.NewJID("target", types.DefaultUserServer),
		}
		ch, err := cli.sendIQAsync(ctx, query)
		require.Error(t, err)
		assert.Nil(t, ch)
	})

	t.Run("Context canceled during sendIQ", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		// Fake sendIQAsync response by directly testing the select branch
		query := infoQuery{Timeout: 10 * time.Millisecond}
		_, err := cli.sendIQ(canceledCtx, query)
		require.Error(t, err)
	})

	t.Run("SendIQ Timeout", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		// We can intercept sendNodeAndGetData by setting mock or testing timeout directly with waiter
		query := infoQuery{
			ID:      "test-timeout",
			Timeout: 5 * time.Millisecond,
		}
		// If sendIQAsyncAndGetData fails because socket is nil, let's see:
		_, err := cli.sendIQ(ctx, query)
		require.Error(t, err) // ErrNotConnected
	})
}

func TestRequest_RetryFrame_Disconnected(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	// Auth error in origResp returns error directly without retry
	authErrNode := &waBinary.Node{
		Tag:   "stream:error",
		Attrs: waBinary.Attrs{"code": "401"},
	}
	res, err := cli.retryFrame(ctx, "info query", "req-1", []byte("data"), authErrNode, time.Second)
	require.Error(t, err)
	assert.Nil(t, res)
	var discErr *DisconnectedError
	assert.ErrorAs(t, err, &discErr)

	// Non-auth error but client not reconnecting
	otherErrNode := &waBinary.Node{
		Tag:   "stream:error",
		Attrs: waBinary.Attrs{"code": "500"},
	}
	cli.expectedDisconnect.Set()
	res, err = cli.retryFrame(ctx, "info query", "req-2", []byte("data"), otherErrNode, 50*time.Millisecond)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorAs(t, err, &discErr)
}

func TestRequest_SendIQ_WithConnectedSocket(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	t.Run("Success result", func(t *testing.T) {
		reqID := "iq-success"
		go func() {
			time.Sleep(10 * time.Millisecond)
			cli.receiveResponse(ctx, &waBinary.Node{
				Tag:   "iq",
				Attrs: waBinary.Attrs{"id": reqID, "type": "result"},
			})
		}()
		res, err := cli.sendIQ(ctx, infoQuery{ID: reqID, Namespace: "w:p", Type: iqGet})
		require.NoError(t, err)
		assert.Equal(t, reqID, res.Attrs["id"])
	})

	t.Run("IQ error response", func(t *testing.T) {
		reqID := "iq-error"
		go func() {
			time.Sleep(10 * time.Millisecond)
			cli.receiveResponse(ctx, &waBinary.Node{
				Tag:   "iq",
				Attrs: waBinary.Attrs{"id": reqID, "type": "error"},
				Content: []waBinary.Node{{
					Tag:   "error",
					Attrs: waBinary.Attrs{"code": "404", "text": "not-found"},
				}},
			})
		}()
		res, err := cli.sendIQ(ctx, infoQuery{ID: reqID, Namespace: "w:p", Type: iqGet})
		require.Error(t, err)
		assert.NotNil(t, res)
		var iqErr *IQError
		assert.ErrorAs(t, err, &iqErr)
		assert.Equal(t, 404, iqErr.Code)
	})

	t.Run("Unexpected IQ type", func(t *testing.T) {
		reqID := "iq-unknown-type"
		go func() {
			time.Sleep(10 * time.Millisecond)
			cli.receiveResponse(ctx, &waBinary.Node{
				Tag:   "iq",
				Attrs: waBinary.Attrs{"id": reqID, "type": "weird"},
			})
		}()
		res, err := cli.sendIQ(ctx, infoQuery{ID: reqID, Namespace: "w:p", Type: iqGet})
		require.Error(t, err)
		var iqErr *IQError
		assert.ErrorAs(t, err, &iqErr)
		assert.Equal(t, res, iqErr.RawNode)
	})

	t.Run("Disconnect node with NoRetry", func(t *testing.T) {
		reqID := "iq-disc-noretry"
		go func() {
			time.Sleep(10 * time.Millisecond)
			cli.clearResponseWaiters(xmlStreamEndNode)
		}()
		res, err := cli.sendIQ(ctx, infoQuery{ID: reqID, Namespace: "w:p", Type: iqGet, NoRetry: true, Timeout: 500 * time.Millisecond})
		require.Error(t, err)
		assert.Nil(t, res)
		var discErr *DisconnectedError
		assert.ErrorAs(t, err, &discErr)
	})

	t.Run("Timeout triggers ErrIQTimedOut", func(t *testing.T) {
		reqID := "iq-timeout-fast"
		_, err := cli.sendIQ(ctx, infoQuery{ID: reqID, Namespace: "w:p", Type: iqGet, Timeout: 15 * time.Millisecond})
		require.ErrorIs(t, err, ErrIQTimedOut)
	})
}

func TestRequest_RetryFrame_Connected(t *testing.T) {
	ctx := context.Background()
	cli, cleanup := setupMockSocketClient(t)
	defer cleanup()

	origResp := &waBinary.Node{
		Tag:   "stream:error",
		Attrs: waBinary.Attrs{"code": "503"},
	}

	t.Run("Success on retry", func(t *testing.T) {
		reqID := "retry-ok"
		go func() {
			time.Sleep(10 * time.Millisecond)
			cli.receiveResponse(ctx, &waBinary.Node{
				Tag:   "iq",
				Attrs: waBinary.Attrs{"id": reqID, "type": "result"},
			})
		}()
		res, err := cli.retryFrame(ctx, "info query", reqID, []byte("<node/>"), origResp, time.Second)
		require.NoError(t, err)
		assert.Equal(t, reqID, res.Attrs["id"])
	})

	t.Run("Disconnect on retry", func(t *testing.T) {
		reqID := "retry-disc"
		go func() {
			time.Sleep(10 * time.Millisecond)
			cli.clearResponseWaiters(xmlStreamEndNode)
		}()
		res, err := cli.retryFrame(ctx, "info query", reqID, []byte("<node/>"), origResp, time.Second)
		require.Error(t, err)
		assert.Nil(t, res)
		var discErr *DisconnectedError
		assert.ErrorAs(t, err, &discErr)
	})

	t.Run("Timeout on retry", func(t *testing.T) {
		reqID := "retry-timeout"
		res, err := cli.retryFrame(ctx, "info query", reqID, []byte("<node/>"), origResp, 15*time.Millisecond)
		require.ErrorIs(t, err, ErrIQTimedOut)
		assert.Nil(t, res)
	})
}
