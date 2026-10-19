// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"context"

	"go.mau.fi/libsignal/keys/prekey"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/session"
	"go.mau.fi/libsignal/state/record"
	"go.mau.fi/libsignal/util/keyhelper"

	"github.com/PakaiWA/whatsmeow/util/keys"
)

func createRealSession() *record.Session {
	ctx := context.Background()

	aliceDev := &Device{
		IdentityKey:    keys.NewKeyPair(),
		RegistrationID: 1001,
	}
	aliceSessions := make(map[string][]byte)
	aliceIdentities := make(map[string][32]byte)
	aliceDev.Sessions = &mockSignalStores{
		putSessionFunc: func(_ context.Context, address string, session []byte) error {
			aliceSessions[address] = session
			return nil
		},
		getSessionFunc: func(_ context.Context, address string) ([]byte, error) {
			return aliceSessions[address], nil
		},
	}
	aliceDev.Identities = &mockSignalStores{
		isTrustedIdentityFunc: func(_ context.Context, _ string, _ [32]byte) (bool, error) {
			return true, nil
		},
		putIdentityFunc: func(_ context.Context, addr string, key [32]byte) error {
			aliceIdentities[addr] = key
			return nil
		},
	}

	bobIDKey, _ := keyhelper.GenerateIdentityKeyPair()
	bobRegID := keyhelper.GenerateRegistrationID()
	bobPreKeys, _ := keyhelper.GeneratePreKeys(1, 1, SignalProtobufSerializer.PreKeyRecord)
	bobSignedPreKey, _ := keyhelper.GenerateSignedPreKey(bobIDKey, 1, SignalProtobufSerializer.SignedPreKeyRecord)

	bundle := prekey.NewBundle(
		bobRegID,
		1,
		bobPreKeys[0].ID(),
		bobSignedPreKey.ID(),
		bobPreKeys[0].KeyPair().PublicKey(),
		bobSignedPreKey.KeyPair().PublicKey(),
		bobSignedPreKey.Signature(),
		bobIDKey.PublicKey(),
	)

	bobAddr := protocol.NewSignalAddress("bob", 1)
	builder := session.NewBuilder(aliceDev, aliceDev, aliceDev, aliceDev, bobAddr, SignalProtobufSerializer)
	_ = builder.ProcessBundle(ctx, bundle)

	sessBytes := aliceSessions[bobAddr.String()]
	sess, _ := record.NewSessionFromBytes(sessBytes, SignalProtobufSerializer.Session, SignalProtobufSerializer.State)
	return sess
}
