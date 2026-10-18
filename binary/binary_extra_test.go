// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/types"
)

// ============================================================
// Node.String (xml.go)
// ============================================================

func TestNode_String_Empty(t *testing.T) {
	n := binary.Node{Tag: "ping"}
	s := n.String()
	assert.Equal(t, "<ping/>", s)
}

func TestNode_String_WithAttributes(t *testing.T) {
	n := binary.Node{
		Tag:   "iq",
		Attrs: binary.Attrs{"type": "get", "id": "123"},
	}
	s := n.String()
	assert.Contains(t, s, "<iq ")
	assert.Contains(t, s, "type=\"get\"")
	assert.Contains(t, s, "id=\"123\"")
}

func TestNode_String_WithTextContent(t *testing.T) {
	n := binary.Node{
		Tag:     "message",
		Content: []byte("hello world"),
	}
	s := n.String()
	assert.Contains(t, s, "hello world")
	assert.Contains(t, s, "<message>")
	assert.Contains(t, s, "</message>")
}

func TestNode_String_WithBinaryContent_Short(t *testing.T) {
	// 4 bytes non-UTF8 binary → hex encoded
	n := binary.Node{
		Tag:     "data",
		Content: []byte{0x01, 0x02, 0xAB, 0xCD},
	}
	s := n.String()
	assert.Contains(t, s, "0102abcd")
}

func TestNode_String_WithBinaryContent_Long(t *testing.T) {
	// > MaxBytesToPrintAsHex (128) bytes of non-printable binary data
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = 0xFF // non-printable
	}
	n := binary.Node{
		Tag:     "data",
		Content: largeData,
	}
	s := n.String()
	assert.Contains(t, s, "200 bytes")
}

func TestNode_String_WithChildNodes(t *testing.T) {
	n := binary.Node{
		Tag: "root",
		Content: []binary.Node{
			{Tag: "child", Attrs: binary.Attrs{"key": "val"}},
		},
	}
	s := n.String()
	assert.Contains(t, s, "<root")
	assert.Contains(t, s, "<child")
	assert.Contains(t, s, "</root>")
}

func TestNode_String_IndentXML(t *testing.T) {
	orig := binary.IndentXML
	binary.IndentXML = true
	defer func() { binary.IndentXML = orig }()

	n := binary.Node{
		Tag: "root",
		Content: []binary.Node{
			{Tag: "a"},
			{Tag: "b"},
		},
	}
	s := n.String()
	assert.Contains(t, s, "\n")
}

func TestNode_String_IndentXML_TextContent(t *testing.T) {
	orig := binary.IndentXML
	binary.IndentXML = true
	defer func() { binary.IndentXML = orig }()

	// printable text content with IndentXML
	n := binary.Node{
		Tag:     "msg",
		Content: []byte("line1\nline2"),
	}
	s := n.String()
	assert.Contains(t, s, "msg")
}

func TestNode_String_IndentXML_BinaryContent(t *testing.T) {
	orig := binary.IndentXML
	binary.IndentXML = true
	defer func() { binary.IndentXML = orig }()

	// binary data with IndentXML → hex wrapped in 80-char lines
	data := make([]byte, 100)
	for i := range data {
		data[i] = 0xAB
	}
	n := binary.Node{Tag: "b", Content: data}
	s := n.String()
	assert.Contains(t, s, "ab")
}

// ============================================================
// Node.UnmarshalJSON
// ============================================================

func TestNode_UnmarshalJSON_WithJIDAttr(t *testing.T) {
	// Content must be null, base64 string, or JSON array
	jsonData := `{"Tag":"message","Attrs":{"from":"628123456789@s.whatsapp.net","count":42}}`
	var n binary.Node
	err := json.Unmarshal([]byte(jsonData), &n)
	require.NoError(t, err)
	assert.Equal(t, "message", n.Tag)

	fromVal, ok := n.Attrs["from"]
	require.True(t, ok)
	jid, isJID := fromVal.(types.JID)
	assert.True(t, isJID, "from attr should be parsed as JID")
	assert.Equal(t, "628123456789", jid.User)

	// float64 attribute should be int64
	countVal := n.Attrs["count"]
	_, isInt64 := countVal.(int64)
	assert.True(t, isInt64, "count attr should be converted to int64")
}

func TestNode_UnmarshalJSON_WithBase64Content(t *testing.T) {
	// Content as a base64 string (JSON string)
	jsonData := `{"Tag":"text","Attrs":{},"Content":"aGVsbG8="}`
	var n binary.Node
	err := json.Unmarshal([]byte(jsonData), &n)
	require.NoError(t, err)
	assert.Equal(t, "text", n.Tag)
	contentBytes, ok := n.Content.([]byte)
	require.True(t, ok)
	assert.Equal(t, []byte("hello"), contentBytes)
}

func TestNode_UnmarshalJSON_WithChildNodes(t *testing.T) {
	// Content as JSON array of nodes
	jsonData := `{"Tag":"root","Attrs":{},"Content":[{"Tag":"child","Attrs":{}}]}`
	var n binary.Node
	err := json.Unmarshal([]byte(jsonData), &n)
	require.NoError(t, err)
	children := n.GetChildren()
	require.Len(t, children, 1)
	assert.Equal(t, "child", children[0].Tag)
}

func TestNode_UnmarshalJSON_InvalidContent(t *testing.T) {
	// Content starting with digit (not array or string) → error
	jsonData := `{"Tag":"x","Attrs":{},"Content":42}`
	var n binary.Node
	err := json.Unmarshal([]byte(jsonData), &n)
	assert.Error(t, err)
}

// ============================================================
// Node.GetChildByTag
// ============================================================

func TestNode_GetChildByTag_Found(t *testing.T) {
	parent := binary.Node{
		Tag: "root",
		Content: []binary.Node{
			{Tag: "item", Attrs: binary.Attrs{"id": "1"}},
			{Tag: "other"},
		},
	}

	child := parent.GetChildByTag("item")
	assert.Equal(t, "item", child.Tag)
	assert.Equal(t, "1", child.Attrs["id"])
}

func TestNode_GetChildByTag_NotFound(t *testing.T) {
	parent := binary.Node{
		Tag: "root",
		Content: []binary.Node{
			{Tag: "item"},
		},
	}

	// When not found, GetOptionalChildByTag returns ok=false and returns the root node
	_, ok := parent.GetOptionalChildByTag("nonexistent")
	assert.False(t, ok)
}

// ============================================================
// AttrUtility — additional coverage for missing branches
// ============================================================

func TestAttrUtility_UnixMilli_ValidAndZero(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	ms := now.UnixMilli()

	node := binary.Node{
		Tag: "m",
		Attrs: binary.Attrs{
			"ms":   fmt.Sprintf("%d", ms),
			"zero": "0",
		},
	}

	ag := node.AttrGetter()

	// OptionalUnixMilli with valid milliseconds
	result := ag.OptionalUnixMilli("ms")
	assert.Equal(t, ms, result.UnixMilli())

	// UnixMilli required
	ag2 := node.AttrGetter()
	result2 := ag2.UnixMilli("ms")
	assert.Equal(t, ms, result2.UnixMilli())
	assert.True(t, ag2.OK())

	// Zero value
	ag3 := node.AttrGetter()
	zeroResult := ag3.UnixMilli("zero")
	assert.True(t, zeroResult.IsZero())

	// Missing optional
	ag4 := node.AttrGetter()
	missing := ag4.OptionalUnixMilli("missing")
	assert.True(t, missing.IsZero())
}

func TestAttrUtility_UnixMilli_ParseError(t *testing.T) {
	node := binary.Node{
		Tag:   "m",
		Attrs: binary.Attrs{"ms": "not-a-number"},
	}
	ag := node.AttrGetter()
	_ = ag.UnixMilli("ms")
	assert.False(t, ag.OK())
}

func TestAttrUtility_ErrorList_Error(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"count": "not-a-number"},
	}
	ag := node.AttrGetter()
	_ = ag.Int("count")        // parse error
	_ = ag.String("notexist")  // required missing
	_ = ag.JID("alsonotexist") // required JID missing

	assert.False(t, ag.OK())
	err := ag.Error()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count")
}

func TestAttrUtility_GetJID_WrongType(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"from": 12345}, // int, not JID or string
	}
	ag := node.AttrGetter()
	jid := ag.JID("from")
	assert.True(t, jid.IsEmpty())
	assert.False(t, ag.OK())
}

func TestAttrUtility_GetBool_InvalidValue(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"flag": "notabool"},
	}
	ag := node.AttrGetter()
	result := ag.Bool("flag")
	assert.False(t, result)
	assert.False(t, ag.OK())
}

func TestAttrUtility_GetInt64_Invalid(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"n": "abc"},
	}
	ag := node.AttrGetter()
	assert.Equal(t, int64(0), ag.Int64("n"))
	assert.False(t, ag.OK())
}

func TestAttrUtility_GetUint64_Invalid(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"n": "abc"},
	}
	ag := node.AttrGetter()
	assert.Equal(t, uint64(0), ag.Uint64("n"))
	assert.False(t, ag.OK())
}

func TestAttrUtility_OptionalInt_Invalid(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"n": "notanint"},
	}
	ag := node.AttrGetter()
	result := ag.OptionalInt("n")
	assert.Equal(t, 0, result)
	assert.False(t, ag.OK())
}

func TestAttrUtility_Int_Invalid(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"n": "notanint"},
	}
	ag := node.AttrGetter()
	result := ag.Int("n")
	assert.Equal(t, 0, result)
	assert.False(t, ag.OK())
}

func TestAttrUtility_UnixTime_Zero(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{"t": "0"},
	}
	ag := node.AttrGetter()
	result := ag.UnixTime("t")
	assert.True(t, result.IsZero())
}

func TestAttrUtility_OptionalUnixTime_Missing(t *testing.T) {
	node := binary.Node{
		Tag:   "x",
		Attrs: binary.Attrs{},
	}
	ag := node.AttrGetter()
	result := ag.OptionalUnixTime("missing")
	assert.True(t, result.IsZero())
	assert.True(t, ag.OK())
}

// ============================================================
// Encode/Decode with AD JID (exercises writeJID/readJIDPair more)
// ============================================================

func TestMarshalUnmarshal_ADJIDAttribute(t *testing.T) {
	adJID := types.JID{
		User:     "628123456789",
		RawAgent: 0,
		Device:   3,
		Server:   types.DefaultUserServer,
	}
	n := binary.Node{
		Tag:   "iq",
		Attrs: binary.Attrs{"to": adJID},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)

	toVal := decoded.Attrs["to"]
	decodedJID, ok := toVal.(types.JID)
	require.True(t, ok)
	assert.Equal(t, adJID.User, decodedJID.User)
}

// ============================================================
// Unpack
// ============================================================

func TestUnpack_NonCompressed(t *testing.T) {
	// First byte 0x00 → not compressed (bit 1 of byte is 0), returns the rest
	data, err := binary.Unpack([]byte{0x00, 0xAB, 0xCD})
	require.NoError(t, err)
	assert.Equal(t, []byte{0xAB, 0xCD}, data)
}

func TestUnpack_CompressedInvalidBody(t *testing.T) {
	// First byte 0x02 → compressed (bit 1 is set), but rest is not valid zlib
	_, err := binary.Unpack([]byte{0x02, 0x00, 0x00})
	assert.Error(t, err)
}

// ============================================================
// writeBytes — exercise via Marshal+Unmarshal with byte content
// ============================================================

func TestMarshalUnmarshal_BytesContent(t *testing.T) {
	// Binary content triggers writeBytes path
	binaryData := []byte{0x01, 0x02, 0x03, 0xFF, 0xFE}
	n := binary.Node{
		Tag:     "data",
		Content: binaryData,
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)

	contentBytes, ok := decoded.Content.([]byte)
	require.True(t, ok, "Content should be []byte")
	assert.Equal(t, binaryData, contentBytes)
}

func TestMarshalUnmarshal_LargeBytesContent(t *testing.T) {
	// > 255 bytes → triggers pushInt20 path in writeByteLength
	largeData := make([]byte, 300)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	n := binary.Node{
		Tag:     "blob",
		Content: largeData,
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)

	contentBytes, ok := decoded.Content.([]byte)
	require.True(t, ok)
	assert.Equal(t, largeData, contentBytes)
}

// ============================================================
// writeJID variants — MessengerServer (FBJID), HostedServer (ADJID)
// ============================================================

func TestMarshalUnmarshal_MessengerJID(t *testing.T) {
	// MessengerServer → FBJID path in writeJID
	fbJID := types.JID{
		User:   "123456789",
		Device: 0,
		Server: types.MessengerServer,
	}
	n := binary.Node{
		Tag:   "msg",
		Attrs: binary.Attrs{"from": fbJID},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)

	fromVal := decoded.Attrs["from"]
	decodedJID, ok := fromVal.(types.JID)
	require.True(t, ok)
	assert.Equal(t, fbJID.User, decodedJID.User)
	assert.Equal(t, types.MessengerServer, decodedJID.Server)
}

func TestMarshalUnmarshal_HostedJID(t *testing.T) {
	// HostedServer → ADJID path
	hostedJID := types.JID{
		User:   "user123",
		Device: 2,
		Server: types.HostedServer,
	}
	n := binary.Node{
		Tag:   "msg",
		Attrs: binary.Attrs{"to": hostedJID},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	// Just verify it round-trips without error
	_ = decoded
}

func TestMarshalUnmarshal_InteropJID(t *testing.T) {
	// InteropServer → InteropJID path
	interopJID := types.JID{
		User:       "user999",
		Device:     1,
		Integrator: 5,
		Server:     types.InteropServer,
	}
	n := binary.Node{
		Tag:   "msg",
		Attrs: binary.Attrs{"from": interopJID},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)
	require.NotNil(t, decoded)
}

func TestMarshalUnmarshal_EmptyJID(t *testing.T) {
	// JIDPair with empty user → pushByte(ListEmpty)
	emptyUserJID := types.NewJID("", types.GroupServer)
	n := binary.Node{
		Tag:   "msg",
		Attrs: binary.Attrs{"to": emptyUserJID},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	toVal := decoded.Attrs["to"]
	decodedJID, ok := toVal.(types.JID)
	require.True(t, ok)
	assert.Equal(t, "", decodedJID.User)
	assert.Equal(t, types.GroupServer, decodedJID.Server)
}

// ============================================================
// Multiple attribute types via Marshal+Unmarshal
// ============================================================

func TestMarshalUnmarshal_VariousAttrTypes(t *testing.T) {
	n := binary.Node{
		Tag: "iq",
		Attrs: binary.Attrs{
			"str":    "hello",
			"int":    42,
			"int32":  int32(100),
			"int64":  int64(9999999),
			"uint":   uint(55),
			"uint32": uint32(77),
			"uint64": uint64(88),
			"bool":   true,
		},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// ============================================================
// Large list (> 256 items) → writeListStart with List16 path
// ============================================================

func TestMarshalUnmarshal_LargeChildList(t *testing.T) {
	// 260 children → triggers List16 path
	children := make([]binary.Node, 260)
	for i := range children {
		children[i] = binary.Node{Tag: "item", Attrs: binary.Attrs{"i": fmt.Sprintf("%d", i)}}
	}
	n := binary.Node{
		Tag:     "root",
		Content: children,
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)

	assert.Len(t, decoded.GetChildren(), 260)
}

// ============================================================
// Hex-encoded string path (writePackedBytes with hex)
// ============================================================

func TestMarshalUnmarshal_HexString(t *testing.T) {
	// A string like "DEADBEEF" → writePackedBytes with Hex8
	n := binary.Node{
		Tag:   "msg",
		Attrs: binary.Attrs{"hash": "DEADBEEF"},
	}

	data, err := binary.Marshal(n)
	require.NoError(t, err)

	unpacked, err := binary.Unpack(data)
	require.NoError(t, err)

	decoded, err := binary.Unmarshal(unpacked)
	require.NoError(t, err)
	assert.Equal(t, "DEADBEEF", decoded.Attrs["hash"])
}
