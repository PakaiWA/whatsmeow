// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argo

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetArgoState() {
	loadOnce = sync.Once{}
	initErr = nil
	Store = nil
	QueryIDToMessageName = nil
}

func TestArgo_Success(t *testing.T) {
	origWireBytes := wireTypeStoreBytes
	origJSONBytes := jsonMapBytes
	defer func() {
		wireTypeStoreBytes = origWireBytes
		jsonMapBytes = origJSONBytes
		resetArgoState()
		_ = Init()
	}()

	resetArgoState()

	err := Init()
	require.NoError(t, err)
	assert.NotEmpty(t, Store)
	assert.NotEmpty(t, QueryIDToMessageName)

	store, err := GetStore()
	require.NoError(t, err)
	assert.Equal(t, Store, store)

	queryMap, err := GetQueryIDToMessageName()
	require.NoError(t, err)
	assert.Equal(t, QueryIDToMessageName, queryMap)

	// Idempotency: calling Init or Getters repeatedly returns the same results
	errSecond := Init()
	require.NoError(t, errSecond)
}

func TestArgo_DecodeWireTypeStoreFileError(t *testing.T) {
	origWireBytes := wireTypeStoreBytes
	origJSONBytes := jsonMapBytes
	defer func() {
		wireTypeStoreBytes = origWireBytes
		jsonMapBytes = origJSONBytes
		resetArgoState()
		_ = Init()
	}()

	resetArgoState()
	wireTypeStoreBytes = []byte("corrupt-wire-data")

	err := Init()
	assert.Error(t, err)
	assert.Nil(t, Store)

	store, errStore := GetStore()
	assert.Error(t, errStore)
	assert.Nil(t, store)

	queryMap, errMap := GetQueryIDToMessageName()
	assert.Error(t, errMap)
	assert.Nil(t, queryMap)
}

func TestArgo_JSONUnmarshalError(t *testing.T) {
	origWireBytes := wireTypeStoreBytes
	origJSONBytes := jsonMapBytes
	defer func() {
		wireTypeStoreBytes = origWireBytes
		jsonMapBytes = origJSONBytes
		resetArgoState()
		_ = Init()
	}()

	resetArgoState()
	jsonMapBytes = []byte("{invalid-json-data")

	err := Init()
	assert.Error(t, err)
	assert.Nil(t, QueryIDToMessageName)

	store, errStore := GetStore()
	assert.Error(t, errStore)
	assert.Nil(t, store)

	queryMap, errMap := GetQueryIDToMessageName()
	assert.Error(t, errMap)
	assert.Nil(t, queryMap)
}
