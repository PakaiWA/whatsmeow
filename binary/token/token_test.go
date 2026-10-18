// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package token_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/binary/token"
)

func TestSingleByteTokens(t *testing.T) {
	t.Run("Known tokens lookup", func(t *testing.T) {
		idx, ok := token.IndexOfSingleToken("s.whatsapp.net")
		require.True(t, ok)
		assert.Equal(t, byte(3), idx)
		assert.Equal(t, "s.whatsapp.net", token.SingleByteTokens[idx])
	})

	t.Run("Non-existent token lookup", func(t *testing.T) {
		_, ok := token.IndexOfSingleToken("non_existent_token_xyz")
		assert.False(t, ok)
	})
}

func TestDoubleByteTokens(t *testing.T) {
	t.Run("Valid double byte token lookup", func(t *testing.T) {
		tok, err := token.GetDoubleToken(0, 0)
		require.NoError(t, err)
		assert.Equal(t, "read-self", tok)

		dict, idx, ok := token.IndexOfDoubleByteToken("read-self")
		require.True(t, ok)
		assert.Equal(t, byte(0), dict)
		assert.Equal(t, byte(0), idx)
	})

	t.Run("Out of bounds double byte token index", func(t *testing.T) {
		_, err := token.GetDoubleToken(-1, 0)
		assert.Error(t, err)

		_, err = token.GetDoubleToken(0, -1)
		assert.Error(t, err)

		_, err = token.GetDoubleToken(len(token.DoubleByteTokens)+1, 0)
		assert.Error(t, err)

		_, err = token.GetDoubleToken(0, 9999)
		assert.Error(t, err)
	})

	t.Run("Non-existent double token", func(t *testing.T) {
		_, _, ok := token.IndexOfDoubleByteToken("unregistered_double_token_test")
		assert.False(t, ok)
	})
}
