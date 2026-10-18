// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/proto/waServerSync"
	"github.com/PakaiWA/whatsmeow/store"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestExpandAppStateKeys(t *testing.T) {
	keyData := []byte("01234567890123456789012345678901") // 32 bytes
	keys := expandAppStateKeys(keyData)

	assert.Len(t, keys.Index, 32)
	assert.Len(t, keys.ValueEncryption, 32)
	assert.Len(t, keys.ValueMAC, 32)
	assert.Len(t, keys.SnapshotMAC, 32)
	assert.Len(t, keys.PatchMAC, 32)

	// Deterministic
	keys2 := expandAppStateKeys(keyData)
	assert.Equal(t, keys, keys2)

	// Distinct subkeys
	assert.NotEqual(t, keys.Index, keys.ValueEncryption)
	assert.NotEqual(t, keys.ValueEncryption, keys.ValueMAC)
	assert.NotEqual(t, keys.ValueMAC, keys.SnapshotMAC)
	assert.NotEqual(t, keys.SnapshotMAC, keys.PatchMAC)
}

func TestNewProcessor(t *testing.T) {
	dev := &store.Device{
		AppStateKeys: newMockAppStateSyncKeyStore(),
		AppState:     newMockAppStateStore(),
		Log:          waLog.Noop,
	}
	proc := NewProcessor(dev, waLog.Noop)
	require.NotNil(t, proc)
	assert.NotNil(t, proc.keyCache)
	assert.Equal(t, dev, proc.Store)
	assert.Equal(t, waLog.Noop, proc.Log)
}

func TestProcessor_GetAppStateKey(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, _ := newTestProcessor()

	keyID := []byte("test-key-id-1")
	keyData := []byte("secret-sync-key-data-32-bytes___")

	t.Run("Key not found in store", func(t *testing.T) {
		keys, err := proc.getAppStateKey(ctx, keyID)
		assert.ErrorIs(t, err, ErrKeyNotFound)
		assert.Empty(t, keys.Index)
	})

	t.Run("Store returns error", func(t *testing.T) {
		expectedErr := errors.New("db error")
		keyStore.getErr = expectedErr
		defer func() { keyStore.getErr = nil }()

		_, err := proc.getAppStateKey(ctx, []byte("error-key"))
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("Key found and cached", func(t *testing.T) {
		err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
			Data:      keyData,
			Timestamp: time.Now().Unix(),
		})
		require.NoError(t, err)

		keys, err := proc.getAppStateKey(ctx, keyID)
		require.NoError(t, err)
		assert.NotEmpty(t, keys.Index)

		// Overwrite keyStore with error to ensure next call reads from cache
		keyStore.getErr = errors.New("should not be called")
		defer func() { keyStore.getErr = nil }()

		cachedKeys, err := proc.getAppStateKey(ctx, keyID)
		require.NoError(t, err)
		assert.Equal(t, keys, cachedKeys)
	})
}

func TestProcessor_GetMissingKeyIDs(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, _ := newTestProcessor()

	key1 := []byte("key-existing-1")
	key2 := []byte("key-missing-2")
	key3 := []byte("key-missing-3")

	err := keyStore.PutAppStateSyncKey(ctx, key1, store.AppStateSyncKey{
		Data:      []byte("some-data-32-bytes-sync-key____"),
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	t.Run("Nil key and duplicate handling", func(t *testing.T) {
		pl := &PatchList{
			Snapshot: &waServerSync.SyncdSnapshot{
				KeyID: &waServerSync.KeyId{ID: key1},
				Records: []*waServerSync.SyncdRecord{
					{KeyID: &waServerSync.KeyId{ID: key2}},
					{KeyID: nil},                           // nil key ID should be ignored
					{KeyID: &waServerSync.KeyId{ID: key2}}, // duplicate
				},
			},
			Patches: []*waServerSync.SyncdPatch{
				{KeyID: &waServerSync.KeyId{ID: key3}},
				{KeyID: &waServerSync.KeyId{ID: key1}}, // existing
			},
		}

		missing := proc.GetMissingKeyIDs(ctx, pl)
		require.Len(t, missing, 2)
		assert.Contains(t, missing, key2)
		assert.Contains(t, missing, key3)
	})

	t.Run("Store error does not mark as missing", func(t *testing.T) {
		keyStore.getErr = errors.New("temporary failure")
		defer func() { keyStore.getErr = nil }()

		pl := &PatchList{
			Patches: []*waServerSync.SyncdPatch{
				{KeyID: &waServerSync.KeyId{ID: []byte("err-key")}},
			},
		}

		missing := proc.GetMissingKeyIDs(ctx, pl)
		assert.Empty(t, missing)
	})
}
