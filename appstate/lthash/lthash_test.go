// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lthash_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PakaiWA/whatsmeow/appstate/lthash"
)

func TestLTHash_SubtractThenAdd(t *testing.T) {
	lt := lthash.WAPatchIntegrity

	t.Run("Adding and subtracting mutations should cancel out", func(t *testing.T) {
		initialState := make([]byte, 128)
		mutation1 := []byte("mutation_alpha_value")
		mutation2 := []byte("mutation_beta_value")

		// Add mutation 1 and 2
		stateAfterAdd := lt.SubtractThenAdd(initialState, nil, [][]byte{mutation1, mutation2})
		assert.False(t, bytes.Equal(initialState, stateAfterAdd), "State should change after adding mutations")

		// Remove mutation 1 and 2
		stateAfterRemove := lt.SubtractThenAdd(stateAfterAdd, [][]byte{mutation1, mutation2}, nil)
		assert.True(t, bytes.Equal(initialState, stateAfterRemove), "State should return to initial state after removing same mutations")
	})

	t.Run("Mutation addition order should be commutative", func(t *testing.T) {
		initialState := make([]byte, 128)
		mutationA := []byte("user_chat_setting_pinned")
		mutationB := []byte("user_chat_setting_muted")

		// Order: A then B
		stateAB := lt.SubtractThenAdd(initialState, nil, [][]byte{mutationA, mutationB})

		// Order: B then A
		stateBA := lt.SubtractThenAdd(initialState, nil, [][]byte{mutationB, mutationA})

		assert.Equal(t, stateAB, stateBA, "LTHash summation must be commutative regardless of mutation order")
	})

	t.Run("SubtractThenAdd with both additions and removals in single step", func(t *testing.T) {
		initialState := make([]byte, 128)
		mutA := []byte("item_a")
		mutB := []byte("item_b")
		mutC := []byte("item_c")

		// Start with mutA
		state1 := lt.SubtractThenAdd(initialState, nil, [][]byte{mutA})

		// In one operation, remove mutA and add mutB, mutC
		state2 := lt.SubtractThenAdd(state1, [][]byte{mutA}, [][]byte{mutB, mutC})

		// State2 should be identical to adding mutB and mutC directly from initialState
		expectedState := lt.SubtractThenAdd(initialState, nil, [][]byte{mutB, mutC})
		assert.Equal(t, expectedState, state2)
	})
}
