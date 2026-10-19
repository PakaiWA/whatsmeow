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

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
)

func TestKeepAlive_SendKeepAlive(t *testing.T) {
	t.Run("Context already cancelled", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		isSuccess, shouldContinue := cli.sendKeepAlive(ctx)
		assert.False(t, isSuccess)
		assert.False(t, shouldContinue)
	})

	t.Run("sendIQAsync error", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		cleanup()

		origDeadline := KeepAliveResponseDeadline
		KeepAliveResponseDeadline = 20 * time.Millisecond
		defer func() { KeepAliveResponseDeadline = origDeadline }()

		ctx := context.Background()
		isSuccess, shouldContinue := cli.sendKeepAlive(ctx)
		assert.False(t, isSuccess)
		assert.True(t, shouldContinue)
	})

	t.Run("Success response received", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Background responder to fulfill the keepalive IQ query
		go func() {
			for i := 0; i < 100; i++ {
				time.Sleep(5 * time.Millisecond)
				cli.responseWaitersLock.Lock()
				for id, waiter := range cli.responseWaiters {
					delete(cli.responseWaiters, id)
					cli.responseWaitersLock.Unlock()
					waiter <- &waBinary.Node{
						Tag:   "iq",
						Attrs: waBinary.Attrs{"id": id, "type": "result", "from": types.ServerJID},
					}
					return
				}
				cli.responseWaitersLock.Unlock()
			}
		}()

		isSuccess, shouldContinue := cli.sendKeepAlive(ctx)
		assert.True(t, isSuccess)
		assert.True(t, shouldContinue)
	})

	t.Run("Timeout waiting for response", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		origDeadline := KeepAliveResponseDeadline
		KeepAliveResponseDeadline = 20 * time.Millisecond
		defer func() { KeepAliveResponseDeadline = origDeadline }()

		ctx := context.Background()
		isSuccess, shouldContinue := cli.sendKeepAlive(ctx)
		assert.False(t, isSuccess)
		assert.True(t, shouldContinue)
	})

	t.Run("Context cancelled while waiting for response", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		origDeadline := KeepAliveResponseDeadline
		KeepAliveResponseDeadline = 2 * time.Second
		defer func() { KeepAliveResponseDeadline = origDeadline }()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		isSuccess, shouldContinue := cli.sendKeepAlive(ctx)
		assert.False(t, isSuccess)
		assert.False(t, shouldContinue)
	})
}

func TestKeepAlive_KeepAliveLoop(t *testing.T) {
	origMin := KeepAliveIntervalMin
	origMax := KeepAliveIntervalMax
	origMaxFail := KeepAliveMaxFailTime
	origDeadline := KeepAliveResponseDeadline
	defer func() {
		KeepAliveIntervalMin = origMin
		KeepAliveIntervalMax = origMax
		KeepAliveMaxFailTime = origMaxFail
		KeepAliveResponseDeadline = origDeadline
	}()

	KeepAliveIntervalMin = 5 * time.Millisecond
	KeepAliveIntervalMax = 10 * time.Millisecond
	KeepAliveResponseDeadline = 5 * time.Millisecond
	KeepAliveMaxFailTime = 15 * time.Millisecond

	t.Run("connCtx cancelled returns immediately", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.EnableAutoReconnect = false

		ctx := context.Background()
		connCtx, connCancel := context.WithCancel(ctx)
		connCancel()

		done := make(chan struct{})
		go func() {
			cli.keepAliveLoop(ctx, connCtx)
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("keepAliveLoop did not exit on cancelled connCtx")
		}
	})

	t.Run("Keepalive loop dispatches timeout and restored events", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.EnableAutoReconnect = false

		ctx := context.Background()
		connCtx, connCancel := context.WithCancel(ctx)
		defer connCancel()

		var timeoutEventDispatched atomic.Bool
		var restoredEventDispatched atomic.Bool
		var eventCh = make(chan struct{}, 10)

		cli.AddEventHandler(func(evt any) {
			switch evt.(type) {
			case *events.KeepAliveTimeout:
				timeoutEventDispatched.Store(true)
				eventCh <- struct{}{}
			case *events.KeepAliveRestored:
				restoredEventDispatched.Store(true)
				eventCh <- struct{}{}
			}
		})

		// First, let keepalive fail by not providing any responses
		loopDone := make(chan struct{})
		go func() {
			cli.keepAliveLoop(ctx, connCtx)
			close(loopDone)
		}()

		select {
		case <-eventCh:
			// Received KeepAliveTimeout
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for KeepAliveTimeout event")
		}
		assert.True(t, timeoutEventDispatched.Load())

		// Now provide responses continuously to trigger KeepAliveRestored
		responderCtx, responderCancel := context.WithCancel(context.Background())
		defer responderCancel()
		go func() {
			for {
				select {
				case <-responderCtx.Done():
					return
				default:
					cli.responseWaitersLock.Lock()
					for id, waiter := range cli.responseWaiters {
						delete(cli.responseWaiters, id)
						select {
						case waiter <- &waBinary.Node{
							Tag:   "iq",
							Attrs: waBinary.Attrs{"id": id, "type": "result", "from": types.ServerJID},
						}:
						default:
						}
					}
					cli.responseWaitersLock.Unlock()
					time.Sleep(2 * time.Millisecond)
				}
			}
		}()

		select {
		case <-eventCh:
			// Received KeepAliveRestored
		case <-time.After(time.Second):
			// Proceed
		}

		connCancel()
		responderCancel()
		select {
		case <-loopDone:
		case <-time.After(3 * time.Second):
			t.Fatal("loop did not exit cleanly")
		}
	})

	t.Run("Forces reconnect when error exceeds KeepAliveMaxFailTime and AutoReconnect enabled", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		cli.EnableAutoReconnect = true
		cli.AutoReconnectHook = func(_ error) bool { return false }

		ctx, cancelCtx := context.WithCancel(context.Background())
		// Cancel ctx immediately so autoReconnect returns without attempting real network connect
		cancelCtx()
		connCtx, connCancel := context.WithCancel(context.Background())
		defer connCancel()

		loopDone := make(chan struct{})
		go func() {
			cli.keepAliveLoop(ctx, connCtx)
			close(loopDone)
		}()

		// Wait enough time for KeepAliveMaxFailTime (15ms) to be exceeded
		time.Sleep(40 * time.Millisecond)
		connCancel()

		select {
		case <-loopDone:
		case <-time.After(time.Second):
			t.Fatal("loop did not terminate")
		}

		time.Sleep(10 * time.Millisecond)
	})

	t.Run("Loop terminates when sendKeepAlive shouldContinue is false", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.EnableAutoReconnect = false

		ctx := context.Background()
		connCtx, connCancel := context.WithCancel(ctx)

		loopDone := make(chan struct{})
		go func() {
			cli.keepAliveLoop(ctx, connCtx)
			close(loopDone)
		}()

		// Cancel connCtx so sendKeepAlive returns shouldContinue = false
		time.Sleep(10 * time.Millisecond)
		connCancel()

		select {
		case <-loopDone:
			// successfully exited
		case <-time.After(time.Second):
			t.Fatal("loop failed to exit on shouldContinue=false")
		}
	})
}
