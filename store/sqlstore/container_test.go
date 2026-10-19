// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/proto/waAdv"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func newTestContainer(t *testing.T) *Container {
	ctx := context.Background()
	// Use cache=shared in-memory SQLite so connections share state
	container, err := New(ctx, "sqlite3", "file:test_"+uuid.NewString()+"?mode=memory&cache=shared&_foreign_keys=on", waLog.Noop)
	require.NoError(t, err)
	require.NotNil(t, container)
	t.Cleanup(func() {
		_ = container.Close()
	})
	return container
}

func TestContainer_New_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid address/driver", func(t *testing.T) {
		_, err := New(ctx, "invalid_dialect", "whatever", waLog.Noop)
		assert.Error(t, err)
	})

	t.Run("Foreign keys disabled in SQLite", func(t *testing.T) {
		_, err := New(ctx, "sqlite3", "file:fk_test_"+uuid.NewString()+"?mode=memory&cache=shared&_foreign_keys=off", waLog.Noop)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "foreign keys are not enabled")
	})

	t.Run("NewWithDB invalid dialect panics", func(t *testing.T) {
		db, err := sql.Open("sqlite3", "file::memory:")
		require.NoError(t, err)
		defer db.Close()

		assert.Panics(t, func() {
			_ = NewWithDB(db, "unknown_dialect", nil)
		})
	})

	t.Run("Close nil container or nil db is no-op", func(t *testing.T) {
		var nilContainer *Container
		assert.NoError(t, nilContainer.Close())

		c := &Container{}
		assert.NoError(t, c.Close())
	})
}

func TestContainer_DeviceLifecycle(t *testing.T) {
	container := newTestContainer(t)
	ctx := context.Background()

	t.Run("GetFirstDevice when empty creates new unsaved device", func(t *testing.T) {
		dev, err := container.GetFirstDevice(ctx)
		require.NoError(t, err)
		require.NotNil(t, dev)
		assert.Nil(t, dev.ID)
		assert.NotNil(t, dev.SignedPreKey)
		assert.NotNil(t, dev.NoiseKey)
		assert.NotNil(t, dev.IdentityKey)
	})

	t.Run("PutDevice requires non-nil ID", func(t *testing.T) {
		dev := container.NewDevice()
		err := container.PutDevice(ctx, dev)
		assert.ErrorIs(t, err, ErrDeviceIDMustBeSet)

		err = container.DeleteDevice(ctx, dev)
		assert.ErrorIs(t, err, ErrDeviceIDMustBeSet)
	})

	t.Run("PutDevice, GetDevice, GetAllDevices, and GetFirstDevice roundtrip", func(t *testing.T) {
		dev := container.NewDevice()
		jid := types.NewJID("628111111111", types.DefaultUserServer)
		lid := types.NewJID("10001", types.HiddenUserServer)
		dev.ID = &jid
		dev.LID = lid
		dev.Platform = "Chrome"
		dev.BusinessName = "PakaiWA Business"
		dev.PushName = "Kelvin"
		dev.Account = &waAdv.ADVSignedDeviceIdentity{
			Details:             []byte("details"),
			AccountSignature:    make([]byte, 64),
			DeviceSignature:     make([]byte, 64),
			AccountSignatureKey: make([]byte, 32),
		}
		dev.FacebookUUID = uuid.New()
		dev.LIDMigrationTimestamp = 1700000000
		dev.CompanionMetaNonce = "nonce123"

		// 1. PutDevice
		err := container.PutDevice(ctx, dev)
		require.NoError(t, err)
		assert.True(t, dev.Initialized)

		// 2. GetDevice found
		loaded, err := container.GetDevice(ctx, jid)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, jid, *loaded.ID)
		assert.Equal(t, lid, loaded.LID)
		assert.Equal(t, "Chrome", loaded.Platform)
		assert.Equal(t, "PakaiWA Business", loaded.BusinessName)
		assert.Equal(t, "Kelvin", loaded.PushName)
		assert.Equal(t, dev.FacebookUUID, loaded.FacebookUUID)
		assert.Equal(t, int64(1700000000), loaded.LIDMigrationTimestamp)
		assert.Equal(t, "nonce123", loaded.CompanionMetaNonce)
		assert.True(t, loaded.Initialized)

		// 3. Update existing device via PutDevice ON CONFLICT
		dev.PushName = "Kelvin Anggara"
		dev.Platform = "Firefox"
		err = container.PutDevice(ctx, dev)
		require.NoError(t, err)

		updated, err := container.GetDevice(ctx, jid)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, "Kelvin Anggara", updated.PushName)
		assert.Equal(t, "Firefox", updated.Platform)

		// 4. GetAllDevices
		devices, err := container.GetAllDevices(ctx)
		require.NoError(t, err)
		assert.Len(t, devices, 1)

		// 5. GetFirstDevice when populated
		first, err := container.GetFirstDevice(ctx)
		require.NoError(t, err)
		require.NotNil(t, first)
		assert.Equal(t, jid, *first.ID)

		// 6. GetDevice not found
		unknownJID := types.NewJID("628999999999", types.DefaultUserServer)
		missing, err := container.GetDevice(ctx, unknownJID)
		require.NoError(t, err)
		assert.Nil(t, missing)

		// 7. DeleteDevice
		err = container.DeleteDevice(ctx, dev)
		require.NoError(t, err)

		missing, err = container.GetDevice(ctx, jid)
		require.NoError(t, err)
		assert.Nil(t, missing)
	})

	t.Run("scanDevice error handling", func(t *testing.T) {
		// 1. Scan returns error
		scanErr := errors.New("scan failure")
		mockErrRow := &mockScannable{
			scanFunc: func(dest ...any) error {
				return scanErr
			},
		}
		_, err := container.scanDevice(mockErrRow)
		assert.ErrorIs(t, err, scanErr)

		// 2. Scan returns invalid key length
		mockLenRow := &mockScannable{
			scanFunc: func(dest ...any) error {
				jid := types.NewJID("628000000000", types.DefaultUserServer)
				*dest[0].(**types.JID) = &jid
				*dest[3].(*[]byte) = []byte("short") // noisePriv length != 32
				*dest[4].(*[]byte) = make([]byte, 32)
				*dest[5].(*[]byte) = make([]byte, 32)
				*dest[7].(*[]byte) = make([]byte, 64)
				return nil
			},
		}
		_, err = container.scanDevice(mockLenRow)
		assert.ErrorIs(t, err, ErrInvalidLength)
	})
}

type mockScannable struct {
	scanFunc func(dest ...any) error
}

func (m *mockScannable) Scan(dest ...any) error {
	return m.scanFunc(dest...)
}
