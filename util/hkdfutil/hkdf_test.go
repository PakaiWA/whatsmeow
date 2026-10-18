// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hkdfutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PakaiWA/whatsmeow/util/hkdfutil"
)

func TestSHA256(t *testing.T) {
	key := []byte("master_secret_input_key_32_bytes")
	salt := []byte("test_salt_16_byt")
	info := []byte("WhatsApp HKDF Info")

	t.Run("Standard expansion 32 bytes", func(t *testing.T) {
		out32 := hkdfutil.SHA256(key, salt, info, 32)
		assert.Len(t, out32, 32)
	})

	t.Run("Standard expansion 64 bytes", func(t *testing.T) {
		out64 := hkdfutil.SHA256(key, salt, info, 64)
		assert.Len(t, out64, 64)
	})

	t.Run("Determinism with same inputs", func(t *testing.T) {
		res1 := hkdfutil.SHA256(key, salt, info, 32)
		res2 := hkdfutil.SHA256(key, salt, info, 32)
		assert.Equal(t, res1, res2)
	})

	t.Run("Different output for different info", func(t *testing.T) {
		res1 := hkdfutil.SHA256(key, salt, []byte("Info A"), 32)
		res2 := hkdfutil.SHA256(key, salt, []byte("Info B"), 32)
		assert.NotEqual(t, res1, res2)
	})
}
