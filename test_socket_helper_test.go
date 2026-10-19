// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"go.mau.fi/libsignal/keys/prekey"
	"go.mau.fi/libsignal/session"
	"go.mau.fi/libsignal/util/keyhelper"

	"github.com/PakaiWA/whatsmeow/socket"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockSignalStores struct {
	sessionsLock sync.RWMutex
	sessions     map[string][]byte

	identitiesLock sync.RWMutex
	identities     map[string][32]byte
}

func newMockSignalStores() *mockSignalStores {
	return &mockSignalStores{
		sessions:   make(map[string][]byte),
		identities: make(map[string][32]byte),
	}
}

func (m *mockSignalStores) PutSession(_ context.Context, address string, session []byte) error {
	m.sessionsLock.Lock()
	defer m.sessionsLock.Unlock()
	m.sessions[address] = session
	return nil
}

func (m *mockSignalStores) GetSession(_ context.Context, address string) ([]byte, error) {
	m.sessionsLock.RLock()
	defer m.sessionsLock.RUnlock()
	return m.sessions[address], nil
}

func (m *mockSignalStores) HasSession(_ context.Context, address string) (bool, error) {
	m.sessionsLock.RLock()
	defer m.sessionsLock.RUnlock()
	_, ok := m.sessions[address]
	return ok, nil
}

func (m *mockSignalStores) GetManySessions(_ context.Context, addresses []string) (map[string][]byte, error) {
	m.sessionsLock.RLock()
	defer m.sessionsLock.RUnlock()
	res := make(map[string][]byte, len(addresses))
	for _, addr := range addresses {
		if val, ok := m.sessions[addr]; ok {
			res[addr] = val
		}
	}
	return res, nil
}

func (m *mockSignalStores) PutManySessions(_ context.Context, sessions map[string][]byte) error {
	m.sessionsLock.Lock()
	defer m.sessionsLock.Unlock()
	for k, v := range sessions {
		m.sessions[k] = v
	}
	return nil
}

func (m *mockSignalStores) DeleteAllSessions(_ context.Context, _ string) error {
	m.sessionsLock.Lock()
	defer m.sessionsLock.Unlock()
	clear(m.sessions)
	return nil
}

func (m *mockSignalStores) DeleteSession(_ context.Context, address string) error {
	m.sessionsLock.Lock()
	defer m.sessionsLock.Unlock()
	delete(m.sessions, address)
	return nil
}

func (m *mockSignalStores) MigratePNToLID(_ context.Context, _, _ types.JID) error {
	return nil
}

func (m *mockSignalStores) PutIdentity(_ context.Context, address string, key [32]byte) error {
	m.identitiesLock.Lock()
	defer m.identitiesLock.Unlock()
	m.identities[address] = key
	return nil
}

func (m *mockSignalStores) DeleteAllIdentities(_ context.Context, _ string) error {
	m.identitiesLock.Lock()
	defer m.identitiesLock.Unlock()
	clear(m.identities)
	return nil
}

func (m *mockSignalStores) DeleteIdentity(_ context.Context, address string) error {
	m.identitiesLock.Lock()
	defer m.identitiesLock.Unlock()
	delete(m.identities, address)
	return nil
}

func (m *mockSignalStores) IsTrustedIdentity(_ context.Context, _ string, _ [32]byte) (bool, error) {
	return true, nil
}

type mockSenderKeyStore struct {
	keysLock sync.RWMutex
	keys     map[string][]byte
}

func (m *mockSenderKeyStore) PutSenderKey(_ context.Context, group, user string, session []byte) error {
	m.keysLock.Lock()
	defer m.keysLock.Unlock()
	m.keys[group+user] = session
	return nil
}

func (m *mockSenderKeyStore) GetSenderKey(_ context.Context, group, user string) ([]byte, error) {
	m.keysLock.RLock()
	defer m.keysLock.RUnlock()
	return m.keys[group+user], nil
}

type mockMsgSecretStore struct{}

func (m *mockMsgSecretStore) PutMessageSecrets(_ context.Context, _ []store.MessageSecretInsert) error {
	return nil
}

func (m *mockMsgSecretStore) PutMessageSecret(_ context.Context, _, _ types.JID, _ types.MessageID, _ []byte) error {
	return nil
}

func (m *mockMsgSecretStore) GetMessageSecret(_ context.Context, _, _ types.JID, _ types.MessageID) ([]byte, types.JID, error) {
	return nil, types.EmptyJID, nil
}

type mockNCTSaltStore struct{}

func (m *mockNCTSaltStore) PutNCTSalt(_ context.Context, _ []byte) error {
	return nil
}

func (m *mockNCTSaltStore) GetNCTSalt(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (m *mockNCTSaltStore) DeleteNCTSalt(_ context.Context) error {
	return nil
}

func setupMockSocketClient(t *testing.T) (*Client, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusInternalError, "")
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				break
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	fs := socket.NewFrameSocket(waLog.Noop, nil)
	fs.URL = wsURL
	err := fs.Connect(context.Background())
	require.NoError(t, err)

	nh := socket.NewNoiseHandshake()
	nh.Start("test", []byte("header"))
	ns, err := nh.Finish(fs, func(_ context.Context, _ []byte) {}, func(_ context.Context, _ *socket.NoiseSocket, _ bool) {})
	require.NoError(t, err)

	dev := &store.Device{
		ID:             &types.JID{User: "12345", Server: types.DefaultUserServer},
		IdentityKey:    keys.NewKeyPair(),
		NoiseKey:       keys.NewKeyPair(),
		RegistrationID: 1001,
	}
	dev.SignedPreKey = dev.IdentityKey.CreateSignedPreKey(1)

	mockStore := newMockSignalStores()
	dev.Sessions = mockStore
	dev.Identities = mockStore
	dev.LIDs = newMockLIDStore()
	dev.SenderKeys = &mockSenderKeyStore{keys: make(map[string][]byte)}
	dev.PrivacyTokens = newMockPrivacyTokenStore()
	dev.MsgSecrets = &mockMsgSecretStore{}
	dev.NCTSalt = &mockNCTSaltStore{}

	cli := NewClient(dev, waLog.Noop)
	cli.socket = ns
	cli.isLoggedIn.Store(true)

	cleanup := func() {
		cli.socketLock.RLock()
		sock := cli.socket
		cli.socketLock.RUnlock()
		if sock != nil {
			sock.Stop(false, false)
		}
		server.Close()
	}

	return cli, cleanup
}

func setupSignalBundle(targetJID types.JID) (*prekey.Bundle, error) {
	targetIDKey, err := keyhelper.GenerateIdentityKeyPair()
	if err != nil {
		return nil, err
	}
	targetRegID := keyhelper.GenerateRegistrationID()
	targetPreKeys, err := keyhelper.GeneratePreKeys(1, 1, pbSerializer.PreKeyRecord)
	if err != nil {
		return nil, err
	}
	targetSignedPreKey, err := keyhelper.GenerateSignedPreKey(targetIDKey, 1, pbSerializer.SignedPreKeyRecord)
	if err != nil {
		return nil, err
	}

	bundle := prekey.NewBundle(
		targetRegID,
		uint32(targetJID.Device),
		targetPreKeys[0].ID(),
		targetSignedPreKey.ID(),
		targetPreKeys[0].KeyPair().PublicKey(),
		targetSignedPreKey.KeyPair().PublicKey(),
		targetSignedPreKey.Signature(),
		targetIDKey.PublicKey(),
	)
	return bundle, nil
}

func establishSignalSession(ctx context.Context, cli *Client, targetJID types.JID) (*prekey.Bundle, error) {
	bundle, err := setupSignalBundle(targetJID)
	if err != nil {
		return nil, err
	}
	builder := session.NewBuilderFromSignal(cli.Store, targetJID.SignalAddress(), pbSerializer)
	err = builder.ProcessBundle(ctx, bundle)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}
