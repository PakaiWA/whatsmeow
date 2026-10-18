// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcmutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/util/gcmutil"
)

var (
	gcmKey32 = []byte("12345678901234567890123456789012") // 32 bytes AES-256
	gcmKey16 = []byte("1234567890123456")                 // 16 bytes AES-128
	gcmIV12  = []byte("123456789012")                     // 12 bytes standard GCM nonce
)

// ============================================================
// Prepare
// ============================================================

func TestPrepare_ValidKey(t *testing.T) {
	gcm, err := gcmutil.Prepare(gcmKey32)
	require.NoError(t, err)
	assert.NotNil(t, gcm)
}

func TestPrepare_AES128Key(t *testing.T) {
	gcm, err := gcmutil.Prepare(gcmKey16)
	require.NoError(t, err)
	assert.NotNil(t, gcm)
}

func TestPrepare_InvalidKey(t *testing.T) {
	_, err := gcmutil.Prepare([]byte("short"))
	assert.Error(t, err)
}

// ============================================================
// Encrypt / Decrypt round-trip
// ============================================================

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		key            []byte
		plaintext      []byte
		additionalData []byte
	}{
		{"AES-256 no AD", gcmKey32, []byte("hello, world!"), nil},
		{"AES-256 with AD", gcmKey32, []byte("authenticated data"), []byte("aad context")},
		{"AES-128 no AD", gcmKey16, []byte("short"), nil},
		{"AES-256 empty plaintext", gcmKey32, nil, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := gcmutil.Encrypt(tc.key, gcmIV12, tc.plaintext, tc.additionalData)
			require.NoError(t, err)

			plaintext, err := gcmutil.Decrypt(tc.key, gcmIV12, ciphertext, tc.additionalData)
			require.NoError(t, err)
			assert.Equal(t, tc.plaintext, plaintext)
		})
	}
}

func TestEncrypt_InvalidKey(t *testing.T) {
	_, err := gcmutil.Encrypt([]byte("bad"), gcmIV12, []byte("data"), nil)
	assert.Error(t, err)
}

func TestDecrypt_InvalidKey(t *testing.T) {
	// Encrypt with valid key first
	ct, err := gcmutil.Encrypt(gcmKey32, gcmIV12, []byte("data"), nil)
	require.NoError(t, err)

	_, err = gcmutil.Decrypt([]byte("bad"), gcmIV12, ct, nil)
	assert.Error(t, err)
}

func TestDecrypt_Tampered_Ciphertext(t *testing.T) {
	ct, err := gcmutil.Encrypt(gcmKey32, gcmIV12, []byte("original"), nil)
	require.NoError(t, err)

	// Tamper the ciphertext
	ct[0] ^= 0xFF

	_, err = gcmutil.Decrypt(gcmKey32, gcmIV12, ct, nil)
	assert.Error(t, err, "decryption of tampered ciphertext should fail")
}

func TestDecrypt_WrongAdditionalData(t *testing.T) {
	ct, err := gcmutil.Encrypt(gcmKey32, gcmIV12, []byte("data"), []byte("correct aad"))
	require.NoError(t, err)

	_, err = gcmutil.Decrypt(gcmKey32, gcmIV12, ct, []byte("wrong aad"))
	assert.Error(t, err, "decryption with wrong AAD should fail")
}

func TestEncryptDecrypt_Deterministic(t *testing.T) {
	// Same key+iv+plaintext should produce same ciphertext (GCM is deterministic for same nonce)
	ct1, err := gcmutil.Encrypt(gcmKey32, gcmIV12, []byte("data"), nil)
	require.NoError(t, err)
	ct2, err := gcmutil.Encrypt(gcmKey32, gcmIV12, []byte("data"), nil)
	require.NoError(t, err)
	assert.Equal(t, ct1, ct2)
}
