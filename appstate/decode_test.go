// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waServerSync"
	"github.com/PakaiWA/whatsmeow/proto/waSyncAction"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/util/cbcutil"
)

// Helper to construct a valid encrypted SyncdMutation
func createTestMutation(
	keys ExpandedAppStateKeys,
	keyID []byte,
	op waServerSync.SyncdMutation_SyncdOperation,
	index []string,
	val *waSyncAction.SyncActionValue,
	version int32,
) (*waServerSync.SyncdMutation, []byte, []byte, error) {
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return nil, nil, nil, err
	}
	syncData := &waSyncAction.SyncActionData{
		Index:   indexBytes,
		Value:   val,
		Padding: []byte{},
		Version: &version,
	}
	content, err := proto.Marshal(syncData)
	if err != nil {
		return nil, nil, nil, err
	}
	encrypted, err := cbcutil.Encrypt(keys.ValueEncryption, nil, content)
	if err != nil {
		return nil, nil, nil, err
	}
	valueMAC := generateContentMAC(op, encrypted, keyID, keys.ValueMAC)
	indexMAC := concatAndHMAC(sha256.New, keys.Index, indexBytes)

	mut := &waServerSync.SyncdMutation{
		Operation: op.Enum(),
		Record: &waServerSync.SyncdRecord{
			Index: &waServerSync.SyncdIndex{Blob: indexMAC},
			Value: &waServerSync.SyncdValue{Blob: append(encrypted, valueMAC...)},
			KeyID: &waServerSync.KeyId{ID: keyID},
		},
	}
	return mut, indexMAC, valueMAC, nil
}

func TestPatchOutput_AddAndRemoveMAC(t *testing.T) {
	out := &patchOutput{}
	idx1 := []byte("index-mac-1-32-bytes-long______")
	val1 := []byte("value-mac-1-32-bytes-long______")
	idx2 := []byte("index-mac-2-32-bytes-long______")
	val2 := []byte("value-mac-2-32-bytes-long______")

	out.AddMAC(idx1, val1)
	out.AddMAC(idx2, val2)
	assert.Len(t, out.AddedMACs, 2)

	// Remove idx1 - should remove from AddedMACs and append to RemovedMACs
	out.RemoveMAC(idx1)
	require.Len(t, out.AddedMACs, 1)
	assert.Equal(t, idx2, out.AddedMACs[0].IndexMAC)
	require.Len(t, out.RemovedMACs, 1)
	assert.Equal(t, idx1, out.RemovedMACs[0])
}

func TestIndexMACToArray(t *testing.T) {
	t.Run("Valid 32-byte slice", func(t *testing.T) {
		b := make([]byte, 32)
		b[0] = 0xab
		b[31] = 0xcd
		arr := indexMACToArray(b)
		assert.Equal(t, byte(0xab), arr[0])
		assert.Equal(t, byte(0xcd), arr[31])
	})

	t.Run("Invalid slice length", func(t *testing.T) {
		arrShort := indexMACToArray([]byte("short"))
		assert.Equal(t, [32]byte{}, arrShort)

		arrLong := indexMACToArray(make([]byte, 33))
		assert.Equal(t, [32]byte{}, arrLong)
	})
}

func TestParsePatchList(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty node with attributes only", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "collection",
			Attrs: waBinary.Attrs{
				"name":             "regular_high",
				"has_more_patches": "true",
			},
		}
		pl, err := ParsePatchList(ctx, node, nil)
		require.NoError(t, err)
		assert.Equal(t, WAPatchRegularHigh, pl.Name)
		assert.True(t, pl.HasMorePatches)
		assert.Nil(t, pl.Snapshot)
		assert.Empty(t, pl.Patches)
	})

	t.Run("Snapshot with external blob download", func(t *testing.T) {
		snapProto := &waServerSync.SyncdSnapshot{
			Version: &waServerSync.SyncdVersion{Version: proto.Uint64(3)},
		}
		snapBytes, err := proto.Marshal(snapProto)
		require.NoError(t, err)

		blobRef := &waServerSync.ExternalBlobReference{
			DirectPath: proto.String("/path/to/snapshot"),
		}
		blobRefBytes, err := proto.Marshal(blobRef)
		require.NoError(t, err)

		node := &waBinary.Node{
			Tag: "collection",
			Attrs: waBinary.Attrs{
				"name": "regular_low",
			},
			Content: []waBinary.Node{
				{
					Tag:     "snapshot",
					Content: blobRefBytes,
				},
			},
		}

		downloader := func(_ context.Context, ref *waServerSync.ExternalBlobReference) ([]byte, error) {
			assert.Equal(t, "/path/to/snapshot", ref.GetDirectPath())
			return snapBytes, nil
		}

		pl, err := ParsePatchList(ctx, node, downloader)
		require.NoError(t, err)
		require.NotNil(t, pl.Snapshot)
		assert.Equal(t, uint64(3), pl.Snapshot.GetVersion().GetVersion())
	})

	t.Run("Snapshot unmarshal error", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "collection",
			Content: []waBinary.Node{
				{
					Tag:     "snapshot",
					Content: []byte{0xff, 0xff},
				},
			},
		}
		_, err := ParsePatchList(ctx, node, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal snapshot")
	})

	t.Run("Snapshot download external error", func(t *testing.T) {
		blobRef := &waServerSync.ExternalBlobReference{}
		blobRefBytes, _ := proto.Marshal(blobRef)
		node := &waBinary.Node{
			Tag: "collection",
			Content: []waBinary.Node{
				{
					Tag:     "snapshot",
					Content: blobRefBytes,
				},
			},
		}
		downloader := func(_ context.Context, _ *waServerSync.ExternalBlobReference) ([]byte, error) {
			return nil, errors.New("network timeout")
		}
		_, err := ParsePatchList(ctx, node, downloader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download external mutations")
	})

	t.Run("Snapshot downloaded content unmarshal error", func(t *testing.T) {
		blobRef := &waServerSync.ExternalBlobReference{}
		blobRefBytes, _ := proto.Marshal(blobRef)
		node := &waBinary.Node{
			Tag: "collection",
			Content: []waBinary.Node{
				{
					Tag:     "snapshot",
					Content: blobRefBytes,
				},
			},
		}
		downloader := func(_ context.Context, _ *waServerSync.ExternalBlobReference) ([]byte, error) {
			return []byte{0xff, 0xff}, nil
		}
		_, err := ParsePatchList(ctx, node, downloader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal mutation list")
	})

	t.Run("Patches with external mutations download and errors", func(t *testing.T) {
		// Valid patch with external mutations
		mutationsProto := &waServerSync.SyncdMutations{
			Mutations: []*waServerSync.SyncdMutation{
				{Operation: waServerSync.SyncdMutation_SET.Enum()},
			},
		}
		mutBytes, err := proto.Marshal(mutationsProto)
		require.NoError(t, err)

		patchWithExt := &waServerSync.SyncdPatch{
			Version: &waServerSync.SyncdVersion{Version: proto.Uint64(1)},
			ExternalMutations: &waServerSync.ExternalBlobReference{
				DirectPath: proto.String("/ext/mutations"),
			},
		}
		patchBytes, err := proto.Marshal(patchWithExt)
		require.NoError(t, err)

		node := &waBinary.Node{
			Tag: "collection",
			Attrs: waBinary.Attrs{
				"name": "regular_high",
			},
			Content: []waBinary.Node{
				{
					Tag: "patches",
					Content: []waBinary.Node{
						{
							Tag:     "patch",
							Content: patchBytes,
						},
						{
							Tag:     "ignore_tag",
							Content: []byte("skip me"),
						},
					},
				},
			},
		}

		downloader := func(_ context.Context, _ *waServerSync.ExternalBlobReference) ([]byte, error) {
			return mutBytes, nil
		}

		pl, err := ParsePatchList(ctx, node, downloader)
		require.NoError(t, err)
		require.Len(t, pl.Patches, 1)
		assert.Len(t, pl.Patches[0].Mutations, 1)

		// Download returns 0 mutations error
		emptyMutBytes, _ := proto.Marshal(&waServerSync.SyncdMutations{})
		emptyDownloader := func(_ context.Context, _ *waServerSync.ExternalBlobReference) ([]byte, error) {
			return emptyMutBytes, nil
		}
		_, err = ParsePatchList(ctx, node, emptyDownloader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "didn't get any mutations from download")

		// Downloader returns error
		errDownloader := func(_ context.Context, _ *waServerSync.ExternalBlobReference) ([]byte, error) {
			return nil, errors.New("download failed")
		}
		_, err = ParsePatchList(ctx, node, errDownloader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download external mutations")

		// Downloader returns invalid proto bytes
		invalidDownloader := func(_ context.Context, _ *waServerSync.ExternalBlobReference) ([]byte, error) {
			return []byte{0xff, 0xff}, nil
		}
		_, err = ParsePatchList(ctx, node, invalidDownloader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal mutation list")
	})

	t.Run("Patch node unmarshal error", func(t *testing.T) {
		node := &waBinary.Node{
			Tag: "collection",
			Content: []waBinary.Node{
				{
					Tag: "patches",
					Content: []waBinary.Node{
						{
							Tag:     "patch",
							Content: []byte{0xff, 0xff},
						},
					},
				},
			},
		}
		_, err := ParsePatchList(ctx, node, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal patch #1")
	})
}

func TestProcessor_DecodeMutation(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, _ := newTestProcessor()

	keyID := []byte("mutation-key-1")
	keyData := []byte("key-data-32-bytes-sync-mutation_")
	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      keyData,
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	keys, err := proc.getAppStateKey(ctx, keyID)
	require.NoError(t, err)

	val := &waSyncAction.SyncActionValue{
		PinAction: &waSyncAction.PinAction{Pinned: proto.Bool(true)},
	}
	mut, expectedIndexMAC, expectedValueMAC, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_SET, []string{IndexPin, "chat@s.whatsapp.net"}, val, 5)
	require.NoError(t, err)

	t.Run("Success decode mutation with validateMACs=true", func(t *testing.T) {
		idxMAC, valMAC, index, syncAction, expandedKeys, err := proc.decodeMutation(ctx, mut, 0, true)
		require.NoError(t, err)
		assert.Equal(t, expectedIndexMAC, idxMAC)
		assert.Equal(t, expectedValueMAC, valMAC)
		assert.Equal(t, []string{IndexPin, "chat@s.whatsapp.net"}, index)
		assert.True(t, syncAction.GetValue().GetPinAction().GetPinned())
		assert.Equal(t, int32(5), syncAction.GetVersion())
		assert.Equal(t, keys.Index, expandedKeys.Index)
	})

	t.Run("Key not found error", func(t *testing.T) {
		mutUnknownKey := proto.Clone(mut).(*waServerSync.SyncdMutation)
		mutUnknownKey.Record.KeyID = &waServerSync.KeyId{ID: []byte("unknown-key")}

		_, _, _, _, _, err := proc.decodeMutation(ctx, mutUnknownKey, 0, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get key")
	})

	t.Run("Value MAC mismatch error", func(t *testing.T) {
		corruptedMut := proto.Clone(mut).(*waServerSync.SyncdMutation)
		blob := corruptedMut.Record.Value.Blob
		// Corrupt value MAC at the end
		blob[len(blob)-1] ^= 0xff

		_, _, _, _, _, err := proc.decodeMutation(ctx, corruptedMut, 0, true)
		assert.ErrorIs(t, err, ErrMismatchingContentMAC)

		// Without MAC validation, should fail at decryption or proto unmarshal if content altered
		_, _, _, _, _, errNoVal := proc.decodeMutation(ctx, corruptedMut, 0, false)
		assert.NoError(t, errNoVal) // since only MAC bytes were flipped, decryption of content still succeeds
	})

	t.Run("Decryption error", func(t *testing.T) {
		corruptedMut := proto.Clone(mut).(*waServerSync.SyncdMutation)
		// 16 bytes IV + 10 bytes unaligned ciphertext + 32 bytes value MAC
		corruptedMut.Record.Value.Blob = append(append(make([]byte, 16), []byte("bad-cipher")...), make([]byte, 32)...)

		_, _, _, _, _, err := proc.decodeMutation(ctx, corruptedMut, 0, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decrypt mutation #1")
	})

	t.Run("SyncActionData unmarshal error", func(t *testing.T) {
		// Encrypt garbage bytes
		badPlaintext := []byte("garbage not protobuf")
		enc, _ := cbcutil.Encrypt(keys.ValueEncryption, nil, badPlaintext)
		badMut := &waServerSync.SyncdMutation{
			Operation: waServerSync.SyncdMutation_SET.Enum(),
			Record: &waServerSync.SyncdRecord{
				KeyID: &waServerSync.KeyId{ID: keyID},
				Value: &waServerSync.SyncdValue{Blob: append(enc, make([]byte, 32)...)},
			},
		}
		_, _, _, _, _, err := proc.decodeMutation(ctx, badMut, 0, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal mutation #1")
	})

	t.Run("Index MAC mismatch error", func(t *testing.T) {
		corruptedMut := proto.Clone(mut).(*waServerSync.SyncdMutation)
		corruptedMut.Record.Index.Blob = make([]byte, 32) // wrong index MAC

		_, _, _, _, _, err := proc.decodeMutation(ctx, corruptedMut, 0, true)
		assert.ErrorIs(t, err, ErrMismatchingIndexMAC)
	})

	t.Run("Index JSON unmarshal error", func(t *testing.T) {
		// Valid protobuf SyncActionData but Index is invalid JSON
		badJSONData := &waSyncAction.SyncActionData{
			Index: []byte("{invalid-json"),
		}
		marshaled, _ := proto.Marshal(badJSONData)
		enc, _ := cbcutil.Encrypt(keys.ValueEncryption, nil, marshaled)
		badJSONMut := &waServerSync.SyncdMutation{
			Operation: waServerSync.SyncdMutation_SET.Enum(),
			Record: &waServerSync.SyncdRecord{
				KeyID: &waServerSync.KeyId{ID: keyID},
				Value: &waServerSync.SyncdValue{Blob: append(enc, make([]byte, 32)...)},
			},
		}
		_, _, _, _, _, err := proc.decodeMutation(ctx, badJSONMut, 0, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal index of mutation #1")
	})
}

func TestProcessor_StoreMACs(t *testing.T) {
	ctx := context.Background()
	proc, _, appStateStore := newTestProcessor()

	out := &patchOutput{
		AddedMACs: []store.AppStateMutationMAC{
			{IndexMAC: []byte("idx1"), ValueMAC: []byte("val1")},
		},
		RemovedMACs: [][]byte{[]byte("idx0")},
	}
	state := HashState{Version: 10, Hash: [128]byte{1, 2, 3}}

	t.Run("PutAppStateVersion error", func(t *testing.T) {
		appStateStore.putVersionErr = errors.New("put version failed")
		defer func() { appStateStore.putVersionErr = nil }()

		err := proc.storeMACs(ctx, WAPatchRegularLow, state, out)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update app state version in the database")
	})

	t.Run("DeleteAppStateMutationMACs error", func(t *testing.T) {
		appStateStore.deleteMACsErr = errors.New("delete macs failed")
		defer func() { appStateStore.deleteMACsErr = nil }()

		err := proc.storeMACs(ctx, WAPatchRegularLow, state, out)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove deleted mutation MACs from the database")
	})

	t.Run("PutAppStateMutationMACs error", func(t *testing.T) {
		appStateStore.putMACsErr = errors.New("put macs failed")
		defer func() { appStateStore.putMACsErr = nil }()

		err := proc.storeMACs(ctx, WAPatchRegularLow, state, out)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert added mutation MACs to the database")
	})

	t.Run("Success storing MACs", func(t *testing.T) {
		err := proc.storeMACs(ctx, WAPatchRegularLow, state, out)
		require.NoError(t, err)

		ver, hash, err := appStateStore.GetAppStateVersion(ctx, string(WAPatchRegularLow))
		require.NoError(t, err)
		assert.Equal(t, uint64(10), ver)
		assert.Equal(t, state.Hash, hash)
	})
}

func TestProcessor_ValidateSnapshotMAC(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, _ := newTestProcessor()

	keyID := []byte("snap-key-1")
	keyData := []byte("sync-key-data-32-bytes-snapshot_")
	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      keyData,
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	keys, err := proc.getAppStateKey(ctx, keyID)
	require.NoError(t, err)

	state := HashState{Version: 1, Hash: [128]byte{5, 6, 7}}
	validMAC := state.generateSnapshotMAC(WAPatchRegularHigh, keys.SnapshotMAC)

	t.Run("Valid snapshot MAC", func(t *testing.T) {
		retKeys, err := proc.validateSnapshotMAC(ctx, WAPatchRegularHigh, state, keyID, validMAC)
		require.NoError(t, err)
		assert.Equal(t, keys.SnapshotMAC, retKeys.SnapshotMAC)
	})

	t.Run("Mismatching snapshot MAC", func(t *testing.T) {
		invalidMAC := make([]byte, 32)
		_, err := proc.validateSnapshotMAC(ctx, WAPatchRegularHigh, state, keyID, invalidMAC)
		assert.ErrorIs(t, err, ErrMismatchingLTHash)
	})

	t.Run("Key not found", func(t *testing.T) {
		_, err := proc.validateSnapshotMAC(ctx, WAPatchRegularHigh, state, []byte("missing-key"), validMAC)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get key")
	})
}

func TestProcessor_ValidatePatch(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, appStateStore := newTestProcessor()

	keyID := []byte("patch-key-1")
	keyData := []byte("sync-key-data-32-bytes-patch____")
	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      keyData,
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	keys, err := proc.getAppStateKey(ctx, keyID)
	require.NoError(t, err)

	val := &waSyncAction.SyncActionValue{
		PinAction: &waSyncAction.PinAction{Pinned: proto.Bool(true)},
	}
	mut1, idxMAC1, _, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_SET, []string{IndexPin, "chat1@s.whatsapp.net"}, val, 5)
	require.NoError(t, err)

	// Remove mutation referencing mut1
	mut2, _, _, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_REMOVE, []string{IndexPin, "chat1@s.whatsapp.net"}, nil, 5)
	require.NoError(t, err)
	mut2.Record.Index.Blob = idxMAC1 // same index MAC

	// Remove mutation referencing a key in the database
	idxMAC3 := []byte("database-index-mac-32-bytes_____")
	valMAC3 := []byte("database-value-mac-32-bytes_____")
	err = appStateStore.PutAppStateMutationMACs(ctx, string(WAPatchRegularLow), 1, []store.AppStateMutationMAC{
		{IndexMAC: idxMAC3, ValueMAC: valMAC3},
	})
	require.NoError(t, err)

	mut3, _, _, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_REMOVE, []string{IndexPin, "chat3@s.whatsapp.net"}, nil, 5)
	require.NoError(t, err)
	mut3.Record.Index.Blob = idxMAC3

	// Remove mutation after remove mutation (found a REMOVE operation in patch -> returns nil, nil)
	mut4 := proto.Clone(mut2).(*waServerSync.SyncdMutation)

	t.Run("Patch with previous value in patch and in DB", func(t *testing.T) {
		patch := &waServerSync.SyncdPatch{
			Version:   &waServerSync.SyncdVersion{Version: proto.Uint64(2)},
			KeyID:     &waServerSync.KeyId{ID: keyID},
			Mutations: []*waServerSync.SyncdMutation{mut1, mut2, mut3, mut4},
		}

		initialState := HashState{Version: 1}
		newState, warn, err := proc.validatePatch(ctx, WAPatchRegularLow, patch, initialState, false)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), newState.Version)
		assert.NotEmpty(t, warn) // mut4 produces missing previous value warning
	})

	t.Run("Patch validation with MAC verification", func(t *testing.T) {
		initialState := HashState{Version: 1}
		patch := &waServerSync.SyncdPatch{
			Version:   &waServerSync.SyncdVersion{Version: proto.Uint64(2)},
			KeyID:     &waServerSync.KeyId{ID: keyID},
			Mutations: []*waServerSync.SyncdMutation{mut1},
		}

		// Calculate correct snapshotMAC and patchMAC
		tempState := initialState
		tempState.Version = 2
		_, _ = tempState.updateHash(patch.Mutations, func(_ []byte, _ int) ([]byte, error) {
			return nil, nil
		})
		patch.SnapshotMAC = tempState.generateSnapshotMAC(WAPatchRegularLow, keys.SnapshotMAC)
		patch.PatchMAC = generatePatchMAC(patch, WAPatchRegularLow, keys.PatchMAC, 2)

		newState, warn, err := proc.validatePatch(ctx, WAPatchRegularLow, patch, initialState, true)
		require.NoError(t, err)
		assert.Empty(t, warn)
		assert.Equal(t, tempState.Hash, newState.Hash)

		// Mismatching snapshot MAC
		corruptedSnap := proto.Clone(patch).(*waServerSync.SyncdPatch)
		corruptedSnap.SnapshotMAC = make([]byte, 32)
		_, _, err = proc.validatePatch(ctx, WAPatchRegularLow, corruptedSnap, initialState, true)
		assert.ErrorIs(t, err, ErrMismatchingLTHash)

		// Mismatching patch MAC
		corruptedPatchMAC := proto.Clone(patch).(*waServerSync.SyncdPatch)
		corruptedPatchMAC.PatchMAC = make([]byte, 32)
		_, _, err = proc.validatePatch(ctx, WAPatchRegularLow, corruptedPatchMAC, initialState, true)
		assert.ErrorIs(t, err, ErrMismatchingPatchMAC)
	})

	t.Run("DB error while updating hash returns error", func(t *testing.T) {
		appStateStore.getMACErr = errors.New("db get error")
		defer func() { appStateStore.getMACErr = nil }()

		patch := &waServerSync.SyncdPatch{
			Version:   &waServerSync.SyncdVersion{Version: proto.Uint64(2)},
			KeyID:     &waServerSync.KeyId{ID: keyID},
			Mutations: []*waServerSync.SyncdMutation{mut3},
		}
		_, _, err := proc.validatePatch(ctx, WAPatchRegularLow, patch, HashState{Version: 1}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update state hash")
	})
}

func TestProcessor_DecodeSnapshotAndDecodePatches(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, appStateStore := newTestProcessor()

	keyID := []byte("full-test-key-id")
	keyData := []byte("full-test-key-data-32-bytes-long")
	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      keyData,
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	keys, err := proc.getAppStateKey(ctx, keyID)
	require.NoError(t, err)

	val1 := &waSyncAction.SyncActionValue{
		PinAction: &waSyncAction.PinAction{Pinned: proto.Bool(true)},
	}
	mut1, idxMAC1, valMAC1, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_SET, []string{IndexPin, "chat1@s.whatsapp.net"}, val1, 5)
	require.NoError(t, err)

	// Create snapshot
	record1 := mut1.Record
	snapshot := &waServerSync.SyncdSnapshot{
		Version: &waServerSync.SyncdVersion{Version: proto.Uint64(1)},
		Records: []*waServerSync.SyncdRecord{record1},
		KeyID:   &waServerSync.KeyId{ID: keyID},
	}

	// Calculate snapshot MAC
	var snapState HashState
	snapState.Version = 1
	_, err = snapState.updateHash([]*waServerSync.SyncdMutation{{
		Operation: waServerSync.SyncdMutation_SET.Enum(),
		Record:    record1,
	}}, func(_ []byte, _ int) ([]byte, error) { return nil, nil })
	require.NoError(t, err)
	snapshot.Mac = snapState.generateSnapshotMAC(WAPatchRegularLow, keys.SnapshotMAC)

	// Create patch v2
	val2 := &waSyncAction.SyncActionValue{
		PinAction: &waSyncAction.PinAction{Pinned: proto.Bool(false)},
	}
	mut2, idxMAC2, valMAC2, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_SET, []string{IndexPin, "chat2@s.whatsapp.net"}, val2, 5)
	require.NoError(t, err)

	patch2State := snapState
	patch2State.Version = 2
	_, err = patch2State.updateHash([]*waServerSync.SyncdMutation{mut2}, func(_ []byte, _ int) ([]byte, error) {
		return nil, nil
	})
	require.NoError(t, err)

	patch2 := &waServerSync.SyncdPatch{
		Version:     &waServerSync.SyncdVersion{Version: proto.Uint64(2)},
		KeyID:       &waServerSync.KeyId{ID: keyID},
		Mutations:   []*waServerSync.SyncdMutation{mut2},
		SnapshotMAC: patch2State.generateSnapshotMAC(WAPatchRegularLow, keys.SnapshotMAC),
	}
	patch2.PatchMAC = generatePatchMAC(patch2, WAPatchRegularLow, keys.PatchMAC, 2)

	t.Run("DecodePatches with both snapshot and patch (validateMACs=true)", func(t *testing.T) {
		list := &PatchList{
			Name:     WAPatchRegularLow,
			Snapshot: snapshot,
			Patches:  []*waServerSync.SyncdPatch{patch2},
		}

		mutations, finalState, err := proc.DecodePatches(ctx, list, HashState{}, true)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), finalState.Version)
		assert.Equal(t, patch2State.Hash, finalState.Hash)
		require.Len(t, mutations, 2)

		assert.Equal(t, []string{IndexPin, "chat1@s.whatsapp.net"}, mutations[0].Index)
		assert.True(t, mutations[0].Action.GetPinAction().GetPinned())
		assert.Equal(t, uint64(1), mutations[0].PatchVersion)

		assert.Equal(t, []string{IndexPin, "chat2@s.whatsapp.net"}, mutations[1].Index)
		assert.False(t, mutations[1].Action.GetPinAction().GetPinned())
		assert.Equal(t, uint64(2), mutations[1].PatchVersion)

		// Verify database state
		v, h, err := appStateStore.GetAppStateVersion(ctx, string(WAPatchRegularLow))
		require.NoError(t, err)
		assert.Equal(t, uint64(2), v)
		assert.Equal(t, patch2State.Hash, h)

		storedValMAC1, _ := appStateStore.GetAppStateMutationMAC(ctx, string(WAPatchRegularLow), idxMAC1)
		assert.Equal(t, valMAC1, storedValMAC1)
		storedValMAC2, _ := appStateStore.GetAppStateMutationMAC(ctx, string(WAPatchRegularLow), idxMAC2)
		assert.Equal(t, valMAC2, storedValMAC2)
	})

	t.Run("DecodePatches with snapshot failure", func(t *testing.T) {
		corruptedSnapshot := proto.Clone(snapshot).(*waServerSync.SyncdSnapshot)
		corruptedSnapshot.Mac = make([]byte, 32) // wrong snapshot MAC

		list := &PatchList{
			Name:     WAPatchRegularLow,
			Snapshot: corruptedSnapshot,
		}
		_, _, err := proc.DecodePatches(ctx, list, HashState{}, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify snapshot")
	})

	t.Run("DecodePatches with patch validation failure", func(t *testing.T) {
		appStateStore.macs = make(map[string]map[string][]byte)
		appStateStore.versions = make(map[string]struct {
			ver  uint64
			hash [128]byte
		})
		corruptedPatch := proto.Clone(patch2).(*waServerSync.SyncdPatch)
		corruptedPatch.PatchMAC = make([]byte, 32)

		list := &PatchList{
			Name:    WAPatchRegularLow,
			Patches: []*waServerSync.SyncdPatch{corruptedPatch},
		}
		_, _, err := proc.DecodePatches(ctx, list, snapState, true)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrMismatchingPatchMAC)
	})

	t.Run("DecodePatches with decodeMutations failure in patch", func(t *testing.T) {
		corruptedMutPatch := proto.Clone(patch2).(*waServerSync.SyncdPatch)
		corruptedMutPatch.Mutations[0].Record.KeyID = &waServerSync.KeyId{ID: []byte("unknown-key")}

		list := &PatchList{
			Name:    WAPatchRegularLow,
			Patches: []*waServerSync.SyncdPatch{corruptedMutPatch},
		}
		_, _, err := proc.DecodePatches(ctx, list, snapState, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get key")
	})

	t.Run("DecodePatches with storeMACs failure in patch", func(t *testing.T) {
		appStateStore.putVersionErr = errors.New("put version db error")
		defer func() { appStateStore.putVersionErr = nil }()

		list := &PatchList{
			Name:    WAPatchRegularLow,
			Patches: []*waServerSync.SyncdPatch{patch2},
		}
		_, _, err := proc.DecodePatches(ctx, list, snapState, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update app state version in the database")
	})

	t.Run("DecodePatches with snapshot decodeMutations failure", func(t *testing.T) {
		corruptedRecordSnap := proto.Clone(snapshot).(*waServerSync.SyncdSnapshot)
		corruptedRecordSnap.Records[0].KeyID = &waServerSync.KeyId{ID: []byte("unknown-key")}

		list := &PatchList{
			Name:     WAPatchRegularLow,
			Snapshot: corruptedRecordSnap,
		}
		_, _, err := proc.DecodePatches(ctx, list, HashState{}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode snapshot")
	})

	t.Run("DecodePatches with snapshot storeMACs failure", func(t *testing.T) {
		appStateStore.putVersionErr = errors.New("put version db error on snapshot")
		defer func() { appStateStore.putVersionErr = nil }()

		list := &PatchList{
			Name:     WAPatchRegularLow,
			Snapshot: snapshot,
		}
		_, _, err := proc.DecodePatches(ctx, list, HashState{}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update app state version in the database")
	})
}

func TestProcessor_DecodeMutations_FakeIndexes(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, _ := newTestProcessor()

	keyID := []byte("fake-idx-key")
	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      []byte("32-bytes-sync-key-data-for-fake_"),
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	keys, err := proc.getAppStateKey(ctx, keyID)
	require.NoError(t, err)

	removeMut, idxMAC, _, err := createTestMutation(keys, keyID, waServerSync.SyncdMutation_REMOVE, []string{IndexPin, "chat@s.whatsapp.net"}, nil, 5)
	require.NoError(t, err)

	altMAC := []byte("alt-mac-32-bytes-long___________")
	fakeIndexes := map[[32]byte][]byte{
		indexMACToArray(idxMAC): altMAC,
	}

	out := &patchOutput{
		AddedMACs: []store.AppStateMutationMAC{
			{IndexMAC: altMAC, ValueMAC: []byte("val")},
		},
	}

	err = proc.decodeMutations(ctx, []*waServerSync.SyncdMutation{removeMut}, out, false, 5, fakeIndexes)
	require.NoError(t, err)
	assert.Contains(t, out.RemovedMACs, idxMAC)
	assert.Contains(t, out.RemovedMACs, altMAC)
}
