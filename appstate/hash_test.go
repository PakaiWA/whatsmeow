// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/proto/waServerSync"
)

func TestUint64ToBytes(t *testing.T) {
	val := uint64(0x0102030405060708)
	b := uint64ToBytes(val)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, b)

	zero := uint64ToBytes(0)
	assert.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0}, zero)
}

func TestConcatAndHMAC(t *testing.T) {
	key := []byte("secret_key_32_bytes_long_hmac___")
	d1 := []byte("hello")
	d2 := []byte("world")

	mac1 := concatAndHMAC(sha256.New, key, d1, d2)
	assert.Len(t, mac1, 32)

	// Deterministic
	mac2 := concatAndHMAC(sha256.New, key, d1, d2)
	assert.Equal(t, mac1, mac2)

	// Different key yields different MAC
	diffKeyMac := concatAndHMAC(sha256.New, []byte("diff_key_32_bytes_long_hmac_____"), d1, d2)
	assert.NotEqual(t, mac1, diffKeyMac)
}

func TestHashState_GenerateSnapshotMAC(t *testing.T) {
	hs := &HashState{
		Version: 5,
		Hash:    [128]byte{1, 2, 3, 4},
	}
	key := []byte("snapshot_mac_key_32_bytes_long__")
	mac := hs.generateSnapshotMAC(WAPatchRegularLow, key)
	assert.Len(t, mac, 32)

	// Same inputs yield same MAC
	mac2 := hs.generateSnapshotMAC(WAPatchRegularLow, key)
	assert.Equal(t, mac, mac2)

	// Different version yields different MAC
	hsDiff := &HashState{Version: 6, Hash: hs.Hash}
	assert.NotEqual(t, mac, hsDiff.generateSnapshotMAC(WAPatchRegularLow, key))
}

func TestGeneratePatchMAC(t *testing.T) {
	key := []byte("patch_mac_key_32_bytes_long_____")
	snapMAC := make([]byte, 32)
	valBlob := make([]byte, 48) // 16 bytes IV + 32 bytes MAC
	for i := range valBlob {
		valBlob[i] = byte(i)
	}

	patch := &waServerSync.SyncdPatch{
		SnapshotMAC: snapMAC,
		Mutations: []*waServerSync.SyncdMutation{
			{
				Operation: waServerSync.SyncdMutation_SET.Enum(),
				Record: &waServerSync.SyncdRecord{
					Value: &waServerSync.SyncdValue{Blob: valBlob},
				},
			},
		},
	}

	pmac := generatePatchMAC(patch, WAPatchRegular, key, 10)
	assert.Len(t, pmac, 32)

	// Determinism
	pmac2 := generatePatchMAC(patch, WAPatchRegular, key, 10)
	assert.Equal(t, pmac, pmac2)
}

func TestGenerateContentMAC(t *testing.T) {
	key := []byte("content_mac_key_32_bytes_long___")
	keyID := []byte("key-id-123")
	data := []byte("encrypted_payload_bytes_here")

	cmac := generateContentMAC(waServerSync.SyncdMutation_SET, data, keyID, key)
	assert.Len(t, cmac, 32)

	// Different operation produces different MAC
	cmacRemove := generateContentMAC(waServerSync.SyncdMutation_REMOVE, data, keyID, key)
	assert.NotEqual(t, cmac, cmacRemove)
}

func TestHashState_UpdateHash(t *testing.T) {
	valBlob := make([]byte, 48) // last 32 bytes is valueMAC
	for i := range valBlob {
		valBlob[i] = byte(i + 1)
	}
	indexBlob := []byte("index_mac_32_bytes_test_data____")

	t.Run("SET operation adds value MAC to hash", func(t *testing.T) {
		hs := &HashState{}
		initialHash := hs.Hash

		mutations := []*waServerSync.SyncdMutation{
			{
				Operation: waServerSync.SyncdMutation_SET.Enum(),
				Record: &waServerSync.SyncdRecord{
					Index: &waServerSync.SyncdIndex{Blob: indexBlob},
					Value: &waServerSync.SyncdValue{Blob: valBlob},
				},
			},
		}

		warnings, err := hs.updateHash(mutations, func(_ []byte, _ int) ([]byte, error) {
			return nil, nil
		})
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.NotEqual(t, initialHash, hs.Hash)
	})

	t.Run("REMOVE operation with previous value subtracts from hash", func(t *testing.T) {
		hs := &HashState{}
		prevValueMAC := valBlob[len(valBlob)-32:]

		mutations := []*waServerSync.SyncdMutation{
			{
				Operation: waServerSync.SyncdMutation_REMOVE.Enum(),
				Record: &waServerSync.SyncdRecord{
					Index: &waServerSync.SyncdIndex{Blob: indexBlob},
				},
			},
		}

		warnings, err := hs.updateHash(mutations, func(_ []byte, _ int) ([]byte, error) {
			return prevValueMAC, nil
		})
		require.NoError(t, err)
		assert.Empty(t, warnings)
	})

	t.Run("REMOVE operation missing previous value generates warning", func(t *testing.T) {
		hs := &HashState{}
		mutations := []*waServerSync.SyncdMutation{
			{
				Operation: waServerSync.SyncdMutation_REMOVE.Enum(),
				Record: &waServerSync.SyncdRecord{
					Index: &waServerSync.SyncdIndex{Blob: indexBlob},
				},
			},
		}

		warnings, err := hs.updateHash(mutations, func(_ []byte, _ int) ([]byte, error) {
			return nil, nil // not found
		})
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		assert.ErrorIs(t, warnings[0], ErrMissingPreviousSetValueOperation)
	})

	t.Run("Callback error returns error", func(t *testing.T) {
		hs := &HashState{}
		expectedErr := errors.New("database error")

		mutations := []*waServerSync.SyncdMutation{
			{
				Operation: waServerSync.SyncdMutation_SET.Enum(),
				Record: &waServerSync.SyncdRecord{
					Index: &waServerSync.SyncdIndex{Blob: indexBlob},
					Value: &waServerSync.SyncdValue{Blob: valBlob},
				},
			},
		}

		_, err := hs.updateHash(mutations, func(_ []byte, _ int) ([]byte, error) {
			return nil, expectedErr
		})
		assert.ErrorIs(t, err, expectedErr)
	})
}
