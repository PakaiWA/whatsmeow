// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/PakaiWA/whatsmeow/proto/waE2E"
	"github.com/PakaiWA/whatsmeow/store"
	"github.com/PakaiWA/whatsmeow/types"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func TestReportingToken_ShouldIncludeReportingToken(t *testing.T) {
	cli := &Client{
		SendReportingTokens: false,
	}
	msg := &waE2E.Message{
		Conversation: proto.String("hello"),
	}
	assert.False(t, cli.shouldIncludeReportingToken(msg))

	cli.SendReportingTokens = true
	assert.True(t, cli.shouldIncludeReportingToken(msg))

	// ReactionMessage -> false
	msgReaction := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{Text: proto.String("👍")},
	}
	assert.False(t, cli.shouldIncludeReportingToken(msgReaction))

	// EncReactionMessage -> false
	msgEncReaction := &waE2E.Message{
		EncReactionMessage: &waE2E.EncReactionMessage{},
	}
	assert.False(t, cli.shouldIncludeReportingToken(msgEncReaction))

	// EncEventResponseMessage -> false
	msgEncEvent := &waE2E.Message{
		EncEventResponseMessage: &waE2E.EncEventResponseMessage{},
	}
	assert.False(t, cli.shouldIncludeReportingToken(msgEncEvent))

	// PollUpdateMessage -> false
	msgPollUpdate := &waE2E.Message{
		PollUpdateMessage: &waE2E.PollUpdateMessage{},
	}
	assert.False(t, cli.shouldIncludeReportingToken(msgPollUpdate))
}

func TestReportingToken_GetMessageReportingToken(t *testing.T) {
	cli := NewClient(&store.Device{}, waLog.Noop)
	sender := types.NewJID("12345", types.DefaultUserServer)
	remote := types.NewJID("67890", types.DefaultUserServer)
	msgID := "msg-test-123"

	msg := &waE2E.Message{
		Conversation: proto.String("Hello reporting token"),
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageSecret: []byte("thirty-two-bytes-secret-12345678"),
		},
	}
	msgBytes, err := proto.Marshal(msg)
	require.NoError(t, err)

	node := cli.getMessageReportingToken(msgBytes, msg, sender, remote, msgID)
	assert.Equal(t, "reporting", node.Tag)
	children := node.GetChildren()
	require.Len(t, children, 1)
	assert.Equal(t, "reporting_token", children[0].Tag)
	assert.Equal(t, "2", children[0].Attrs["v"])
	tokenBytes, ok := children[0].Content.([]byte)
	require.True(t, ok)
	assert.Len(t, tokenBytes, 16)
}

func TestReportingToken_GetConfigForField(t *testing.T) {
	fields := []reportingField{
		{FieldNumber: 1, IsMessage: false},
		{FieldNumber: 2, IsMessage: true},
	}
	assert.NotNil(t, getConfigForField(fields, 1))
	assert.NotNil(t, getConfigForField(fields, 2))
	assert.Nil(t, getConfigForField(fields, 99))
}

func encodeProtobufTag(fieldNum int, wireType int) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, uint64(fieldNum<<3|wireType))
	return buf[:n]
}

func encodeVarint(val uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, val)
	return buf[:n]
}

func TestReportingToken_ExtractReportingTokenContent(t *testing.T) {
	cfg := []reportingField{
		{FieldNumber: 1}, // varint
		{FieldNumber: 2}, // 64-bit
		{FieldNumber: 3}, // bytes
		{FieldNumber: 4}, // 32-bit
		{
			FieldNumber: 5,
			IsMessage:   true,
			Subfields: []reportingField{
				{FieldNumber: 1},
			},
		},
	}

	// Build payload containing fields 1 (varint), 2 (64-bit), 3 (bytes), 4 (32-bit), 5 (submessage), and an unconfigured field 10
	var payload []byte

	// Field 1: varint 42
	payload = append(payload, encodeProtobufTag(1, wireVarint)...)
	payload = append(payload, encodeVarint(42)...)

	// Field 10 (unconfigured): varint 100
	payload = append(payload, encodeProtobufTag(10, wireVarint)...)
	payload = append(payload, encodeVarint(100)...)

	// Field 2: 64-bit
	payload = append(payload, encodeProtobufTag(2, wire64bit)...)
	payload = append(payload, []byte{1, 2, 3, 4, 5, 6, 7, 8}...)

	// Field 11 (unconfigured): 64-bit
	payload = append(payload, encodeProtobufTag(11, wire64bit)...)
	payload = append(payload, []byte{8, 7, 6, 5, 4, 3, 2, 1}...)

	// Field 3: bytes "hello"
	payload = append(payload, encodeProtobufTag(3, wireBytes)...)
	payload = append(payload, encodeVarint(5)...)
	payload = append(payload, []byte("hello")...)

	// Field 12 (unconfigured): bytes "skip-me"
	payload = append(payload, encodeProtobufTag(12, wireBytes)...)
	payload = append(payload, encodeVarint(7)...)
	payload = append(payload, []byte("skip-me")...)

	// Field 4: 32-bit
	payload = append(payload, encodeProtobufTag(4, wire32bit)...)
	payload = append(payload, []byte{1, 2, 3, 4}...)

	// Field 13 (unconfigured): 32-bit
	payload = append(payload, encodeProtobufTag(13, wire32bit)...)
	payload = append(payload, []byte{4, 3, 2, 1}...)

	// Field 5: submessage with subfield 1 varint
	var subMsg []byte
	subMsg = append(subMsg, encodeProtobufTag(1, wireVarint)...)
	subMsg = append(subMsg, encodeVarint(999)...)

	payload = append(payload, encodeProtobufTag(5, wireBytes)...)
	payload = append(payload, encodeVarint(uint64(len(subMsg)))...)
	payload = append(payload, subMsg...)

	extracted := extractReportingTokenContent(payload, cfg)
	require.NotEmpty(t, extracted)

	// Test real message extraction using actual getReportingFields()
	realMsg := &waE2E.Message{
		Conversation: proto.String("Testing real reporting fields"),
	}
	realBytes, err := proto.Marshal(realMsg)
	require.NoError(t, err)
	res := getReportingToken(realBytes)
	assert.NotEmpty(t, res)
}

func TestReportingToken_ExtractReportingTokenContent_Malformed(t *testing.T) {
	cfg := []reportingField{
		{FieldNumber: 1},
	}

	t.Run("Invalid wire type in unconfigured field", func(t *testing.T) {
		invalidTag := encodeProtobufTag(10, 7) // wire type 7 is invalid
		res := extractReportingTokenContent(invalidTag, cfg)
		assert.Nil(t, res)
	})

	t.Run("Invalid wire type in configured field", func(t *testing.T) {
		invalidTag := encodeProtobufTag(1, 7)
		res := extractReportingTokenContent(invalidTag, cfg)
		assert.Nil(t, res)
	})

	t.Run("Truncated bytes field configured", func(t *testing.T) {
		cfgBytes := []reportingField{{FieldNumber: 3}}
		tag := encodeProtobufTag(3, wireBytes)
		tag = append(tag, encodeVarint(100)...) // specifies 100 bytes length, but data is truncated
		tag = append(tag, []byte("short")...)
		res := extractReportingTokenContent(tag, cfgBytes)
		assert.Nil(t, res)
	})

	t.Run("Truncated bytes field unconfigured", func(t *testing.T) {
		tag := encodeProtobufTag(99, wireBytes)
		tag = append(tag, encodeVarint(100)...) // 100 bytes expected
		tag = append(tag, []byte("short")...)
		res := extractReportingTokenContent(tag, cfg)
		assert.Nil(t, res)
	})
}
