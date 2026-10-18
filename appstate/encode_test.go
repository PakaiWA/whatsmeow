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
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waCommon"
	"github.com/PakaiWA/whatsmeow/proto/waServerSync"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
)

func TestBuildMute(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")

	t.Run("Mute forever when duration is 0", func(t *testing.T) {
		patch := BuildMute(target, true, 0)
		assert.Equal(t, WAPatchRegularHigh, patch.Type)
		require.Len(t, patch.Mutations, 1)

		mut := patch.Mutations[0]
		assert.Equal(t, []string{IndexMute, target.String()}, mut.Index)
		assert.Equal(t, int32(2), mut.Version)
		require.NotNil(t, mut.Value.GetMuteAction())
		assert.True(t, mut.Value.GetMuteAction().GetMuted())
		assert.Equal(t, int64(-1), mut.Value.GetMuteAction().GetMuteEndTimestamp())
	})

	t.Run("Mute with duration", func(t *testing.T) {
		dur := 2 * time.Hour
		before := time.Now().Add(dur).UnixMilli()
		patch := BuildMute(target, true, dur)
		after := time.Now().Add(dur).UnixMilli()

		require.Len(t, patch.Mutations, 1)
		mut := patch.Mutations[0]
		assert.True(t, mut.Value.GetMuteAction().GetMuted())
		endTs := mut.Value.GetMuteAction().GetMuteEndTimestamp()
		assert.GreaterOrEqual(t, endTs, before)
		assert.LessOrEqual(t, endTs, after)
	})

	t.Run("Unmute", func(t *testing.T) {
		patch := BuildMute(target, false, 0)
		require.Len(t, patch.Mutations, 1)
		mut := patch.Mutations[0]
		assert.False(t, mut.Value.GetMuteAction().GetMuted())
		assert.Nil(t, mut.Value.GetMuteAction().MuteEndTimestamp)
	})
}

func TestBuildMuteAbs(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")

	t.Run("Absolute timestamp specified", func(t *testing.T) {
		ts := int64(1700000000000)
		patch := BuildMuteAbs(target, true, &ts)
		assert.Equal(t, WAPatchRegularHigh, patch.Type)
		require.Len(t, patch.Mutations, 1)
		assert.Equal(t, ts, patch.Mutations[0].Value.GetMuteAction().GetMuteEndTimestamp())
	})

	t.Run("Mute with nil timestamp defaults to -1", func(t *testing.T) {
		patch := BuildMuteAbs(target, true, nil)
		assert.Equal(t, int64(-1), patch.Mutations[0].Value.GetMuteAction().GetMuteEndTimestamp())
	})

	t.Run("Unmute with nil timestamp stays nil", func(t *testing.T) {
		patch := BuildMuteAbs(target, false, nil)
		assert.Nil(t, patch.Mutations[0].Value.GetMuteAction().MuteEndTimestamp)
	})
}

func TestBuildPin(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")

	patchPin := BuildPin(target, true)
	assert.Equal(t, WAPatchRegularLow, patchPin.Type)
	require.Len(t, patchPin.Mutations, 1)
	assert.Equal(t, []string{IndexPin, target.String()}, patchPin.Mutations[0].Index)
	assert.Equal(t, int32(5), patchPin.Mutations[0].Version)
	assert.True(t, patchPin.Mutations[0].Value.GetPinAction().GetPinned())

	patchUnpin := BuildPin(target, false)
	assert.False(t, patchUnpin.Mutations[0].Value.GetPinAction().GetPinned())
}

func TestBuildArchive(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")
	msgKey := &waCommon.MessageKey{
		ID: proto.String("msg-123"),
	}
	ts := time.Unix(1600000000, 0)

	t.Run("Archive chat appends unpin mutation", func(t *testing.T) {
		patch := BuildArchive(target, true, ts, msgKey)
		assert.Equal(t, WAPatchRegularLow, patch.Type)
		require.Len(t, patch.Mutations, 2)

		// Mutation 0: Archive action
		archiveMut := patch.Mutations[0]
		assert.Equal(t, []string{IndexArchive, target.String()}, archiveMut.Index)
		assert.Equal(t, int32(3), archiveMut.Version)
		assert.True(t, archiveMut.Value.GetArchiveChatAction().GetArchived())
		require.NotNil(t, archiveMut.Value.GetArchiveChatAction().GetMessageRange())
		assert.Equal(t, ts.Unix(), archiveMut.Value.GetArchiveChatAction().GetMessageRange().GetLastMessageTimestamp())
		require.Len(t, archiveMut.Value.GetArchiveChatAction().GetMessageRange().GetMessages(), 1)
		assert.Equal(t, "msg-123", archiveMut.Value.GetArchiveChatAction().GetMessageRange().GetMessages()[0].GetKey().GetID())

		// Mutation 1: Unpin action
		unpinMut := patch.Mutations[1]
		assert.Equal(t, []string{IndexPin, target.String()}, unpinMut.Index)
		assert.False(t, unpinMut.Value.GetPinAction().GetPinned())
	})

	t.Run("Unarchive chat does not append unpin mutation", func(t *testing.T) {
		patch := BuildArchive(target, false, time.Time{}, nil)
		assert.Equal(t, WAPatchRegularLow, patch.Type)
		require.Len(t, patch.Mutations, 1)
		assert.False(t, patch.Mutations[0].Value.GetArchiveChatAction().GetArchived())
		assert.Empty(t, patch.Mutations[0].Value.GetArchiveChatAction().GetMessageRange().GetMessages())
	})
}

func TestBuildMarkChatAsRead(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")
	ts := time.Unix(1700000000, 0)

	patch := BuildMarkChatAsRead(target, true, ts, nil)
	assert.Equal(t, WAPatchRegularLow, patch.Type)
	require.Len(t, patch.Mutations, 1)
	mut := patch.Mutations[0]
	assert.Equal(t, []string{IndexMarkChatAsRead, target.String()}, mut.Index)
	assert.Equal(t, int32(3), mut.Version)
	assert.True(t, mut.Value.GetMarkChatAsReadAction().GetRead())
	assert.Equal(t, ts.Unix(), mut.Value.GetMarkChatAsReadAction().GetMessageRange().GetLastMessageTimestamp())
}

func TestBuildLabelChat(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")
	patch := BuildLabelChat(target, "label-42", true)

	assert.Equal(t, WAPatchRegular, patch.Type)
	require.Len(t, patch.Mutations, 1)
	mut := patch.Mutations[0]
	assert.Equal(t, []string{IndexLabelAssociationChat, "label-42", target.String()}, mut.Index)
	assert.Equal(t, int32(3), mut.Version)
	assert.True(t, mut.Value.GetLabelAssociationAction().GetLabeled())
}

func TestBuildLabelMessage(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")
	patch := BuildLabelMessage(target, "label-99", "msg-001", true)

	assert.Equal(t, WAPatchRegular, patch.Type)
	require.Len(t, patch.Mutations, 1)
	mut := patch.Mutations[0]
	assert.Equal(t, []string{IndexLabelAssociationMessage, "label-99", target.String(), "msg-001", "0", "0"}, mut.Index)
	assert.Equal(t, int32(3), mut.Version)
	assert.True(t, mut.Value.GetLabelAssociationAction().GetLabeled())
}

func TestBuildLabelEdit(t *testing.T) {
	patch := BuildLabelEdit("label-5", "VIP Customers", 2, false)

	assert.Equal(t, WAPatchRegular, patch.Type)
	require.Len(t, patch.Mutations, 1)
	mut := patch.Mutations[0]
	assert.Equal(t, []string{IndexLabelEdit, "label-5"}, mut.Index)
	assert.Equal(t, int32(3), mut.Version)
	assert.Equal(t, "VIP Customers", mut.Value.GetLabelEditAction().GetName())
	assert.Equal(t, int32(2), mut.Value.GetLabelEditAction().GetColor())
	assert.False(t, mut.Value.GetLabelEditAction().GetDeleted())
}

func TestBuildSettingPushName(t *testing.T) {
	patch := BuildSettingPushName("PakaiWA Bot")

	assert.Equal(t, WAPatchCriticalBlock, patch.Type)
	require.Len(t, patch.Mutations, 1)
	mut := patch.Mutations[0]
	assert.Equal(t, []string{IndexSettingPushName}, mut.Index)
	assert.Equal(t, int32(1), mut.Version)
	assert.Equal(t, "PakaiWA Bot", mut.Value.GetPushNameSetting().GetName())
}

func TestBuildStar(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")
	senderDiff := types.NewJID("67890", "s.whatsapp.net")

	t.Run("Sender different from target and fromMe=false", func(t *testing.T) {
		patch := BuildStar(target, senderDiff, "msg-star-1", false, true)
		assert.Equal(t, WAPatchRegularHigh, patch.Type)
		require.Len(t, patch.Mutations, 1)
		mut := patch.Mutations[0]
		assert.Equal(t, []string{IndexStar, target.String(), "msg-star-1", "0", senderDiff.String()}, mut.Index)
		assert.Equal(t, int32(2), mut.Version)
		assert.True(t, mut.Value.GetStarAction().GetStarred())
	})

	t.Run("Sender same user as target and fromMe=true", func(t *testing.T) {
		senderSame := types.NewJID("12345", "s.whatsapp.net")
		patch := BuildStar(target, senderSame, "msg-star-2", true, false)
		require.Len(t, patch.Mutations, 1)
		mut := patch.Mutations[0]
		// When target.User == sender.User, senderJID is replaced with "0"
		assert.Equal(t, []string{IndexStar, target.String(), "msg-star-2", "1", "0"}, mut.Index)
		assert.False(t, mut.Value.GetStarAction().GetStarred())
	})
}

func TestBuildDeleteChat(t *testing.T) {
	target := types.NewJID("12345", "s.whatsapp.net")
	ts := time.Unix(1680000000, 0)
	msgKey := &waCommon.MessageKey{ID: proto.String("last-msg")}

	t.Run("Delete chat with media", func(t *testing.T) {
		patch := BuildDeleteChat(target, ts, msgKey, true)
		assert.Equal(t, WAPatchRegularHigh, patch.Type)
		require.Len(t, patch.Mutations, 1)
		mut := patch.Mutations[0]
		assert.Equal(t, []string{IndexDeleteChat, target.String(), "1"}, mut.Index)
		assert.Equal(t, int32(6), mut.Version)
		require.NotNil(t, mut.Value.GetDeleteChatAction().GetMessageRange())
		assert.Equal(t, ts.Unix(), mut.Value.GetDeleteChatAction().GetMessageRange().GetLastMessageTimestamp())
	})

	t.Run("Delete chat without media", func(t *testing.T) {
		patch := BuildDeleteChat(target, time.Time{}, nil, false)
		require.Len(t, patch.Mutations, 1)
		mut := patch.Mutations[0]
		assert.Equal(t, []string{IndexDeleteChat, target.String(), "0"}, mut.Index)
	})
}

func TestProcessor_EncodePatch(t *testing.T) {
	ctx := context.Background()
	proc, keyStore, appStateStore := newTestProcessor()

	keyID := []byte("key-id-encode-test")
	keyData := []byte("32_byte_sync_key_for_patch_encode")

	err := keyStore.PutAppStateSyncKey(ctx, keyID, store.AppStateSyncKey{
		Data:      keyData,
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	target := types.NewJID("98765", "s.whatsapp.net")
	patchInfo := BuildPin(target, true)

	t.Run("Fails when key ID not found", func(t *testing.T) {
		state := HashState{Version: 0}
		_, err := proc.EncodePatch(ctx, []byte("non-existent-key"), state, patchInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get app state key details")
	})

	t.Run("Fails when store returns error on mutation MAC lookup", func(t *testing.T) {
		appStateStore.getMACErr = errors.New("db get mac error")
		defer func() { appStateStore.getMACErr = nil }()

		state := HashState{Version: 0}
		_, err := proc.EncodePatch(ctx, keyID, state, patchInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update state hash")
	})

	t.Run("Successful encoding with zero timestamp auto-filled", func(t *testing.T) {
		patchInfoZeroTime := BuildPin(target, true)
		patchInfoZeroTime.Timestamp = time.Time{}

		state := HashState{Version: 1}
		encoded, err := proc.EncodePatch(ctx, keyID, state, patchInfoZeroTime)
		require.NoError(t, err)
		assert.NotEmpty(t, encoded)

		var syncdPatch waServerSync.SyncdPatch
		err = proto.Unmarshal(encoded, &syncdPatch)
		require.NoError(t, err)

		assert.Equal(t, keyID, syncdPatch.GetKeyID().GetID())
		assert.NotEmpty(t, syncdPatch.GetSnapshotMAC())
		assert.NotEmpty(t, syncdPatch.GetPatchMAC())
		require.Len(t, syncdPatch.GetMutations(), 1)

		mut := syncdPatch.GetMutations()[0]
		assert.Equal(t, waServerSync.SyncdMutation_SET, mut.GetOperation())
		assert.Equal(t, keyID, mut.GetRecord().GetKeyID().GetID())
		assert.NotEmpty(t, mut.GetRecord().GetIndex().GetBlob())
		assert.NotEmpty(t, mut.GetRecord().GetValue().GetBlob())
	})

	t.Run("Successful encoding with multiple mutations and custom timestamp", func(t *testing.T) {
		fixedTime := time.Unix(1710000000, 0)
		archivePatch := BuildArchive(target, true, fixedTime, nil)
		archivePatch.Timestamp = fixedTime

		state := HashState{Version: 2}
		encoded, err := proc.EncodePatch(ctx, keyID, state, archivePatch)
		require.NoError(t, err)

		var syncdPatch waServerSync.SyncdPatch
		err = proto.Unmarshal(encoded, &syncdPatch)
		require.NoError(t, err)

		require.Len(t, syncdPatch.GetMutations(), 2)
	})
}
