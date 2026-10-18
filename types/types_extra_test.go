// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/types"
)

// ============================================================
// JID — covering untested functions
// ============================================================

func TestJID_NewADJID(t *testing.T) {
	t.Run("WhatsAppDomain maps to DefaultUserServer", func(t *testing.T) {
		jid := types.NewADJID("628123456789", types.WhatsAppDomain, 3)
		assert.Equal(t, types.DefaultUserServer, jid.Server)
		assert.Equal(t, uint16(3), jid.Device)
		assert.Equal(t, uint8(0), jid.RawAgent)
	})

	t.Run("LIDDomain maps to HiddenUserServer", func(t *testing.T) {
		jid := types.NewADJID("10000000000001", types.LIDDomain, 1)
		assert.Equal(t, types.HiddenUserServer, jid.Server)
		assert.Equal(t, uint8(0), jid.RawAgent) // agent is reset for domain types
	})

	t.Run("HostedDomain maps to HostedServer", func(t *testing.T) {
		jid := types.NewADJID("user123", types.HostedDomain, 2)
		assert.Equal(t, types.HostedServer, jid.Server)
		assert.Equal(t, uint8(0), jid.RawAgent)
	})

	t.Run("HostedLIDDomain maps to HostedLIDServer", func(t *testing.T) {
		jid := types.NewADJID("user123", types.HostedLIDDomain, 2)
		assert.Equal(t, types.HostedLIDServer, jid.Server)
		assert.Equal(t, uint8(0), jid.RawAgent)
	})
}

func TestJID_ADString(t *testing.T) {
	jid := types.JID{
		User:     "628123456789",
		RawAgent: 2,
		Device:   5,
		Server:   types.DefaultUserServer,
	}
	assert.Equal(t, "628123456789.2:5@s.whatsapp.net", jid.ADString())
}

func TestJID_String_ServerOnly(t *testing.T) {
	// Server-only JID (no user)
	jid := types.NewJID("", types.GroupServer)
	assert.Equal(t, types.GroupServer, jid.String())
}

func TestJID_MarshalUnmarshalText(t *testing.T) {
	original := types.NewJID("628123456789", types.DefaultUserServer)
	b, err := original.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "628123456789@s.whatsapp.net", string(b))

	var parsed types.JID
	err = parsed.UnmarshalText(b)
	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func TestJID_UnmarshalText_Invalid(t *testing.T) {
	var jid types.JID
	err := jid.UnmarshalText([]byte("user.1.2:0@s.whatsapp.net"))
	assert.Error(t, err)
}

func TestJID_IsZero(t *testing.T) {
	empty := types.EmptyJID
	assert.True(t, empty.IsZero())

	nonEmpty := types.NewJID("user", types.DefaultUserServer)
	assert.False(t, nonEmpty.IsZero())
}

func TestJID_SignalAddress(t *testing.T) {
	jid := types.JID{
		User:   "628123456789",
		Device: 3,
		Server: types.DefaultUserServer,
	}
	addr := jid.SignalAddress()
	require.NotNil(t, addr)
	assert.Equal(t, "628123456789", addr.Name())
	assert.Equal(t, uint32(3), uint32(addr.DeviceID()))
}

func TestJID_SignalAddressUser_WithAgent(t *testing.T) {
	// LID server → agent != 0
	jid := types.NewJID("user123", types.HiddenUserServer)
	addrUser := jid.SignalAddressUser()
	// LIDDomain=1, so expect "user123_1"
	assert.Equal(t, "user123_1", addrUser)
}

func TestJID_SignalAddressUser_NoAgent(t *testing.T) {
	jid := types.NewJID("user123", types.DefaultUserServer)
	// WhatsAppDomain=0, no suffix
	assert.Equal(t, "user123", jid.SignalAddressUser())
}

func TestJID_Scan_ByteSlice(t *testing.T) {
	var jid types.JID
	err := jid.Scan([]byte("628123456789@s.whatsapp.net"))
	require.NoError(t, err)
	assert.Equal(t, "628123456789", jid.User)
	assert.Equal(t, types.DefaultUserServer, jid.Server)
}

func TestJID_Scan_UnsupportedType(t *testing.T) {
	var jid types.JID
	err := jid.Scan(12345) // int is not supported
	assert.Error(t, err)
}

func TestJID_Value_Empty(t *testing.T) {
	val, err := types.EmptyJID.Value()
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestJID_ActualAgent_Default(t *testing.T) {
	// Unknown server → returns RawAgent
	jid := types.JID{User: "u", Server: "unknown.server", RawAgent: 99}
	assert.Equal(t, uint8(99), jid.ActualAgent())
}

// ============================================================
// MessageSource
// ============================================================

func TestMessageSource_IsIncomingBroadcast(t *testing.T) {
	broadcastListJID := types.NewJID("123456789", types.BroadcastServer)

	t.Run("Incoming from broadcast list", func(t *testing.T) {
		ms := types.MessageSource{
			Chat:     broadcastListJID,
			Sender:   types.NewJID("628111111111", types.DefaultUserServer),
			IsFromMe: false,
		}
		assert.True(t, ms.IsIncomingBroadcast())
	})

	t.Run("From me to broadcast list", func(t *testing.T) {
		ms := types.MessageSource{
			Chat:     broadcastListJID,
			Sender:   types.NewJID("628111111111", types.DefaultUserServer),
			IsFromMe: true,
			// BroadcastListOwner is empty → not incoming broadcast
		}
		assert.False(t, ms.IsIncomingBroadcast())
	})

	t.Run("From me with BroadcastListOwner set", func(t *testing.T) {
		ms := types.MessageSource{
			Chat:               broadcastListJID,
			Sender:             types.NewJID("628111111111", types.DefaultUserServer),
			IsFromMe:           true,
			BroadcastListOwner: types.NewJID("628999999999", types.DefaultUserServer),
		}
		assert.True(t, ms.IsIncomingBroadcast())
	})

	t.Run("Normal group chat is not broadcast", func(t *testing.T) {
		ms := types.MessageSource{
			Chat:     types.NewJID("120363025112345678", types.GroupServer),
			Sender:   types.NewJID("628111111111", types.DefaultUserServer),
			IsFromMe: false,
		}
		assert.False(t, ms.IsIncomingBroadcast())
	})
}

func TestMessageSource_SourceString(t *testing.T) {
	t.Run("Sender differs from chat (group context)", func(t *testing.T) {
		ms := types.MessageSource{
			Chat:   types.NewJID("120363025112345678", types.GroupServer),
			Sender: types.NewJID("628111111111", types.DefaultUserServer),
		}
		s := ms.SourceString()
		assert.Contains(t, s, "628111111111")
		assert.Contains(t, s, "120363025112345678")
		assert.Contains(t, s, " in ")
	})

	t.Run("Sender equals chat (DM context)", func(t *testing.T) {
		jid := types.NewJID("628111111111", types.DefaultUserServer)
		ms := types.MessageSource{
			Chat:   jid,
			Sender: jid,
		}
		s := ms.SourceString()
		assert.Equal(t, jid.String(), s)
		assert.NotContains(t, s, " in ")
	})
}

// ============================================================
// ReceiptType.GoString
// ============================================================

func TestReceiptType_GoString(t *testing.T) {
	tests := []struct {
		rt       types.ReceiptType
		expected string
	}{
		{types.ReceiptTypeRead, "types.ReceiptTypeRead"},
		{types.ReceiptTypeReadSelf, "types.ReceiptTypeReadSelf"},
		{types.ReceiptTypeDelivered, "types.ReceiptTypeDelivered"},
		{types.ReceiptTypePlayed, "types.ReceiptTypePlayed"},
		{types.ReceiptTypeRetry, `types.ReceiptType("retry")`},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, tc.rt.GoString(), "GoString for %q", string(tc.rt))
	}
}

// ============================================================
// StickerPackItem getters
// ============================================================

func TestStickerPackItem_Getters(t *testing.T) {
	item := &types.StickerPackItem{
		DirectPath:  "/sticker/path",
		MediaKey:    []byte("mediakey"),
		FileHash:    []byte("filehash"),
		EncFileHash: []byte("encfilehash"),
		FileSize:    12345,
	}

	assert.Equal(t, "/sticker/path", item.GetDirectPath())
	assert.Equal(t, []byte("mediakey"), item.GetMediaKey())
	assert.Equal(t, []byte("filehash"), item.GetFileSHA256())
	assert.Equal(t, []byte("encfilehash"), item.GetFileEncSHA256())
	assert.Equal(t, int64(12345), item.GetFileSizeBytes())
}

// ============================================================
// Newsletter types — UnmarshalText
// ============================================================

func TestNewsletterVerificationState_UnmarshalText(t *testing.T) {
	var nvs types.NewsletterVerificationState
	err := nvs.UnmarshalText([]byte("VERIFIED"))
	require.NoError(t, err)
	assert.Equal(t, types.NewsletterVerificationStateVerified, nvs)
}

func TestNewsletterPrivacy_UnmarshalText(t *testing.T) {
	var np types.NewsletterPrivacy
	err := np.UnmarshalText([]byte("PUBLIC"))
	require.NoError(t, err)
	assert.Equal(t, types.NewsletterPrivacyPublic, np)
}

func TestNewsletterState_UnmarshalText(t *testing.T) {
	var ns types.NewsletterState
	err := ns.UnmarshalText([]byte("ACTIVE"))
	require.NoError(t, err)
	assert.Equal(t, types.NewsletterStateActive, ns)
}

func TestNewsletterMuteState_UnmarshalText(t *testing.T) {
	var nms types.NewsletterMuteState
	err := nms.UnmarshalText([]byte("ON"))
	require.NoError(t, err)
	assert.Equal(t, types.NewsletterMuteOn, nms)
}

func TestNewsletterRole_UnmarshalText(t *testing.T) {
	var nr types.NewsletterRole
	err := nr.UnmarshalText([]byte("OWNER"))
	require.NoError(t, err)
	assert.Equal(t, types.NewsletterRoleOwner, nr)
}

// ============================================================
// GraphQLError / GraphQLErrors
// ============================================================

func TestGraphQLError_Error(t *testing.T) {
	gqle := types.GraphQLError{
		Extensions: types.GraphQLErrorExtensions{
			ErrorCode:   400,
			IsRetryable: false,
			Severity:    "CRITICAL",
		},
		Message: "bad request",
	}
	errStr := gqle.Error()
	assert.Contains(t, errStr, "400")
	assert.Contains(t, errStr, "bad request")
	assert.Contains(t, errStr, "CRITICAL")
}

func TestGraphQLErrors_Error(t *testing.T) {
	t.Run("Empty slice", func(t *testing.T) {
		var errs types.GraphQLErrors
		assert.Empty(t, errs.Error())
	})

	t.Run("Single error", func(t *testing.T) {
		errs := types.GraphQLErrors{
			{
				Extensions: types.GraphQLErrorExtensions{ErrorCode: 404, Severity: "ERROR"},
				Message:    "not found",
			},
		}
		assert.Contains(t, errs.Error(), "404")
	})

	t.Run("Multiple errors", func(t *testing.T) {
		errs := types.GraphQLErrors{
			{
				Extensions: types.GraphQLErrorExtensions{ErrorCode: 500, Severity: "ERROR"},
				Message:    "server error",
			},
			{
				Extensions: types.GraphQLErrorExtensions{ErrorCode: 503, Severity: "WARN"},
				Message:    "service unavailable",
			},
		}
		errStr := errs.Error()
		assert.Contains(t, errStr, "500")
		assert.Contains(t, errStr, "1 other error")
	})
}

func TestGraphQLErrors_Unwrap(t *testing.T) {
	errs := types.GraphQLErrors{
		{Extensions: types.GraphQLErrorExtensions{ErrorCode: 500, Severity: "E"}, Message: "e1"},
		{Extensions: types.GraphQLErrorExtensions{ErrorCode: 503, Severity: "E"}, Message: "e2"},
	}
	unwrapped := errs.Unwrap()
	assert.Len(t, unwrapped, 2)
}

// ============================================================
// Blocklist helpers
// ============================================================

func TestBlocklist_IsBlocked(t *testing.T) {
	lid1 := types.NewJID("10000000000001", types.HiddenUserServer)
	pn1 := types.NewJID("628111111111", types.DefaultUserServer)

	bl := &types.Blocklist{
		Items: []types.BlocklistItem{
			{LID: lid1, PN: pn1, Active: true},
		},
	}

	assert.True(t, bl.IsBlocked(lid1), "LID should be blocked")
	assert.True(t, bl.IsBlocked(pn1), "PN should be blocked")
	assert.False(t, bl.IsBlocked(types.NewJID("628999999999", types.DefaultUserServer)), "unknown should not be blocked")
}

func TestBlocklist_LIDs(t *testing.T) {
	lid1 := types.NewJID("10000000000001", types.HiddenUserServer)
	lid2 := types.NewJID("10000000000002", types.HiddenUserServer)

	bl := &types.Blocklist{
		Items: []types.BlocklistItem{
			{LID: lid1},
			{LID: lid2},
		},
	}

	lids := bl.LIDs()
	assert.Len(t, lids, 2)
	assert.Contains(t, lids, lid1)
	assert.Contains(t, lids, lid2)
}
