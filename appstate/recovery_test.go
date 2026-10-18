// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/proto/waServerSync"
	"github.com/PakaiWA/whatsmeow/proto/waSyncAction"
	"github.com/PakaiWA/whatsmeow/proto/waSyncdSnapshotRecovery"
	"github.com/PakaiWA/whatsmeow/store"
)

func TestParseRecovery(t *testing.T) {
	recoveryData := &waSyncdSnapshotRecovery.SyncdSnapshotRecovery{
		CollectionName:   proto.String("critical_block"),
		CollectionLthash: make([]byte, 128),
		Version:          &waSyncdSnapshotRecovery.SyncdVersion{Version: proto.Uint64(1)},
	}
	marshaled, err := proto.Marshal(recoveryData)
	require.NoError(t, err)

	t.Run("Uncompressed recovery response", func(t *testing.T) {
		resp := &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
			CollectionSnapshot: marshaled,
			IsCompressed:       proto.Bool(false),
		}

		parsed, err := ParseRecovery(resp)
		require.NoError(t, err)
		assert.Equal(t, "critical_block", parsed.GetCollectionName())
		assert.Equal(t, uint64(1), parsed.GetVersion().GetVersion())
	})

	t.Run("Compressed recovery response", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write(marshaled)
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		resp := &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
			CollectionSnapshot: buf.Bytes(),
			IsCompressed:       proto.Bool(true),
		}

		parsed, err := ParseRecovery(resp)
		require.NoError(t, err)
		assert.Equal(t, "critical_block", parsed.GetCollectionName())
		assert.Equal(t, uint64(1), parsed.GetVersion().GetVersion())
	})

	t.Run("Invalid gzip data returns error", func(t *testing.T) {
		resp := &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
			CollectionSnapshot: []byte("not-valid-gzip-header"),
			IsCompressed:       proto.Bool(true),
		}

		_, err := ParseRecovery(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start decompressing")
	})

	t.Run("Invalid protobuf data returns error", func(t *testing.T) {
		resp := &waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse{
			CollectionSnapshot: []byte{0xff, 0xff, 0xff},
			IsCompressed:       proto.Bool(false),
		}

		_, err := ParseRecovery(resp)
		assert.Error(t, err)
	})
}

func TestProcessor_ProcessRecovery(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, appStateStore := newTestProcessor()

	keyID := []byte("recovery-key-id-1")
	keyData := []byte("recovery-sync-key-32-bytes-long_")
	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      keyData,
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	idxJSON, err := json.Marshal([]string{"mute", "12345@s.whatsapp.net"})
	require.NoError(t, err)

	validLthash := make([]byte, 128)
	for i := range validLthash {
		validLthash[i] = byte(i)
	}

	buildValidRecovery := func() *waSyncdSnapshotRecovery.SyncdSnapshotRecovery {
		return &waSyncdSnapshotRecovery.SyncdSnapshotRecovery{
			CollectionName:   proto.String("regular_high"),
			CollectionLthash: validLthash,
			Version:          &waSyncdSnapshotRecovery.SyncdVersion{Version: proto.Uint64(10)},
			MutationRecords: []*waSyncdSnapshotRecovery.SyncdPlainTextRecord{
				{
					KeyID: keyID,
					Mac:   []byte("value-mac-32-bytes-for-recovery_"),
					Value: &waSyncAction.SyncActionData{
						Index:   idxJSON,
						Version: proto.Int32(2),
						Value: &waSyncAction.SyncActionValue{
							MuteAction: &waSyncAction.MuteAction{Muted: proto.Bool(true)},
						},
					},
				},
			},
		}
	}

	t.Run("Invalid lthash length returns error", func(t *testing.T) {
		rec := buildValidRecovery()
		rec.CollectionLthash = []byte("too-short")

		_, err := proc.ProcessRecovery(ctx, rec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid lthash length: 9")
	})

	t.Run("Unknown key ID returns error", func(t *testing.T) {
		rec := buildValidRecovery()
		rec.MutationRecords[0].KeyID = []byte("unknown-key")

		_, err := proc.ProcessRecovery(ctx, rec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get key")
	})

	t.Run("Invalid JSON index returns error", func(t *testing.T) {
		rec := buildValidRecovery()
		rec.MutationRecords[0].Value.Index = []byte("not-json")

		_, err := proc.ProcessRecovery(ctx, rec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal index for mutation #1")
	})

	t.Run("DeleteAppStateVersion error returns error", func(t *testing.T) {
		rec := buildValidRecovery()
		appStateStore.deleteVersionErr = errors.New("delete version error")
		defer func() { appStateStore.deleteVersionErr = nil }()

		_, err := proc.ProcessRecovery(ctx, rec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to reset app state version in database")
	})

	t.Run("PutAppStateVersion error returns error", func(t *testing.T) {
		rec := buildValidRecovery()
		appStateStore.putVersionErr = errors.New("put version error")
		defer func() { appStateStore.putVersionErr = nil }()

		_, err := proc.ProcessRecovery(ctx, rec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update app state version in the database")
	})

	t.Run("PutAppStateMutationMACs error returns error", func(t *testing.T) {
		rec := buildValidRecovery()
		appStateStore.putMACsErr = errors.New("put macs error")
		defer func() { appStateStore.putMACsErr = nil }()

		_, err := proc.ProcessRecovery(ctx, rec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert added mutation MACs to the database")
	})

	t.Run("Successful processing", func(t *testing.T) {
		rec := buildValidRecovery()

		mutations, err := proc.ProcessRecovery(ctx, rec)
		require.NoError(t, err)
		require.Len(t, mutations, 1)

		mut := mutations[0]
		assert.Equal(t, keyID, mut.KeyID)
		assert.Equal(t, waServerSync.SyncdMutation_SET, mut.Operation)
		assert.Equal(t, []string{"mute", "12345@s.whatsapp.net"}, mut.Index)
		assert.Equal(t, int32(2), mut.Version)
		assert.Equal(t, uint64(10), mut.PatchVersion)
		assert.True(t, mut.Action.GetMuteAction().GetMuted())
		assert.NotEmpty(t, mut.IndexMAC)
		assert.Equal(t, []byte("value-mac-32-bytes-for-recovery_"), mut.ValueMAC)

		// Verify stored version and MACs
		ver, hash, err := appStateStore.GetAppStateVersion(ctx, "regular_high")
		require.NoError(t, err)
		assert.Equal(t, uint64(10), ver)
		assert.Equal(t, *(*[128]byte)(validLthash), hash)

		storedMAC, err := appStateStore.GetAppStateMutationMAC(ctx, "regular_high", mut.IndexMAC)
		require.NoError(t, err)
		assert.Equal(t, mut.ValueMAC, storedMAC)
	})
}
