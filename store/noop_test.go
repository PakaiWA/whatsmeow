// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/types"
)

func TestNoopStore(t *testing.T) {
	testErr := errors.New("test noop error")
	s := &NoopStore{Error: testErr}
	ctx := context.Background()

	// IdentityStore
	assert.ErrorIs(t, s.PutIdentity(ctx, "addr", [32]byte{}), testErr)
	assert.ErrorIs(t, s.DeleteAllIdentities(ctx, "phone"), testErr)
	assert.ErrorIs(t, s.DeleteIdentity(ctx, "addr"), testErr)
	trusted, err := s.IsTrustedIdentity(ctx, "addr", [32]byte{})
	assert.False(t, trusted)
	assert.ErrorIs(t, err, testErr)

	// SessionStore
	sessBytes, err := s.GetSession(ctx, "addr")
	assert.Nil(t, sessBytes)
	assert.ErrorIs(t, err, testErr)

	hasSess, err := s.HasSession(ctx, "addr")
	assert.False(t, hasSess)
	assert.ErrorIs(t, err, testErr)

	manySess, err := s.GetManySessions(ctx, []string{"addr"})
	assert.Nil(t, manySess)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.PutSession(ctx, "addr", []byte("sess")), testErr)
	assert.ErrorIs(t, s.PutManySessions(ctx, map[string][]byte{"addr": []byte("sess")}), testErr)
	assert.ErrorIs(t, s.DeleteAllSessions(ctx, "phone"), testErr)
	assert.ErrorIs(t, s.DeleteSession(ctx, "addr"), testErr)
	assert.ErrorIs(t, s.MigratePNToLID(ctx, types.EmptyJID, types.EmptyJID), testErr)

	// PreKeyStore
	preKeys, err := s.GetOrGenPreKeys(ctx, 10)
	assert.Nil(t, preKeys)
	assert.ErrorIs(t, err, testErr)

	onePreKey, err := s.GenOnePreKey(ctx)
	assert.Nil(t, onePreKey)
	assert.ErrorIs(t, err, testErr)

	getPreKey, err := s.GetPreKey(ctx, 1)
	assert.Nil(t, getPreKey)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.RemovePreKey(ctx, 1), testErr)
	assert.ErrorIs(t, s.MarkPreKeysAsUploaded(ctx, 5), testErr)
	count, err := s.UploadedPreKeyCount(ctx)
	assert.Equal(t, 0, count)
	assert.ErrorIs(t, err, testErr)

	// SenderKeyStore
	assert.ErrorIs(t, s.PutSenderKey(ctx, "group", "user", []byte("key")), testErr)
	senderKey, err := s.GetSenderKey(ctx, "group", "user")
	assert.Nil(t, senderKey)
	assert.ErrorIs(t, err, testErr)

	// AppStateSyncKeyStore
	assert.ErrorIs(t, s.PutAppStateSyncKey(ctx, []byte("id"), AppStateSyncKey{}), testErr)
	appSyncKey, err := s.GetAppStateSyncKey(ctx, []byte("id"))
	assert.Nil(t, appSyncKey)
	assert.ErrorIs(t, err, testErr)

	latestKeyID, err := s.GetLatestAppStateSyncKeyID(ctx)
	assert.Nil(t, latestKeyID)
	assert.ErrorIs(t, err, testErr)

	allSyncKeys, err := s.GetAllAppStateSyncKeys(ctx)
	assert.Nil(t, allSyncKeys)
	assert.NoError(t, err)

	// AppStateStore
	assert.ErrorIs(t, s.PutAppStateVersion(ctx, "name", 1, [128]byte{}), testErr)
	v, h, err := s.GetAppStateVersion(ctx, "name")
	assert.Equal(t, uint64(0), v)
	assert.Equal(t, [128]byte{}, h)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.DeleteAppStateVersion(ctx, "name"), testErr)
	assert.ErrorIs(t, s.PutAppStateMutationMACs(ctx, "name", 1, nil), testErr)
	assert.ErrorIs(t, s.DeleteAppStateMutationMACs(ctx, "name", nil), testErr)
	macVal, err := s.GetAppStateMutationMAC(ctx, "name", []byte("idx"))
	assert.Nil(t, macVal)
	assert.ErrorIs(t, err, testErr)

	// ContactStore
	inserted, oldName, err := s.PutPushName(ctx, types.EmptyJID, "push")
	assert.False(t, inserted)
	assert.Empty(t, oldName)
	assert.ErrorIs(t, err, testErr)

	inserted, oldBiz, err := s.PutBusinessName(ctx, types.EmptyJID, "biz")
	assert.False(t, inserted)
	assert.Empty(t, oldBiz)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.PutContactName(ctx, types.EmptyJID, "full", "first"), testErr)
	assert.ErrorIs(t, s.PutAllContactNames(ctx, nil), testErr)
	assert.ErrorIs(t, s.PutManyRedactedPhones(ctx, nil), testErr)

	contactInfo, err := s.GetContact(ctx, types.EmptyJID)
	assert.Equal(t, types.ContactInfo{}, contactInfo)
	assert.ErrorIs(t, err, testErr)

	contactsMap, err := s.GetAllContacts(ctx)
	assert.Nil(t, contactsMap)
	assert.ErrorIs(t, err, testErr)

	// ChatSettingsStore
	now := time.Now()
	assert.ErrorIs(t, s.PutMutedUntil(ctx, types.EmptyJID, now), testErr)
	assert.ErrorIs(t, s.PutPinned(ctx, types.EmptyJID, true), testErr)
	assert.ErrorIs(t, s.PutArchived(ctx, types.EmptyJID, true), testErr)
	chatSettings, err := s.GetChatSettings(ctx, types.EmptyJID)
	assert.Equal(t, types.LocalChatSettings{}, chatSettings)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.PutWASARootSecretID(ctx, types.EmptyJID, "id"), testErr)
	secretID, err := s.GetWASARootSecretID(ctx, types.EmptyJID)
	assert.Empty(t, secretID)
	assert.ErrorIs(t, err, testErr)

	// MsgSecretStore
	assert.ErrorIs(t, s.PutMessageSecrets(ctx, nil), testErr)
	assert.ErrorIs(t, s.PutMessageSecret(ctx, types.EmptyJID, types.EmptyJID, "id", []byte("sec")), testErr)
	secBytes, senderJID, err := s.GetMessageSecret(ctx, types.EmptyJID, types.EmptyJID, "id")
	assert.Nil(t, secBytes)
	assert.Equal(t, types.EmptyJID, senderJID)
	assert.ErrorIs(t, err, testErr)

	// PrivacyTokenStore & NCTSaltStore
	assert.ErrorIs(t, s.PutPrivacyTokens(ctx, PrivacyToken{}), testErr)
	token, err := s.GetPrivacyToken(ctx, types.EmptyJID)
	assert.Nil(t, token)
	assert.ErrorIs(t, err, testErr)

	delCount, err := s.DeleteExpiredPrivacyTokens(ctx, now)
	assert.Equal(t, int64(0), delCount)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.PutNCTSalt(ctx, []byte("salt")), testErr)
	salt, err := s.GetNCTSalt(ctx)
	assert.Nil(t, salt)
	assert.ErrorIs(t, err, testErr)
	assert.ErrorIs(t, s.DeleteNCTSalt(ctx), testErr)

	// DeviceContainer
	assert.ErrorIs(t, s.PutDevice(ctx, nil), testErr)
	assert.ErrorIs(t, s.DeleteDevice(ctx, nil), testErr)

	// EventBuffer
	evt, err := s.GetBufferedEvent(ctx, [32]byte{})
	assert.Nil(t, evt)
	assert.NoError(t, err)

	assert.NoError(t, s.PutBufferedEvent(ctx, [32]byte{}, []byte("plain"), now))
	txnErr := s.DoDecryptionTxn(ctx, func(txnCtx context.Context) error {
		return nil
	})
	assert.NoError(t, txnErr)

	assert.NoError(t, s.ClearBufferedEventPlaintext(ctx, [32]byte{}))
	assert.NoError(t, s.DeleteOldBufferedHashes(ctx))
	assert.NoError(t, s.DeleteOldOutgoingEvents(ctx))

	outFormat, outBytes, err := s.GetOutgoingEvent(ctx, types.EmptyJID, types.EmptyJID, "id")
	assert.Empty(t, outFormat)
	assert.Nil(t, outBytes)
	assert.NoError(t, err)

	assert.NoError(t, s.AddOutgoingEvent(ctx, types.EmptyJID, "id", "format", []byte("plain")))

	// LIDStore
	lid, err := s.GetLIDForPN(ctx, types.EmptyJID)
	assert.Equal(t, types.JID{}, lid)
	assert.ErrorIs(t, err, testErr)

	manyLIDs, err := s.GetManyLIDsForPNs(ctx, []types.JID{types.EmptyJID})
	assert.Nil(t, manyLIDs)
	assert.ErrorIs(t, err, testErr)

	pn, err := s.GetPNForLID(ctx, types.EmptyJID)
	assert.Equal(t, types.JID{}, pn)
	assert.ErrorIs(t, err, testErr)

	assert.ErrorIs(t, s.PutManyLIDMappings(ctx, nil), testErr)
	assert.ErrorIs(t, s.PutLIDMapping(ctx, types.EmptyJID, types.EmptyJID), testErr)

	// NoopDevice global var
	require.NotNil(t, NoopDevice)
	assert.Equal(t, &types.EmptyJID, NoopDevice.ID)
}
