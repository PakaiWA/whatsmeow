// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
)

func TestCachedLIDMap(t *testing.T) {
	container := newTestContainer(t)
	lidMap := container.LIDMap
	ctx := context.Background()

	pn1 := types.NewJID("6281111", types.DefaultUserServer)
	lid1 := types.NewJID("10001", types.HiddenUserServer)

	pn2 := types.NewJID("6282222", types.DefaultUserServer)
	lid2 := types.NewJID("10002", types.HiddenUserServer)

	t.Run("Invalid server errors in GetLIDForPN and GetPNForLID", func(t *testing.T) {
		_, err := lidMap.GetLIDForPN(ctx, lid1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GetLIDForPN")

		_, err = lidMap.GetPNForLID(ctx, pn1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GetPNForLID")

		err = lidMap.PutLIDMapping(ctx, pn1, lid1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PutLIDMapping")
	})

	t.Run("Get when not found returns empty JID", func(t *testing.T) {
		unknownPN := types.NewJID("6289999", types.DefaultUserServer)
		gotLID, err := lidMap.GetLIDForPN(ctx, unknownPN)
		require.NoError(t, err)
		assert.Equal(t, types.EmptyJID, gotLID)

		unknownLID := types.NewJID("99999", types.HiddenUserServer)
		gotPN, err := lidMap.GetPNForLID(ctx, unknownLID)
		require.NoError(t, err)
		assert.Equal(t, types.EmptyJID, gotPN)
	})

	t.Run("PutLIDMapping and Get roundtrip", func(t *testing.T) {
		err := lidMap.PutLIDMapping(ctx, lid1, pn1)
		require.NoError(t, err)

		// Repeated PutLIDMapping with same cached mapping is no-op
		err = lidMap.PutLIDMapping(ctx, lid1, pn1)
		require.NoError(t, err)

		// GetLIDForPN
		gotLID, err := lidMap.GetLIDForPN(ctx, pn1)
		require.NoError(t, err)
		assert.Equal(t, lid1, gotLID)

		// GetPNForLID
		gotPN, err := lidMap.GetPNForLID(ctx, lid1)
		require.NoError(t, err)
		assert.Equal(t, pn1, gotPN)
	})

	t.Run("PutLIDMapping overwriting old mapping clears stale cache", func(t *testing.T) {
		// New LID for pn1
		newLID1 := types.NewJID("10099", types.HiddenUserServer)
		err := lidMap.PutLIDMapping(ctx, newLID1, pn1)
		require.NoError(t, err)

		gotLID, err := lidMap.GetLIDForPN(ctx, pn1)
		require.NoError(t, err)
		assert.Equal(t, newLID1, gotLID)

		// Old LID now returns empty
		oldPN, err := lidMap.GetPNForLID(ctx, lid1)
		require.NoError(t, err)
		assert.Equal(t, types.EmptyJID, oldPN)
	})

	t.Run("PutManyLIDMappings and GetManyLIDsForPNs", func(t *testing.T) {
		// PutManyLIDMappings empty or invalid entries
		err := lidMap.PutManyLIDMappings(ctx, nil)
		require.NoError(t, err)

		invalidMappings := []store.LIDMapping{
			{LID: pn2, PN: lid2}, // inverted servers, should be ignored
		}
		err = lidMap.PutManyLIDMappings(ctx, invalidMappings)
		require.NoError(t, err)

		// Put valid batch
		validMappings := []store.LIDMapping{
			{LID: lid2, PN: pn2},
		}
		err = lidMap.PutManyLIDMappings(ctx, validMappings)
		require.NoError(t, err)

		// GetManyLIDsForPNs empty
		m, err := lidMap.GetManyLIDsForPNs(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, m)

		// GetManyLIDsForPNs with device preserve
		pn2WithDevice := pn2
		pn2WithDevice.Device = 5
		pns := []types.JID{pn2WithDevice, types.NewJID("group", types.GroupServer)}

		res, err := lidMap.GetManyLIDsForPNs(ctx, pns)
		require.NoError(t, err)
		require.NotNil(t, res)

		expectedLID2Dev := lid2
		expectedLID2Dev.Device = 5
		assert.Equal(t, expectedLID2Dev, res[pn2WithDevice])

		// Missing from cache hits DB fallback
		pn3 := types.NewJID("6283333", types.DefaultUserServer)
		lid3 := types.NewJID("10003", types.HiddenUserServer)

		// Directly insert into DB to bypass cache
		_, err = container.db.Exec(ctx, putLIDMappingQuery, lid3.User, pn3.User)
		require.NoError(t, err)

		// Create a clean LIDMap without cached pn3
		cleanLIDMap := NewCachedLIDMap(container.db)
		resDB, err := cleanLIDMap.GetManyLIDsForPNs(ctx, []types.JID{pn3})
		require.NoError(t, err)
		assert.Equal(t, lid3, resDB[pn3])

		// FillCache loads all mappings into memory
		err = cleanLIDMap.FillCache(ctx)
		require.NoError(t, err)
		assert.True(t, cleanLIDMap.cacheFilled)

		// After FillCache, unknown PN returns empty without hitting DB
		cachedEmpty, err := cleanLIDMap.GetLIDForPN(ctx, types.NewJID("6280000", types.DefaultUserServer))
		require.NoError(t, err)
		assert.Equal(t, types.EmptyJID, cachedEmpty)
	})
}
