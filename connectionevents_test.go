// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
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

func TestConnectionEvents_HandleStreamError(t *testing.T) {
	ctx := context.Background()

	setupClient := func() (*Client, *mockCustomContainer) {
		mockContainer := &mockCustomContainer{}
		dev := &store.Device{
			ID:        &types.JID{User: "123456", Server: types.DefaultUserServer},
			Container: mockContainer,
		}
		cli := NewClient(dev, waLog.Noop)
		cli.isLoggedIn.Store(true)
		return cli, mockContainer
	}

	t.Run("Code 515 with login autoreconnect disabled dispatches ManualLoginReconnect", func(t *testing.T) {
		cli, _ := setupClient()
		cli.DisableLoginAutoReconnect = true

		var manualEvt *events.ManualLoginReconnect
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.ManualLoginReconnect); ok {
				manualEvt = e
			}
		})

		node := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "515"},
		}
		cli.handleStreamError(ctx, node)

		assert.False(t, cli.IsLoggedIn())
		require.NotNil(t, manualEvt)
	})

	t.Run("Code 515 with login autoreconnect enabled triggers reconnect", func(t *testing.T) {
		cli, _ := setupClient()
		cli.DisableLoginAutoReconnect = false
		// Set store deleted so connect fails immediately without panic
		cli.Store.Deleted = true

		node := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "515"},
		}
		cli.handleStreamError(ctx, node)
		assert.False(t, cli.IsLoggedIn())
	})

	t.Run("Code 401 device_removed deletes store and dispatches LoggedOut", func(t *testing.T) {
		cli, mockContainer := setupClient()

		loggedOutChan := make(chan *events.LoggedOut, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.LoggedOut); ok {
				loggedOutChan <- e
			}
		})

		node := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "401"},
			Content: []waBinary.Node{{
				Tag:   "conflict",
				Attrs: waBinary.Attrs{"type": "device_removed"},
			}},
		}
		cli.handleStreamError(ctx, node)

		assert.True(t, cli.isExpectedDisconnect())
		assert.Equal(t, 1, mockContainer.deleteCalls)

		var loggedOutEvt *events.LoggedOut
		select {
		case loggedOutEvt = <-loggedOutChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, loggedOutEvt)
		assert.False(t, loggedOutEvt.OnConnect)
		assert.Equal(t, events.ConnectFailureLoggedOut, loggedOutEvt.Reason)
	})

	t.Run("Conflict replaced dispatches StreamReplaced", func(t *testing.T) {
		cli, _ := setupClient()

		replacedChan := make(chan *events.StreamReplaced, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.StreamReplaced); ok {
				replacedChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "stream:error",
			Content: []waBinary.Node{{
				Tag:   "conflict",
				Attrs: waBinary.Attrs{"type": "replaced"},
			}},
		}
		cli.handleStreamError(ctx, node)

		assert.True(t, cli.isExpectedDisconnect())
		var replacedEvt *events.StreamReplaced
		select {
		case replacedEvt = <-replacedChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, replacedEvt)
	})

	t.Run("Code 503 logs warning", func(t *testing.T) {
		cli, _ := setupClient()
		node := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "503"},
		}
		cli.handleStreamError(ctx, node)
		assert.False(t, cli.IsLoggedIn())
	})

	t.Run("CAT invalid or expired refreshes CAT", func(t *testing.T) {
		cli, _ := setupClient()
		refreshed := false
		cli.RefreshCAT = func(c context.Context) error {
			refreshed = true
			return nil
		}

		node := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": events.ConnectFailureCATInvalid.NumberString()},
		}
		cli.handleStreamError(ctx, node)
		assert.True(t, refreshed)

		// With refresh error
		catErrChan := make(chan *events.CATRefreshError, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.CATRefreshError); ok {
				catErrChan <- e
			}
		})
		refreshErr := errors.New("cat rotation failed")
		cli.RefreshCAT = func(c context.Context) error {
			return refreshErr
		}
		nodeExpired := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": events.ConnectFailureCATExpired.NumberString()},
		}
		cli.handleStreamError(ctx, nodeExpired)
		assert.True(t, cli.isExpectedDisconnect())
		var catErrEvt *events.CATRefreshError
		select {
		case catErrEvt = <-catErrChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, catErrEvt)
		assert.ErrorIs(t, catErrEvt.Error, refreshErr)
	})

	t.Run("Unknown stream error dispatches StreamError event", func(t *testing.T) {
		cli, _ := setupClient()
		streamErrChan := make(chan *events.StreamError, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.StreamError); ok {
				streamErrChan <- e
			}
		})

		node := &waBinary.Node{
			Tag:   "stream:error",
			Attrs: waBinary.Attrs{"code": "999"},
		}
		cli.handleStreamError(ctx, node)
		var streamErrEvt *events.StreamError
		select {
		case streamErrEvt = <-streamErrChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, streamErrEvt)
		assert.Equal(t, "999", streamErrEvt.Code)
		assert.Equal(t, node, streamErrEvt.Raw)
	})
}

func TestConnectionEvents_HandleIB(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	qrChan := make(chan *events.QRScannedWithoutMultidevice, 1)
	previewChan := make(chan *events.OfflineSyncPreview, 1)
	completedChan := make(chan *events.OfflineSyncCompleted, 1)

	cli.AddEventHandler(func(evt any) {
		switch e := evt.(type) {
		case *events.QRScannedWithoutMultidevice:
			qrChan <- e
		case *events.OfflineSyncPreview:
			previewChan <- e
		case *events.OfflineSyncCompleted:
			completedChan <- e
		}
	})

	node := &waBinary.Node{
		Tag: "ib",
		Content: []waBinary.Node{
			{Tag: "downgrade_webclient"},
			{
				Tag: "offline_preview",
				Attrs: waBinary.Attrs{
					"count":        "100",
					"appdata":      "10",
					"message":      "50",
					"notification": "20",
					"receipt":      "20",
				},
			},
			{
				Tag: "offline",
				Attrs: waBinary.Attrs{
					"count": "42",
				},
			},
			{Tag: "dirty"}, // ignored branch
			{Tag: "unknown"},
		},
	}

	cli.handleIB(ctx, node)

	var qrScannedEvt *events.QRScannedWithoutMultidevice
	var previewEvt *events.OfflineSyncPreview
	var completedEvt *events.OfflineSyncCompleted

	select {
	case qrScannedEvt = <-qrChan:
	case <-time.After(200 * time.Millisecond):
	}
	select {
	case previewEvt = <-previewChan:
	case <-time.After(200 * time.Millisecond):
	}
	select {
	case completedEvt = <-completedChan:
	case <-time.After(200 * time.Millisecond):
	}

	require.NotNil(t, qrScannedEvt)
	require.NotNil(t, previewEvt)
	assert.Equal(t, 100, previewEvt.Total)
	assert.Equal(t, 10, previewEvt.AppDataChanges)
	assert.Equal(t, 50, previewEvt.Messages)
	assert.Equal(t, 20, previewEvt.Notifications)
	assert.Equal(t, 20, previewEvt.Receipts)

	require.NotNil(t, completedEvt)
	assert.Equal(t, 42, completedEvt.Count)
}

func TestConnectionEvents_HandleConnectFailure(t *testing.T) {
	ctx := context.Background()

	setupClient := func() (*Client, *mockCustomContainer) {
		mockContainer := &mockCustomContainer{}
		dev := &store.Device{
			ID:        &types.JID{User: "123456", Server: types.DefaultUserServer},
			Container: mockContainer,
		}
		cli := NewClient(dev, waLog.Noop)
		return cli, mockContainer
	}

	t.Run("LoggedOut failure deletes store and dispatches LoggedOut", func(t *testing.T) {
		cli, mockContainer := setupClient()

		loggedOutChan := make(chan *events.LoggedOut, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.LoggedOut); ok {
				loggedOutChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason": events.ConnectFailureLoggedOut.NumberString(),
			},
		}
		cli.handleConnectFailure(ctx, node)

		assert.Equal(t, 1, mockContainer.deleteCalls)
		var loggedOutEvt *events.LoggedOut
		select {
		case loggedOutEvt = <-loggedOutChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, loggedOutEvt)
		assert.True(t, loggedOutEvt.OnConnect)
		assert.Equal(t, events.ConnectFailureLoggedOut, loggedOutEvt.Reason)
	})

	t.Run("403 failure with logout message headers", func(t *testing.T) {
		cli, mockContainer := setupClient()
		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason":                 "403",
				"logout_message_header":  "Header",
				"logout_message_subtext": "Subtext",
			},
		}
		cli.handleConnectFailure(ctx, node)
		assert.Equal(t, 1, mockContainer.deleteCalls)
	})

	t.Run("Temporary ban connect failure", func(t *testing.T) {
		cli, _ := setupClient()
		banChan := make(chan *events.TemporaryBan, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.TemporaryBan); ok {
				banChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason": events.ConnectFailureTempBanned.NumberString(),
				"code":   "1",
				"expire": "3600",
			},
		}
		cli.handleConnectFailure(ctx, node)

		var banEvt *events.TemporaryBan
		select {
		case banEvt = <-banChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, banEvt)
		assert.Equal(t, events.TempBanReason(1), banEvt.Code)
		assert.Equal(t, 3600*time.Second, banEvt.Expire)
	})

	t.Run("Client outdated 405 failure", func(t *testing.T) {
		cli, _ := setupClient()
		outdatedChan := make(chan *events.ClientOutdated, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.ClientOutdated); ok {
				outdatedChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason": events.ConnectFailureClientOutdated.NumberString(),
			},
		}
		cli.handleConnectFailure(ctx, node)

		var outdatedEvt *events.ClientOutdated
		select {
		case outdatedEvt = <-outdatedChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, outdatedEvt)
	})

	t.Run("CAT invalid or expired failure refreshes CAT", func(t *testing.T) {
		cli, _ := setupClient()
		refreshed := false
		cli.RefreshCAT = func(c context.Context) error {
			refreshed = true
			return nil
		}

		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason":  events.ConnectFailureCATInvalid.NumberString(),
				"message": "cat invalid",
			},
		}
		cli.handleConnectFailure(ctx, node)
		assert.True(t, refreshed)

		// With refresh error
		catErrChan := make(chan *events.CATRefreshError, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.CATRefreshError); ok {
				catErrChan <- e
			}
		})
		refreshErr := errors.New("cat fail")
		cli.RefreshCAT = func(c context.Context) error {
			return refreshErr
		}
		nodeExpired := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason":  events.ConnectFailureCATExpired.NumberString(),
				"message": "cat expired",
			},
		}
		cli.handleConnectFailure(ctx, nodeExpired)
		assert.True(t, cli.isExpectedDisconnect())
		var catErrEvt *events.CATRefreshError
		select {
		case catErrEvt = <-catErrChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, catErrEvt)
	})

	t.Run("503 Service unavailable auto-reconnect branch", func(t *testing.T) {
		cli, _ := setupClient()
		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason": events.ConnectFailureServiceUnavailable.NumberString(),
			},
		}
		cli.handleConnectFailure(ctx, node)
		assert.False(t, cli.isExpectedDisconnect())
	})

	t.Run("Unknown failure dispatches ConnectFailure", func(t *testing.T) {
		cli, _ := setupClient()
		connFailChan := make(chan *events.ConnectFailure, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.ConnectFailure); ok {
				connFailChan <- e
			}
		})

		node := &waBinary.Node{
			Tag: "failure",
			Attrs: waBinary.Attrs{
				"reason":  "9999",
				"message": "custom-failure",
			},
		}
		cli.handleConnectFailure(ctx, node)

		assert.True(t, cli.isExpectedDisconnect())
		var connFailEvt *events.ConnectFailure
		select {
		case connFailEvt = <-connFailChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, connFailEvt)
		assert.Equal(t, events.ConnectFailureReason(9999), connFailEvt.Reason)
		assert.Equal(t, "custom-failure", connFailEvt.Message)
	})
}

func TestConnectionEvents_HandleConnectSuccess(t *testing.T) {
	ctx := context.Background()

	t.Run("Ignores when not paired", func(t *testing.T) {
		cli := NewClient(&store.Device{}, waLog.Noop)
		cli.paired.Store(false)
		called := false
		cli.AddEventHandler(func(_ any) { called = true })

		node := &waBinary.Node{Tag: "success"}
		cli.handleConnectSuccess(ctx, node)
		assert.False(t, cli.IsLoggedIn())
		assert.False(t, called)
	})

	t.Run("Success updates LID and PN mapping if empty and dispatches Connected", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		cli.paired.Store(true)
		cli.Store.LID = types.EmptyJID
		cli.Store.ID = &types.JID{User: "628123456", Server: types.DefaultUserServer}
		mockContainer := &mockCustomContainer{}
		cli.Store.Container = mockContainer

		mockPK := newMockCustomPreKeyStore()
		cli.Store.PreKeys = mockPK
		cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{
				Tag: "iq",
				Content: []waBinary.Node{{
					Tag:   "count",
					Attrs: waBinary.Attrs{"value": "100"},
				}},
			}, nil
		}

		connectedChan := make(chan *events.Connected, 1)
		cli.AddEventHandler(func(evt any) {
			if e, ok := evt.(*events.Connected); ok {
				connectedChan <- e
			}
		})

		newLID := types.NewJID("10099", types.HiddenUserServer)
		node := &waBinary.Node{
			Tag: "success",
			Attrs: waBinary.Attrs{
				"lid": newLID,
				"t":   time.Now().Unix(),
			},
		}

		cli.handleConnectSuccess(ctx, node)

		assert.True(t, cli.IsLoggedIn())
		assert.Equal(t, newLID, cli.Store.LID)
		assert.Equal(t, 1, mockContainer.putCalls)

		var connectedEvt *events.Connected
		select {
		case connectedEvt = <-connectedChan:
		case <-time.After(200 * time.Millisecond):
		}
		require.NotNil(t, connectedEvt)
	})

	t.Run("Mismatch in existing LID resets store LID", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()

		cli.paired.Store(true)
		cli.Store.LID = types.NewJID("10001", types.HiddenUserServer)
		cli.Store.ID = &types.JID{User: "628123456", Server: types.DefaultUserServer}
		mockContainer := &mockCustomContainer{}
		cli.Store.Container = mockContainer

		mockPK := newMockCustomPreKeyStore()
		cli.Store.PreKeys = mockPK
		cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
			return &waBinary.Node{Tag: "iq"}, nil
		}

		diffLID := types.NewJID("99999", types.HiddenUserServer)
		node := &waBinary.Node{
			Tag: "success",
			Attrs: waBinary.Attrs{
				"lid": diffLID,
				"t":   time.Now().Unix(),
			},
		}

		cli.handleConnectSuccess(ctx, node)
		assert.True(t, cli.IsLoggedIn())
		assert.Equal(t, diffLID, cli.Store.LID)
	})
}

func TestConnectionEvents_SetPassive(t *testing.T) {
	ctx := context.Background()
	cli := NewClient(&store.Device{}, waLog.Noop)

	var capturedQuery infoQuery
	cli.sendIQHook = func(_ context.Context, query infoQuery) (*waBinary.Node, error) {
		capturedQuery = query
		return &waBinary.Node{Tag: "iq"}, nil
	}

	// Set passive = false -> tag 'active'
	err := cli.SetPassive(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, "passive", capturedQuery.Namespace)
	assert.Equal(t, iqSet, capturedQuery.Type)
	contentNodes := capturedQuery.Content.([]waBinary.Node)
	assert.Equal(t, "active", contentNodes[0].Tag)

	// Set passive = true -> tag 'passive'
	err = cli.SetPassive(ctx, true)
	require.NoError(t, err)
	contentNodes = capturedQuery.Content.([]waBinary.Node)
	assert.Equal(t, "passive", contentNodes[0].Tag)

	// Error path
	cli.sendIQHook = func(_ context.Context, _ infoQuery) (*waBinary.Node, error) {
		return nil, errors.New("passive iq failed")
	}
	err = cli.SetPassive(ctx, true)
	assert.ErrorContains(t, err, "passive iq failed")
}
