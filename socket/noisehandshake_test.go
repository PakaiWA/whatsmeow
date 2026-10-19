// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"

	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestNoiseHandshake_Lifecycle(t *testing.T) {
	t.Run("NewNoiseHandshake", func(t *testing.T) {
		nh := NewNoiseHandshake()
		require.NotNil(t, nh)
	})

	t.Run("sha256Slice", func(t *testing.T) {
		data := []byte("test-data-for-sha256")
		hash := sha256Slice(data)
		assert.Len(t, hash, 32)
	})

	t.Run("Start with 32-byte pattern and non-32-byte pattern", func(t *testing.T) {
		header := []byte("prologue-header-data")

		// 32-byte pattern
		nh32 := NewNoiseHandshake()
		nh32.Start(NoiseStartPattern, header)
		assert.Len(t, nh32.hash, 32)
		assert.Len(t, nh32.salt, 32)
		assert.NotNil(t, nh32.key)

		// Non 32-byte pattern
		nhShort := NewNoiseHandshake()
		nhShort.Start("short_pattern", header)
		assert.Len(t, nhShort.hash, 32)
		assert.Len(t, nhShort.salt, 32)
		assert.NotNil(t, nhShort.key)
	})

	t.Run("Encrypt and Decrypt roundtrip", func(t *testing.T) {
		nhSender := NewNoiseHandshake()
		nhSender.Start(NoiseStartPattern, []byte("prologue"))

		nhReceiver := NewNoiseHandshake()
		nhReceiver.Start(NoiseStartPattern, []byte("prologue"))

		msg1 := []byte("hello noise protocol")
		ciphertext1 := nhSender.Encrypt(msg1)
		assert.NotEmpty(t, ciphertext1)

		plaintext1, err := nhReceiver.Decrypt(ciphertext1)
		require.NoError(t, err)
		assert.Equal(t, msg1, plaintext1)

		// Sequential message to verify counter increment
		msg2 := []byte("second sequential message")
		ciphertext2 := nhSender.Encrypt(msg2)
		plaintext2, err := nhReceiver.Decrypt(ciphertext2)
		require.NoError(t, err)
		assert.Equal(t, msg2, plaintext2)

		// Decrypt error with corrupted ciphertext
		badCiphertext := append([]byte(nil), ciphertext2...)
		badCiphertext[0] ^= 0xff
		_, errBad := nhReceiver.Decrypt(badCiphertext)
		assert.Error(t, errBad)
	})

	t.Run("MixIntoKey", func(t *testing.T) {
		nh := NewNoiseHandshake()
		nh.Start(NoiseStartPattern, nil)

		sharedData := []byte("diffie-hellman-shared-secret-data")
		err := nh.MixIntoKey(sharedData)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), nh.counter)
	})

	t.Run("MixSharedSecretIntoKey", func(t *testing.T) {
		nh := NewNoiseHandshake()
		nh.Start(NoiseStartPattern, nil)

		var privAlice, privBob [32]byte
		_, err := io.ReadFull(rand.Reader, privAlice[:])
		require.NoError(t, err)
		_, err = io.ReadFull(rand.Reader, privBob[:])
		require.NoError(t, err)

		pubBob, err := curve25519.X25519(privBob[:], curve25519.Basepoint)
		require.NoError(t, err)

		var pubBobArray [32]byte
		copy(pubBobArray[:], pubBob)

		err = nh.MixSharedSecretIntoKey(privAlice, pubBobArray)
		require.NoError(t, err)

		// Low-order point causes curve25519 error
		var zeroPub [32]byte
		err = nh.MixSharedSecretIntoKey(privAlice, zeroPub)
		assert.Error(t, err)
	})

	t.Run("extractAndExpand", func(t *testing.T) {
		nh := NewNoiseHandshake()
		write, read, err := nh.extractAndExpand([]byte("salt1234"), []byte("data1234"))
		require.NoError(t, err)
		assert.Len(t, write, 32)
		assert.Len(t, read, 32)
		assert.NotEqual(t, write, read)
	})

	t.Run("Finish creates NoiseSocket", func(t *testing.T) {
		nh := NewNoiseHandshake()
		nh.Start(NoiseStartPattern, nil)

		fs := NewFrameSocket(waLog.Noop, nil)
		frameReceived := make(chan []byte, 1)
		onFrame := func(_ context.Context, data []byte) {
			frameReceived <- data
		}
		onDisconnect := func(_ context.Context, _ *NoiseSocket, _ bool) {}

		ns, err := nh.Finish(fs, onFrame, onDisconnect)
		require.NoError(t, err)
		require.NotNil(t, ns)
		assert.Equal(t, fs, ns.fs)

		ns.Stop(false, false)
	})
}
