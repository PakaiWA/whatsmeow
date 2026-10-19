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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/libsignal/protocol"

	"github.com/PakaiWA/whatsmeow/util/keys"
)

type mockSignalStores struct {
	IdentityStore
	SessionStore
	PreKeyStore
	SenderKeyStore

	// Identity overrides
	putIdentityFunc       func(ctx context.Context, address string, key [32]byte) error
	isTrustedIdentityFunc func(ctx context.Context, address string, key [32]byte) (bool, error)

	// PreKey overrides
	getPreKeyFunc    func(ctx context.Context, id uint32) (*keys.PreKey, error)
	removePreKeyFunc func(ctx context.Context, id uint32) error

	// Session overrides
	getSessionFunc      func(ctx context.Context, address string) ([]byte, error)
	getManySessionsFunc func(ctx context.Context, addresses []string) (map[string][]byte, error)
	putSessionFunc      func(ctx context.Context, address string, session []byte) error
	hasSessionFunc      func(ctx context.Context, address string) (bool, error)

	// SenderKey overrides
	putSenderKeyFunc func(ctx context.Context, group, user string, session []byte) error
	getSenderKeyFunc func(ctx context.Context, group, user string) ([]byte, error)
}

func (m *mockSignalStores) PutIdentity(ctx context.Context, address string, key [32]byte) error {
	if m.putIdentityFunc != nil {
		return m.putIdentityFunc(ctx, address, key)
	}
	return nil
}

func (m *mockSignalStores) IsTrustedIdentity(ctx context.Context, address string, key [32]byte) (bool, error) {
	if m.isTrustedIdentityFunc != nil {
		return m.isTrustedIdentityFunc(ctx, address, key)
	}
	return true, nil
}

func (m *mockSignalStores) GetPreKey(ctx context.Context, id uint32) (*keys.PreKey, error) {
	if m.getPreKeyFunc != nil {
		return m.getPreKeyFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockSignalStores) RemovePreKey(ctx context.Context, id uint32) error {
	if m.removePreKeyFunc != nil {
		return m.removePreKeyFunc(ctx, id)
	}
	return nil
}

func (m *mockSignalStores) GetSession(ctx context.Context, address string) ([]byte, error) {
	if m.getSessionFunc != nil {
		return m.getSessionFunc(ctx, address)
	}
	return nil, nil
}

func (m *mockSignalStores) GetManySessions(ctx context.Context, addresses []string) (map[string][]byte, error) {
	if m.getManySessionsFunc != nil {
		return m.getManySessionsFunc(ctx, addresses)
	}
	return nil, nil
}

func (m *mockSignalStores) PutSession(ctx context.Context, address string, session []byte) error {
	if m.putSessionFunc != nil {
		return m.putSessionFunc(ctx, address, session)
	}
	return nil
}

func (m *mockSignalStores) HasSession(ctx context.Context, address string) (bool, error) {
	if m.hasSessionFunc != nil {
		return m.hasSessionFunc(ctx, address)
	}
	return false, nil
}

func (m *mockSignalStores) PutSenderKey(ctx context.Context, group, user string, session []byte) error {
	if m.putSenderKeyFunc != nil {
		return m.putSenderKeyFunc(ctx, group, user, session)
	}
	return nil
}

func (m *mockSignalStores) GetSenderKey(ctx context.Context, group, user string) ([]byte, error) {
	if m.getSenderKeyFunc != nil {
		return m.getSenderKeyFunc(ctx, group, user)
	}
	return nil, nil
}

func TestDevice_SignalIdentityAndRegistration(t *testing.T) {
	idKey := keys.NewKeyPair()
	device := &Device{
		IdentityKey:    idKey,
		RegistrationID: 998877,
	}

	assert.Equal(t, uint32(998877), device.GetLocalRegistrationID())

	kp := device.GetIdentityKeyPair()
	require.NotNil(t, kp)
	pub := kp.PublicKey().PublicKey().PublicKey()
	assert.Equal(t, idKey.Pub[:], pub[:])

	ctx := context.Background()
	addr := protocol.NewSignalAddress("test_user", 1)
	otherKey := kp.PublicKey()

	mock := &mockSignalStores{}
	device.Identities = mock

	// SaveIdentity success and error
	err := device.SaveIdentity(ctx, addr, otherKey)
	require.NoError(t, err)

	saveErr := errors.New("save id error")
	mock.putIdentityFunc = func(_ context.Context, _ string, _ [32]byte) error {
		return saveErr
	}
	err = device.SaveIdentity(ctx, addr, otherKey)
	assert.ErrorIs(t, err, saveErr)

	// IsTrustedIdentity success and error
	mock.isTrustedIdentityFunc = func(_ context.Context, _ string, _ [32]byte) (bool, error) {
		return true, nil
	}
	trusted, err := device.IsTrustedIdentity(ctx, addr, otherKey)
	require.NoError(t, err)
	assert.True(t, trusted)

	trustErr := errors.New("trust error")
	mock.isTrustedIdentityFunc = func(_ context.Context, _ string, _ [32]byte) (bool, error) {
		return false, trustErr
	}
	trusted, err = device.IsTrustedIdentity(ctx, addr, otherKey)
	assert.False(t, trusted)
	assert.ErrorIs(t, err, trustErr)
}

func TestDevice_SignalPreKeys(t *testing.T) {
	device := &Device{}
	mock := &mockSignalStores{}
	device.PreKeys = mock
	ctx := context.Background()

	// LoadPreKey nil
	pk, err := device.LoadPreKey(ctx, 1)
	require.NoError(t, err)
	assert.Nil(t, pk)

	// LoadPreKey found
	genKey := keys.NewPreKey(1)
	mock.getPreKeyFunc = func(_ context.Context, id uint32) (*keys.PreKey, error) {
		return genKey, nil
	}
	pk, err = device.LoadPreKey(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, pk)
	assert.Equal(t, uint32(1), pk.ID().Value)

	// LoadPreKey error
	pkErr := errors.New("prekey error")
	mock.getPreKeyFunc = func(_ context.Context, id uint32) (*keys.PreKey, error) {
		return nil, pkErr
	}
	_, err = device.LoadPreKey(ctx, 1)
	assert.ErrorIs(t, err, pkErr)

	// RemovePreKey success & error
	mock.removePreKeyFunc = func(_ context.Context, id uint32) error {
		return nil
	}
	assert.NoError(t, device.RemovePreKey(ctx, 1))

	mock.removePreKeyFunc = func(_ context.Context, id uint32) error {
		return pkErr
	}
	assert.ErrorIs(t, device.RemovePreKey(ctx, 1), pkErr)

	// Panics
	assert.Panics(t, func() { _ = device.StorePreKey(ctx, 1, nil) })
	assert.Panics(t, func() { _, _ = device.ContainsPreKey(ctx, 1) })
}

func TestDevice_SignalSignedPreKeys(t *testing.T) {
	idKey := keys.NewKeyPair()
	signedPreKey := idKey.CreateSignedPreKey(42)
	device := &Device{
		IdentityKey:  idKey,
		SignedPreKey: signedPreKey,
	}
	ctx := context.Background()

	// LoadSignedPreKey matching
	spk, err := device.LoadSignedPreKey(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, spk)
	assert.Equal(t, uint32(42), spk.ID())

	// LoadSignedPreKey non-matching
	spk, err = device.LoadSignedPreKey(ctx, 99)
	require.NoError(t, err)
	assert.Nil(t, spk)

	// Panics
	assert.Panics(t, func() { _, _ = device.LoadSignedPreKeys(ctx) })
	assert.Panics(t, func() { _ = device.StoreSignedPreKey(ctx, 1, nil) })
	assert.Panics(t, func() { _, _ = device.ContainsSignedPreKey(ctx, 1) })
	assert.Panics(t, func() { _ = device.RemoveSignedPreKey(ctx, 1) })
}

func TestDevice_SignalSessions(t *testing.T) {
	device := &Device{}
	mock := &mockSignalStores{}
	device.Sessions = mock
	ctx := context.Background()
	addr := protocol.NewSignalAddress("addr1", 1)

	// LoadSession when DB returns nil returns new session
	sess, err := device.LoadSession(ctx, addr)
	require.NoError(t, err)
	require.NotNil(t, sess)

	// LoadSession when DB returns bytes
	newSess := createRealSession()
	mock.getSessionFunc = func(_ context.Context, address string) ([]byte, error) {
		return newSess.Serialize(), nil
	}
	sess, err = device.LoadSession(ctx, addr)
	require.NoError(t, err)
	require.NotNil(t, sess)

	// LoadSession DB error
	sessErr := errors.New("sess err")
	mock.getSessionFunc = func(_ context.Context, address string) ([]byte, error) {
		return nil, sessErr
	}
	_, err = device.LoadSession(ctx, addr)
	assert.ErrorIs(t, err, sessErr)

	// LoadSession deserialize error
	mock.getSessionFunc = func(_ context.Context, address string) ([]byte, error) {
		return []byte("corrupt data"), nil
	}
	_, err = device.LoadSession(ctx, addr)
	assert.Error(t, err)

	// StoreSession direct to DB
	stored := false
	mock.putSessionFunc = func(_ context.Context, address string, session []byte) error {
		stored = true
		return nil
	}
	err = device.StoreSession(ctx, addr, newSess)
	require.NoError(t, err)
	assert.True(t, stored)

	// StoreSession DB error
	mock.putSessionFunc = func(_ context.Context, address string, session []byte) error {
		return sessErr
	}
	err = device.StoreSession(ctx, addr, newSess)
	assert.ErrorIs(t, err, sessErr)

	// StoreSession and LoadSession using context cache
	mock.getManySessionsFunc = func(_ context.Context, addresses []string) (map[string][]byte, error) {
		return map[string][]byte{addr.String(): newSess.Serialize()}, nil
	}
	_, cacheCtx, err := device.WithCachedSessions(ctx, []string{addr.String()})
	require.NoError(t, err)

	// LoadSession hits cache
	cachedSess, err := device.LoadSession(cacheCtx, addr)
	require.NoError(t, err)
	require.NotNil(t, cachedSess)

	// StoreSession hits cache
	err = device.StoreSession(cacheCtx, addr, cachedSess)
	require.NoError(t, err)

	// ContainsSession success and error
	mock.hasSessionFunc = func(_ context.Context, address string) (bool, error) {
		return true, nil
	}
	has, err := device.ContainsSession(ctx, addr)
	require.NoError(t, err)
	assert.True(t, has)

	mock.hasSessionFunc = func(_ context.Context, address string) (bool, error) {
		return false, sessErr
	}
	has, err = device.ContainsSession(ctx, addr)
	assert.False(t, has)
	assert.ErrorIs(t, err, sessErr)

	// Panics
	assert.Panics(t, func() { _, _ = device.GetSubDeviceSessions(ctx, "test") })
	assert.Panics(t, func() { _ = device.DeleteSession(ctx, addr) })
	assert.Panics(t, func() { _ = device.DeleteAllSessions(ctx) })
}

func TestDevice_SignalSenderKeys(t *testing.T) {
	device := &Device{}
	mock := &mockSignalStores{}
	device.SenderKeys = mock
	ctx := context.Background()

	senderKeyName := protocol.NewSenderKeyName("group123", protocol.NewSignalAddress("user123", 1))

	// LoadSenderKey when DB returns nil returns empty sender key
	sk, err := device.LoadSenderKey(ctx, senderKeyName)
	require.NoError(t, err)
	require.NotNil(t, sk)

	// StoreSenderKey success and error
	stored := false
	mock.putSenderKeyFunc = func(_ context.Context, group, user string, session []byte) error {
		stored = true
		return nil
	}
	err = device.StoreSenderKey(ctx, senderKeyName, sk)
	require.NoError(t, err)
	assert.True(t, stored)

	skErr := errors.New("sk err")
	mock.putSenderKeyFunc = func(_ context.Context, group, user string, session []byte) error {
		return skErr
	}
	err = device.StoreSenderKey(ctx, senderKeyName, sk)
	assert.ErrorIs(t, err, skErr)

	// LoadSenderKey when DB returns valid bytes
	mock.getSenderKeyFunc = func(_ context.Context, group, user string) ([]byte, error) {
		return sk.Serialize(), nil
	}
	loadedSK, err := device.LoadSenderKey(ctx, senderKeyName)
	require.NoError(t, err)
	require.NotNil(t, loadedSK)

	// LoadSenderKey error
	mock.getSenderKeyFunc = func(_ context.Context, group, user string) ([]byte, error) {
		return nil, skErr
	}
	_, err = device.LoadSenderKey(ctx, senderKeyName)
	assert.ErrorIs(t, err, skErr)

	// LoadSenderKey deserialization error
	mock.getSenderKeyFunc = func(_ context.Context, group, user string) ([]byte, error) {
		return []byte("corrupt sender key"), nil
	}
	_, err = device.LoadSenderKey(ctx, senderKeyName)
	assert.Error(t, err)
}
