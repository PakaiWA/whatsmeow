// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestFrameSocket_NewAndIsConnected(t *testing.T) {
	fs := NewFrameSocket(waLog.Noop, nil)
	require.NotNil(t, fs)
	assert.Equal(t, URL, fs.URL)
	assert.Equal(t, Origin, fs.HTTPHeaders.Get("Origin"))
	assert.Equal(t, WAConnHeader, fs.Header)
	assert.False(t, fs.IsConnected())
	assert.NotNil(t, fs.Frames)
}

func TestFrameSocket_ProcessData(t *testing.T) {
	t.Run("Single complete frame", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		payload := []byte("hello")
		frame := []byte{0, 0, 5, 'h', 'e', 'l', 'l', 'o'}

		go fs.processData(frame)

		select {
		case got := <-fs.Frames:
			assert.Equal(t, payload, got)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for frame")
		}
	})

	t.Run("Multiple frames in a single message", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		frame1 := []byte{0, 0, 2, 'h', 'i'}
		frame2 := []byte{0, 0, 5, 'w', 'o', 'r', 'l', 'd'}
		combined := append(frame1, frame2...)

		go fs.processData(combined)

		select {
		case got1 := <-fs.Frames:
			assert.Equal(t, []byte("hi"), got1)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for frame 1")
		}

		select {
		case got2 := <-fs.Frames:
			assert.Equal(t, []byte("world"), got2)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for frame 2")
		}
	})

	t.Run("Partial header across chunks", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		chunk1 := []byte{0, 0}
		chunk2 := []byte{4, 't', 'e', 's', 't'}

		go func() {
			fs.processData(chunk1)
			fs.processData(chunk2)
		}()

		select {
		case got := <-fs.Frames:
			assert.Equal(t, []byte("test"), got)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for frame with partial header")
		}
	})

	t.Run("Partial payload across multiple chunks", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		chunk1 := []byte{0, 0, 6, 'a', 'b'}
		chunk2 := []byte{'c', 'd'}
		chunk3 := []byte{'e', 'f'}

		go func() {
			fs.processData(chunk1)
			fs.processData(chunk2)
			fs.processData(chunk3)
		}()

		select {
		case got := <-fs.Frames:
			assert.Equal(t, []byte("abcdef"), got)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for multi-chunk frame")
		}
	})
}

func TestFrameSocket_SendFrameErrors(t *testing.T) {
	fs := NewFrameSocket(waLog.Noop, nil)

	t.Run("SendFrame when disconnected returns ErrSocketClosed", func(t *testing.T) {
		err := fs.SendFrame([]byte("data"))
		assert.ErrorIs(t, err, ErrSocketClosed)
	})

	t.Run("SendFrame too large returns ErrFrameTooLarge", func(t *testing.T) {
		// Mock an open connection state
		dummyConn := &websocket.Conn{}
		fs.conn.Store(dummyConn)
		defer fs.conn.Store(nil)

		oversized := make([]byte, FrameMaxSize)
		err := fs.SendFrame(oversized)
		assert.ErrorIs(t, err, ErrFrameTooLarge)
	})
}

func TestFrameSocket_Close(t *testing.T) {
	fs := NewFrameSocket(waLog.Noop, nil)

	t.Run("Close when already closed is no-op", func(t *testing.T) {
		fs.Close(websocket.StatusNormalClosure)
		assert.False(t, fs.IsConnected())
	})
}

func TestFrameSocket_ConnectAndReadWritePump(t *testing.T) {
	serverReceived := make(chan []byte, 1)

	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.CloseNow()

		// 1. Read frame sent by client
		msgType, msg, err := conn.Read(r.Context())
		if err == nil && msgType == websocket.MessageBinary {
			serverReceived <- msg
		}

		// 2. Send an unexpected text message (tests warning in readPump)
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ignore me text"))

		// 3. Send binary frame to client: 3-byte length + payload
		frame := []byte{0, 0, 4, 'p', 'o', 'n', 'g'}
		_ = conn.Write(r.Context(), websocket.MessageBinary, frame)

		// Wait a little before exiting
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	fs := NewFrameSocket(waLog.Noop, nil)
	fs.URL = wsURL

	disconnectReceived := make(chan struct{}, 1)
	fs.OnDisconnect = func(_ context.Context, _ bool) {
		select {
		case disconnectReceived <- struct{}{}:
		default:
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Connect
	err := fs.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, fs.IsConnected())
	assert.NotNil(t, fs.Context())

	// 2. Calling Connect again returns ErrSocketAlreadyOpen
	errSecond := fs.Connect(ctx)
	assert.ErrorIs(t, errSecond, ErrSocketAlreadyOpen)

	// 3. SendFrame
	clientPayload := []byte("ping")
	err = fs.SendFrame(clientPayload)
	require.NoError(t, err)

	// Verify server received the frame with header and length prefix
	select {
	case received := <-serverReceived:
		assert.Contains(t, string(received), "ping")
		// Header was consumed and reset to nil
		assert.Nil(t, fs.Header)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server to receive frame")
	}

	// 4. Verify client received the response frame from server
	select {
	case frameData := <-fs.Frames:
		assert.Equal(t, []byte("pong"), frameData)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client to receive frame")
	}

	// 5. Close connection
	fs.Close(websocket.StatusNormalClosure)
	assert.False(t, fs.IsConnected())

	select {
	case <-disconnectReceived:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for OnDisconnect")
	}
}

func TestFrameSocket_ConnectErrors(t *testing.T) {
	t.Run("Dial failure invalid URL", func(t *testing.T) {
		fs := NewFrameSocket(waLog.Noop, nil)
		fs.URL = "ws://127.0.0.1:0/invalid"

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := fs.Connect(ctx)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrDialFailed)
	})

	t.Run("Dial failure with HTTP error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}))
		defer server.Close()

		fs := NewFrameSocket(waLog.Noop, nil)
		fs.URL = "ws" + strings.TrimPrefix(server.URL, "http")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := fs.Connect(ctx)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrDialFailed)

		var statusErr ErrWithStatusCode
		if errors.As(err, &statusErr) {
			assert.Equal(t, http.StatusForbidden, statusErr.StatusCode)
		}
	})
}
