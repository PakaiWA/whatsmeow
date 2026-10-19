// Copyright (c) 2026 Kelvin Anggara, PakaiWA Developers and Contributors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"crypto/md5"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/util/keys"
)

func TestParseVersion(t *testing.T) {
	t.Run("Valid version", func(t *testing.T) {
		v, err := ParseVersion("2.3000.1050728964")
		require.NoError(t, err)
		assert.Equal(t, WAVersionContainer{2, 3000, 1050728964}, v)
	})

	t.Run("Invalid part count", func(t *testing.T) {
		_, err := ParseVersion("2.3000")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't contain three dot-separated parts")

		_, err = ParseVersion("2.3000.1.2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't contain three dot-separated parts")
	})

	t.Run("Invalid first part", func(t *testing.T) {
		_, err := ParseVersion("abc.3000.1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "first part")
	})

	t.Run("Invalid second part", func(t *testing.T) {
		_, err := ParseVersion("2.abc.1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "second part")
	})

	t.Run("Invalid third part", func(t *testing.T) {
		_, err := ParseVersion("2.3000.abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "third part")
	})
}

func TestWAVersionContainer_Methods(t *testing.T) {
	v1 := WAVersionContainer{2, 3000, 100}
	v2 := WAVersionContainer{2, 3000, 200}
	v3 := WAVersionContainer{2, 3001, 50}
	v4 := WAVersionContainer{3, 1000, 10}

	t.Run("LessThan", func(t *testing.T) {
		assert.True(t, v1.LessThan(v2))
		assert.False(t, v2.LessThan(v1))
		assert.False(t, v1.LessThan(v1))

		assert.True(t, v2.LessThan(v3))
		assert.False(t, v3.LessThan(v2))

		assert.True(t, v3.LessThan(v4))
		assert.False(t, v4.LessThan(v3))
	})

	t.Run("IsZero", func(t *testing.T) {
		assert.True(t, WAVersionContainer{0, 0, 0}.IsZero())
		assert.False(t, v1.IsZero())
	})

	t.Run("String and Hash", func(t *testing.T) {
		assert.Equal(t, "2.3000.100", v1.String())
		expectedHash := md5.Sum([]byte("2.3000.100"))
		assert.Equal(t, expectedHash, v1.Hash())
	})

	t.Run("ProtoAppVersion", func(t *testing.T) {
		protoVer := v1.ProtoAppVersion()
		require.NotNil(t, protoVer)
		assert.Equal(t, uint32(2), protoVer.GetPrimary())
		assert.Equal(t, uint32(3000), protoVer.GetSecondary())
		assert.Equal(t, uint32(100), protoVer.GetTertiary())
	})
}

func TestGetAndSetWAVersion(t *testing.T) {
	orig := GetWAVersion()
	defer SetWAVersion(orig)

	// Setting zero version is a no-op
	SetWAVersion(WAVersionContainer{0, 0, 0})
	assert.Equal(t, orig, GetWAVersion())

	// Setting new version
	newVer := WAVersionContainer{2, 3000, 99999999}
	SetWAVersion(newVer)
	assert.Equal(t, newVer, GetWAVersion())
	assert.Equal(t, newVer.Hash(), waVersionHash)
	assert.Equal(t, uint32(99999999), BaseClientPayload.UserAgent.AppVersion.GetTertiary())
}

func TestSetOSInfo(t *testing.T) {
	SetOSInfo("PakaiWA-TestOS", [3]uint32{1, 2, 3})
	assert.Equal(t, "PakaiWA-TestOS", DeviceProps.GetOs())
	assert.Equal(t, uint32(1), DeviceProps.GetVersion().GetPrimary())
	assert.Equal(t, uint32(2), DeviceProps.GetVersion().GetSecondary())
	assert.Equal(t, uint32(3), DeviceProps.GetVersion().GetTertiary())
	assert.Equal(t, "1.2.3", BaseClientPayload.UserAgent.GetOsVersion())
	assert.Equal(t, "1.2.3", BaseClientPayload.UserAgent.GetOsBuildNumber())
}

func TestDevice_GetClientPayload(t *testing.T) {
	identityKey := keys.NewKeyPair()
	signedPreKey := identityKey.CreateSignedPreKey(10)
	noiseKey := keys.NewKeyPair()

	t.Run("Registration payload when device.ID is nil", func(t *testing.T) {
		device := &Device{
			RegistrationID: 12345,
			IdentityKey:    identityKey,
			SignedPreKey:   signedPreKey,
			NoiseKey:       noiseKey,
		}

		payload := device.GetClientPayload()
		require.NotNil(t, payload)
		assert.False(t, payload.GetPassive())
		assert.False(t, payload.GetPull())
		assert.NotNil(t, payload.DevicePairingData)
		assert.Equal(t, identityKey.Pub[:], payload.DevicePairingData.EIdent)
		assert.Equal(t, signedPreKey.Pub[:], payload.DevicePairingData.ESkeyVal)
		assert.Equal(t, signedPreKey.Signature[:], payload.DevicePairingData.ESkeySig)
		assert.Equal(t, waVersionHash[:], payload.DevicePairingData.BuildHash)
	})

	t.Run("Login payload when device.ID is set", func(t *testing.T) {
		jid := types.NewJID("628123456789", types.DefaultUserServer)
		jid.Device = 1
		device := &Device{
			ID:             &jid,
			RegistrationID: 12345,
			IdentityKey:    identityKey,
			SignedPreKey:   signedPreKey,
		}

		payload := device.GetClientPayload()
		require.NotNil(t, payload)
		assert.True(t, payload.GetPassive())
		assert.True(t, payload.GetPull())
		assert.True(t, payload.GetLidDbMigrated())
		assert.Equal(t, uint64(628123456789), payload.GetUsername())
		assert.Equal(t, uint32(1), payload.GetDevice())
		assert.Equal(t, int32(1), payload.GetLc())
	})

	t.Run("Panics when device.ID is empty JID", func(t *testing.T) {
		emptyJID := types.EmptyJID
		device := &Device{
			ID: &emptyJID,
		}

		assert.Panics(t, func() {
			device.GetClientPayload()
		})
	})
}
