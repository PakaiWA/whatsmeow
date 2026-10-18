// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/libsignal/ecc"

	"github.com/PakaiWA/whatsmeow/util/keys"
)

func TestKeyPair_GenerationAndSigning(t *testing.T) {
	t.Run("Generate random keypair", func(t *testing.T) {
		kp := keys.NewKeyPair()
		require.NotNil(t, kp)
		require.NotNil(t, kp.Pub)
		require.NotNil(t, kp.Priv)

		// Verification: Deriving public key from the private key should match
		derived := keys.NewKeyPairFromPrivateKey(*kp.Priv)
		assert.Equal(t, kp.Pub, derived.Pub)
		assert.Equal(t, kp.Priv, derived.Priv)
	})

	t.Run("CreateSignedPreKey and verify signature", func(t *testing.T) {
		identityKey := keys.NewKeyPair()
		keyID := uint32(101)

		preKey := identityKey.CreateSignedPreKey(keyID)
		require.NotNil(t, preKey)
		assert.Equal(t, keyID, preKey.KeyID)
		require.NotNil(t, preKey.Signature)

		// Verify signature using libsignal ecc
		pubKeyForSignature := make([]byte, 33)
		pubKeyForSignature[0] = ecc.DjbType
		copy(pubKeyForSignature[1:], preKey.Pub[:])

		djbPubKey := ecc.NewDjbECPublicKey(*identityKey.Pub)
		isValid := ecc.VerifySignature(djbPubKey, pubKeyForSignature, *preKey.Signature)
		assert.True(t, isValid, "PreKey signature created by identity key must be valid")
	})

	t.Run("PreKey generated independently", func(t *testing.T) {
		preKey := keys.NewPreKey(50)
		require.NotNil(t, preKey)
		assert.Equal(t, uint32(50), preKey.KeyID)
		assert.Nil(t, preKey.Signature, "Initial PreKey without signing should have nil signature")
	})
}
