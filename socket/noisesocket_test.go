// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/util/gcmutil"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestGenerateIV(t *testing.T) {
	iv := generateIV(0x01020304)
	assert.Len(t, iv, 12)
	assert.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, iv)

	zeroIV := generateIV(0)
	assert.Equal(t, make([]byte, 12), zeroIV)
}

func TestNoiseSocket(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i + 1)
		key2[i] = byte(i + 33)
	}

	aeadWrite, err := gcmutil.Prepare(key1)
	require.NoError(t, err)
	aeadRead, err := gcmutil.Prepare(key2)
	require.NoError(t, err)

	t.Run("Consume frames loop, cancellation, and disconnect handler", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		frameChan := make(chan []byte, 1)
		disconnectCalled := make(chan bool, 1)

		ns, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, data []byte) {
			frameChan <- data
		}, func(_ context.Context, _ *NoiseSocket, remote bool) {
			disconnectCalled <- remote
		})
		require.NoError(t, err)

		// Test fs.OnDisconnect proxy to disconnectHandler
		fs.OnDisconnect(context.Background(), true)
		select {
		case remote := <-disconnectCalled:
			assert.True(t, remote)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for disconnectHandler")
		}

		// Calling consumeFrames with nil ctx (fs.Context() is nil before connect) should return immediately
		ns.consumeFrames(fs.Context(), fs.Frames)

		ctx, cancel := context.WithCancel(context.Background())
		frames := make(chan []byte, 1)

		done := make(chan struct{})
		go func() {
			ns.consumeFrames(ctx, frames)
			close(done)
		}()

		// Send frame into frames channel
		plain := []byte("loop message")
		iv := generateIV(0)
		cipher := aeadRead.Seal(nil, iv, plain, nil)
		frames <- cipher

		select {
		case got := <-frameChan:
			assert.Equal(t, plain, got)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for frame")
		}

		// Cancel context to stop consumeFrames
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for consumeFrames to exit on ctx cancellation")
		}

		// Test stopConsumer exit path
		ns2, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, _ []byte) {}, func(_ context.Context, _ *NoiseSocket, _ bool) {})
		require.NoError(t, err)
		doneStop := make(chan struct{})
		go func() {
			ns2.consumeFrames(context.Background(), frames)
			close(doneStop)
		}()
		close(ns2.stopConsumer)
		select {
		case <-doneStop:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for consumeFrames to exit on stopConsumer")
		}
	})

	t.Run("Receive encrypted frame and decrypt success and failure", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		frameChan := make(chan []byte, 1)

		ns, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, data []byte) {
			frameChan <- data
		}, func(_ context.Context, _ *NoiseSocket, _ bool) {})
		require.NoError(t, err)
		defer ns.Stop(false, false)

		ctx := context.Background()
		plain := []byte("secret payload")
		iv := generateIV(0)
		cipher := aeadRead.Seal(nil, iv, plain, nil)

		// Valid encrypted frame
		ns.receiveEncryptedFrame(ctx, cipher)
		select {
		case got := <-frameChan:
			assert.Equal(t, plain, got)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for decrypted frame")
		}

		// Invalid ciphertext (decrypt error)
		badCipher := []byte("invalid-encrypted-payload")
		ns.receiveEncryptedFrame(ctx, badCipher)
		// Should not call onFrame
		assert.Empty(t, frameChan)
	})

	t.Run("SendFrame with cancelled context returns ctx.Err", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		ns, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, _ []byte) {}, func(_ context.Context, _ *NoiseSocket, _ bool) {})
		require.NoError(t, err)
		defer ns.Stop(false, false)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = ns.SendFrame(ctx, []byte("test"))
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("SendFrame when socket is closed returns error", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		ns, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, _ []byte) {}, func(_ context.Context, _ *NoiseSocket, _ bool) {})
		require.NoError(t, err)
		defer ns.Stop(false, false)

		err = ns.SendFrame(context.Background(), []byte("test"))
		assert.ErrorIs(t, err, ErrSocketClosed)
	})

	t.Run("IsConnected delegates to fs.IsConnected", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		ns, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, _ []byte) {}, func(_ context.Context, _ *NoiseSocket, _ bool) {})
		require.NoError(t, err)
		defer ns.Stop(false, false)

		assert.False(t, ns.IsConnected())
	})

	t.Run("Stop with allowOnDisconnect and disconnect options", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		disconnectedCalled := false
		onDisconnect := func(_ context.Context, _ *NoiseSocket, _ bool) {
			disconnectedCalled = true
		}

		ns, err := newNoiseSocket(fs, aeadWrite, aeadRead, func(_ context.Context, _ []byte) {}, onDisconnect)
		require.NoError(t, err)

		// First Stop with allowOnDisconnect=false
		ns.Stop(true, false)
		assert.Nil(t, fs.OnDisconnect)

		// Second Stop is idempotent
		ns.Stop(true, true)
		assert.False(t, disconnectedCalled)
	})
}
