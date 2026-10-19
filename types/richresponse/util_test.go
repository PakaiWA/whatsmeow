// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package richresponse

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyTarget struct {
	TypeName string `json:"__typename"`
	Count    int    `json:"count"`
}

type dummyUnknown []byte

func TestUnmarshalWithTypeName(t *testing.T) {
	registry := map[string]reflect.Type{
		"Dummy": reflect.TypeFor[dummyTarget](),
	}

	t.Run("Invalid JSON for TypeNameContainer", func(t *testing.T) {
		val, err := unmarshalWithTypeName[dummyUnknown]([]byte(`{invalid json`), registry)
		require.Error(t, err)
		assert.Nil(t, val)
	})

	t.Run("Unknown type fallback", func(t *testing.T) {
		input := []byte(`{"__typename":"UnregisteredType","foo":"bar"}`)
		val, err := unmarshalWithTypeName[dummyUnknown](input, registry)
		require.NoError(t, err)
		unk, ok := val.(dummyUnknown)
		require.True(t, ok)
		assert.Equal(t, dummyUnknown(input), unk)
	})

	t.Run("Known type successful unmarshal", func(t *testing.T) {
		input := []byte(`{"__typename":"Dummy","count":42}`)
		val, err := unmarshalWithTypeName[dummyUnknown](input, registry)
		require.NoError(t, err)
		res, ok := val.(*dummyTarget)
		require.True(t, ok)
		assert.Equal(t, "Dummy", res.TypeName)
		assert.Equal(t, 42, res.Count)
	})

	t.Run("Known type invalid payload unmarshal error", func(t *testing.T) {
		input := []byte(`{"__typename":"Dummy","count":"not-a-number"}`)
		val, err := unmarshalWithTypeName[dummyUnknown](input, registry)
		require.Error(t, err)
		assert.Nil(t, val)
	})
}
