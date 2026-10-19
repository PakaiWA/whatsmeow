// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/proto/waAdv"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
)

func newTestSQLStore(t *testing.T) (*SQLStore, *Container) {
	container := newTestContainer(t)
	jid := types.NewJID("628123456789", types.DefaultUserServer)
	dev := container.NewDevice()
	dev.ID = &jid
	dev.Account = &waAdv.ADVSignedDeviceIdentity{
		Details:             []byte("details"),
		AccountSignature:    make([]byte, 64),
		AccountSignatureKey: make([]byte, 32),
		DeviceSignature:     make([]byte, 64),
	}
	err := container.PutDevice(context.Background(), dev)
	require.NoError(t, err)

	sqlStore := NewSQLStore(container, jid)
	return sqlStore, container
}

func TestSQLStore_IdentityStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()
	addr := "628999999999.0:1"
	var key1, key2 [32]byte
	key1[0] = 1
	key2[0] = 2

	// Initially trusted (first use)
	trusted, err := s.IsTrustedIdentity(ctx, addr, key1)
	require.NoError(t, err)
	assert.True(t, trusted)

	// Save identity
	err = s.PutIdentity(ctx, addr, key1)
	require.NoError(t, err)

	// Check same key is trusted
	trusted, err = s.IsTrustedIdentity(ctx, addr, key1)
	require.NoError(t, err)
	assert.True(t, trusted)

	// Different key is NOT trusted
	trusted, err = s.IsTrustedIdentity(ctx, addr, key2)
	require.NoError(t, err)
	assert.False(t, trusted)

	// Overwrite identity (e.g. key change confirmed)
	err = s.PutIdentity(ctx, addr, key2)
	require.NoError(t, err)
	trusted, err = s.IsTrustedIdentity(ctx, addr, key2)
	require.NoError(t, err)
	assert.True(t, trusted)

	// DeleteIdentity
	err = s.DeleteIdentity(ctx, addr)
	require.NoError(t, err)

	// DeleteAllIdentities
	err = s.PutIdentity(ctx, "628888888888.0:1", key1)
	require.NoError(t, err)
	err = s.DeleteAllIdentities(ctx, "628888888888")
	require.NoError(t, err)
}

func TestSQLStore_SessionStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()
	addr1 := "628111111111:1"
	addr2 := "628222222222:1"

	// HasSession on empty
	has, err := s.HasSession(ctx, addr1)
	require.NoError(t, err)
	assert.False(t, has)

	// GetSession on empty
	sess, err := s.GetSession(ctx, addr1)
	require.NoError(t, err)
	assert.Nil(t, sess)

	// PutSession & GetSession
	data1 := []byte("session-data-1")
	err = s.PutSession(ctx, addr1, data1)
	require.NoError(t, err)

	has, err = s.HasSession(ctx, addr1)
	require.NoError(t, err)
	assert.True(t, has)

	sess, err = s.GetSession(ctx, addr1)
	require.NoError(t, err)
	assert.Equal(t, data1, sess)

	// PutManySessions & GetManySessions
	data2 := []byte("session-data-2")
	err = s.PutManySessions(ctx, map[string][]byte{
		addr1: data1,
		addr2: data2,
	})
	require.NoError(t, err)

	many, err := s.GetManySessions(ctx, []string{addr1, addr2, "unknown:1"})
	require.NoError(t, err)
	assert.Equal(t, data1, many[addr1])
	assert.Equal(t, data2, many[addr2])
	assert.Nil(t, many["unknown:1"])

	// Empty GetManySessions
	emptyMany, err := s.GetManySessions(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyMany)

	// MigratePNToLID
	pnJID := types.NewJID("628111111111", types.DefaultUserServer)
	lidJID := types.NewJID("10001", types.HiddenUserServer)
	err = s.MigratePNToLID(ctx, pnJID, lidJID)
	require.NoError(t, err)

	// Calling MigratePNToLID again hits migrated cache
	err = s.MigratePNToLID(ctx, pnJID, lidJID)
	require.NoError(t, err)

	// DeleteSession
	err = s.DeleteSession(ctx, addr2)
	require.NoError(t, err)
	has, err = s.HasSession(ctx, addr2)
	require.NoError(t, err)
	assert.False(t, has)

	// DeleteAllSessions
	err = s.DeleteAllSessions(ctx, "628111111111")
	require.NoError(t, err)
}

func TestSQLStore_PreKeyStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	// Initially 0 prekeys
	count, err := s.UploadedPreKeyCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// GetOrGenPreKeys generates count
	pks, err := s.GetOrGenPreKeys(ctx, 5)
	require.NoError(t, err)
	assert.Len(t, pks, 5)

	// UploadedPreKeyCount is 0 before MarkPreKeysAsUploaded
	count, err = s.UploadedPreKeyCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Mark uploaded up to ID
	err = s.MarkPreKeysAsUploaded(ctx, pks[2].KeyID)
	require.NoError(t, err)

	count, err = s.UploadedPreKeyCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// GenOnePreKey
	one, err := s.GenOnePreKey(ctx)
	require.NoError(t, err)
	require.NotNil(t, one)

	// GetPreKey found & not found
	pk, err := s.GetPreKey(ctx, one.KeyID)
	require.NoError(t, err)
	require.NotNil(t, pk)
	assert.Equal(t, one.KeyID, pk.KeyID)

	missing, err := s.GetPreKey(ctx, 999999)
	require.NoError(t, err)
	assert.Nil(t, missing)

	// RemovePreKey
	err = s.RemovePreKey(ctx, one.KeyID)
	require.NoError(t, err)
	missing, err = s.GetPreKey(ctx, one.KeyID)
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestSQLStore_SenderKeyStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	group := "12345-67890@g.us"
	user := "628111111111.0:1"
	data := []byte("sender-key-payload")

	// Get on empty returns nil
	got, err := s.GetSenderKey(ctx, group, user)
	require.NoError(t, err)
	assert.Nil(t, got)

	// Put & Get
	err = s.PutSenderKey(ctx, group, user, data)
	require.NoError(t, err)

	got, err = s.GetSenderKey(ctx, group, user)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestSQLStore_AppStateSyncKeyStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	keyID := []byte("key-id-1")
	keyData := store.AppStateSyncKey{
		Data:        []byte("sync-key-data"),
		Fingerprint: []byte("sync-key-fingerprint"),
		Timestamp:   time.Now().Unix(),
	}

	// Get on empty returns nil
	got, err := s.GetAppStateSyncKey(ctx, keyID)
	require.NoError(t, err)
	assert.Nil(t, got)

	// PutAppStateSyncKey & GetAppStateSyncKey
	err = s.PutAppStateSyncKey(ctx, keyID, keyData)
	require.NoError(t, err)

	got, err = s.GetAppStateSyncKey(ctx, keyID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, keyData.Data, got.Data)
	assert.Equal(t, keyData.Fingerprint, got.Fingerprint)
	assert.Equal(t, keyData.Timestamp, got.Timestamp)

	// GetLatestAppStateSyncKeyID
	latest, err := s.GetLatestAppStateSyncKeyID(ctx)
	require.NoError(t, err)
	assert.Equal(t, keyID, latest)

	// GetAllAppStateSyncKeys
	all, err := s.GetAllAppStateSyncKeys(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestSQLStore_AppStateStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	name := "critical_unblock_low"
	var hash [128]byte
	hash[0] = 0xab

	// Get on empty returns 0 version
	ver, gotHash, err := s.GetAppStateVersion(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), ver)
	assert.Equal(t, [128]byte{}, gotHash)

	// PutAppStateVersion & GetAppStateVersion
	err = s.PutAppStateVersion(ctx, name, 5, hash)
	require.NoError(t, err)

	ver, gotHash, err = s.GetAppStateVersion(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, uint64(5), ver)
	assert.Equal(t, hash, gotHash)

	// Mutation MACs (must be 32 bytes)
	idx1 := make([]byte, 32)
	copy(idx1, "idx1")
	val1 := make([]byte, 32)
	copy(val1, "val1")

	idx2 := make([]byte, 32)
	copy(idx2, "idx2")
	val2 := make([]byte, 32)
	copy(val2, "val2")

	muts := []store.AppStateMutationMAC{
		{IndexMAC: idx1, ValueMAC: val1},
		{IndexMAC: idx2, ValueMAC: val2},
	}
	err = s.PutAppStateMutationMACs(ctx, name, 5, muts)
	require.NoError(t, err)

	valMAC, err := s.GetAppStateMutationMAC(ctx, name, idx1)
	require.NoError(t, err)
	assert.Equal(t, val1, valMAC)

	unknownIdx := make([]byte, 32)
	copy(unknownIdx, "unknown")
	missingMAC, err := s.GetAppStateMutationMAC(ctx, name, unknownIdx)
	require.NoError(t, err)
	assert.Nil(t, missingMAC)

	// DeleteAppStateMutationMACs
	err = s.DeleteAppStateMutationMACs(ctx, name, [][]byte{idx1})
	require.NoError(t, err)
	deletedMAC, err := s.GetAppStateMutationMAC(ctx, name, idx1)
	require.NoError(t, err)
	assert.Nil(t, deletedMAC)

	// DeleteAppStateVersion
	err = s.DeleteAppStateVersion(ctx, name)
	require.NoError(t, err)
	ver, _, err = s.GetAppStateVersion(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), ver)
}

func TestSQLStore_ContactStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	userJID := types.NewJID("628111111111", types.DefaultUserServer)

	// PutPushName
	changed, old, err := s.PutPushName(ctx, userJID, "First Push")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Empty(t, old)

	changed, old, err = s.PutPushName(ctx, userJID, "Second Push")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "First Push", old)

	// PutBusinessName
	changed, old, err = s.PutBusinessName(ctx, userJID, "Biz Corp")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Empty(t, old)

	// PutContactName
	err = s.PutContactName(ctx, userJID, "First", "Full Name")
	require.NoError(t, err)

	// GetContact
	info, err := s.GetContact(ctx, userJID)
	require.NoError(t, err)
	assert.True(t, info.Found)
	assert.Equal(t, "Second Push", info.PushName)
	assert.Equal(t, "Biz Corp", info.BusinessName)
	assert.Equal(t, "Full Name", info.FullName)
	assert.Equal(t, "First", info.FirstName)

	// PutAllContactNames
	user2 := types.NewJID("628222222222", types.DefaultUserServer)
	contacts := []store.ContactEntry{
		{JID: user2, FullName: "Alice Wonderland", FirstName: "Alice"},
	}
	err = s.PutAllContactNames(ctx, contacts)
	require.NoError(t, err)

	all, err := s.GetAllContacts(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 2)
	assert.Equal(t, "Alice Wonderland", all[user2].FullName)

	// PutManyRedactedPhones
	redacted := []store.RedactedPhoneEntry{
		{JID: user2, RedactedPhone: "+62822***"},
	}
	err = s.PutManyRedactedPhones(ctx, redacted)
	require.NoError(t, err)

	info2, err := s.GetContact(ctx, user2)
	require.NoError(t, err)
	assert.Equal(t, "+62822***", info2.RedactedPhone)
}

func TestSQLStore_ChatSettingsStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	chatJID := types.NewJID("12345-67890", types.GroupServer)

	// Get on empty
	settings, err := s.GetChatSettings(ctx, chatJID)
	require.NoError(t, err)
	assert.False(t, settings.Found)

	// PutMutedUntil
	now := time.Now().Truncate(time.Second)
	err = s.PutMutedUntil(ctx, chatJID, now)
	require.NoError(t, err)

	// PutPinned
	err = s.PutPinned(ctx, chatJID, true)
	require.NoError(t, err)

	// PutArchived
	err = s.PutArchived(ctx, chatJID, true)
	require.NoError(t, err)

	settings, err = s.GetChatSettings(ctx, chatJID)
	require.NoError(t, err)
	assert.True(t, settings.Found)
	assert.True(t, settings.Pinned)
	assert.True(t, settings.Archived)

	// WASARootSecretID
	err = s.PutWASARootSecretID(ctx, chatJID, "secret-msg-id-123")
	require.NoError(t, err)

	secID, err := s.GetWASARootSecretID(ctx, chatJID)
	require.NoError(t, err)
	assert.Equal(t, types.MessageID("secret-msg-id-123"), secID)
}

func TestSQLStore_MsgSecretStore(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	chat := types.NewJID("12345", types.GroupServer)
	sender := types.NewJID("628111111111", types.DefaultUserServer)
	id := types.MessageID("MSG123")
	secret := []byte("secret32byteslong................")

	// Get on empty
	sec, senderOut, err := s.GetMessageSecret(ctx, chat, sender, id)
	require.NoError(t, err)
	assert.Nil(t, sec)
	assert.Equal(t, types.EmptyJID, senderOut)

	// PutMessageSecret & GetMessageSecret
	err = s.PutMessageSecret(ctx, chat, sender, id, secret)
	require.NoError(t, err)

	sec, senderOut, err = s.GetMessageSecret(ctx, chat, sender, id)
	require.NoError(t, err)
	assert.Equal(t, secret, sec)
	assert.Equal(t, sender, senderOut)

	// PutMessageSecrets batch
	id2 := types.MessageID("MSG456")
	inserts := []store.MessageSecretInsert{
		{Chat: chat, Sender: sender, ID: id2, Secret: secret},
	}
	err = s.PutMessageSecrets(ctx, inserts)
	require.NoError(t, err)

	sec2, _, err := s.GetMessageSecret(ctx, chat, sender, id2)
	require.NoError(t, err)
	assert.Equal(t, secret, sec2)
}

func TestSQLStore_PrivacyTokenStore_And_NCTSalt(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	user := types.NewJID("628111111111", types.DefaultUserServer)
	now := time.Now().Truncate(time.Second)

	// GetPrivacyToken empty
	token, err := s.GetPrivacyToken(ctx, user)
	require.NoError(t, err)
	assert.Nil(t, token)

	// PutPrivacyTokens & GetPrivacyToken
	pToken := store.PrivacyToken{
		User:            user,
		Token:           []byte("privacy-token-data"),
		Timestamp:       now,
		SenderTimestamp: now,
	}
	err = s.PutPrivacyTokens(ctx, pToken)
	require.NoError(t, err)

	token, err = s.GetPrivacyToken(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, pToken.Token, token.Token)

	// DeleteExpiredPrivacyTokens
	deleted, err := s.DeleteExpiredPrivacyTokens(ctx, now.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// NCTSaltStore
	salt, err := s.GetNCTSalt(ctx)
	require.NoError(t, err)
	assert.Nil(t, salt)

	saltData := []byte("nct-salt-32-bytes...............")
	err = s.PutNCTSalt(ctx, saltData)
	require.NoError(t, err)

	salt, err = s.GetNCTSalt(ctx)
	require.NoError(t, err)
	assert.Equal(t, saltData, salt)

	err = s.DeleteNCTSalt(ctx)
	require.NoError(t, err)
	salt, err = s.GetNCTSalt(ctx)
	require.NoError(t, err)
	assert.Nil(t, salt)
}

func TestSQLStore_EventBuffer_And_OutgoingEvents(t *testing.T) {
	s, _ := newTestSQLStore(t)
	ctx := context.Background()

	var cipherHash [32]byte
	cipherHash[0] = 0x42
	plaintext := []byte("event plaintext payload")
	serverTime := time.Now().Truncate(time.Second)

	// Get on empty
	evt, err := s.GetBufferedEvent(ctx, cipherHash)
	require.NoError(t, err)
	assert.Nil(t, evt)

	// PutBufferedEvent & GetBufferedEvent
	err = s.PutBufferedEvent(ctx, cipherHash, plaintext, serverTime)
	require.NoError(t, err)

	evt, err = s.GetBufferedEvent(ctx, cipherHash)
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, plaintext, evt.Plaintext)

	// ClearBufferedEventPlaintext
	err = s.ClearBufferedEventPlaintext(ctx, cipherHash)
	require.NoError(t, err)

	evt, err = s.GetBufferedEvent(ctx, cipherHash)
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Nil(t, evt.Plaintext)

	// DoDecryptionTxn
	err = s.DoDecryptionTxn(ctx, func(txnCtx context.Context) error {
		return s.DeleteOldBufferedHashes(txnCtx)
	})
	require.NoError(t, err)

	// OutgoingEvents
	chat := types.NewJID("12345", types.GroupServer)
	altChat := types.NewJID("12345-alt", types.GroupServer)
	msgID := types.MessageID("OUT123")

	format, outData, err := s.GetOutgoingEvent(ctx, chat, altChat, msgID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Empty(t, format)
	assert.Nil(t, outData)

	err = s.AddOutgoingEvent(ctx, chat, msgID, "proto", []byte("outgoing-event-payload"))
	require.NoError(t, err)

	format, outData, err = s.GetOutgoingEvent(ctx, chat, altChat, msgID)
	require.NoError(t, err)
	assert.Equal(t, "proto", format)
	assert.Equal(t, []byte("outgoing-event-payload"), outData)

	err = s.DeleteOldOutgoingEvents(ctx)
	require.NoError(t, err)
}
