// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cbcutil_test

import (
	"bytes"
	"crypto/aes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/util/cbcutil"
)

var (
	testKey16 = []byte("1234567890123456")                 // 16 bytes AES-128
	testKey32 = []byte("12345678901234567890123456789012") // 32 bytes AES-256
	testIV    = []byte("1234567890123456")                 // 16 bytes IV
)

// ============================================================
// Encrypt / Decrypt round-trip
// ============================================================

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		key       []byte
		plaintext []byte
	}{
		{"AES-128 short", testKey16, []byte("hello world!!!!!")},
		{"AES-256 short", testKey32, []byte("hello world!!!!!")},
		{"AES-256 empty", testKey32, []byte{}},
		{"AES-256 exactly 1 block", testKey32, bytes.Repeat([]byte("A"), aes.BlockSize)},
		{"AES-256 multi-block", testKey32, bytes.Repeat([]byte("B"), 64)},
		{"AES-256 unaligned", testKey32, []byte("unaligned data 13 bytes")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := cbcutil.Encrypt(tc.key, testIV, tc.plaintext)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)

			// Decrypt requires a copy because CryptBlocks modifies in-place
			ctCopy := make([]byte, len(ciphertext))
			copy(ctCopy, ciphertext)

			plaintext, err := cbcutil.Decrypt(tc.key, testIV, ctCopy)
			require.NoError(t, err)
			assert.Equal(t, tc.plaintext, plaintext)
		})
	}
}

func TestEncrypt_NilIV_GeneratesRandomIV(t *testing.T) {
	plaintext := []byte("test with random iv")
	ct1, err := cbcutil.Encrypt(testKey32, nil, plaintext)
	require.NoError(t, err)
	assert.True(t, len(ct1) > aes.BlockSize, "ciphertext should include prepended IV")

	ct2, err := cbcutil.Encrypt(testKey32, nil, plaintext)
	require.NoError(t, err)

	// With random IV, two encryptions of the same plaintext should differ
	assert.NotEqual(t, ct1, ct2)
}

func TestEncrypt_InvalidKey(t *testing.T) {
	_, err := cbcutil.Encrypt([]byte("shortkey"), testIV, []byte("data"))
	assert.Error(t, err)
}

func TestDecrypt_InvalidKey(t *testing.T) {
	ciphertext := make([]byte, 32) // 2 blocks of zeros
	_, err := cbcutil.Decrypt([]byte("shortkey"), testIV, ciphertext)
	assert.Error(t, err)
}

func TestDecrypt_EmptyCiphertext(t *testing.T) {
	_, err := cbcutil.Decrypt(testKey32, testIV, []byte{})
	assert.Error(t, err)
}

func TestDecrypt_NotMultipleOfBlockSize(t *testing.T) {
	_, err := cbcutil.Decrypt(testKey32, testIV, []byte{1, 2, 3})
	assert.Error(t, err)
}

// ============================================================
// EncryptStream
// ============================================================

func TestEncryptStream_Basic(t *testing.T) {
	macKey := []byte("mac_key_for_hmac_signing_32_byte")
	plaintext := []byte("streaming plaintext data for CBC encryption test")

	var outBuf bytes.Buffer
	plainHash, cipherHash, size, encSize, err := cbcutil.EncryptStream(
		testKey32,
		testIV,
		macKey,
		bytes.NewReader(plaintext),
		&outBuf,
	)
	require.NoError(t, err)
	assert.Equal(t, uint64(len(plaintext)), size)
	assert.Greater(t, encSize, size) // should include padding + MAC
	assert.Len(t, plainHash, 32)
	assert.Len(t, cipherHash, 32)
	assert.Greater(t, outBuf.Len(), 0)
}

func TestEncryptStream_EmptyInput(t *testing.T) {
	macKey := []byte("mac_key_for_hmac_signing_32_byte")
	var outBuf bytes.Buffer
	_, _, size, _, err := cbcutil.EncryptStream(
		testKey32,
		testIV,
		macKey,
		bytes.NewReader([]byte{}),
		&outBuf,
	)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), size)
}

func TestEncryptStream_LargerThanBuffer(t *testing.T) {
	macKey := []byte("mac_key_for_hmac_signing_32_byte")
	// 40 KB > default 32 KB buffer in EncryptStream
	plaintext := bytes.Repeat([]byte("X"), 40*1024)
	var outBuf bytes.Buffer
	_, _, size, _, err := cbcutil.EncryptStream(
		testKey32,
		testIV,
		macKey,
		bytes.NewReader(plaintext),
		&outBuf,
	)
	require.NoError(t, err)
	assert.Equal(t, uint64(len(plaintext)), size)
}

func TestEncryptStream_InvalidKey(t *testing.T) {
	var outBuf bytes.Buffer
	_, _, _, _, err := cbcutil.EncryptStream(
		[]byte("bad"),
		testIV,
		[]byte("mackey"),
		bytes.NewReader([]byte("data")),
		&outBuf,
	)
	assert.Error(t, err)
}

// ============================================================
// DecryptFile
// ============================================================

func TestDecryptFile_RoundTrip(t *testing.T) {
	plaintext := bytes.Repeat([]byte("A"), 48) // 3 blocks, no padding needed for test

	// First encrypt to get a proper padded ciphertext
	ciphertext, err := cbcutil.Encrypt(testKey32, testIV, plaintext)
	require.NoError(t, err)

	// Write ciphertext to a temp file
	f, err := os.CreateTemp(t.TempDir(), "cbc-test-*.bin")
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Write(ciphertext)
	require.NoError(t, err)

	// Seek back to start for DecryptFile
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	err = cbcutil.DecryptFile(testKey32, testIV, f)
	require.NoError(t, err)

	// Read decrypted content
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	stat, err := f.Stat()
	require.NoError(t, err)
	decrypted := make([]byte, stat.Size())
	_, err = f.Read(decrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptFile_InvalidKey(t *testing.T) {
	ciphertext, err := cbcutil.Encrypt(testKey32, testIV, []byte("some data here!"))
	require.NoError(t, err)

	f, err := os.CreateTemp(t.TempDir(), "cbc-badkey-*.bin")
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Write(ciphertext)
	require.NoError(t, err)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	err = cbcutil.DecryptFile([]byte("bad"), testIV, f)
	assert.Error(t, err)
}

func TestDecryptFile_NotMultipleOfBlockSize(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "cbc-unaligned-*.bin")
	require.NoError(t, err)
	defer f.Close()

	// Write non-aligned data (3 bytes, not a multiple of 16)
	_, err = f.Write([]byte{1, 2, 3})
	require.NoError(t, err)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	err = cbcutil.DecryptFile(testKey32, testIV, f)
	assert.Error(t, err)
}
