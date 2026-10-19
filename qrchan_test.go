// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestQRChan_GetQRChannel_Validations(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil client", func(t *testing.T) {
		var nilCli *Client
		_, err := nilCli.GetQRChannel(ctx)
		require.ErrorIs(t, err, ErrClientIsNil)
	})

	t.Run("Already connected", func(t *testing.T) {
		cli, cleanup := setupMockSocketClient(t)
		defer cleanup()
		cli.Store.ID = nil
		_, err := cli.GetQRChannel(ctx)
		require.ErrorIs(t, err, ErrQRAlreadyConnected)
	})

	t.Run("Store already contains ID", func(t *testing.T) {
		jid := types.NewJID("12345", types.DefaultUserServer)
		device := &store.Device{ID: &jid}
		cli := NewClient(device, waLog.Noop)
		_, err := cli.GetQRChannel(ctx)
		require.ErrorIs(t, err, ErrQRStoreContainsID)
	})

	t.Run("Success", func(t *testing.T) {
		device := &store.Device{ID: nil}
		cli := NewClient(device, waLog.Noop)
		ch, err := cli.GetQRChannel(ctx)
		require.NoError(t, err)
		require.NotNil(t, ch)
	})
}

func TestQRChan_HandleEvent(t *testing.T) {
	ctx := context.Background()

	setupQRC := func(t *testing.T) (*qrChannel, <-chan QRChannelItem) {
		device := &store.Device{ID: nil}
		cli := NewClient(device, waLog.Noop)
		ch := make(chan QRChannelItem, 16)
		qrc := &qrChannel{
			output:    ch,
			stopQRs:   make(chan struct{}),
			rotateAdv: make(chan *events.RotateADVSecret, 4),
			cli:       cli,
			log:       cli.Log.Sub("QRChannel"),
			ctx:       ctx,
		}
		qrc.handlerID = cli.AddEventHandler(qrc.handleEvent)
		return qrc, ch
	}

	t.Run("QR event starts emitting", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		codes := []string{"code1", "code2"}
		qrc.handleEvent(&events.QR{Codes: codes})

		item := <-ch
		assert.Equal(t, QRChannelEventCode, item.Event)
		assert.Equal(t, "code1", item.Code)
		assert.Equal(t, 20*time.Second, item.Timeout)

		// Close so emitter exits
		qrc.close()
		close(qrc.stopQRs)
	})

	t.Run("RotateADVSecret event", func(t *testing.T) {
		qrc, _ := setupQRC(t)
		rot := &events.RotateADVSecret{OldSecret: "old", NewSecret: "new"}
		qrc.handleEvent(rot)
		select {
		case r := <-qrc.rotateAdv:
			assert.Equal(t, rot, r)
		default:
			t.Fatal("rotateAdv channel did not receive event")
		}

		// Fill channel to trigger default branch (drop)
		for i := 0; i < cap(qrc.rotateAdv); i++ {
			qrc.rotateAdv <- rot
		}
		qrc.handleEvent(rot)
	})

	t.Run("QRScannedWithoutMultidevice", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		qrc.handleEvent(&events.QRScannedWithoutMultidevice{})
		item := <-ch
		assert.Equal(t, QRChannelScannedWithoutMultidevice, item)
	})

	t.Run("PairPasskeyRequest", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		req := &events.PairPasskeyRequest{}
		qrc.handleEvent(req)
		item := <-ch
		assert.Equal(t, QRChannelEventPasskeyRequest, item.Event)
		assert.Equal(t, req, item.PasskeyRequest)
	})

	t.Run("PairPasskeyConfirmation without SkipHandoffUX", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		conf := &events.PairPasskeyConfirmation{SkipHandoffUX: false}
		qrc.handleEvent(conf)
		item := <-ch
		assert.Equal(t, QRChannelEventPasskeyResponse, item.Event)
		assert.Equal(t, conf, item.PasskeyConfirmation)
	})

	t.Run("PairPasskeyConfirmation with SkipHandoffUX error", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		conf := &events.PairPasskeyConfirmation{SkipHandoffUX: true}
		// cli is not connected so SendPasskeyConfirmation will error
		qrc.handleEvent(conf)
		item := <-ch
		assert.Equal(t, QRChannelEventError, item.Event)
		assert.Error(t, item.Error)
	})

	t.Run("PairPasskeyError", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		testErr := errors.New("passkey error")
		qrc.handleEvent(&events.PairPasskeyError{Error: testErr})
		item := <-ch
		assert.Equal(t, QRChannelEventError, item.Event)
		assert.Equal(t, testErr, item.Error)
	})

	t.Run("Terminal events", func(t *testing.T) {
		testCases := []struct {
			name     string
			event    any
			expected QRChannelItem
		}{
			{"ClientOutdated", &events.ClientOutdated{}, QRChannelClientOutdated},
			{"PairSuccess", &events.PairSuccess{}, QRChannelSuccess},
			{"PairError", &events.PairError{Error: errors.New("pair err")}, QRChannelItem{Event: QRChannelEventError, Error: errors.New("pair err")}},
			{"Disconnected", &events.Disconnected{}, QRChannelTimeout},
			{"Connected", &events.Connected{}, QRChannelErrUnexpectedEvent},
			{"ConnectFailure", &events.ConnectFailure{}, QRChannelErrUnexpectedEvent},
			{"LoggedOut", &events.LoggedOut{}, QRChannelErrUnexpectedEvent},
			{"TemporaryBan", &events.TemporaryBan{}, QRChannelErrUnexpectedEvent},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				qrc, ch := setupQRC(t)
				qrc.handleEvent(tc.event)
				item, ok := <-ch
				assert.True(t, ok)
				assert.Equal(t, tc.expected.Event, item.Event)
				if tc.expected.Error != nil {
					assert.Equal(t, tc.expected.Error.Error(), item.Error.Error())
				}
				// Verify channel was closed
				_, open := <-ch
				assert.False(t, open)

				// Event after close is dropped
				qrc.handleEvent(&events.PairSuccess{})
			})
		}
	})

	t.Run("Unknown event is ignored", func(t *testing.T) {
		qrc, ch := setupQRC(t)
		qrc.handleEvent(&events.Receipt{})
		select {
		case item := <-ch:
			t.Fatalf("unexpected item: %+v", item)
		default:
			// OK
		}
	})
}

func TestQRChan_EmitQRs_Branches(t *testing.T) {
	ctx := context.Background()

	newQRC := func(ch chan QRChannelItem, stopCh chan struct{}, rotCh chan *events.RotateADVSecret, customCtx context.Context) (*qrChannel, *Client) {
		device := &store.Device{ID: nil}
		cli := NewClient(device, waLog.Noop)
		c := ctx
		if customCtx != nil {
			c = customCtx
		}
		if stopCh == nil {
			stopCh = make(chan struct{})
		}
		if rotCh == nil {
			rotCh = make(chan *events.RotateADVSecret, 4)
		}
		return &qrChannel{
			output:    ch,
			stopQRs:   stopCh,
			rotateAdv: rotCh,
			cli:       cli,
			log:       cli.Log.Sub("QRChannel"),
			ctx:       c,
		}, cli
	}

	t.Run("Ran out of QR codes", func(t *testing.T) {
		ch := make(chan QRChannelItem, 4)
		qrc, _ := newQRC(ch, nil, nil, nil)
		qrc.emitQRs([]string{})
		item := <-ch
		assert.Equal(t, QRChannelTimeout, item)
		_, open := <-ch
		assert.False(t, open)

		// Ran out of QR codes when already closed
		ch2 := make(chan QRChannelItem, 4)
		qrc2, _ := newQRC(ch2, nil, nil, nil)
		qrc2.close()
		qrc2.emitQRs([]string{})
	})

	t.Run("Closed flag loaded at start of loop", func(t *testing.T) {
		ch := make(chan QRChannelItem, 4)
		qrc, _ := newQRC(ch, nil, nil, nil)
		qrc.closed.Store(true)
		qrc.emitQRs([]string{"code1"})
		select {
		case item := <-ch:
			t.Fatalf("unexpected item emitted: %+v", item)
		default:
			// OK
		}
	})

	t.Run("Output channel full triggers default branch", func(t *testing.T) {
		unbufferedCh := make(chan QRChannelItem) // won't accept without receiver
		qrc, _ := newQRC(unbufferedCh, nil, nil, nil)
		qrc.emitQRs([]string{"code1"})
		assert.True(t, qrc.closed.Load())
	})

	t.Run("StopQRs signal received", func(t *testing.T) {
		ch := make(chan QRChannelItem, 4)
		stopCh := make(chan struct{})
		qrc, _ := newQRC(ch, stopCh, nil, nil)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			qrc.emitQRs([]string{"c1", "c2"})
		}()
		item := <-ch
		assert.Equal(t, "c1", item.Code)
		close(stopCh)
		wg.Wait()
	})

	t.Run("Expected disconnect received", func(t *testing.T) {
		ch := make(chan QRChannelItem, 4)
		stopCh := make(chan struct{})
		qrc, cli := newQRC(ch, stopCh, nil, nil)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			qrc.emitQRs([]string{"c1", "c2"})
		}()
		item := <-ch
		assert.Equal(t, "c1", item.Code)
		cli.expectedDisconnect.Set()
		wg.Wait()
		cli.expectedDisconnect.Clear()
	})

	t.Run("Context cancellation stops emitter", func(t *testing.T) {
		cCtx, cancelC := context.WithCancel(context.Background())
		ch := make(chan QRChannelItem, 4)
		qrc, _ := newQRC(ch, nil, nil, cCtx)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			qrc.emitQRs([]string{"c1", "c2"})
		}()
		item := <-ch
		assert.Equal(t, "c1", item.Code)
		cancelC()
		wg.Wait()
		assert.True(t, qrc.closed.Load())
	})

	t.Run("Rotate ADV secret in remaining codes", func(t *testing.T) {
		ch := make(chan QRChannelItem, 4)
		stopCh := make(chan struct{})
		rotCh := make(chan *events.RotateADVSecret, 4)
		qrc, _ := newQRC(ch, stopCh, rotCh, nil)
		codes := []string{"old_1", "old_2", "old_3", "old_4", "old_5", "old_6"}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			qrc.emitQRs(slices.Clone(codes))
		}()
		// First item emitted has 6 codes -> 60s timeout
		item1 := <-ch
		assert.Equal(t, "old_1", item1.Code)
		assert.Equal(t, 60*time.Second, item1.Timeout)

		// Send rotate ADV secret
		rotCh <- &events.RotateADVSecret{OldSecret: "old", NewSecret: "new"}
		// The emitter should rotate remaining codes and nextCode, and re-emit "new_1"
		item2 := <-ch
		assert.Equal(t, "new_1", item2.Code)
		close(stopCh)
		wg.Wait()
	})
}
