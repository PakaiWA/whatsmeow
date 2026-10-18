// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/types"
)

func TestNode_ChildrenTraversal(t *testing.T) {
	child1 := binary.Node{Tag: "item", Attrs: binary.Attrs{"id": "1"}}
	child2 := binary.Node{Tag: "item", Attrs: binary.Attrs{"id": "2"}}
	child3 := binary.Node{Tag: "nested", Content: []binary.Node{{Tag: "deep_item", Attrs: binary.Attrs{"key": "val"}}}}

	parent := binary.Node{
		Tag:     "root",
		Content: []binary.Node{child1, child2, child3},
	}

	t.Run("GetChildren", func(t *testing.T) {
		children := parent.GetChildren()
		assert.Len(t, children, 3)

		leaf := binary.Node{Tag: "leaf", Content: []byte("raw")}
		assert.Nil(t, leaf.GetChildren())
	})

	t.Run("GetChildrenByTag", func(t *testing.T) {
		items := parent.GetChildrenByTag("item")
		assert.Len(t, items, 2)
		assert.Equal(t, "1", items[0].Attrs["id"])
		assert.Equal(t, "2", items[1].Attrs["id"])

		none := parent.GetChildrenByTag("nonexistent")
		assert.Empty(t, none)
	})

	t.Run("GetOptionalChildByTag nested", func(t *testing.T) {
		deep, ok := parent.GetOptionalChildByTag("nested", "deep_item")
		require.True(t, ok)
		assert.Equal(t, "val", deep.Attrs["key"])

		_, ok = parent.GetOptionalChildByTag("nested", "missing")
		assert.False(t, ok)
	})
}

func TestNode_MarshalAndUnpackRoundTrip(t *testing.T) {
	userJID := types.NewJID("628123456789", types.DefaultUserServer)
	originalNode := binary.Node{
		Tag: "iq",
		Attrs: binary.Attrs{
			"id":   "test-12345",
			"type": "get",
			"to":   userJID,
		},
		Content: []binary.Node{
			{
				Tag: "ping",
				Attrs: binary.Attrs{
					"xmlns": "urn:xmpp:ping",
				},
			},
		},
	}

	marshaledBytes, err := binary.Marshal(originalNode)
	require.NoError(t, err)
	require.NotEmpty(t, marshaledBytes)

	unpackedBytes, err := binary.Unpack(marshaledBytes)
	require.NoError(t, err)

	decodedNode, err := binary.Unmarshal(unpackedBytes)
	require.NoError(t, err)
	require.NotNil(t, decodedNode)

	assert.Equal(t, originalNode.Tag, decodedNode.Tag)
	assert.Equal(t, "test-12345", decodedNode.Attrs["id"])
	assert.Equal(t, "get", decodedNode.Attrs["type"])
	assert.Equal(t, userJID, decodedNode.Attrs["to"])

	children := decodedNode.GetChildren()
	require.Len(t, children, 1)
	assert.Equal(t, "ping", children[0].Tag)
	assert.Equal(t, "urn:xmpp:ping", children[0].Attrs["xmlns"])
}

func TestAttrUtility(t *testing.T) {
	testJID := types.NewJID("628123456789", types.DefaultUserServer)
	now := time.Now().Truncate(time.Second)

	node := binary.Node{
		Tag: "message",
		Attrs: binary.Attrs{
			"from":      testJID,
			"id":        "msg-abc",
			"count":     "42",
			"big_count": "99999999999",
			"active":    "true",
			"t":         strconv.FormatInt(now.Unix(), 10),
		},
	}

	ag := node.AttrGetter()

	t.Run("JID getters", func(t *testing.T) {
		jid := ag.JID("from")
		assert.Equal(t, testJID, jid)

		optJID := ag.OptionalJID("from")
		require.NotNil(t, optJID)
		assert.Equal(t, testJID, *optJID)

		missingJID := ag.OptionalJID("missing")
		assert.Nil(t, missingJID)

		emptyJID := ag.OptionalJIDOrEmpty("missing")
		assert.True(t, emptyJID.IsEmpty())
	})

	t.Run("String getters", func(t *testing.T) {
		assert.Equal(t, "msg-abc", ag.String("id"))
		assert.Equal(t, "msg-abc", ag.OptionalString("id"))
		assert.Empty(t, ag.OptionalString("missing"))
	})

	t.Run("Number getters", func(t *testing.T) {
		assert.Equal(t, 42, ag.Int("count"))
		assert.Equal(t, 42, ag.OptionalInt("count"))
		assert.Equal(t, int64(99999999999), ag.Int64("big_count"))
		assert.Equal(t, uint64(99999999999), ag.Uint64("big_count"))
	})

	t.Run("Boolean and Time getters", func(t *testing.T) {
		assert.True(t, ag.Bool("active"))
		assert.True(t, ag.OptionalBool("active"))
		assert.False(t, ag.OptionalBool("missing"))

		assert.Equal(t, now.Unix(), ag.UnixTime("t").Unix())
		assert.Equal(t, now.Unix(), ag.OptionalUnixTime("t").Unix())
	})

	t.Run("Validation status", func(t *testing.T) {
		assert.True(t, ag.OK())
		assert.NoError(t, ag.Error())

		// Trigger required error
		_ = ag.String("strictly_required_missing_attr")
		assert.False(t, ag.OK())
		assert.Error(t, ag.Error())
	})
}
