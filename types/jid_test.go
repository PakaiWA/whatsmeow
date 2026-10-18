// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types_test

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/types"
)

func TestParseJID(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		expected  types.JID
		expectErr bool
	}{
		{
			name: "Standard user JID",
			raw:  "628123456789@s.whatsapp.net",
			expected: types.JID{
				User:   "628123456789",
				Server: types.DefaultUserServer,
			},
		},
		{
			name: "Group JID",
			raw:  "120363025112345678@g.us",
			expected: types.JID{
				User:   "120363025112345678",
				Server: types.GroupServer,
			},
		},
		{
			name: "Newsletter JID",
			raw:  "120363161234567890@newsletter",
			expected: types.JID{
				User:   "120363161234567890",
				Server: types.NewsletterServer,
			},
		},
		{
			name: "LID JID",
			raw:  "10000000000001@lid",
			expected: types.JID{
				User:   "10000000000001",
				Server: types.HiddenUserServer,
			},
		},
		{
			name: "AD JID with agent and device",
			raw:  "628123456789.0:2@s.whatsapp.net",
			expected: types.JID{
				User:     "628123456789",
				RawAgent: 0,
				Device:   2,
				Server:   types.DefaultUserServer,
			},
		},
		{
			name: "AD JID with device only",
			raw:  "628123456789:5@s.whatsapp.net",
			expected: types.JID{
				User:   "628123456789",
				Device: 5,
				Server: types.DefaultUserServer,
			},
		},
		{
			name: "Bare server name without user",
			raw:  "s.whatsapp.net",
			expected: types.JID{
				User:   "",
				Server: types.DefaultUserServer,
			},
		},
		{
			name:      "Invalid JID with multiple dots in user part",
			raw:       "user.1.2:0@s.whatsapp.net",
			expectErr: true,
		},
		{
			name:      "Invalid JID with invalid device integer",
			raw:       "user.0:invalid@s.whatsapp.net",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := types.ParseJID(tc.raw)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected.User, actual.User)
				assert.Equal(t, tc.expected.Server, actual.Server)
				assert.Equal(t, tc.expected.RawAgent, actual.RawAgent)
				assert.Equal(t, tc.expected.Device, actual.Device)
			}
		})
	}
}

func TestJID_StringAndFormatting(t *testing.T) {
	t.Run("Standard user JID String", func(t *testing.T) {
		jid := types.NewJID("628123456789", types.DefaultUserServer)
		assert.Equal(t, "628123456789@s.whatsapp.net", jid.String())
	})

	t.Run("AD JID String representation", func(t *testing.T) {
		jid := types.JID{
			User:     "628123456789",
			RawAgent: 0,
			Device:   2,
			Server:   types.DefaultUserServer,
		}
		assert.Equal(t, "628123456789:2@s.whatsapp.net", jid.String())
	})

	t.Run("Empty JID String representation", func(t *testing.T) {
		assert.Empty(t, types.EmptyJID.String())
		assert.True(t, types.EmptyJID.IsEmpty())
	})
}

func TestJID_PropertiesAndHelpers(t *testing.T) {
	t.Run("UserInt parsing", func(t *testing.T) {
		jid := types.NewJID("628123456789", types.DefaultUserServer)
		assert.Equal(t, uint64(628123456789), jid.UserInt())
	})

	t.Run("ToNonAD conversion", func(t *testing.T) {
		jid := types.JID{
			User:       "628123456789",
			RawAgent:   1,
			Device:     4,
			Integrator: 10,
			Server:     types.DefaultUserServer,
		}
		nonAD := jid.ToNonAD()
		assert.Equal(t, "628123456789", nonAD.User)
		assert.Equal(t, types.DefaultUserServer, nonAD.Server)
		assert.Equal(t, uint8(0), nonAD.RawAgent)
		assert.Equal(t, uint16(0), nonAD.Device)
		assert.Equal(t, uint16(10), nonAD.Integrator)
	})

	t.Run("ActualAgent derivation", func(t *testing.T) {
		userJID := types.NewJID("user", types.DefaultUserServer)
		assert.Equal(t, types.WhatsAppDomain, userJID.ActualAgent())

		lidJID := types.NewJID("user", types.HiddenUserServer)
		assert.Equal(t, types.LIDDomain, lidJID.ActualAgent())

		hostedJID := types.NewJID("user", types.HostedServer)
		assert.Equal(t, types.HostedDomain, hostedJID.ActualAgent())

		hostedLIDJID := types.NewJID("user", types.HostedLIDServer)
		assert.Equal(t, types.HostedLIDDomain, hostedLIDJID.ActualAgent())
	})

	t.Run("IsBot detection", func(t *testing.T) {
		botServerJID := types.NewJID("123456", types.BotServer)
		assert.True(t, botServerJID.IsBot())

		metaAIUserJID := types.NewJID("13135550002", types.DefaultUserServer)
		assert.True(t, metaAIUserJID.IsBot())

		normalUserJID := types.NewJID("628123456789", types.DefaultUserServer)
		assert.False(t, normalUserJID.IsBot())
	})

	t.Run("IsBroadcastList detection", func(t *testing.T) {
		statusJID := types.StatusBroadcastJID
		assert.False(t, statusJID.IsBroadcastList(), "Status broadcast should not be treated as a generic broadcast list")

		broadcastListJID := types.NewJID("123456789", types.BroadcastServer)
		assert.True(t, broadcastListJID.IsBroadcastList())
	})
}

func TestJID_DatabaseValuerAndScanner(t *testing.T) {
	jid := types.NewJID("628123456789", types.DefaultUserServer)

	val, err := jid.Value()
	require.NoError(t, err)
	assert.Equal(t, driver.Value("628123456789@s.whatsapp.net"), val)

	var scanned types.JID
	err = scanned.Scan("628123456789@s.whatsapp.net")
	require.NoError(t, err)
	assert.Equal(t, jid, scanned)

	var empty types.JID
	err = empty.Scan(nil)
	require.NoError(t, err)
	assert.True(t, empty.IsEmpty())
}
