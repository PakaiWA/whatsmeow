// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"context"

	"github.com/PakaiWA/whatsmeow/store"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

type mockAppStateSyncKeyStore struct {
	keys   map[string]*store.AppStateSyncKey
	getErr error
	putErr error
}

func newMockAppStateSyncKeyStore() *mockAppStateSyncKeyStore {
	return &mockAppStateSyncKeyStore{keys: make(map[string]*store.AppStateSyncKey)}
}

func (m *mockAppStateSyncKeyStore) PutAppStateSyncKey(_ context.Context, id []byte, key store.AppStateSyncKey) error {
	if m.putErr != nil {
		return m.putErr
	}
	m.keys[string(id)] = &key
	return nil
}

func (m *mockAppStateSyncKeyStore) GetAppStateSyncKey(_ context.Context, id []byte) (*store.AppStateSyncKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	k, ok := m.keys[string(id)]
	if !ok {
		return nil, nil
	}
	return k, nil
}

func (m *mockAppStateSyncKeyStore) GetLatestAppStateSyncKeyID(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (m *mockAppStateSyncKeyStore) GetAllAppStateSyncKeys(_ context.Context) ([]*store.AppStateSyncKey, error) {
	var res []*store.AppStateSyncKey
	for _, v := range m.keys {
		res = append(res, v)
	}
	return res, nil
}

type mockAppStateStore struct {
	versions map[string]struct {
		ver  uint64
		hash [128]byte
	}
	macs             map[string]map[string][]byte
	putVersionErr    error
	getVersionErr    error
	deleteVersionErr error
	putMACsErr       error
	deleteMACsErr    error
	getMACErr        error
}

func newMockAppStateStore() *mockAppStateStore {
	return &mockAppStateStore{
		versions: make(map[string]struct {
			ver  uint64
			hash [128]byte
		}),
		macs: make(map[string]map[string][]byte),
	}
}

func (m *mockAppStateStore) PutAppStateVersion(_ context.Context, name string, version uint64, hash [128]byte) error {
	if m.putVersionErr != nil {
		return m.putVersionErr
	}
	m.versions[name] = struct {
		ver  uint64
		hash [128]byte
	}{ver: version, hash: hash}
	return nil
}

func (m *mockAppStateStore) GetAppStateVersion(_ context.Context, name string) (uint64, [128]byte, error) {
	if m.getVersionErr != nil {
		return 0, [128]byte{}, m.getVersionErr
	}
	v, ok := m.versions[name]
	if !ok {
		return 0, [128]byte{}, nil
	}
	return v.ver, v.hash, nil
}

func (m *mockAppStateStore) DeleteAppStateVersion(_ context.Context, name string) error {
	if m.deleteVersionErr != nil {
		return m.deleteVersionErr
	}
	delete(m.versions, name)
	return nil
}

func (m *mockAppStateStore) PutAppStateMutationMACs(_ context.Context, name string, _ uint64, mutations []store.AppStateMutationMAC) error {
	if m.putMACsErr != nil {
		return m.putMACsErr
	}
	if m.macs[name] == nil {
		m.macs[name] = make(map[string][]byte)
	}
	for _, mut := range mutations {
		m.macs[name][string(mut.IndexMAC)] = mut.ValueMAC
	}
	return nil
}

func (m *mockAppStateStore) DeleteAppStateMutationMACs(_ context.Context, name string, indexMACs [][]byte) error {
	if m.deleteMACsErr != nil {
		return m.deleteMACsErr
	}
	if m.macs[name] == nil {
		return nil
	}
	for _, idx := range indexMACs {
		delete(m.macs[name], string(idx))
	}
	return nil
}

func (m *mockAppStateStore) GetAppStateMutationMAC(_ context.Context, name string, indexMAC []byte) ([]byte, error) {
	if m.getMACErr != nil {
		return nil, m.getMACErr
	}
	if m.macs[name] == nil {
		return nil, nil
	}
	return m.macs[name][string(indexMAC)], nil
}

func newTestProcessor() (*Processor, *mockAppStateSyncKeyStore, *mockAppStateStore) {
	keyStore := newMockAppStateSyncKeyStore()
	appStateStore := newMockAppStateStore()
	dev := &store.Device{
		AppStateKeys: keyStore,
		AppState:     appStateStore,
		Log:          waLog.Noop,
	}
	return NewProcessor(dev, waLog.Noop), keyStore, appStateStore
}
